package dispatch

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

const defaultAnomalyLatencyThresholdMS = 1200

// DecisionAnomalyMonitorConfig configures spike-level anomaly monitoring.
type DecisionAnomalyMonitorConfig struct {
	PreferEBPF         bool
	LatencyThresholdMS int
	EBPFChecker        func() (bool, string)
	Redaction          TelemetryRedactionConfig
}

// AnomalyEvent is the telemetry envelope for event-driven dispatch anomalies.
type AnomalyEvent struct {
	AnomalyID             string   `json:"anomaly_id"`
	DecisionID            string   `json:"decision_id"`
	CapabilityEpoch       uint64   `json:"capability_epoch"`
	SelectedProvider      string   `json:"selected_provider"`
	ReasonCode            string   `json:"reason_code"`
	Severity              string   `json:"severity"`
	ObservedValue         float64  `json:"observed_value"`
	ThresholdValue        float64  `json:"threshold_value"`
	SignalPath            string   `json:"signal_path"`
	PrivilegeRequirement  string   `json:"privilege_requirement"`
	DetectionLatencyMS    int      `json:"detection_latency_ms"`
	DetectionLatencyMode  string   `json:"detection_latency_mode"`
	DetectionLatencyIsAdvisory bool `json:"detection_latency_is_advisory"`
	FeatureFlagsApplied   []string `json:"feature_flags_applied,omitempty"`
	QualityGuardrailState string   `json:"quality_guardrail_state"`
	DetectedAt            string   `json:"detected_at"`
	Notes                 string   `json:"notes,omitempty"`
}

// DecisionAnomalyMonitor emits anomaly events from dispatch decision telemetry.
type DecisionAnomalyMonitor struct {
	publisher          memorybroker.Publisher
	now                func() time.Time
	nextID             atomic.Uint64
	preferEBPF         bool
	latencyThresholdMS int
	checkEBPF          func() (bool, string)
	redactor           telemetryRedactor
}

func NewDecisionAnomalyMonitor(publisher memorybroker.Publisher, cfg DecisionAnomalyMonitorConfig) *DecisionAnomalyMonitor {
	if publisher == nil {
		return nil
	}
	if cfg.LatencyThresholdMS <= 0 {
		cfg.LatencyThresholdMS = defaultAnomalyLatencyThresholdMS
	}
	check := cfg.EBPFChecker
	if check == nil {
		check = defaultEBPFChecker
	}
	return &DecisionAnomalyMonitor{
		publisher:          publisher,
		now:                time.Now,
		preferEBPF:         cfg.PreferEBPF,
		latencyThresholdMS: cfg.LatencyThresholdMS,
		checkEBPF:          check,
		redactor:           newTelemetryRedactor(cfg.Redaction),
	}
}

func (m *DecisionAnomalyMonitor) ObserveDecision(event DecisionEvent) error {
	if m == nil || m.publisher == nil {
		return nil
	}
	detectedAt := m.now().UTC()
	signalPath, privilege, notes := m.resolveSignalPath()

	anomalies := make([]AnomalyEvent, 0, 2)
	if event.ActualLatencyMS >= m.latencyThresholdMS {
		anomalies = append(anomalies, m.newAnomaly(
			event,
			detectedAt,
			signalPath,
			privilege,
			notes,
			"latency_threshold_exceeded",
			"warning",
			float64(event.ActualLatencyMS),
			float64(m.latencyThresholdMS),
		))
	}
	if event.FallbackCount > 0 {
		anomalies = append(anomalies, m.newAnomaly(
			event,
			detectedAt,
			signalPath,
			privilege,
			notes,
			"fallback_engaged",
			"warning",
			float64(event.FallbackCount),
			0,
		))
	}

	for _, anomaly := range anomalies {
		if err := m.publisher.Publish(TopicDispatchAnomaly, anomaly); err != nil {
			return err
		}
	}
	return nil
}

func (m *DecisionAnomalyMonitor) newAnomaly(
	event DecisionEvent,
	detectedAt time.Time,
	signalPath, privilege, notes, reasonCode, severity string,
	observed, threshold float64,
) AnomalyEvent {
	latencyMS, mode, advisory := detectionLatency(event.EmittedAt, detectedAt, signalPath)
	return AnomalyEvent{
		AnomalyID:             fmt.Sprintf("anomaly-%d", m.nextID.Add(1)),
		DecisionID:            event.DecisionID,
		CapabilityEpoch:       event.CapabilityEpoch,
		SelectedProvider:      event.SelectedProvider,
		ReasonCode:            reasonCode,
		Severity:              severity,
		ObservedValue:         observed,
		ThresholdValue:        threshold,
		SignalPath:            signalPath,
		PrivilegeRequirement:  privilege,
		DetectionLatencyMS:    latencyMS,
		DetectionLatencyMode:  mode,
		DetectionLatencyIsAdvisory: advisory,
		FeatureFlagsApplied:   event.FeatureFlagsApplied,
		QualityGuardrailState: event.QualityGuardrailState,
		DetectedAt:            detectedAt.Format(time.RFC3339Nano),
		Notes:                 m.redactor.redactAnomalyNotes(notes),
	}
}

func (m *DecisionAnomalyMonitor) resolveSignalPath() (path, privilege, notes string) {
	if m.preferEBPF {
		ok, reason := m.checkEBPF()
		if ok {
			return "ebpf-hook", "CAP_BPF/CAP_PERFMON or root", ""
		}
		return "fallback-userspace", "none", fmt.Sprintf("ebpf unavailable: %s", strings.TrimSpace(reason))
	}
	return "fallback-userspace", "none", ""
}

func detectionLatency(emittedAt string, detectedAt time.Time, path string) (int, string, bool) {
	if path == "ebpf-hook" {
		return 0, "advisory", true
	}
	emittedAt = strings.TrimSpace(emittedAt)
	if emittedAt == "" {
		return 0, "estimated", false
	}
	ts, err := time.Parse(time.RFC3339Nano, emittedAt)
	if err != nil {
		return 0, "estimated", false
	}
	delta := detectedAt.Sub(ts.UTC())
	if delta < 0 {
		delta = 0
	}
	return int(delta.Milliseconds()), "measured", false
}

func defaultEBPFChecker() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "non-linux host"
	}
	if os.Geteuid() != 0 {
		return false, "missing root/CAP_BPF privileges"
	}
	if _, err := os.Stat("/sys/fs/bpf"); err != nil {
		return false, "bpffs is unavailable"
	}
	return true, ""
}
