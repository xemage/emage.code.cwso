package rollout

import (
	"fmt"
	"sort"

	"github.com/emage/cwso/orchestrator/internal/config"
)

// BuilderStrategy selects how completion records assemble into trainer traces (Polar §3.4).
type BuilderStrategy string

const (
	StrategyPerRequest  BuilderStrategy = "per_request"
	StrategyPrefixMerge BuilderStrategy = "prefix_merge"
)

// BuilderConfig wires Polar trajectory reconstruction (T149).
type BuilderConfig struct {
	Active           bool
	DefaultStrategy  BuilderStrategy
	InterstitialEOTs bool // mask harness interstitial tokens with loss_mask=0
}

// ResolveStrategy returns the per-task strategy or the configured default.
func (c BuilderConfig) ResolveStrategy(taskStrategy BuilderStrategy) BuilderStrategy {
	if taskStrategy != "" {
		return taskStrategy
	}
	if c.DefaultStrategy != "" {
		return c.DefaultStrategy
	}
	return StrategyPrefixMerge
}

// BuilderConfigFrom loads trajectory builder settings from orchestrator config (T149).
func BuilderConfigFrom(cfg *config.Config) BuilderConfig {
	if cfg == nil {
		return BuilderConfig{}
	}
	strategy := BuilderStrategy(cfg.RolloutTrajectoryBuilderStrategy)
	if strategy == "" {
		strategy = StrategyPrefixMerge
	}
	return BuilderConfig{
		Active:           cfg.RolloutTrajectoryBuilderEnabled,
		DefaultStrategy:  strategy,
		InterstitialEOTs: true,
	}
}

// BuildTrajectory applies the selected builder strategy to completion records.
func BuildTrajectory(sessionID string, records []CompletionRecord, strategy BuilderStrategy, interstitialEOTs bool) TrajectoryGroup {
	if strategy == StrategyPerRequest {
		return buildPerRequest(sessionID, records)
	}
	return buildPrefixMergePolar(sessionID, records, interstitialEOTs)
}

func buildPerRequest(sessionID string, records []CompletionRecord) TrajectoryGroup {
	sorted := sortCompletionRecords(records)
	var chains []Chain
	for _, rec := range sorted {
		if len(rec.SampledTokenIDs) == 0 {
			continue
		}
		chains = append(chains, Chain{
			ChainID:        fmt.Sprintf("chain-%s", rec.RequestID),
			PrefixTokenIDs: append([]uint32(nil), rec.PromptTokenIDs...),
			Steps: []Step{{
				TokenIDs: append([]uint32(nil), rec.SampledTokenIDs...),
				LossMask: lossMaskOnes(len(rec.SampledTokenIDs)),
				Logprobs: append([]float64(nil), rec.Logprobs...),
			}},
		})
	}
	return TrajectoryGroup{
		SessionID: normalizeSessionID(sessionID),
		Chains:    chains,
		Metadata:  map[string]string{"builder": "per_request", "builder_version": "v2"},
	}
}

func buildPrefixMergePolar(sessionID string, records []CompletionRecord, interstitialEOTs bool) TrajectoryGroup {
	sorted := sortCompletionRecords(records)
	var chains []Chain
	for _, rec := range sorted {
		if len(rec.SampledTokenIDs) == 0 {
			continue
		}
		step := assistantStep(rec)
		if idx := findPolarExtendableChain(chains, rec, interstitialEOTs); idx >= 0 {
			if interstitialEOTs {
				if interstitial := interstitialTokens(&chains[idx], rec.PromptTokenIDs); len(interstitial) > 0 {
					chains[idx].Steps = append(chains[idx].Steps, interstitialStep(interstitial))
				}
			}
			chains[idx].Steps = append(chains[idx].Steps, step)
			continue
		}
		chains = append(chains, Chain{
			ChainID:        fmt.Sprintf("chain-%s", rec.RequestID),
			PrefixTokenIDs: append([]uint32(nil), rec.PromptTokenIDs...),
			Steps:          []Step{step},
			Metadata:       chainMetadataFromRecord(rec),
		})
	}
	return TrajectoryGroup{
		SessionID: normalizeSessionID(sessionID),
		Chains:    chains,
		Metadata:  map[string]string{"builder": "prefix_merge", "builder_version": "v2"},
	}
}

