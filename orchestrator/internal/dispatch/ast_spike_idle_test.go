package dispatch

import (
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

// TestASTSpikePipelineZeroIdleEmissions verifies the Phase 7 "0% idle CPU" semantics for the
// AST spike pipeline: monitors and filters are purely event-driven (ObserveWrite) with no
// background polling goroutines, so an idle pipeline must emit zero broker records.
func TestASTSpikePipelineZeroIdleEmissions(t *testing.T) {
	broker := memorybroker.New(memorybroker.WithCapacity(32))
	pub := memorybroker.NewTeePublisher(nil, broker)

	monitor := NewASTWriteSpikeMonitor(pub, ASTWriteSpikeMonitorConfig{
		WindowMS: 1000, Threshold: 3, DebounceMS: 500, MaxHotPaths: 5,
	})
	filter := NewASTSpikeFilter(pub, ASTSpikeFilterConfig{
		SemanticThreshold: SpikeKindSignatureChange,
		ConflictWindowMS:  2000,
	})
	sink := NewWriteEventFanout(monitor, filter)
	if sink == nil {
		t.Fatal("expected non-nil fanout sink")
	}

	// Idle window — no writes fed. A polling monitor would emit spurious records here.
	time.Sleep(50 * time.Millisecond)

	topics := []string{TopicASTSpike, TopicASTSemanticSpike, TopicASTConflictWarning}
	recs := broker.Query(memorybroker.QueryOptions{Topics: topics})
	if len(recs) != 0 {
		t.Fatalf("idle pipeline emitted %d records, want 0 (0%% idle CPU): %+v", len(recs), recs)
	}
}
