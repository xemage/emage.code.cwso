package dispatch

import (
	"fmt"
	"strings"
)

// Latency requirement values accepted by hardware-aware dispatch (Feature A).
const (
	LatencyRealtime = "realtime"
	LatencyBatch    = "batch"
)

// Workload tags emitted by the profiler. They align with provider
// SupportedWorkloadTags so the deterministic policy engine can match eligible
// backends without any model-side coordination.
const (
	WorkloadDefault           = "default"
	WorkloadRealtime          = "realtime"
	WorkloadLongContext       = "long-context"
	WorkloadInferenceHeavy    = "inference-heavy"
	WorkloadDeterministicEdit = "deterministic-edit"
)

// Recommended hardware classes. These are advisory hints surfaced to the
// caller; the authoritative routing decision is the policy engine selection
// across whatever providers are actually registered.
const (
	HardwareClassLPU       = "lpu"
	HardwareClassGPU       = "gpu"
	HardwareClassSSM       = "ssm"
	HardwareClassWasmLocal = "wasm-local"
	HardwareClassCPU       = "cpu"
)

// Profiler thresholds. Exported so routing behavior is testable and tunable
// without code changes leaking into callers.
const (
	// RealtimeContextCeiling caps the context size that may route to an LPU;
	// above it, even realtime requests fall through to denser backends.
	RealtimeContextCeiling = 8192
	// LongContextFloor is the context size at or above which SSM (linear
	// context scaling) routing applies regardless of latency intent.
	LongContextFloor = 32768
)

// WorkloadProfile is the deterministic "tensor tag" derived from a task. It is
// pure data: identical inputs always yield an identical profile.
type WorkloadProfile struct {
	Tags                []string
	RecommendedClass    string
	ContextSizeEstimate int
	LatencyRequirement  string
	// RequestLabels carries optional structured signals for assist paths, e.g.
	// "sequence_length=120000" consumed by the SSM sequence-assist scorer.
	RequestLabels []string
}

// NormalizeLatencyRequirement validates and canonicalizes latency intent.
// Empty input defaults to batch; unknown values return an error.
func NormalizeLatencyRequirement(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", LatencyBatch:
		return LatencyBatch, nil
	case LatencyRealtime:
		return LatencyRealtime, nil
	default:
		return "", fmt.Errorf("latency_requirement must be one of: %s, %s", LatencyRealtime, LatencyBatch)
	}
}

// ProfileTask derives a deterministic workload profile from task signals,
// mirroring the Heterogeneous Hardware Dispatcher routing matrix:
//
//   - very large context               -> SSM (linear context scaling)
//   - realtime + small context         -> LPU (ultra-low latency)
//   - batch + deterministic small edit -> Wasm-local micro-agent
//   - everything else                  -> dense GPU model
//
// Large-context routing is evaluated first so a huge codebase analysis never
// gets pinned to a latency-optimized backend that would OOM on context.
func ProfileTask(taskDescription string, contextSizeEstimate int, latencyRequirement string) WorkloadProfile {
	if contextSizeEstimate < 0 {
		contextSizeEstimate = 0
	}
	latency, _ := NormalizeLatencyRequirement(latencyRequirement)

	profile := WorkloadProfile{
		ContextSizeEstimate: contextSizeEstimate,
		LatencyRequirement:  latency,
	}

	switch {
	case contextSizeEstimate >= LongContextFloor:
		profile.Tags = []string{WorkloadLongContext}
		profile.RecommendedClass = HardwareClassSSM
		profile.RequestLabels = append(profile.RequestLabels, fmt.Sprintf("sequence_length=%d", contextSizeEstimate))
	case latency == LatencyRealtime && contextSizeEstimate <= RealtimeContextCeiling:
		profile.Tags = []string{WorkloadRealtime}
		profile.RecommendedClass = HardwareClassLPU
	case latency == LatencyBatch && isDeterministicEdit(taskDescription):
		profile.Tags = []string{WorkloadDeterministicEdit, WorkloadInferenceHeavy}
		profile.RecommendedClass = HardwareClassWasmLocal
	default:
		profile.Tags = []string{WorkloadInferenceHeavy}
		profile.RecommendedClass = HardwareClassGPU
	}

	profile.Tags = dedupeAndSort(profile.Tags)
	return profile
}

// isDeterministicEdit heuristically detects small, mechanical edit tasks that
// are safe to route to ultra-light Wasm micro-agents (Feature B). The list is
// intentionally conservative: anything not clearly mechanical falls through to
// a dense model.
func isDeterministicEdit(taskDescription string) bool {
	desc := strings.ToLower(taskDescription)
	for _, kw := range deterministicEditKeywords {
		if strings.Contains(desc, kw) {
			return true
		}
	}
	return false
}

var deterministicEditKeywords = []string{
	"rename", "format", "gofmt", "lint", "add type", "add types",
	"type annotation", "reorder imports", "whitespace", "docstring",
}
