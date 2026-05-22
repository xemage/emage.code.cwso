package dispatch

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

const (
	TopicDispatchCapabilities = "dispatch/capabilities"
	TopicDispatchDecision     = "dispatch/decision"
	TopicDispatchAnomaly      = "dispatch/anomaly"
)

// DecisionEvent is the auditable dispatch decision telemetry envelope.
type DecisionEvent struct {
	DecisionID            string   `json:"decision_id"`
	RequestID             string   `json:"request_id,omitempty"`
	PolicyVersion         string   `json:"policy_version"`
	CapabilityEpoch       uint64   `json:"capability_epoch"`
	SelectedProvider      string   `json:"selected_provider"`
	FallbackChain         []string `json:"fallback_chain"`
	FallbackCount         int      `json:"fallback_count"`
	ReasonCode            string   `json:"reason_code"`
	Confidence            float64  `json:"confidence"`
	EstimatedLatencyMS    int      `json:"estimated_latency_ms"`
	ActualLatencyMS       int      `json:"actual_latency_ms"`
	FeatureFlagsApplied   []string `json:"feature_flags_applied,omitempty"`
	QualityGuardrailState string   `json:"quality_guardrail_state"`
	EmittedAt             string   `json:"emitted_at"`
}

// DecisionEmitter publishes decision and capability telemetry events.
type DecisionEmitter struct {
	publisher memorybroker.Publisher
	anomaly   *DecisionAnomalyMonitor
	now       func() time.Time
	nextID    atomic.Uint64
}

func NewDecisionEmitter(publisher memorybroker.Publisher) *DecisionEmitter {
	return NewDecisionEmitterWithAnomalyMonitor(publisher, nil)
}

func NewDecisionEmitterWithAnomalyMonitor(publisher memorybroker.Publisher, anomaly *DecisionAnomalyMonitor) *DecisionEmitter {
	return &DecisionEmitter{publisher: publisher, anomaly: anomaly, now: time.Now}
}

func (e *DecisionEmitter) EmitCapabilitySnapshot(snapshot CapabilitySnapshot) error {
	if e == nil || e.publisher == nil {
		return nil
	}
	if len(snapshot.Providers) == 0 {
		return nil
	}
	payload := map[string]any{
		"capability_epoch": snapshot.Epoch,
		"captured_at":      snapshot.CapturedAt.UTC().Format(time.RFC3339Nano),
		"providers":        snapshot.Providers,
	}
	return e.publisher.Publish(TopicDispatchCapabilities, payload)
}

func (e *DecisionEmitter) EmitDecision(event DecisionEvent) error {
	if e == nil || e.publisher == nil {
		return nil
	}
	if event.PolicyVersion == "" {
		event.PolicyVersion = "cpu-baseline-default"
	}
	if event.SelectedProvider == "" {
		event.SelectedProvider = "cpu-baseline"
	}
	if event.ReasonCode == "" {
		event.ReasonCode = "accepted"
	}
	if event.QualityGuardrailState == "" {
		event.QualityGuardrailState = "not-evaluated"
	}
	if event.DecisionID == "" {
		event.DecisionID = fmt.Sprintf("decision-%d", e.nextID.Add(1))
	}
	if event.EmittedAt == "" {
		event.EmittedAt = e.now().UTC().Format(time.RFC3339Nano)
	}
	if event.FallbackCount < 0 {
		event.FallbackCount = 0
	}
	if event.FallbackChain == nil {
		event.FallbackChain = []string{}
	}
	if event.FeatureFlagsApplied == nil {
		event.FeatureFlagsApplied = []string{}
	}
	if err := e.publisher.Publish(TopicDispatchDecision, event); err != nil {
		return err
	}
	if e.anomaly != nil {
		_ = e.anomaly.ObserveDecision(event)
	}
	return nil
}
