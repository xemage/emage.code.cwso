package dispatch

import (
	"sort"
	"strings"
)

const ReasonQualityGuardrailAutodisable = "quality_guardrail_autodisable"

var sparseEscalationWorkloadTags = []string{"inference-heavy", "deterministic-edit"}

// QualityGuardrailBreached reports whether an observed quality score is below the configured minimum.
// A nil score is not a breach (no signal).
func QualityGuardrailBreached(qualityScore *float64, minScore float64) bool {
	if qualityScore == nil {
		return false
	}
	return *qualityScore < minScore
}

// SelectDenseGPUEscalation routes a sparse-tier quality-floor breach to a dense GPU backend via the
// standard policy engine (sparse/quantized assist off). The reason code matches the existing
// quality_guardrail_autodisable path for observability.
func SelectDenseGPUEscalation(engine *PolicyEngineV2, snapshot CapabilitySnapshot) PolicyDecision {
	if engine == nil {
		return PolicyDecision{
			PolicyVersion:       DefaultPolicyVersionV2,
			CapabilityEpoch:     snapshot.Epoch,
			SelectedProvider:    DefaultBaselineProviderID,
			RankedFallbackChain: []string{DefaultBaselineProviderID},
			Confidence:          1,
			ReasonCode:          ReasonQualityGuardrailAutodisable,
		}
	}
	decision := engine.Select(snapshot, PolicyInput{WorkloadTags: sparseEscalationWorkloadTags})
	decision.ReasonCode = ReasonQualityGuardrailAutodisable
	if decision.SelectedProvider == "" || decision.SelectedProvider == engine.cfg.BaselineProviderID {
		if dense := FirstDenseGPUProvider(snapshot); dense != "" {
			decision.SelectedProvider = dense
			chain := []string{dense}
			for _, id := range decision.RankedFallbackChain {
				if id != dense {
					chain = append(chain, id)
				}
			}
			decision.RankedFallbackChain = chain
		}
	}
	if decision.SelectedProvider == "" {
		decision.SelectedProvider = engine.cfg.BaselineProviderID
	}
	if len(decision.RankedFallbackChain) == 0 {
		decision.RankedFallbackChain = []string{decision.SelectedProvider}
	}
	return decision
}

// EscalationEngineForDenseGPU returns a policy engine with sparse/quantized assist disabled so
// escalation selects a dense GPU provider instead of re-triggering the assist guardrail short-circuit.
func EscalationEngineForDenseGPU(base PolicyV2Config) *PolicyEngineV2 {
	base.SparseQuantized.Enabled = false
	return NewPolicyEngineV2(base)
}

// FirstDenseGPUProvider returns the lexicographically first healthy provider that supports dense
// inference workloads. Used as a deterministic fallback when policy selection yields baseline only.
func FirstDenseGPUProvider(snapshot CapabilitySnapshot) string {
	type pick struct {
		id string
	}
	var candidates []pick
	for _, p := range snapshot.Providers {
		if p.ProviderID == DefaultBaselineProviderID {
			continue
		}
		if !stringsEqualFold(p.HealthState, HealthHealthy) {
			continue
		}
		if providerSupportsDenseInference(p) {
			candidates = append(candidates, pick{id: p.ProviderID})
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].id < candidates[j].id })
	return candidates[0].id
}

func providerSupportsDenseInference(p ProviderCapability) bool {
	for _, tag := range p.SupportedWorkloadTags {
		switch stringsTrimLower(tag) {
		case "inference-heavy", "deterministic-edit", "default":
			return true
		}
	}
	return false
}

func stringsEqualFold(a, b string) bool {
	return stringsTrimLower(a) == stringsTrimLower(b)
}

func stringsTrimLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
