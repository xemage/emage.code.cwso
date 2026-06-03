package dispatch

import (
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

func waitForTopicRecords(t *testing.T, broker *memorybroker.Broker, topic string, expected int) []memorybroker.Record {
	t.Helper()
	if expected <= 0 {
		expected = 1
	}
	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		records := broker.Query(memorybroker.QueryOptions{Topics: []string{topic}, Limit: expected + 4})
		if len(records) >= expected {
			return records
		}
		if time.Now().After(deadline) {
			return records
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSpikeFilterEmitsOnSignatureChange(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	filter := NewASTSpikeFilter(memorybroker.NewTeePublisher(nil, broker), ASTSpikeFilterConfig{
		SemanticThreshold: SpikeKindSignatureChange,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	// First touch of the symbol → symbol_added (below threshold, no emit).
	if err := filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Path: "src/a.go", Symbol: "pkg.Foo", SignatureHash: "h1", At: base}); err != nil {
		t.Fatalf("observe write 1: %v", err)
	}
	if records := waitForTopicRecords(t, broker, TopicASTSemanticSpike, 1); len(records) != 0 {
		t.Fatalf("expected no semantic spike for symbol_added under signature_change threshold, got %d", len(records))
	}

	// Signature changes → signature_change (at threshold, emits).
	if err := filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Path: "src/a.go", Symbol: "pkg.Foo", SignatureHash: "h2", At: base.Add(10 * time.Millisecond)}); err != nil {
		t.Fatalf("observe write 2: %v", err)
	}
	records := waitForTopicRecords(t, broker, TopicASTSemanticSpike, 1)
	if len(records) != 1 {
		t.Fatalf("expected one semantic spike on signature change, got %d", len(records))
	}
	var event SemanticSpikeEvent
	if err := decodePayload(records[0].Payload, &event); err != nil {
		t.Fatalf("decode semantic spike: %v", err)
	}
	if event.SpikeKind != string(SpikeKindSignatureChange) {
		t.Fatalf("expected signature_change, got %q", event.SpikeKind)
	}
	if event.Symbol != "pkg.Foo" || event.Workspace != "ws-1" {
		t.Fatalf("unexpected spike payload: %+v", event)
	}
	if event.SignalPath != signalPathUserspace {
		t.Fatalf("expected userspace signal path, got %q", event.SignalPath)
	}
}

func TestSpikeFilterThresholdAnyEmitsCosmetic(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	filter := NewASTSpikeFilter(memorybroker.NewTeePublisher(nil, broker), ASTSpikeFilterConfig{
		SemanticThreshold: SpikeThresholdAny,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	// Seed the symbol, then re-write with the same signature → cosmetic.
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Symbol: "pkg.Foo", SignatureHash: "h1", At: base})
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Symbol: "pkg.Foo", SignatureHash: "h1", At: base.Add(10 * time.Millisecond)})

	records := waitForTopicRecords(t, broker, TopicASTSemanticSpike, 2)
	// symbol_added (rank2 ≥ any) + cosmetic (rank1 ≥ any) both fire under threshold "any".
	if len(records) != 2 {
		t.Fatalf("expected two semantic spikes under threshold any, got %d", len(records))
	}
}

func TestSpikeFilterDefaultThresholdSilentBelow(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	filter := NewASTSpikeFilter(memorybroker.NewTeePublisher(nil, broker), ASTSpikeFilterConfig{})
	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)

	// A write with no semantic signal classifies as none → never emits.
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Path: "src/a.go", At: base})
	if records := waitForTopicRecords(t, broker, TopicASTSemanticSpike, 1); len(records) != 0 {
		t.Fatalf("expected silence for non-semantic write, got %d", len(records))
	}
}

func TestSpikeFilterEmitsConflictPreWarning(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	filter := NewASTSpikeFilter(memorybroker.NewTeePublisher(nil, broker), ASTSpikeFilterConfig{
		SemanticThreshold: SpikeKindSignatureChange,
		ConflictWindowMS:  2000,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	// Two workspaces change the same symbol's signature within the window.
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Path: "src/a.go", Symbol: "pkg.Foo", ChangeKind: SpikeKindSignatureChange, At: base})
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-2", Path: "src/a.go", Symbol: "pkg.Foo", ChangeKind: SpikeKindSignatureChange, At: base.Add(50 * time.Millisecond)})

	records := waitForTopicRecords(t, broker, TopicASTConflictWarning, 1)
	if len(records) != 1 {
		t.Fatalf("expected one conflict warning, got %d", len(records))
	}
	var warning SemanticConflictWarning
	if err := decodePayload(records[0].Payload, &warning); err != nil {
		t.Fatalf("decode conflict warning: %v", err)
	}
	if warning.Symbol != "pkg.Foo" {
		t.Fatalf("expected conflict on pkg.Foo, got %q", warning.Symbol)
	}
	if len(warning.PotentialConflictWith) != 1 || warning.PotentialConflictWith[0] != "ws-1" {
		t.Fatalf("expected potential conflict with ws-1, got %v", warning.PotentialConflictWith)
	}
	if len(warning.Workspaces) != 2 {
		t.Fatalf("expected both workspaces listed, got %v", warning.Workspaces)
	}
	if warning.Severity != "critical" {
		t.Fatalf("expected critical severity for signature_change conflict, got %q", warning.Severity)
	}
}

