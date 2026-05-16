package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/jobs"
	"github.com/emage/cwso/orchestrator/internal/mcp"
)

func TestDispatchConcurrentJobsImmediateReturn(t *testing.T) {
	mgr, err := jobs.NewManager(jobs.Config{Workers: 1, QueueSize: 4}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	release := make(chan struct{})
	tool := NewDispatchConcurrentJobs(mgr, 300, 4)
	tool.runFuncBuilder = func(dispatchJobSpec) func(context.Context) error {
		return func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-release:
				return nil
			}
		}
	}

	args := json.RawMessage(`{"jobs":[{"agent_role":"worker","objective_prompt":"do it","target_workspace_uuid":"11111111-1111-1111-1111-111111111111"}]}`)
	started := time.Now()
	res, execErr := tool.Execute(context.Background(), args)
	elapsed := time.Since(started)
	close(release)

	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("expected immediate return, took %s", elapsed)
	}

	out := decodeDispatchResult(t, res)
	if out.Accepted != 1 || out.Rejected != 0 {
		t.Fatalf("unexpected counters: %+v", out)
	}
	if out.Results[0].Status != "accepted" || out.Results[0].JobID == "" {
		t.Fatalf("unexpected item result: %+v", out.Results[0])
	}
}

func TestDispatchConcurrentJobsMixedQueuePressure(t *testing.T) {
	mgr, err := jobs.NewManager(jobs.Config{Workers: 1, QueueSize: 2}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	runStarted := make(chan struct{})
	release := make(chan struct{})

	blockingRun := func(ctx context.Context) error {
		select {
		case <-runStarted:
		default:
			close(runStarted)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return nil
		}
	}

	if _, err = mgr.Enqueue(jobs.Request{Name: "prefill-running", Run: blockingRun}); err != nil {
		t.Fatalf("prefill running enqueue: %v", err)
	}
	<-runStarted
	if _, err = mgr.Enqueue(jobs.Request{Name: "prefill-queued", Run: blockingRun}); err != nil {
		t.Fatalf("prefill queued enqueue: %v", err)
	}

	tool := NewDispatchConcurrentJobs(mgr, 300, 4)
	tool.runFuncBuilder = func(dispatchJobSpec) func(context.Context) error { return blockingRun }
	args := json.RawMessage(`{"jobs":[{"agent_role":"worker","objective_prompt":"a","target_workspace_uuid":"11111111-1111-1111-1111-111111111111"},{"agent_role":"worker","objective_prompt":"b","target_workspace_uuid":"22222222-2222-2222-2222-222222222222"}]}`)

	res, execErr := tool.Execute(context.Background(), args)
	close(release)
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}

	out := decodeDispatchResult(t, res)
	if out.Accepted != 1 || out.Rejected != 1 {
		t.Fatalf("unexpected counters: %+v", out)
	}
	if len(out.Results) != 2 {
		t.Fatalf("expected 2 item results, got %d", len(out.Results))
	}
	if out.Results[0].Status != "accepted" || out.Results[0].JobID == "" {
		t.Fatalf("expected first item accepted, got %+v", out.Results[0])
	}
	if out.Results[1].Status != "rejected" || out.Results[1].Code != "queue_full" {
		t.Fatalf("expected second item queue_full rejection, got %+v", out.Results[1])
	}
}

func TestDispatchConcurrentJobsInvalidInputs(t *testing.T) {
	mgr, err := jobs.NewManager(jobs.Config{Workers: 1, QueueSize: 4}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	tool := NewDispatchConcurrentJobs(mgr, 300, 2)

	res, execErr := tool.Execute(context.Background(), json.RawMessage(`{"jobs":[]}`))
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	if !res.IsError {
		t.Fatalf("expected top-level validation error, got %+v", res)
	}

	res, execErr = tool.Execute(context.Background(), json.RawMessage(`{"jobs":[{"agent_role":"","objective_prompt":"","target_workspace_uuid":"bad"},{"agent_role":"worker","objective_prompt":"ok","target_workspace_uuid":"11111111-1111-1111-1111-111111111111"}]}`))
	if execErr != nil {
		t.Fatalf("execute mixed: %v", execErr)
	}
	if res.IsError {
		t.Fatalf("expected per-item mixed result, got tool error: %+v", res)
	}

	out := decodeDispatchResult(t, res)
	if out.Accepted != 1 || out.Rejected != 1 {
		t.Fatalf("unexpected counters: %+v", out)
	}
	if out.Results[0].Status != "rejected" || out.Results[0].Code != "invalid_spec" {
		t.Fatalf("expected first invalid spec rejection, got %+v", out.Results[0])
	}
	if out.Results[1].Status != "accepted" || out.Results[1].JobID == "" {
		t.Fatalf("expected second accepted, got %+v", out.Results[1])
	}
}

func TestDispatchConcurrentJobsAllowedRoles(t *testing.T) {
	tool := NewDispatchConcurrentJobs(nil, 300, 10)
	roles := tool.AllowedRoles()
	if len(roles) != 1 || roles[0] != RoleOrchestrator {
		t.Fatalf("expected orchestrator-only role, got %+v", roles)
	}
}

func TestDispatchJobSpecInvalidSandboxProfileIsRejected(t *testing.T) {
	mgr, err := jobs.NewManager(jobs.Config{Workers: 1, QueueSize: 4}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	tool := NewDispatchConcurrentJobs(mgr, 300, 4)
	args := json.RawMessage(`{"jobs":[{"agent_role":"worker","objective_prompt":"do it","target_workspace_uuid":"11111111-1111-1111-1111-111111111111","sandbox_profile":"hacker-tier"}]}`)
	res, execErr := tool.Execute(context.Background(), args)
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	if res.IsError {
		t.Fatalf("expected per-item rejection (not tool error), got %+v", res)
	}
	out := decodeDispatchResult(t, res)
	if out.Rejected != 1 || out.Accepted != 0 {
		t.Fatalf("expected 1 rejected, got %+v", out)
	}
	if out.Results[0].Code != "invalid_spec" {
		t.Fatalf("expected invalid_spec rejection code, got %q", out.Results[0].Code)
	}
}

func TestDispatchJobSpecValidSandboxProfileIsAccepted(t *testing.T) {
	mgr, err := jobs.NewManager(jobs.Config{Workers: 1, QueueSize: 4}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	tool := NewDispatchConcurrentJobs(mgr, 300, 4)
	args := json.RawMessage(`{"jobs":[{"agent_role":"worker","objective_prompt":"do it","target_workspace_uuid":"11111111-1111-1111-1111-111111111111","sandbox_profile":"gvisor-fast-ephemeral"}]}`)
	res, execErr := tool.Execute(context.Background(), args)
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	out := decodeDispatchResult(t, res)
	if out.Accepted != 1 || out.Rejected != 0 {
		t.Fatalf("expected 1 accepted, got %+v", out)
	}
}

func decodeDispatchResult(t *testing.T, res *mcp.ToolCallResult) dispatchBatchResult {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("expected single content block, got %+v", res.Content)
	}
	var out dispatchBatchResult
	if err := json.Unmarshal([]byte(res.Content[0].Text), &out); err != nil {
		t.Fatalf("unmarshal dispatch result: %v; text=%s", err, res.Content[0].Text)
	}
	return out
}
