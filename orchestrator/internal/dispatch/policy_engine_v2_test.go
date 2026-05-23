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

func TestPolicyEngineV2SparseQuantizedDisabledPreservesBaselineSelection(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		SparseQuantized: SparseQuantizedAssistConfig{
			Enabled:             false,
			CostLatencyTradeoff: 0.8,
		},
	})

	decision := engine.Select(testSparseQuantizedSnapshot(), PolicyInput{WorkloadTags: []string{"merge-assist"}})
	if decision.SelectedProvider != "gpu-a" {
		t.Fatalf("expected baseline selection to remain gpu-a when sparse/quantized assist is disabled, got %+v", decision)
	}
}

func TestPolicyEngineV2SparseQuantizedEnabledAdjustsDecisionPath(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		SparseQuantized: SparseQuantizedAssistConfig{
			Enabled:                  true,
			ProviderFeatureFlag:      "hhd.sparse_quantized_assist",
			CostLatencyTradeoff:      0.8,
			QualityGuardrailMinScore: 0.98,
		},
	})

	decision := engine.Select(testSparseQuantizedSnapshot(), PolicyInput{WorkloadTags: []string{"merge-assist"}})
	if decision.SelectedProvider != "gpu-sq" {
		t.Fatalf("expected sparse/quantized assist to bias selection to gpu-sq, got %+v", decision)
	}
	if decision.ReasonCode != "sparse_quantized_assist_selected" {
		t.Fatalf("expected sparse_quantized_assist_selected reason, got %+v", decision)
	}
}

func TestPolicyEngineV2SparseQuantizedQualityBreachAutoDisablesPath(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		SparseQuantized: SparseQuantizedAssistConfig{
			Enabled:                  true,
			ProviderFeatureFlag:      "hhd.sparse_quantized_assist",
			CostLatencyTradeoff:      0.8,
			QualityGuardrailMinScore: 0.98,
		},
	})

	lowQuality := 0.95
	breach := engine.Select(testSparseQuantizedSnapshot(), PolicyInput{
		WorkloadTags: []string{"merge-assist"},
		QualityScore: &lowQuality,
	})
	if breach.SelectedProvider != "cpu-baseline" {
		t.Fatalf("expected quality breach to fallback to baseline, got %+v", breach)
	}
	if breach.ReasonCode != "quality_guardrail_autodisable" {
		t.Fatalf("expected quality_guardrail_autodisable reason, got %+v", breach)
	}

	highQuality := 0.99
	afterDisable := engine.Select(testSparseQuantizedSnapshot(), PolicyInput{
		WorkloadTags: []string{"merge-assist"},
		QualityScore: &highQuality,
	})
	if afterDisable.SelectedProvider != "gpu-a" {
		t.Fatalf("expected auto-disabled assist to keep baseline policy selection gpu-a, got %+v", afterDisable)
	}
	if afterDisable.ReasonCode != "sparse_quantized_auto_disabled" {
		t.Fatalf("expected sparse_quantized_auto_disabled reason, got %+v", afterDisable)
	}
}

func TestPolicyEngineV2SSMAssistDisabledPreservesBaselineSelection(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		SSM: SSMAssistConfig{
			Enabled:             false,
			ProviderFeatureFlag: "hhd.ssm_sequence_assist",
			ThroughputBias:      0.8,
			MinSequenceLength:   2048,
			MaxSequenceLength:   32768,
			SequenceSensitivity: 1,
		},
	})

	decision := engine.Select(testSSMSequenceAssistSnapshot(), PolicyInput{
		WorkloadTags:  []string{"long-context"},
		RequestLabels: []string{"sequence_length=16384"},
	})
	if decision.SelectedProvider != "gpu-a" {
		t.Fatalf("expected baseline policy scoring to remain gpu-a when SSM assist is disabled, got %+v", decision)
	}
}