func TestSpikeFilterNoConflictForSingleWorkspace(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	filter := NewASTSpikeFilter(memorybroker.NewTeePublisher(nil, broker), ASTSpikeFilterConfig{
		SemanticThreshold: SpikeKindSignatureChange,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Symbol: "pkg.Foo", ChangeKind: SpikeKindSignatureChange, At: base})
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Symbol: "pkg.Foo", ChangeKind: SpikeKindSignatureChange, At: base.Add(50 * time.Millisecond)})

	if records := waitForTopicRecords(t, broker, TopicASTConflictWarning, 1); len(records) != 0 {
		t.Fatalf("expected no conflict warning for a single workspace, got %d", len(records))
	}
}

func TestSpikeFilterConflictWindowPrunesStaleWriters(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	filter := NewASTSpikeFilter(memorybroker.NewTeePublisher(nil, broker), ASTSpikeFilterConfig{
		SemanticThreshold: SpikeKindSignatureChange,
		ConflictWindowMS:  100,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Symbol: "pkg.Foo", ChangeKind: SpikeKindSignatureChange, At: base})
	// ws-2 writes well outside the 100ms correlation window → ws-1 has aged out.
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-2", Symbol: "pkg.Foo", ChangeKind: SpikeKindSignatureChange, At: base.Add(500 * time.Millisecond)})

	if records := waitForTopicRecords(t, broker, TopicASTConflictWarning, 1); len(records) != 0 {
		t.Fatalf("expected no conflict warning once the prior writer aged out, got %d", len(records))
	}
}

func TestSpikeFilterEBPFFallsBackWhenUnavailable(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	filter := NewASTSpikeFilter(memorybroker.NewTeePublisher(nil, broker), ASTSpikeFilterConfig{
		PreferEBPF:        true,
		SemanticThreshold: SpikeKindSignatureChange,
		EBPFChecker:       func() (bool, string) { return false, "capability denied" },
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Symbol: "pkg.Foo", ChangeKind: SpikeKindSignatureChange, At: base})

	records := waitForTopicRecords(t, broker, TopicASTSemanticSpike, 1)
	if len(records) != 1 {
		t.Fatalf("expected one semantic spike, got %d", len(records))
	}
	var event SemanticSpikeEvent
	if err := decodePayload(records[0].Payload, &event); err != nil {
		t.Fatalf("decode semantic spike: %v", err)
	}
	if event.SignalPath != signalPathUserspace {
		t.Fatalf("expected userspace fallback, got %q", event.SignalPath)
	}
	if event.Notes == "" {
		t.Fatalf("expected fallback reason in notes")
	}
}

func TestSpikeFilterRedactionDropsSymbolAndNotes(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	filter := NewASTSpikeFilter(memorybroker.NewTeePublisher(nil, broker), ASTSpikeFilterConfig{
		PreferEBPF:        true,
		SemanticThreshold: SpikeKindSignatureChange,
		EBPFChecker:       func() (bool, string) { return false, "capability denied" },
		Redaction: TelemetryRedactionConfig{
			Enabled:          true,
			AnomalyNotesMode: "drop",
		},
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Path: "src/a.go", Symbol: "pkg.Foo", ChangeKind: SpikeKindSignatureChange, At: base})

	records := waitForTopicRecords(t, broker, TopicASTSemanticSpike, 1)
	if len(records) != 1 {
		t.Fatalf("expected one semantic spike, got %d", len(records))
	}
	var event SemanticSpikeEvent
	if err := decodePayload(records[0].Payload, &event); err != nil {
		t.Fatalf("decode semantic spike: %v", err)
	}
	if event.Symbol != "" || event.Path != "" {
		t.Fatalf("expected symbol/path dropped by redaction, got %+v", event)
	}
	if event.Notes != "" {
		t.Fatalf("expected notes dropped by redaction, got %q", event.Notes)
	}
	// Workspace correlation must still work despite redacted output.
	if event.Workspace != "ws-1" {
		t.Fatalf("expected workspace retained, got %q", event.Workspace)
	}
}

func TestSpikeFilterCustomScorerSeam(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	// A stand-in for the future sparse Wasm mini-model: always reports signature_change.
	scorer := func(ev WriteEvent, prior string, seen bool) (SpikeKind, float64) {
		return SpikeKindSignatureChange, 0.42
	}
	filter := NewASTSpikeFilter(memorybroker.NewTeePublisher(nil, broker), ASTSpikeFilterConfig{
		SemanticThreshold: SpikeKindSignatureChange,
		Scorer:            scorer,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	_ = filter.ObserveWrite(WriteEvent{Workspace: "ws-1", Path: "src/a.go", At: base})

	records := waitForTopicRecords(t, broker, TopicASTSemanticSpike, 1)
	if len(records) != 1 {
		t.Fatalf("expected custom scorer to force a spike, got %d", len(records))
	}
	var event SemanticSpikeEvent
	if err := decodePayload(records[0].Payload, &event); err != nil {
		t.Fatalf("decode semantic spike: %v", err)
	}
	if event.Confidence != 0.42 {
		t.Fatalf("expected custom scorer confidence 0.42, got %v", event.Confidence)
	}
}
