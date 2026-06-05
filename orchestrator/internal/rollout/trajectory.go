package rollout

import (
	"fmt"
	"sort"
)

// BuildTrajectoryGroup applies prefix merging (rollout-architecture-v1 §4, Polar §3.4.2)
// to an ordered completion stream for one session.
func BuildTrajectoryGroup(sessionID string, records []CompletionRecord) TrajectoryGroup {
	if sessionID == "" {
		sessionID = "default"
	}
	sorted := append([]CompletionRecord(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TimestampNS != sorted[j].TimestampNS {
			return sorted[i].TimestampNS < sorted[j].TimestampNS
		}
		return sorted[i].RequestID < sorted[j].RequestID
	})

	var chains []Chain
	for _, rec := range sorted {
		if len(rec.SampledTokenIDs) == 0 {
			continue
		}
		step := Step{
			TokenIDs: append([]uint32(nil), rec.SampledTokenIDs...),
			LossMask: lossMaskOnes(len(rec.SampledTokenIDs)),
			Logprobs: append([]float64(nil), rec.Logprobs...),
		}
		prompt := append([]uint32(nil), rec.PromptTokenIDs...)
		if idx := findExtendableChain(chains, prompt); idx >= 0 {
			chains[idx].Steps = append(chains[idx].Steps, step)
			continue
		}
		chains = append(chains, Chain{
			ChainID:        fmt.Sprintf("chain-%s", rec.RequestID),
			PrefixTokenIDs: prompt,
			Steps:          []Step{step},
		})
	}

	return TrajectoryGroup{
		SessionID: sessionID,
		Chains:    chains,
		Rewards:   nil,
		Metadata:  map[string]string{"builder": "v1"},
	}
}

func findExtendableChain(chains []Chain, prompt []uint32) int {
	for i := range chains {
		if tokenIDsEqual(prompt, chainSuffix(&chains[i])) {
			return i
		}
	}
	return -1
}

func chainSuffix(chain *Chain) []uint32 {
	n := len(chain.PrefixTokenIDs)
	for _, step := range chain.Steps {
		n += len(step.TokenIDs)
	}
	out := make([]uint32, 0, n)
	out = append(out, chain.PrefixTokenIDs...)
	for _, step := range chain.Steps {
		out = append(out, step.TokenIDs...)
	}
	return out
}

func tokenIDsEqual(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func lossMaskOnes(n int) []uint8 {
	mask := make([]uint8, n)
	for i := range mask {
		mask[i] = 1
	}
	return mask
}

// ValidateTrajectoryGroup checks structural invariants of a built group.
func ValidateTrajectoryGroup(group TrajectoryGroup) error {
	if group.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	for ci, chain := range group.Chains {
		if chain.ChainID == "" {
			return fmt.Errorf("chain[%d]: chain_id is required", ci)
		}
		cursor := append([]uint32(nil), chain.PrefixTokenIDs...)
		for si, step := range chain.Steps {
			if len(step.LossMask) != len(step.TokenIDs) {
				return fmt.Errorf("chain[%d] step[%d]: loss_mask length mismatch", ci, si)
			}
			for _, m := range step.LossMask {
				if m != 1 {
					return fmt.Errorf("chain[%d] step[%d]: assistant tokens require loss_mask=1", ci, si)
				}
			}
			if si > 0 && !tokenIDsEqual(cursor, chainSuffixPrefix(&chain, si)) {
				return fmt.Errorf("chain[%d] step[%d]: non-appendable prefix", ci, si)
			}
			cursor = append(cursor, step.TokenIDs...)
		}
	}
	return nil
}

func chainSuffixPrefix(chain *Chain, stepIndex int) []uint32 {
	out := append([]uint32(nil), chain.PrefixTokenIDs...)
	for i := 0; i < stepIndex; i++ {
		out = append(out, chain.Steps[i].TokenIDs...)
	}
	return out
}