func TestPolicyEngineV2SSMAssistEnabledAdjustsLongContextSelection(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		SSM: SSMAssistConfig{
			Enabled:             true,
			ProviderFeatureFlag: "hhd.ssm_sequence_assist",
			ThroughputBias:      0.8,
			MinSequenceLength:   2048,
			MaxSequenceLength:   32768,
			SequenceSensitivity: 1,
		},
	})

	decision := engine.Select(testSSMSequenceAssistSnapshot(), PolicyInput{
		WorkloadTags:  []string{"long-context"},
		RequestLabels: []string{"sequence_length=16384"},
	})
	if decision.SelectedProvider != "gpu-ssm" {
		t.Fatalf("expected SSM assist to bias long-context selection to gpu-ssm, got %+v", decision)
	}
	if decision.ReasonCode != "ssm_assist_selected" {
		t.Fatalf("expected ssm_assist_selected reason, got %+v", decision)
	}
}

func TestPolicyEngineV2SSMAssistGuardrailFallbackOnInvalidSignal(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		SSM: SSMAssistConfig{
			Enabled:             true,
			ProviderFeatureFlag: "hhd.ssm_sequence_assist",
			ThroughputBias:      0.8,
			MinSequenceLength:   2048,
			MaxSequenceLength:   32768,
			SequenceSensitivity: 1,
		},
	})

	decision := engine.Select(testSSMSequenceAssistSnapshot(), PolicyInput{
		WorkloadTags:  []string{"long-context"},
		RequestLabels: []string{"sequence_length=invalid"},
	})
	if decision.SelectedProvider != "cpu-baseline" {
		t.Fatalf("expected invalid sequence signal to fallback to baseline provider, got %+v", decision)
	}
	if decision.ReasonCode != "ssm_signal_invalid_fallback" {
		t.Fatalf("expected ssm_signal_invalid_fallback reason, got %+v", decision)
	}
}

func TestPolicyEngineV2SSMAssistGuardrailFallbackOnOutOfThresholdSignal(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		SSM: SSMAssistConfig{
			Enabled:             true,
			ProviderFeatureFlag: "hhd.ssm_sequence_assist",
			ThroughputBias:      0.8,
			MinSequenceLength:   2048,
			MaxSequenceLength:   32768,
			SequenceSensitivity: 1,
		},
	})

	decision := engine.Select(testSSMSequenceAssistSnapshot(), PolicyInput{
		WorkloadTags:  []string{"long-context"},
		RequestLabels: []string{"sequence_length=64000"},
	})
	if decision.SelectedProvider != "cpu-baseline" {
		t.Fatalf("expected out-of-threshold sequence signal to fallback to baseline provider, got %+v", decision)
	}
	if decision.ReasonCode != "ssm_signal_out_of_threshold_fallback" {
		t.Fatalf("expected ssm_signal_out_of_threshold_fallback reason, got %+v", decision)
	}
}

