package rollout

import "testing"

func TestBuildTrajectoryGroup_singleCompletion(t *testing.T) {
	group := BuildTrajectoryGroup("sess-1", []CompletionRecord{
		{
			RequestID:       "a",
			PromptTokenIDs:  []uint32{1, 2, 3},
			SampledTokenIDs: []uint32{4, 5},
			Logprobs:        []float64{-0.1, -0.2},
			TimestampNS:     10,
		},
	})
	if err := ValidateTrajectoryGroup(group); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(group.Chains) != 1 {
		t.Fatalf("chains = %d, want 1", len(group.Chains))
	}
	chain := group.Chains[0]
	if !tokenIDsEqual(chain.PrefixTokenIDs, []uint32{1, 2, 3}) {
		t.Fatalf("prefix = %v", chain.PrefixTokenIDs)
	}
	if len(chain.Steps) != 1 || !tokenIDsEqual(chain.Steps[0].TokenIDs, []uint32{4, 5}) {
		t.Fatalf("steps = %+v", chain.Steps)
	}
}

func TestBuildTrajectoryGroup_prefixMergeAppendOnly(t *testing.T) {
	group := BuildTrajectoryGroup("sess-1", []CompletionRecord{
		{
			RequestID:       "a",
			PromptTokenIDs:  []uint32{1, 2},
			SampledTokenIDs: []uint32{3},
			TimestampNS:     10,
		},
		{
			RequestID:       "c",
			PromptTokenIDs:  []uint32{1, 2, 3},
			SampledTokenIDs: []uint32{4},
			TimestampNS:     30,
		},
	})
	if err := ValidateTrajectoryGroup(group); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(group.Chains) != 1 {
		t.Fatalf("chains = %d, want 1 merged chain", len(group.Chains))
	}
	if len(group.Chains[0].Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(group.Chains[0].Steps))
	}
	wantSuffix := []uint32{1, 2, 3, 4}
	if !tokenIDsEqual(chainSuffix(&group.Chains[0]), wantSuffix) {
		t.Fatalf("suffix = %v, want %v", chainSuffix(&group.Chains[0]), wantSuffix)
	}
}

func TestBuildTrajectoryGroup_parallelBranches(t *testing.T) {
	group := BuildTrajectoryGroup("sess-2", []CompletionRecord{
		{
			RequestID:       "a",
			PromptTokenIDs:  []uint32{9},
			SampledTokenIDs: []uint32{1},
			TimestampNS:     1,
		},
		{
			RequestID:       "b",
			PromptTokenIDs:  []uint32{9},
			SampledTokenIDs: []uint32{2},
			TimestampNS:     2,
		},
	})
	if len(group.Chains) != 2 {
		t.Fatalf("chains = %d, want 2 parallel branches", len(group.Chains))
	}
}

func TestBuildTrajectoryGroup_skipsEmptySampled(t *testing.T) {
	group := BuildTrajectoryGroup("sess", []CompletionRecord{
		{RequestID: "x", PromptTokenIDs: []uint32{1}, TimestampNS: 1},
		{RequestID: "y", PromptTokenIDs: []uint32{1}, SampledTokenIDs: []uint32{2}, TimestampNS: 2},
	})
	if len(group.Chains) != 1 || len(group.Chains[0].Steps) != 1 {
		t.Fatalf("unexpected chains: %+v", group.Chains)
	}
}
