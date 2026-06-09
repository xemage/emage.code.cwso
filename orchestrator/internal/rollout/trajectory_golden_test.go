package rollout

import "testing"

// polarFig4Fixture models Polar Fig. 4: three main-agent turns, compaction, subagent.
func polarFig4Fixture() []CompletionRecord {
	const main = "main"
	return []CompletionRecord{
		{
			RequestID: "turn-1", TimestampNS: 10, PartitionKey: main, MessageGroupID: "msg-1",
			PromptTokenIDs: []uint32{1, 2, 10}, SampledTokenIDs: []uint32{100}, Logprobs: []float64{-0.1},
		},
		{
			RequestID: "turn-2", TimestampNS: 20, PartitionKey: main, MessageGroupID: "msg-2",
			PromptTokenIDs: []uint32{1, 2, 10, 100, 11}, SampledTokenIDs: []uint32{101}, Logprobs: []float64{-0.2},
		},
		{
			RequestID: "turn-3", TimestampNS: 30, PartitionKey: main, MessageGroupID: "msg-3",
			PromptTokenIDs: []uint32{1, 2, 10, 100, 11, 101, 12}, SampledTokenIDs: []uint32{102},
			Logprobs: []float64{-0.3},
		},
		{
			RequestID: "compact-1", TimestampNS: 40, PartitionKey: "main-compacted", MessageGroupID: "msg-compact",
			PromptTokenIDs: []uint32{1, 2, 50}, SampledTokenIDs: []uint32{200}, Logprobs: []float64{-0.4},
		},
		{
			RequestID: "sub-1", TimestampNS: 50, PartitionKey: "subagent-1", MessageGroupID: "msg-sub",
			PromptTokenIDs: []uint32{1, 2, 300}, SampledTokenIDs: []uint32{301}, Logprobs: []float64{-0.5},
		},
	}
}

func TestGoldenPolarFig4_perRequestFragmentation(t *testing.T) {
	records := polarFig4Fixture()
	group := BuildTrajectory("fig4", records, StrategyPerRequest, true)
	if err := ValidateTrajectoryGroup(group); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(group.Chains) != 5 {
		t.Fatalf("per_request chains = %d, want 5 independent traces", len(group.Chains))
	}
	if got := trainerSampleCount(group); got != 5 {
		t.Fatalf("per_request traces = %d, want 5", got)
	}
	if group.Metadata["builder"] != "per_request" {
		t.Fatalf("metadata: %+v", group.Metadata)
	}
}

func TestGoldenPolarFig4_prefixMergePartitionsChains(t *testing.T) {
	records := polarFig4Fixture()
	group := BuildTrajectory("fig4", records, StrategyPrefixMerge, true)
	if err := ValidateTrajectoryGroup(group); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(group.Chains) != 3 {
		t.Fatalf("prefix_merge chains = %d, want 3 (main + compaction + subagent)", len(group.Chains))
	}
	main := group.Chains[0]
	if len(main.Steps) != 5 {
		t.Fatalf("main chain steps = %d, want 3 assistant + 2 interstitial", len(main.Steps))
	}
	if main.Steps[1].LossMask[0] != 0 || main.Steps[3].LossMask[0] != 0 {
		t.Fatalf("expected masked interstitials: %+v", main.Steps)
	}
	if main.Steps[0].LossMask[0] != 1 || main.Steps[2].LossMask[0] != 1 || main.Steps[4].LossMask[0] != 1 {
		t.Fatalf("expected trainable assistant steps: %+v", main.Steps)
	}
}

func TestGoldenPolarFig4_prefixMergeReducesTrainerSamples(t *testing.T) {
	records := polarFig4Fixture()
	perReq := BuildTrajectory("fig4", records, StrategyPerRequest, true)
	merged := BuildTrajectory("fig4", records, StrategyPrefixMerge, true)
	if trainerSampleCount(merged) >= trainerSampleCount(perReq) {
		t.Fatalf("prefix_merge traces %d must be < per_request %d",
			trainerSampleCount(merged), trainerSampleCount(perReq))
	}
}

func TestGoldenPolarFig4_eotInterstitialForcesPartition(t *testing.T) {
	records := []CompletionRecord{
		{
			RequestID: "a", TimestampNS: 1, PartitionKey: "main",
			PromptTokenIDs: []uint32{1, 2}, SampledTokenIDs: []uint32{10},
		},
		{
			RequestID: "b", TimestampNS: 2, PartitionKey: "main",
			PromptTokenIDs: []uint32{1, 2, 10, eotInterstitialTokenID, 20}, SampledTokenIDs: []uint32{30},
		},
	}
	group := BuildTrajectory("eot", records, StrategyPrefixMerge, true)
	if len(group.Chains) != 2 {
		t.Fatalf("chains = %d, want 2 after EOT boundary", len(group.Chains))
	}
}

func TestGoldenPolarFig4_messageGroupStepsTagged(t *testing.T) {
	records := []CompletionRecord{
		{
			RequestID: "a", TimestampNS: 1, PartitionKey: "p1", MessageGroupID: "g1",
			PromptTokenIDs: []uint32{1}, SampledTokenIDs: []uint32{2},
		},
		{
			RequestID: "b", TimestampNS: 2, PartitionKey: "p1", MessageGroupID: "g2",
			PromptTokenIDs: []uint32{1, 2, 3}, SampledTokenIDs: []uint32{4},
		},
	}
	group := BuildTrajectory("groups", records, StrategyPrefixMerge, true)
	if len(group.Chains) != 1 {
		t.Fatalf("chains = %d, want 1 merged chain for same partition", len(group.Chains))
	}
	steps := group.Chains[0].Steps
	if len(steps) != 3 || steps[0].MessageGroupID != "g1" || steps[2].MessageGroupID != "g2" {
		t.Fatalf("message groups not tagged on assistant steps: %+v", steps)
	}
}

func TestAssembleTrajectoryGroup_featureFlagOffUsesV1(t *testing.T) {
	records := polarFig4Fixture()
	group := assembleTrajectoryGroup("sess", records, BuilderConfig{Active: false}, "")
	if group.Metadata["builder"] != "v1" {
		t.Fatalf("expected v1 builder, got %+v", group.Metadata)
	}
}

func TestAssembleTrajectoryGroup_featureFlagOnUsesTaskStrategy(t *testing.T) {
	records := polarFig4Fixture()
	cfg := BuilderConfig{Active: true, DefaultStrategy: StrategyPrefixMerge, InterstitialEOTs: true}
	group := assembleTrajectoryGroup("sess", records, cfg, StrategyPerRequest)
	if group.Metadata["builder"] != "per_request" {
		t.Fatalf("expected per_request override, got %+v", group.Metadata)
	}
}
