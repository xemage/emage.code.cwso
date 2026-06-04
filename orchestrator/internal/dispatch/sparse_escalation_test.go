package dispatch

import "testing"

func TestQualityGuardrailBreached(t *testing.T) {
	min := 0.98
	below := 0.95
	above := 0.99
	if !QualityGuardrailBreached(&below, min) {
		t.Fatal("expected breach below minimum")
	}
	if QualityGuardrailBreached(&above, min) {
		t.Fatal("expected no breach at or above minimum")
	}
	if QualityGuardrailBreached(nil, min) {
		t.Fatal("expected nil score to skip breach")
	}
}

func TestSelectDenseGPUEscalationUsesGuardrailReason(t *testing.T) {
	base := PolicyV2Config{
		Enabled:            true,
		PolicyVersion:      "policy-v2",
		BaselineProviderID: "cpu-baseline",
		MinConfidence:      0.2,
		MaxQueueDepth:      16,
		Weights:            DefaultPolicyV2Config().Weights,
	}
	engine := EscalationEngineForDenseGPU(base)
	decision := SelectDenseGPUEscalation(engine, testSparseQuantizedSnapshot())
	if decision.ReasonCode != ReasonQualityGuardrailAutodisable {
		t.Fatalf("reason: %+v", decision)
	}
	if decision.SelectedProvider != "gpu-a" {
		t.Fatalf("expected dense GPU gpu-a, got %+v", decision)
	}
}

func TestFirstDenseGPUProviderDeterministic(t *testing.T) {
	id := FirstDenseGPUProvider(testSparseQuantizedSnapshot())
	if id != "gpu-a" {
		t.Fatalf("expected gpu-a, got %q", id)
	}
}
