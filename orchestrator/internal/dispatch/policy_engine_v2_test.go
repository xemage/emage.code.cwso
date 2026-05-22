package dispatch

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPolicyEngineV2DeterministicSelectionForIdenticalInputs(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights: PolicyWeights{
			Health:      0.35,
			Reliability: 0.25,
			Cost:        0.10,
			Latency:     0.10,
			QueueDepth:  0.10,
			Workload:    0.10,
		},
	})

	snapshot := CapabilitySnapshot{
		Epoch: 12,
		Providers: []ProviderCapability{
			{
				ProviderID:            "gpu-b",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "fast",
				CostClass:             "medium",
				QueueDepth:            3,
				SupportedWorkloadTags: []string{"default", "merge-assist"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID:            "gpu-a",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "fast",
				CostClass:             "medium",
				QueueDepth:            3,
				SupportedWorkloadTags: []string{"default", "merge-assist"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID:            "cpu-baseline",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default"},
				ReliabilityClass:      "standard",
			},
		},
	}

	input := PolicyInput{WorkloadTags: []string{"merge-assist"}}
	first := engine.Select(snapshot, input)
	second := engine.Select(snapshot, input)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected identical selection for identical inputs, got first=%+v second=%+v", first, second)
	}
	if len(first.RankedFallbackChain) < 2 {
		t.Fatalf("expected sorted fallback chain, got %+v", first.RankedFallbackChain)
	}
}

func TestPolicyEngineV2FallbackToBaselineOnUnavailableAndFailure(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
	})

	unavailableSnapshot := CapabilitySnapshot{
		Epoch: 22,
		Providers: []ProviderCapability{
			{
				ProviderID:            "gpu-a",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthUnavailable,
				LatencyClass:          "fast",
				CostClass:             "medium",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID:            "cpu-baseline",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default"},
				ReliabilityClass:      "standard",
			},
		},
	}

	unavailableDecision := engine.Select(unavailableSnapshot, PolicyInput{WorkloadTags: []string{"default"}})
	if unavailableDecision.SelectedProvider != "cpu-baseline" {
		t.Fatalf("expected unavailable provider to route to baseline, got %+v", unavailableDecision)
	}

	failureSnapshot := CapabilitySnapshot{
		Epoch: 23,
		Providers: []ProviderCapability{
			{
				ProviderID:            "gpu-a",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "fast",
				CostClass:             "medium",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID:            "cpu-baseline",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default"},
				ReliabilityClass:      "standard",
			},
		},
	}

	decision := engine.Select(failureSnapshot, PolicyInput{WorkloadTags: []string{"default"}})
	if decision.SelectedProvider != "gpu-a" {
		t.Fatalf("expected gpu-a primary selection, got %+v", decision)
	}

	fallback := engine.FallbackOnFailure(decision, "gpu-a", "unavailable")
	if fallback.SelectedProvider != "cpu-baseline" {
		t.Fatalf("expected fallback to cpu-baseline on provider failure, got %+v", fallback)
	}
}

func TestPolicyEngineV2FeatureDisabledKeepsBaselinePath(t *testing.T) {
	engine := NewPolicyEngineV2(DefaultPolicyV2Config())
	decision := engine.Select(CapabilitySnapshot{Epoch: 7}, PolicyInput{WorkloadTags: []string{"default"}})

	if decision.PolicyVersion != "cpu-baseline-default" {
		t.Fatalf("expected baseline policy version when disabled, got %+v", decision)
	}
	if decision.SelectedProvider != "cpu-baseline" {
		t.Fatalf("expected baseline provider when disabled, got %+v", decision)
	}
	if len(decision.RankedFallbackChain) != 1 || decision.RankedFallbackChain[0] != "cpu-baseline" {
		t.Fatalf("expected baseline-only chain when disabled, got %+v", decision.RankedFallbackChain)
	}
}

func TestPolicyEngineV2PluginDisabledKeepsBaselineScoringOrder(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
	})

	decision := engine.Select(testPluginSnapshot(), PolicyInput{WorkloadTags: []string{"default"}})
	if decision.SelectedProvider != "gpu-a" {
		t.Fatalf("expected baseline scoring order to select gpu-a, got %+v", decision)
	}
}

func TestPolicyEngineV2PluginEnabledAdjustsSelection(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		ScoreAdjuster: stubScoreAdjuster{adjust: func(_ context.Context, in ScoreAdjustmentInput) (float64, error) {
			if in.ProviderID == "cpu-baseline" {
				return 1, nil
			}
			return 0.2, nil
		}},
	})

	decision := engine.Select(testPluginSnapshot(), PolicyInput{WorkloadTags: []string{"default"}})
	if decision.SelectedProvider != "cpu-baseline" {
		t.Fatalf("expected plugin-adjusted scoring to select cpu-baseline, got %+v", decision)
	}
}

func TestPolicyEngineV2PluginFailureFallsBackSafely(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		ScoreAdjuster: stubScoreAdjuster{adjust: func(context.Context, ScoreAdjustmentInput) (float64, error) {
			return 0, errors.New("plugin execution failed")
		}},
	})

	decision := engine.Select(testPluginSnapshot(), PolicyInput{WorkloadTags: []string{"default"}})
	if decision.SelectedProvider != "gpu-a" {
		t.Fatalf("expected safe fallback to baseline scoring result, got %+v", decision)
	}
	if decision.ReasonCode != "plugin_failed_fallback" {
		t.Fatalf("expected plugin_failed_fallback reason code, got %+v", decision)
	}
}

func testPluginSnapshot() CapabilitySnapshot {
	return CapabilitySnapshot{
		Epoch: 99,
		Providers: []ProviderCapability{
			{
				ProviderID:            "gpu-a",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "fast",
				CostClass:             "medium",
				QueueDepth:            1,
				SupportedWorkloadTags: []string{"default"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID:            "cpu-baseline",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default"},
				ReliabilityClass:      "standard",
			},
		},
	}
}

type stubScoreAdjuster struct {
	adjust func(context.Context, ScoreAdjustmentInput) (float64, error)
}

func (s stubScoreAdjuster) AdjustScore(ctx context.Context, input ScoreAdjustmentInput) (float64, error) {
	if s.adjust == nil {
		return input.CurrentScore, nil
	}
	return s.adjust(ctx, input)
}
