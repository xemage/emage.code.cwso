package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

const (
	TopicDispatchCapabilities = "dispatch/capabilities"
	TopicDispatchDecision     = "dispatch/decision"
	TopicDispatchAnomaly      = "dispatch/anomaly"

	requestIDModeAllow = "allow"
	requestIDModeHash  = "hash"
	requestIDModeDrop  = "drop"

	anomalyNotesModeAllow = "allow"
	anomalyNotesModeDrop  = "drop"
)

// TelemetryRedactionConfig configures telemetry minimization for sensitive fields.
type TelemetryRedactionConfig struct {
	Enabled          bool
	RequestIDMode    string
	AnomalyNotesMode string
	RequestIDSalt    string
}

type telemetryRedactor struct {
	enabled          bool
	requestIDMode    string
	anomalyNotesMode string
	requestIDSalt    string
}

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
	redactor  telemetryRedactor
	now       func() time.Time
	nextID    atomic.Uint64
}

func NewDecisionEmitter(publisher memorybroker.Publisher) *DecisionEmitter {
	return NewDecisionEmitterWithAnomalyMonitorAndRedaction(publisher, nil, TelemetryRedactionConfig{})
}

func NewDecisionEmitterWithAnomalyMonitor(publisher memorybroker.Publisher, anomaly *DecisionAnomalyMonitor) *DecisionEmitter {
	return NewDecisionEmitterWithAnomalyMonitorAndRedaction(publisher, anomaly, TelemetryRedactionConfig{})
}

func NewDecisionEmitterWithAnomalyMonitorAndRedaction(
	publisher memorybroker.Publisher,
	anomaly *DecisionAnomalyMonitor,
	redaction TelemetryRedactionConfig,
) *DecisionEmitter {
	return &DecisionEmitter{
		publisher: publisher,
		anomaly:   anomaly,
		redactor:  newTelemetryRedactor(redaction),
		now:       time.Now,
	}
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
	event = e.redactor.redactDecision(event)
	if err := e.publisher.Publish(TopicDispatchDecision, event); err != nil {
		return err
	}
	if e.anomaly != nil {
		_ = e.anomaly.ObserveDecision(event)
	}
	return nil
}

func newTelemetryRedactor(cfg TelemetryRedactionConfig) telemetryRedactor {
	requestIDMode := normalizeRequestIDMode(cfg.RequestIDMode)
	if requestIDMode == "" {
		requestIDMode = requestIDModeAllow
	}
	anomalyNotesMode := normalizeAnomalyNotesMode(cfg.AnomalyNotesMode)
	if anomalyNotesMode == "" {
		anomalyNotesMode = anomalyNotesModeAllow
	}
	return telemetryRedactor{
		enabled:          cfg.Enabled,
		requestIDMode:    requestIDMode,
		anomalyNotesMode: anomalyNotesMode,
		requestIDSalt:    strings.TrimSpace(cfg.RequestIDSalt),
	}
}

func (r telemetryRedactor) redactDecision(event DecisionEvent) DecisionEvent {
	if !r.enabled {
		return event
	}
	switch r.requestIDMode {
	case requestIDModeDrop:
		event.RequestID = ""
	case requestIDModeHash:
		event.RequestID = r.hashRequestID(event.RequestID)
	}
	return event
}

func (r telemetryRedactor) redactAnomalyNotes(notes string) string {
	if !r.enabled {
		return notes
	}
	if r.anomalyNotesMode == anomalyNotesModeDrop {
		return ""
	}
	return notes
}

func (r telemetryRedactor) hashRequestID(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ""
	}
	payload := requestID
	if r.requestIDSalt != "" {
		payload = r.requestIDSalt + ":" + requestID
	}
	digest := sha256.Sum256([]byte(payload))
	// Keep a stable short hash for cardinality control while preserving correlation.
	return "sha256:" + hex.EncodeToString(digest[:12])
}

func normalizeRequestIDMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case requestIDModeAllow:
		return requestIDModeAllow
	case requestIDModeHash:
		return requestIDModeHash
	case requestIDModeDrop:
		return requestIDModeDrop
	default:
		return ""
	}
}

func normalizeAnomalyNotesMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case anomalyNotesModeAllow:
		return anomalyNotesModeAllow
	case anomalyNotesModeDrop:
		return anomalyNotesModeDrop
	default:
		return ""
	}
}
