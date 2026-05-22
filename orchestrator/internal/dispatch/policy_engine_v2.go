package dispatch

import (
	"context"
	"sort"
	"strings"
)

const (
	DefaultBaselineProviderID = "cpu-baseline"
	DefaultPolicyVersionV2    = "policy-v2"
)

// PolicyInput carries policy-scoring context for one dispatch decision.
type PolicyInput struct {
	WorkloadTags []string
}

// PolicyWeights controls how capability dimensions influence ranking.
type PolicyWeights struct {
	Health      float64
	Reliability float64
	Cost        float64
	Latency     float64
	QueueDepth  float64
	Workload    float64
}

// PolicyV2Config controls deterministic scoring and fallback behavior.
type PolicyV2Config struct {
	Enabled            bool
	PolicyVersion      string
	BaselineProviderID string
	MinConfidence      float64
	MaxQueueDepth      int
	Weights            PolicyWeights
	ScoreAdjuster      ScoreAdjuster
}

// PolicyDecision is the deterministic routing decision envelope.
type PolicyDecision struct {
	PolicyVersion       string
	CapabilityEpoch     uint64
	SelectedProvider    string
	RankedFallbackChain []string
	Confidence          float64
	ReasonCode          string
}

// PolicyEngineV2 deterministically ranks providers and computes fallback chains.
type PolicyEngineV2 struct {
	cfg           PolicyV2Config
	scoreAdjuster ScoreAdjuster
}

func DefaultPolicyV2Config() PolicyV2Config {
	return PolicyV2Config{
		Enabled:            false,
		PolicyVersion:      DefaultPolicyVersionV2,
		BaselineProviderID: DefaultBaselineProviderID,
		MinConfidence:      0.50,
		MaxQueueDepth:      32,
		Weights: PolicyWeights{
			Health:      0.35,
			Reliability: 0.25,
			Cost:        0.10,
			Latency:     0.10,
			QueueDepth:  0.10,
			Workload:    0.10,
		},
	}
}

func NewPolicyEngineV2(cfg PolicyV2Config) *PolicyEngineV2 {
	normalized := normalizePolicyV2Config(cfg)
	return &PolicyEngineV2{cfg: normalized, scoreAdjuster: normalized.ScoreAdjuster}
}

func (e *PolicyEngineV2) Select(snapshot CapabilitySnapshot, input PolicyInput) PolicyDecision {
	baseline := e.cfg.BaselineProviderID
	if !e.cfg.Enabled {
		return PolicyDecision{
			PolicyVersion:       "cpu-baseline-default",
			CapabilityEpoch:     snapshot.Epoch,
			SelectedProvider:    baseline,
			RankedFallbackChain: []string{baseline},
			Confidence:          1,
			ReasonCode:          "feature_disabled",
		}
	}

	candidates := scoreCandidates(snapshot.Providers, input, e.cfg)
	if len(candidates) == 0 {
		return PolicyDecision{
			PolicyVersion:       e.cfg.PolicyVersion,
			CapabilityEpoch:     snapshot.Epoch,
			SelectedProvider:    baseline,
			RankedFallbackChain: []string{baseline},
			Confidence:          1,
			ReasonCode:          "no_eligible_provider",
		}
	}

	pluginFailed := false
	if e.scoreAdjuster != nil {
		adjusted, err := e.applyScoreAdjustments(input, candidates)
		if err == nil {
			candidates = adjusted
		} else {
			pluginFailed = true
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].healthRank != candidates[j].healthRank {
			return candidates[i].healthRank > candidates[j].healthRank
		}
		if candidates[i].reliabilityRank != candidates[j].reliabilityRank {
			return candidates[i].reliabilityRank > candidates[j].reliabilityRank
		}
		return candidates[i].providerID < candidates[j].providerID
	})

	selectedProvider := candidates[0].providerID
	confidence := candidates[0].confidence
	reasonCode := "selected"
	if confidence < e.cfg.MinConfidence {
		selectedProvider = baseline
		confidence = 1
		reasonCode = "low_confidence_baseline"
	} else if pluginFailed {
		reasonCode = "plugin_failed_fallback"
	}

	chain := make([]string, 0, len(candidates)+1)
	seen := make(map[string]struct{}, len(candidates)+1)
	if selectedProvider == baseline {
		chain = append(chain, baseline)
		seen[baseline] = struct{}{}
	}
	for _, c := range candidates {
		if _, ok := seen[c.providerID]; ok {
			continue
		}
		chain = append(chain, c.providerID)
		seen[c.providerID] = struct{}{}
	}
	if _, ok := seen[baseline]; !ok {
		chain = append(chain, baseline)
	}
	if len(chain) == 0 {
		chain = []string{baseline}
	}
	if selectedProvider == "" {
		selectedProvider = chain[0]
	}

	return PolicyDecision{
		PolicyVersion:       e.cfg.PolicyVersion,
		CapabilityEpoch:     snapshot.Epoch,
		SelectedProvider:    selectedProvider,
		RankedFallbackChain: chain,
		Confidence:          confidence,
		ReasonCode:          reasonCode,
	}
}

