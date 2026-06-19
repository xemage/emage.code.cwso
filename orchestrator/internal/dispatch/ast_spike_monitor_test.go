package dispatch

import (
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

func newSpikeBroker(t *testing.T) *memorybroker.Broker {
	t.Helper()
	return memorybroker.New(
		memorybroker.WithCapacity(32),
		memorybroker.WithIngressQueueSize(32),
	)
}

func waitForSpikeRecords(t *testing.T, broker *memorybroker.Broker, expected int) []memorybroker.Record {
	t.Helper()
	if expected <= 0 {
		expected = 1
	}
	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		records := broker.Query(memorybroker.QueryOptions{Topics: []string{TopicASTSpike}, Limit: expected + 4})
		if len(records) >= expected {
			return records
		}
		if time.Now().After(deadline) {
			return records
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitForCriticalSpike(t *testing.T, broker *memorybroker.Broker, timeout time.Duration) []memorybroker.Record {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		records := broker.Query(memorybroker.QueryOptions{Topics: []string{TopicASTSpike}, Limit: 8})
		for _, record := range records {
			var event ASTSpikeEvent
			if err := decodePayload(record.Payload, &event); err != nil {
				t.Fatalf("decode spike payload: %v", err)
			}
			if event.Severity == "critical" {
				return records
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return broker.Query(memorybroker.QueryOptions{Topics: []string{TopicASTSpike}, Limit: 8})
}

func feedWrites(t *testing.T, m *ASTWriteSpikeMonitor, base time.Time, workspace string, n int, stepMS int) {
	t.Helper()
	for i := 0; i < n; i++ {
		at := base.Add(time.Duration(i*stepMS) * time.Millisecond)
		if err := m.ObserveWrite(WriteEvent{
			Workspace: workspace,
			Path:      pathForIndex(i),
			Language:  "go",
			At:        at,
		}); err != nil {
			t.Fatalf("observe write %d: %v", i, err)
		}
	}
}

func pathForIndex(i int) string {
	switch i % 3 {
	case 0:
		return "src/a.go"
	case 1:
		return "src/b.go"
	default:
		return "src/c.go"
	}
}

func TestASTSpikeMonitorEmitsOnThresholdFallbackPath(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	monitor := NewASTWriteSpikeMonitor(memorybroker.NewTeePublisher(nil, broker), ASTWriteSpikeMonitorConfig{
		PreferEBPF: false,
		WindowMS:   1000,
		Threshold:  5,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	feedWrites(t, monitor, base, "ws-1", 5, 10)

	records := waitForSpikeRecords(t, broker, 1)
	if len(records) != 1 {
		t.Fatalf("expected exactly one spike, got %d", len(records))
	}

	var event ASTSpikeEvent
	if err := decodePayload(records[0].Payload, &event); err != nil {
		t.Fatalf("decode spike payload: %v", err)
	}
	if event.Workspace != "ws-1" {
		t.Fatalf("expected workspace ws-1, got %q", event.Workspace)
	}
	if event.SignalPath != signalPathUserspace {
		t.Fatalf("expected userspace signal path, got %q", event.SignalPath)
	}
	if event.PrivilegeRequirement != "none" {
		t.Fatalf("expected unprivileged path, got %q", event.PrivilegeRequirement)
	}
	if event.DetectionLatencyIsAdvisory {
		t.Fatalf("expected non-advisory userspace latency, got %+v", event)
	}
	if event.ObservedWrites != 5 {
		t.Fatalf("expected 5 observed writes, got %d", event.ObservedWrites)
	}
	if event.DistinctPaths != 3 {
		t.Fatalf("expected 3 distinct paths, got %d", event.DistinctPaths)
	}
	if event.Severity != "warning" {
		t.Fatalf("expected warning severity, got %q", event.Severity)
	}
	if len(event.HotPaths) == 0 {
		t.Fatalf("expected hot paths to be reported, got none")
	}
}

func TestASTSpikeMonitorBelowThresholdStaysSilent(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	monitor := NewASTWriteSpikeMonitor(memorybroker.NewTeePublisher(nil, broker), ASTWriteSpikeMonitorConfig{
		WindowMS:  1000,
		Threshold: 5,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	feedWrites(t, monitor, base, "ws-1", 4, 10)

	records := waitForSpikeRecords(t, broker, 1)
	if len(records) != 0 {
		t.Fatalf("expected no spike below threshold, got %d", len(records))
	}
}

func TestASTSpikeMonitorPrunesOldWritesOutsideWindow(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	monitor := NewASTWriteSpikeMonitor(memorybroker.NewTeePublisher(nil, broker), ASTWriteSpikeMonitorConfig{
		WindowMS:  100,
		Threshold: 3,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	// Two writes spaced far enough apart that earlier ones fall out of the 100ms window.
	for i := 0; i < 5; i++ {
		at := base.Add(time.Duration(i*60) * time.Millisecond)
		if err := monitor.ObserveWrite(WriteEvent{Workspace: "ws-1", Path: "src/a.go", At: at}); err != nil {
			t.Fatalf("observe write: %v", err)
		}
	}

	records := waitForSpikeRecords(t, broker, 1)
	if len(records) != 0 {
		t.Fatalf("expected no spike when writes never co-occur within window, got %d", len(records))
	}
}

func TestASTSpikeMonitorDebouncesSustainedBurst(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	monitor := NewASTWriteSpikeMonitor(memorybroker.NewTeePublisher(nil, broker), ASTWriteSpikeMonitorConfig{
		WindowMS:   1000,
		Threshold:  3,
		DebounceMS: 1000,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	// 6 writes within the window: threshold is met at write 3, but debounce should keep
	// it to a single emission for the rest of the burst.
	feedWrites(t, monitor, base, "ws-1", 6, 10)

	records := waitForSpikeRecords(t, broker, 2)
	if len(records) != 1 {
		t.Fatalf("expected debounce to yield exactly one spike, got %d", len(records))
	}
}

func TestASTSpikeMonitorEscalatesSeverity(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	// Small debounce so each write past threshold re-emits; the burst climbs to 2x the
	// threshold, which must escalate severity to critical.
	monitor := NewASTWriteSpikeMonitor(memorybroker.NewTeePublisher(nil, broker), ASTWriteSpikeMonitorConfig{
		WindowMS:   1000,
		Threshold:  3,
		DebounceMS: 1,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	feedWrites(t, monitor, base, "ws-1", 6, 5)

	records := waitForCriticalSpike(t, broker, 2*time.Second)
	if len(records) == 0 {
		t.Fatalf("expected at least one spike, got none")
	}
	sawCritical := false
	for _, record := range records {
		var event ASTSpikeEvent
		if err := decodePayload(record.Payload, &event); err != nil {
			t.Fatalf("decode spike payload: %v", err)
		}
		if event.Severity == "critical" {
			sawCritical = true
		}
	}
	if !sawCritical {
		t.Fatalf("expected a critical spike at 2x threshold, got none in %d records", len(records))
	}
}

func TestASTSpikeMonitorEBPFPreferredUsesHookSemantics(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	monitor := NewASTWriteSpikeMonitor(memorybroker.NewTeePublisher(nil, broker), ASTWriteSpikeMonitorConfig{
		PreferEBPF:  true,
		WindowMS:    1000,
		Threshold:   3,
		EBPFChecker: func() (bool, string) { return true, "" },
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	feedWrites(t, monitor, base, "ws-1", 3, 5)

	records := waitForSpikeRecords(t, broker, 1)
	if len(records) != 1 {
		t.Fatalf("expected one spike, got %d", len(records))
	}
	var event ASTSpikeEvent
	if err := decodePayload(records[0].Payload, &event); err != nil {
		t.Fatalf("decode spike payload: %v", err)
	}
	if event.SignalPath != signalPathEBPF {
		t.Fatalf("expected ebpf-hook signal path, got %q", event.SignalPath)
	}
	if event.DetectionLatencyMode != detectionModeAdvisory || !event.DetectionLatencyIsAdvisory {
		t.Fatalf("expected advisory eBPF latency semantics, got %+v", event)
	}
}

func TestASTSpikeMonitorEBPFFallsBackWhenUnavailable(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	monitor := NewASTWriteSpikeMonitor(memorybroker.NewTeePublisher(nil, broker), ASTWriteSpikeMonitorConfig{
		PreferEBPF:  true,
		WindowMS:    1000,
		Threshold:   3,
		EBPFChecker: func() (bool, string) { return false, "capability denied" },
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	feedWrites(t, monitor, base, "ws-1", 3, 5)

	records := waitForSpikeRecords(t, broker, 1)
	if len(records) != 1 {
		t.Fatalf("expected one spike, got %d", len(records))
	}
	var event ASTSpikeEvent
	if err := decodePayload(records[0].Payload, &event); err != nil {
		t.Fatalf("decode spike payload: %v", err)
	}
	if event.SignalPath != signalPathUserspace {
		t.Fatalf("expected userspace fallback signal path, got %q", event.SignalPath)
	}
	if event.Notes == "" {
		t.Fatalf("expected fallback reason recorded in notes, got empty")
	}
}

func TestASTSpikeMonitorRedactionDropsHotPathsAndNotes(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	monitor := NewASTWriteSpikeMonitor(memorybroker.NewTeePublisher(nil, broker), ASTWriteSpikeMonitorConfig{
		PreferEBPF:  true,
		WindowMS:    1000,
		Threshold:   3,
		EBPFChecker: func() (bool, string) { return false, "capability denied" },
		Redaction: TelemetryRedactionConfig{
			Enabled:          true,
			AnomalyNotesMode: "drop",
		},
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	feedWrites(t, monitor, base, "ws-1", 3, 5)

	records := waitForSpikeRecords(t, broker, 1)
	if len(records) != 1 {
		t.Fatalf("expected one spike, got %d", len(records))
	}
	var event ASTSpikeEvent
	if err := decodePayload(records[0].Payload, &event); err != nil {
		t.Fatalf("decode spike payload: %v", err)
	}
	if event.Notes != "" {
		t.Fatalf("expected notes dropped by redaction, got %q", event.Notes)
	}
	if len(event.HotPaths) != 0 {
		t.Fatalf("expected hot paths dropped alongside notes, got %v", event.HotPaths)
	}
}

func TestASTSpikeMonitorIsolatesWorkspaces(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()

	monitor := NewASTWriteSpikeMonitor(memorybroker.NewTeePublisher(nil, broker), ASTWriteSpikeMonitorConfig{
		WindowMS:  1000,
		Threshold: 3,
	})

	base := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	// Two writes per workspace: neither alone reaches the threshold of 3.
	for i := 0; i < 2; i++ {
		at := base.Add(time.Duration(i*5) * time.Millisecond)
		_ = monitor.ObserveWrite(WriteEvent{Workspace: "ws-a", Path: "a.go", At: at})
		_ = monitor.ObserveWrite(WriteEvent{Workspace: "ws-b", Path: "b.go", At: at})
	}

	records := waitForSpikeRecords(t, broker, 1)
	if len(records) != 0 {
		t.Fatalf("expected per-workspace windows to stay below threshold, got %d", len(records))
	}
}
