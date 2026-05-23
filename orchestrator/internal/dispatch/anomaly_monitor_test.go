package dispatch

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

func TestDecisionAnomalyMonitorFallbackPath(t *testing.T) {
	broker := memorybroker.New(
		memorybroker.WithCapacity(16),
		memorybroker.WithIngressQueueSize(16),
	)
	defer broker.Close()

	monitor := NewDecisionAnomalyMonitor(memorybroker.NewTeePublisher(nil, broker), DecisionAnomalyMonitorConfig{
		PreferEBPF:         false,
		LatencyThresholdMS: 100,
	})
	monitor.now = func() time.Time {
		return time.Date(2026, time.May, 22, 12, 0, 0, 20*int(time.Millisecond), time.UTC)
	}

	err := monitor.ObserveDecision(DecisionEvent{
		DecisionID:            "decision-1",
		CapabilityEpoch:       3,
		SelectedProvider:      "cpu-baseline",
		FallbackCount:         1,
		ActualLatencyMS:       180,
		EmittedAt:             time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FeatureFlagsApplied:   []string{"hhd.monitoring"},
		QualityGuardrailState: "pass",
	})
	if err != nil {
		t.Fatalf("observe decision: %v", err)
	}

	records := waitForAnomalyRecords(t, broker, 2)
	if len(records) != 2 {
		t.Fatalf("expected 2 anomaly records, got %d", len(records))
	}

	reasons := make(map[string]AnomalyEvent, len(records))
	for _, record := range records {
		var event AnomalyEvent
		if err := decodePayload(record.Payload, &event); err != nil {
			t.Fatalf("decode anomaly payload: %v", err)
		}
		reasons[event.ReasonCode] = event
		if event.SignalPath != "fallback-userspace" {
			t.Fatalf("expected fallback signal path, got %+v", event)
		}
		if event.PrivilegeRequirement != "none" {
			t.Fatalf("expected unprivileged path, got %+v", event)
		}
	}
	if _, ok := reasons["latency_threshold_exceeded"]; !ok {
		t.Fatalf("expected latency anomaly, got %+v", reasons)
	}
	if _, ok := reasons["fallback_engaged"]; !ok {
		t.Fatalf("expected fallback anomaly, got %+v", reasons)
	}
}

func TestDecisionAnomalyMonitorEBPFPreferredUsesHookWhenAvailable(t *testing.T) {
	broker := memorybroker.New(
		memorybroker.WithCapacity(16),
		memorybroker.WithIngressQueueSize(16),
	)
	defer broker.Close()

	monitor := NewDecisionAnomalyMonitor(memorybroker.NewTeePublisher(nil, broker), DecisionAnomalyMonitorConfig{
		PreferEBPF:         true,
		LatencyThresholdMS: 100,
		EBPFChecker: func() (bool, string) {
			return true, ""
		},
	})
	if err := monitor.ObserveDecision(DecisionEvent{
		DecisionID:            "decision-2",
		SelectedProvider:      "gpu-a",
		ActualLatencyMS:       140,
		EmittedAt:             time.Now().UTC().Format(time.RFC3339Nano),
		QualityGuardrailState: "pass",
	}); err != nil {
		t.Fatalf("observe decision: %v", err)
	}

	records := waitForAnomalyRecords(t, broker, 1)
	if len(records) != 1 {
		t.Fatalf("expected one anomaly record, got %d", len(records))
	}

	var event AnomalyEvent
	if err := json.Unmarshal(records[0].Payload, &event); err != nil {
		t.Fatalf("decode anomaly payload: %v", err)
	}
	if event.SignalPath != "ebpf-hook" {
		t.Fatalf("expected ebpf-hook signal path, got %+v", event)
	}
	if event.DetectionLatencyMode != "estimated" || event.DetectionLatencyMS != 2 {
		t.Fatalf("expected estimated eBPF latency, got %+v", event)
	}
}

func TestDecisionAnomalyMonitorEBPFPreferredFallsBackWhenUnavailable(t *testing.T) {
	broker := memorybroker.New(
		memorybroker.WithCapacity(16),
		memorybroker.WithIngressQueueSize(16),
	)
	defer broker.Close()

	monitor := NewDecisionAnomalyMonitor(memorybroker.NewTeePublisher(nil, broker), DecisionAnomalyMonitorConfig{
		PreferEBPF:         true,
		LatencyThresholdMS: 100,
		EBPFChecker: func() (bool, string) {
			return false, "capability denied"
		},
	})
	if err := monitor.ObserveDecision(DecisionEvent{
		DecisionID:       "decision-3",
		SelectedProvider: "gpu-a",
		ActualLatencyMS:  140,
	}); err != nil {
		t.Fatalf("observe decision: %v", err)
	}

	records := waitForAnomalyRecords(t, broker, 1)
	if len(records) != 1 {
		t.Fatalf("expected one anomaly record, got %d", len(records))
	}

	var event AnomalyEvent
	if err := decodePayload(records[0].Payload, &event); err != nil {
		t.Fatalf("decode anomaly payload: %v", err)
	}
	if event.SignalPath != "fallback-userspace" {
		t.Fatalf("expected fallback signal path, got %+v", event)
	}
	if event.Notes == "" {
		t.Fatalf("expected eBPF fallback reason in notes, got %+v", event)
	}
}

func TestDecisionAnomalyMonitorDropsNotesWhenConfigured(t *testing.T) {
	broker := memorybroker.New(
		memorybroker.WithCapacity(16),
		memorybroker.WithIngressQueueSize(16),
	)
	defer broker.Close()

	monitor := NewDecisionAnomalyMonitor(memorybroker.NewTeePublisher(nil, broker), DecisionAnomalyMonitorConfig{
		PreferEBPF:         true,
		LatencyThresholdMS: 100,
		EBPFChecker: func() (bool, string) {
			return false, "capability denied"
		},
		Redaction: TelemetryRedactionConfig{
			Enabled:          true,
			RequestIDMode:    "hash",
			AnomalyNotesMode: "drop",
		},
	})
	if err := monitor.ObserveDecision(DecisionEvent{
		DecisionID:       "decision-4",
		SelectedProvider: "gpu-a",
		ActualLatencyMS:  140,
	}); err != nil {
		t.Fatalf("observe decision: %v", err)
	}

	records := waitForAnomalyRecords(t, broker, 1)
	if len(records) != 1 {
		t.Fatalf("expected one anomaly record, got %d", len(records))
	}

	var event AnomalyEvent
	if err := decodePayload(records[0].Payload, &event); err != nil {
		t.Fatalf("decode anomaly payload: %v", err)
	}
	if event.Notes != "" {
		t.Fatalf("expected notes to be dropped by redaction policy, got %+v", event)
	}
}

func waitForAnomalyRecords(t *testing.T, broker *memorybroker.Broker, expected int) []memorybroker.Record {
	t.Helper()
	if expected <= 0 {
		expected = 1
	}
	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		records := broker.Query(memorybroker.QueryOptions{Topics: []string{TopicDispatchAnomaly}, Limit: expected + 2})
		if len(records) >= expected {
			return records
		}
		if time.Now().After(deadline) {
			return records
		}
		time.Sleep(5 * time.Millisecond)
	}
}