func (e *PolicyEngineV2) applyScoreAdjustments(input PolicyInput, candidates []scoredCandidate) ([]scoredCandidate, error) {
	if e == nil || e.scoreAdjuster == nil || len(candidates) == 0 {
		return candidates, nil
	}

	adjusted := make([]scoredCandidate, len(candidates))
	copy(adjusted, candidates)
	tags := dedupeAndSort(input.WorkloadTags)
	for i := range adjusted {
		score, err := e.scoreAdjuster.AdjustScore(context.Background(), ScoreAdjustmentInput{
			ProviderID:   adjusted[i].providerID,
			CurrentScore: adjusted[i].score,
			WorkloadTags: tags,
		})
		if err != nil {
			return nil, err
		}
		adjusted[i].score = score
		adjusted[i].confidence = clamp01(score)
	}

	return adjusted, nil
}

func (e *PolicyEngineV2) FallbackOnFailure(decision PolicyDecision, failedProviderID, failureClass string) PolicyDecision {
	if len(decision.RankedFallbackChain) == 0 {
		decision.RankedFallbackChain = []string{e.cfg.BaselineProviderID}
	}
	failedProviderID = strings.TrimSpace(failedProviderID)
	if failedProviderID == "" {
		failedProviderID = decision.SelectedProvider
	}

	next := e.cfg.BaselineProviderID
	for i, id := range decision.RankedFallbackChain {
		if id == failedProviderID {
			if i+1 < len(decision.RankedFallbackChain) {
				next = decision.RankedFallbackChain[i+1]
			}
			break
		}
	}

	if strings.TrimSpace(next) == "" {
		next = e.cfg.BaselineProviderID
	}
	decision.SelectedProvider = next
	decision.Confidence = 1
	failureClass = normalizeFailureClass(failureClass)
	decision.ReasonCode = "fallback_" + failureClass
	return decision
}

type scoredCandidate struct {
	providerID      string
	score           float64
	confidence      float64
	healthRank      int
	reliabilityRank int
}

func scoreCandidates(providers []ProviderCapability, input PolicyInput, cfg PolicyV2Config) []scoredCandidate {
	if len(providers) == 0 {
		return nil
	}
	tags := dedupeAndSort(input.WorkloadTags)
	out := make([]scoredCandidate, 0, len(providers))
	for _, provider := range providers {
		if !isPolicyEligible(provider, tags, cfg.BaselineProviderID) {
			continue
		}
		health := healthScore(provider.HealthState)
		reliability := reliabilityScore(provider.ReliabilityClass)
		cost := costScore(provider.CostClass)
		latency := latencyScore(provider.LatencyClass)
		queue := queueDepthScore(provider.QueueDepth, cfg.MaxQueueDepth)
		workload := workloadScore(tags, provider.SupportedWorkloadTags)

		totalWeight := cfg.Weights.Health + cfg.Weights.Reliability + cfg.Weights.Cost + cfg.Weights.Latency + cfg.Weights.QueueDepth + cfg.Weights.Workload
		if totalWeight <= 0 {
			totalWeight = 1
		}
		score :=
			cfg.Weights.Health*health +
				cfg.Weights.Reliability*reliability +
				cfg.Weights.Cost*cost +
				cfg.Weights.Latency*latency +
				cfg.Weights.QueueDepth*queue +
				cfg.Weights.Workload*workload
		confidence := clamp01(score / totalWeight)
		out = append(out, scoredCandidate{
			providerID:      provider.ProviderID,
			score:           score,
			confidence:      confidence,
			healthRank:      healthRank(provider.HealthState),
			reliabilityRank: reliabilityRank(provider.ReliabilityClass),
		})
	}
	return out
}

