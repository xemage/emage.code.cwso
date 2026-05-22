package dispatch

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

func TestDecisionEmitterEmitsDecisionEnvelopeFields(t *testing.T) {
	broker := memorybroker.New(
		memorybroker.WithCapacity(16),
		memorybroker.WithIngressQueueSize(16),
	)
	defer broker.Close()

	emitter := NewDecisionEmitter(memorybroker.NewTeePublisher(nil, broker))
	emitter.now = func() time.Time {
		return time.Date(2026, time.May, 22, 10, 10, 10, 0, time.UTC)
	}

	err := emitter.EmitDecision(DecisionEvent{
		PolicyVersion:         "policy-v2",
		CapabilityEpoch:       4,
		SelectedProvider:      "cpu-baseline",
		FallbackChain:         []string{"cpu-baseline"},
		FallbackCount:         0,
		ReasonCode:            "accepted",
		Confidence:            0.98,
		EstimatedLatencyMS:    120,
		ActualLatencyMS:       95,
		FeatureFlagsApplied:   []string{"hhd.decisions"},
		QualityGuardrailState: "pass",
	})
	if err != nil {
		t.Fatalf("emit decision: %v", err)
	}

	records := waitForRecords(t, broker, TopicDispatchDecision)
	if len(records) != 1 {
		t.Fatalf("expected one decision record, got %d", len(records))
	}

	var got map[string]any
	if err := decodePayload(records[0].Payload, &got); err != nil {
		t.Fatalf("decode decision payload: %v", err)
	}

	required := []string{
		"decision_id",
		"policy_version",
		"capability_epoch",
		"selected_provider",
		"fallback_chain",
		"fallback_count",
		"reason_code",
		"confidence",
		"estimated_latency_ms",
		"actual_latency_ms",
		"feature_flags_applied",
		"quality_guardrail_state",
	}
	for _, field := range required {
		if _, ok := got[field]; !ok {
			t.Fatalf("missing required field %q in payload: %+v", field, got)
		}
	}
}

func TestDecisionEmitterEmitsCapabilitySnapshot(t *testing.T) {
	broker := memorybroker.New(
		memorybroker.WithCapacity(16),
		memorybroker.WithIngressQueueSize(16),
	)
	defer broker.Close()

	emitter := NewDecisionEmitter(memorybroker.NewTeePublisher(nil, broker))
	err := emitter.EmitCapabilitySnapshot(CapabilitySnapshot{
		Epoch:      7,
		CapturedAt: time.Date(2026, time.May, 22, 11, 0, 0, 0, time.UTC),
		Providers: []ProviderCapability{
			{
				ProviderID:            "gpu-a",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "fast",
				CostClass:             "high",
				QueueDepth:            3,
				SupportedWorkloadTags: []string{"inference-heavy"},
				ReliabilityClass:      "gold",
				FeatureFlags:          []string{"hhd.shadow"},
			},
		},
	})
	if err != nil {
		t.Fatalf("emit capability snapshot: %v", err)
	}

	records := waitForRecords(t, broker, TopicDispatchCapabilities)
	if len(records) != 1 {
		t.Fatalf("expected one capability snapshot record, got %d", len(records))
	}

	var got map[string]any
	if err := decodePayload(records[0].Payload, &got); err != nil {
		t.Fatalf("decode capability payload: %v", err)
	}
	if got["capability_epoch"] != float64(7) {
		t.Fatalf("expected epoch 7 in payload, got %+v", got)
	}
	providers, ok := got["providers"].([]any)
	if !ok || len(providers) != 1 {
		t.Fatalf("expected one provider in payload, got %+v", got["providers"])
	}
}

func TestDecisionEmitterEmitsAnomalyEventsWhenMonitorEnabled(t *testing.T) {
	broker := memorybroker.New(
		memorybroker.WithCapacity(16),
		memorybroker.WithIngressQueueSize(16),
	)
	defer broker.Close()

	publisher := memorybroker.NewTeePublisher(nil, broker)
	monitor := NewDecisionAnomalyMonitor(publisher, DecisionAnomalyMonitorConfig{
		PreferEBPF:         true,
		LatencyThresholdMS: 100,
		EBPFChecker: func() (bool, string) {
			return false, "denied"
		},
	})
	emitter := NewDecisionEmitterWithAnomalyMonitor(publisher, monitor)

	err := emitter.EmitDecision(DecisionEvent{
		PolicyVersion:         "policy-v2",
		CapabilityEpoch:       9,
		SelectedProvider:      "gpu-a",
		FallbackChain:         []string{"gpu-a", "cpu-baseline"},
		FallbackCount:         1,
		ReasonCode:            "capacity_exhausted",
		ActualLatencyMS:       140,
		EstimatedLatencyMS:    80,
		QualityGuardrailState: "pass",
	})
	if err != nil {
		t.Fatalf("emit decision: %v", err)
	}

	records := waitForRecords(t, broker, TopicDispatchAnomaly)
	if len(records) != 1 {
		t.Fatalf("expected anomaly record, got %d", len(records))
	}

	var anomaly AnomalyEvent
	if err := decodePayload(records[0].Payload, &anomaly); err != nil {
		t.Fatalf("decode anomaly payload: %v", err)
	}
	if anomaly.SignalPath != "fallback-userspace" {
		t.Fatalf("expected fallback signal path when eBPF unavailable, got %+v", anomaly)
	}
}

func decodePayload(payload json.RawMessage, out any) error {
	return json.Unmarshal(payload, out)
}

func waitForRecords(t *testing.T, broker *memorybroker.Broker, topic string) []memorybroker.Record {
	t.Helper()
	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		records := broker.Query(memorybroker.QueryOptions{Topics: []string{topic}, Limit: 1})
		if len(records) > 0 {
			return records
		}
		if time.Now().After(deadline) {
			return records
		}
		time.Sleep(5 * time.Millisecond)
	}
}
