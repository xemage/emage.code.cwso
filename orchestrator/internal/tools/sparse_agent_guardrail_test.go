package tools

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/hal"
	"github.com/emage/cwso/orchestrator/internal/jobs"
	"github.com/emage/cwso/orchestrator/internal/sparse"
)

type stubCapabilitySnapshot struct {
	snap dispatch.CapabilitySnapshot
}

func (s *stubCapabilitySnapshot) Snapshot() dispatch.CapabilitySnapshot { return s.snap }

type stubHALInferrer struct {
	called atomic.Bool
}

func (s *stubHALInferrer) Infer(_ context.Context, providerID string, _ []string, _ hal.InferenceRequest) (*hal.InferResult, error) {
	s.called.Store(true)
	return &hal.InferResult{
		ServedBy:   providerID,
		Completion: hal.Completion{Output: "dense-ok", TokensOut: 3, Deterministic: true},
	}, nil
}

func TestSparseAgentGuardrailEscalatesWithoutCreatingAgent(t *testing.T) {
	snap := testSparseQuantizedSnapshotForTools()
	guardrail := &SparseAgentGuardrail{
		Enabled: true, MinScore: 0.98,
		Policy:    enabledPolicyV2Config(),
		Snapshots: &stubCapabilitySnapshot{snap: snap},
	}
	tool := NewCreateEphemeralSparseAgent(sparse.NewClient("/nonexistent"), dispatch.NewSparseAgentRegistry(), nil, 512).
		WithSparseQualityGuardrail(guardrail)

	res, err := tool.Execute(context.Background(), json.RawMessage(
		`{"skill_domain":"react-hooks","quality_floor":0.95,"task_description":"fix types"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %s", res.Content[0].Text)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &out); err != nil {
		t.Fatal(err)
	}
	if out["escalated"] != true {
		t.Fatalf("expected escalation, got %+v", out)
	}
	if out["reason_code"] != dispatch.ReasonQualityGuardrailAutodisable {
		t.Fatalf("reason: %+v", out)
	}
	if out["wasm_agent_id"] != nil {
		t.Fatalf("sparse agent should not be created: %+v", out)
	}
	if out["selected_provider"] != "gpu-a" {
		t.Fatalf("provider: %+v", out)
	}
}

func TestSparseAgentGuardrailEscalationEnqueuesHALJob(t *testing.T) {
	snap := testSparseQuantizedSnapshotForTools()
	mgr, err := jobs.NewManager(jobs.Config{Workers: 1, QueueSize: 4}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	halStub := &stubHALInferrer{}
	guardrail := &SparseAgentGuardrail{
		Enabled: true, MinScore: 0.98,
		Policy:    enabledPolicyV2Config(),
		Snapshots: &stubCapabilitySnapshot{snap: snap},
		Jobs:      mgr,
		HAL:       halStub,
	}
	floor := 0.95
	out, err := guardrail.escalate(context.Background(), &floor, "lint-fix")
	if err != nil {
		t.Fatal(err)
	}
	if out["job_id"] == nil || out["job_id"] == "" {
		t.Fatalf("expected job_id: %+v", out)
	}
	jobID, ok := out["job_id"].(string)
	if !ok || jobID == "" {
		t.Fatalf("expected string job_id: %+v", out["job_id"])
	}
	waitSparseGuardrailJob(t, mgr, jobID, 2*time.Second)
	if !halStub.called.Load() {
		t.Fatal("expected HAL infer on escalation job")
	}
}

func waitSparseGuardrailJob(t *testing.T, mgr *jobs.Manager, id string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, ok := mgr.Get(id)
		if ok && job.State == jobs.StateCompleted {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	job, _ := mgr.Get(id)
	t.Fatalf("job %s did not complete, last=%q", id, job.State)
}

func enabledPolicyV2Config() dispatch.PolicyV2Config {
	cfg := dispatch.DefaultPolicyV2Config()
	cfg.Enabled = true
	cfg.MinConfidence = 0.2
	return cfg
}

func testSparseQuantizedSnapshotForTools() dispatch.CapabilitySnapshot {
	return dispatch.CapabilitySnapshot{
		Epoch: 7,
		Providers: []dispatch.ProviderCapability{
			{
				ProviderID: "gpu-a", HealthState: dispatch.HealthHealthy,
				LatencyClass: "fast", CostClass: "high",
				SupportedWorkloadTags: []string{"inference-heavy", "deterministic-edit"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID: dispatch.DefaultBaselineProviderID, HealthState: dispatch.HealthHealthy,
				LatencyClass: "baseline", CostClass: "low",
				SupportedWorkloadTags: []string{"default"},
				ReliabilityClass:      "standard",
			},
		},
	}
}