func normalizePolicyV2Config(cfg PolicyV2Config) PolicyV2Config {
	def := DefaultPolicyV2Config()
	if strings.TrimSpace(cfg.PolicyVersion) == "" {
		cfg.PolicyVersion = def.PolicyVersion
	}
	if strings.TrimSpace(cfg.BaselineProviderID) == "" {
		cfg.BaselineProviderID = def.BaselineProviderID
	}
	if cfg.MinConfidence < 0 || cfg.MinConfidence > 1 {
		cfg.MinConfidence = def.MinConfidence
	}
	if cfg.MaxQueueDepth <= 0 {
		cfg.MaxQueueDepth = def.MaxQueueDepth
	}
	cfg.Weights = normalizeWeights(cfg.Weights, def.Weights)
	return cfg
}

func normalizeWeights(raw, fallback PolicyWeights) PolicyWeights {
	out := raw
	if out.Health < 0 {
		out.Health = fallback.Health
	}
	if out.Reliability < 0 {
		out.Reliability = fallback.Reliability
	}
	if out.Cost < 0 {
		out.Cost = fallback.Cost
	}
	if out.Latency < 0 {
		out.Latency = fallback.Latency
	}
	if out.QueueDepth < 0 {
		out.QueueDepth = fallback.QueueDepth
	}
	if out.Workload < 0 {
		out.Workload = fallback.Workload
	}
	if out.Health+out.Reliability+out.Cost+out.Latency+out.QueueDepth+out.Workload == 0 {
		out = fallback
	}
	return out
}

func isPolicyEligible(provider ProviderCapability, requiredTags []string, baselineProviderID string) bool {
	if strings.TrimSpace(provider.ProviderID) == "" {
		return false
	}
	if strings.EqualFold(provider.HealthState, HealthUnavailable) {
		return false
	}
	if provider.ProviderID == baselineProviderID {
		return true
	}
	if len(requiredTags) == 0 {
		return true
	}
	supported := make(map[string]struct{}, len(provider.SupportedWorkloadTags))
	for _, tag := range provider.SupportedWorkloadTags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag != "" {
			supported[tag] = struct{}{}
		}
	}
	for _, tag := range requiredTags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		if _, ok := supported[tag]; ok {
			return true
		}
	}
	return false
}

func workloadScore(requiredTags, supportedTags []string) float64 {
	if len(requiredTags) == 0 {
		return 1
	}
	if len(supportedTags) == 0 {
		return 0
	}
	supported := make(map[string]struct{}, len(supportedTags))
	for _, tag := range supportedTags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag != "" {
			supported[tag] = struct{}{}
		}
	}
	matched := 0
	for _, tag := range requiredTags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if _, ok := supported[tag]; ok {
			matched++
		}
	}
	return clamp01(float64(matched) / float64(len(requiredTags)))
}

func queueDepthScore(depth, maxDepth int) float64 {
	if maxDepth <= 0 {
		maxDepth = 1
	}
	if depth <= 0 {
		return 1
	}
	return clamp01(1 - (float64(depth) / float64(maxDepth)))
}

func healthScore(raw string) float64 {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case HealthHealthy:
		return 1
	case HealthDegraded:
		return 0.5
	default:
		return 0
	}
}

func healthRank(raw string) int {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case HealthHealthy:
		return 2
	case HealthDegraded:
		return 1
	default:
		return 0
	}
}

func reliabilityScore(raw string) float64 {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "platinum":
		return 1
	case "gold":
		return 0.9
	case "standard":
		return 0.7
	case "silver":
		return 0.6
	case "bronze":
		return 0.4
	default:
		return 0.5
	}
}

func reliabilityRank(raw string) int {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "platinum":
		return 5
	case "gold":
		return 4
	case "standard":
		return 3
	case "silver":
		return 2
	case "bronze":
		return 1
	default:
		return 0
	}
}

func costScore(raw string) float64 {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "low":
		return 1
	case "medium":
		return 0.7
	case "high":
		return 0.4
	default:
		return 0.6
	}
}

func latencyScore(raw string) float64 {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ultra":
		return 1
	case "fast":
		return 0.9
	case "baseline":
		return 0.7
	case "slow":
		return 0.4
	default:
		return 0.6
	}
}

func normalizeFailureClass(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "unavailable", "capacity_exhausted", "timeout", "contract_incompatible", "quality_guardrail":
		return raw
	default:
		return "provider_failure"
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