func sortCompletionRecords(records []CompletionRecord) []CompletionRecord {
	sorted := append([]CompletionRecord(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TimestampNS != sorted[j].TimestampNS {
			return sorted[i].TimestampNS < sorted[j].TimestampNS
		}
		return sorted[i].RequestID < sorted[j].RequestID
	})
	return sorted
}

func chainMetadataFromRecord(rec CompletionRecord) map[string]string {
	if rec.PartitionKey == "" {
		return nil
	}
	return map[string]string{"partition_key": rec.PartitionKey}
}

func normalizeSessionID(sessionID string) string {
	if sessionID == "" {
		return "default"
	}
	return sessionID
}

func findPolarExtendableChain(chains []Chain, rec CompletionRecord, interstitialEOTs bool) int {
	for i := range chains {
		if !chainPartitionCompatible(&chains[i], rec) {
			continue
		}
		suffix := chainSuffix(&chains[i])
		if !tokenIDsHasPrefix(rec.PromptTokenIDs, suffix) {
			continue
		}
		if interstitialEOTs && hasEOTInterstitialBoundary(suffix, rec.PromptTokenIDs) {
			continue
		}
		return i
	}
	return -1
}

func chainPartitionCompatible(chain *Chain, rec CompletionRecord) bool {
	if rec.PartitionKey == "" {
		return true
	}
	chainPartition := chainPartitionKey(chain)
	return chainPartition == "" || chainPartition == rec.PartitionKey
}

func chainPartitionKey(chain *Chain) string {
	if chain.Metadata == nil {
		return ""
	}
	return chain.Metadata["partition_key"]
}

func assistantStep(rec CompletionRecord) Step {
	return Step{
		TokenIDs:       append([]uint32(nil), rec.SampledTokenIDs...),
		LossMask:       lossMaskOnes(len(rec.SampledTokenIDs)),
		Logprobs:       append([]float64(nil), rec.Logprobs...),
		MessageGroupID: rec.MessageGroupID,
	}
}

func interstitialTokens(chain *Chain, prompt []uint32) []uint32 {
	suffix := chainSuffix(chain)
	if !tokenIDsHasPrefix(prompt, suffix) {
		return nil
	}
	return append([]uint32(nil), prompt[len(suffix):]...)
}

func interstitialStep(tokens []uint32) Step {
	return Step{
		TokenIDs: tokens,
		LossMask: lossMaskZeros(len(tokens)),
		Logprobs: make([]float64, len(tokens)),
	}
}

// hasEOTInterstitialBoundary detects canonical end-of-turn markers in interstitial tokens.
func hasEOTInterstitialBoundary(chainSuffix, prompt []uint32) bool {
	if !tokenIDsHasPrefix(prompt, chainSuffix) {
		return false
	}
	interstitial := prompt[len(chainSuffix):]
	for _, token := range interstitial {
		if token == eotInterstitialTokenID {
			return true
		}
	}
	return false
}

const eotInterstitialTokenID uint32 = 0xE07

func lossMaskZeros(n int) []uint8 {
	mask := make([]uint8, n)
	return mask
}

func tokenIDsHasPrefix(prompt, prefix []uint32) bool {
	if len(prompt) < len(prefix) {
		return false
	}
	for i := range prefix {
		if prompt[i] != prefix[i] {
			return false
		}
	}
	return true
}

// trainerSampleCount returns trainer-facing trace count (one chain = one sample).
func trainerSampleCount(group TrajectoryGroup) int {
	return len(group.Chains)
}
