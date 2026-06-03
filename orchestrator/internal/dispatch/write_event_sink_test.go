package dispatch

import (
	"errors"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

type countingSink struct {
	count int
	err   error
}

func (c *countingSink) ObserveWrite(WriteEvent) error {
	c.count++
	return c.err
}

func TestWriteEventFanoutDeliversToAll(t *testing.T) {
	a := &countingSink{}
	b := &countingSink{}
	fan := NewWriteEventFanout(a, b)
	if fan == nil {
		t.Fatal("expected non-nil fanout")
	}
	if err := fan.ObserveWrite(WriteEvent{Path: "x.go"}); err != nil {
		t.Fatalf("observe: %v", err)
	}
	if a.count != 1 || b.count != 1 {
		t.Fatalf("expected both sinks invoked once, got a=%d b=%d", a.count, b.count)
	}
}

func TestWriteEventFanoutDropsNilSinks(t *testing.T) {
	if NewWriteEventFanout() != nil {
		t.Fatal("empty fanout should be nil")
	}
	if NewWriteEventFanout(nil, nil) != nil {
		t.Fatal("all-nil fanout should be nil")
	}
	a := &countingSink{}
	fan := NewWriteEventFanout(nil, a, nil)
	if fan == nil {
		t.Fatal("expected non-nil fanout with one live sink")
	}
	_ = fan.ObserveWrite(WriteEvent{})
	if a.count != 1 {
		t.Fatalf("expected live sink invoked, got %d", a.count)
	}
}

func TestWriteEventFanoutContinuesAfterError(t *testing.T) {
	boom := errors.New("boom")
	a := &countingSink{err: boom}
	b := &countingSink{}
	fan := NewWriteEventFanout(a, b)
	err := fan.ObserveWrite(WriteEvent{})
	if !errors.Is(err, boom) {
		t.Fatalf("expected first error returned, got %v", err)
	}
	if b.count != 1 {
		t.Fatal("a failing sink must not prevent later sinks from observing")
	}
}

// The real monitor + filter satisfy WriteEventSink and run end-to-end through the fanout.
func TestWriteEventFanoutWithRealStages(t *testing.T) {
	broker := newSpikeBroker(t)
	defer broker.Close()
	pub := memorybroker.NewTeePublisher(nil, broker)

	monitor := NewASTWriteSpikeMonitor(pub, ASTWriteSpikeMonitorConfig{WindowMS: 60000, Threshold: 2, DebounceMS: 1})
	filter := NewASTSpikeFilter(pub, ASTSpikeFilterConfig{SemanticThreshold: SpikeThresholdAny})
	fan := NewWriteEventFanout(monitor, filter)

	// First write to a symbol: filter classifies symbol_added (passes 'any'); below volume threshold.
	_ = fan.ObserveWrite(WriteEvent{Workspace: "ws1", Path: "a.go", Symbol: "a.go", SignatureHash: "h1"})
	// Second write crosses the volume threshold and changes the signature.
	_ = fan.ObserveWrite(WriteEvent{Workspace: "ws1", Path: "a.go", Symbol: "a.go", SignatureHash: "h2"})

	if records := waitForTopicRecords(t, broker, TopicASTSpike, 1); len(records) == 0 {
		t.Fatal("expected a volume spike on ast/spike")
	}
	if records := waitForTopicRecords(t, broker, TopicASTSemanticSpike, 1); len(records) == 0 {
		t.Fatal("expected a semantic spike on ast/semantic-spike")
	}
}