func TestPolicyEngineV2MixedBackendForcedFallbackWalksDeterministicChain(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		SparseQuantized: SparseQuantizedAssistConfig{
			Enabled:                  true,
			ProviderFeatureFlag:      "hhd.sparse_quantized_assist",
			CostLatencyTradeoff:      0.4,
			QualityGuardrailMinScore: 0.98,
		},
		SSM: SSMAssistConfig{
			Enabled:             true,
			ProviderFeatureFlag: "hhd.ssm_sequence_assist",
			ThroughputBias:      0.7,
			MinSequenceLength:   2048,
			MaxSequenceLength:   32768,
			SequenceSensitivity: 1,
		},
	})

	snapshot := CapabilitySnapshot{
		Epoch: 140,
		Providers: []ProviderCapability{
			{
				ProviderID:            "gpu-a",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "ultra",
				CostClass:             "high",
				QueueDepth:            1,
				SupportedWorkloadTags: []string{"default", "merge-assist", "long-context"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID:            "gpu-sq",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            2,
				SupportedWorkloadTags: []string{"default", "merge-assist", "long-context"},
				ReliabilityClass:      "standard",
				FeatureFlags:          []string{"hhd.sparse_quantized_assist"},
			},
			{
				ProviderID:            "gpu-ssm",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            8,
				SupportedWorkloadTags: []string{"default", "merge-assist", "long-context"},
				ReliabilityClass:      "gold",
				FeatureFlags:          []string{"hhd.ssm_sequence_assist"},
			},
			{
				ProviderID:            "cpu-baseline",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default", "merge-assist", "long-context"},
				ReliabilityClass:      "standard",
			},
		},
	}

	decision := engine.Select(snapshot, PolicyInput{
		WorkloadTags:  []string{"long-context", "merge-assist"},
		RequestLabels: []string{"sequence_length=16384"},
	})
	if decision.SelectedProvider == "" {
		t.Fatalf("expected selected provider in mixed-backend scenario, got %+v", decision)
	}
	if len(decision.RankedFallbackChain) < 2 {
		t.Fatalf("expected at least one fallback candidate, got %+v", decision)
	}
	if decision.RankedFallbackChain[len(decision.RankedFallbackChain)-1] != "cpu-baseline" {
		t.Fatalf("expected baseline provider to terminate fallback chain, got %+v", decision.RankedFallbackChain)
	}

	walk := decision
	for i := 0; i < len(decision.RankedFallbackChain)-1; i++ {
		failed := walk.SelectedProvider
		if failed == "" {
			failed = decision.RankedFallbackChain[i]
		}
		walk = engine.FallbackOnFailure(walk, failed, "unavailable")
		expectedNext := decision.RankedFallbackChain[i+1]
		if walk.SelectedProvider != expectedNext {
			t.Fatalf("expected deterministic fallback step %d -> %s, got %+v", i+1, expectedNext, walk)
		}
		if walk.ReasonCode != "fallback_unavailable" {
			t.Fatalf("expected normalized failure class reason, got %+v", walk)
		}
	}
}

func TestPolicyEngineV2FaultInjectedScoreAdjusterRemainsDeterministicAcrossRepeats(t *testing.T) {
	engine := NewPolicyEngineV2(PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
		ScoreAdjuster: stubScoreAdjuster{adjust: func(context.Context, ScoreAdjustmentInput) (float64, error) {
			return 0, errors.New("injected scorer failure")
		}},
	})

	input := PolicyInput{WorkloadTags: []string{"default"}}
	first := engine.Select(testPluginSnapshot(), input)
	if first.ReasonCode != "plugin_failed_fallback" {
		t.Fatalf("expected plugin_failed_fallback under injected failure, got %+v", first)
	}

	for i := 0; i < 100; i++ {
		next := engine.Select(testPluginSnapshot(), input)
		if !reflect.DeepEqual(first, next) {
			t.Fatalf("expected deterministic decision under fault injection at iteration %d, first=%+v next=%+v", i, first, next)
		}
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

func testSparseQuantizedSnapshot() CapabilitySnapshot {
	return CapabilitySnapshot{
		Epoch: 101,
		Providers: []ProviderCapability{
			{
				ProviderID:            "gpu-a",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "fast",
				CostClass:             "high",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default", "merge-assist"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID:            "gpu-sq",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "slow",
				CostClass:             "low",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default", "merge-assist"},
				ReliabilityClass:      "standard",
				FeatureFlags:          []string{"hhd.sparse_quantized_assist"},
			},
			{
				ProviderID:            "cpu-baseline",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default", "merge-assist"},
				ReliabilityClass:      "standard",
			},
		},
	}
}

func testSSMSequenceAssistSnapshot() CapabilitySnapshot {
	return CapabilitySnapshot{
		Epoch: 102,
		Providers: []ProviderCapability{
			{
				ProviderID:            "gpu-a",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "ultra",
				CostClass:             "high",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default", "long-context"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID:            "gpu-ssm",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            12,
				SupportedWorkloadTags: []string{"default", "long-context"},
				ReliabilityClass:      "gold",
				FeatureFlags:          []string{"hhd.ssm_sequence_assist"},
			},
			{
				ProviderID:            "cpu-baseline",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default", "long-context"},
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
