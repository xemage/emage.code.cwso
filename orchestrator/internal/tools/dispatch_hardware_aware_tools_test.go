package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/jobs"
	"github.com/emage/cwso/orchestrator/internal/mcp"
)

func hwAwareSnapshot() dispatch.CapabilitySnapshot {
	return dispatch.CapabilitySnapshot{
		Epoch: 7,
		Providers: []dispatch.ProviderCapability{
			{
				ProviderID:            "cpu-baseline",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           dispatch.HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "low",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"default"},
				ReliabilityClass:      "standard",
			},
			{
				ProviderID:            "lpu-realtime",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           dispatch.HealthHealthy,
				LatencyClass:          "ultra",
				CostClass:             "medium",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"realtime"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID:            "gpu-accelerated",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           dispatch.HealthHealthy,
				LatencyClass:          "fast",
				CostClass:             "high",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"inference-heavy", "deterministic-edit"},
				ReliabilityClass:      "gold",
			},
			{
				ProviderID:            "ssm-longctx",
				ContractVersion:       "dispatch.provider/v1.0",
				HealthState:           dispatch.HealthHealthy,
				LatencyClass:          "baseline",
				CostClass:             "medium",
				QueueDepth:            0,
				SupportedWorkloadTags: []string{"long-context"},
				ReliabilityClass:      "gold",
			},
		},
	}
}

func newHWAwareTool(t *testing.T, emitter dispatchDecisionEmitter) (*DispatchHardwareAwareJob, *jobs.Manager) {
	t.Helper()
	mgr, err := jobs.NewManager(jobs.Config{Workers: 1, QueueSize: 8}, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	policyCfg := dispatch.DefaultPolicyV2Config()
	policyCfg.Enabled = true
	tool := NewDispatchHardwareAwareJob(
		mgr,
		300,
		emitter,
		stubCapabilitySnapshotReader{snapshot: hwAwareSnapshot()},
		policyCfg,
	)
	return tool, mgr
}

func decodeHWAwareResult(t *testing.T, res *mcp.ToolCallResult) hwAwareResult {
	t.Helper()
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected single content block, got %+v", res.Content)
	}
	var out hwAwareResult
	if err := json.Unmarshal([]byte(res.Content[0].Text), &out); err != nil {
		t.Fatalf("unmarshal hw-aware result: %v; text=%s", err, res.Content[0].Text)
	}
	return out
}

func TestHardwareAwareDispatchRoutesByProfile(t *testing.T) {
	cases := []struct {
		name         string
		args         string
		wantProvider string
		wantClass    string
	}{
		{
			name:         "realtime small context selects lpu",
			args:         `{"task_description":"fix typo","context_size_estimate":1000,"latency_requirement":"realtime"}`,
			wantProvider: "lpu-realtime",
			wantClass:    "lpu",
		},
		{
			name:         "huge context selects ssm",
			args:         `{"task_description":"audit whole repo","context_size_estimate":120000,"latency_requirement":"batch"}`,
			wantProvider: "ssm-longctx",
			wantClass:    "ssm",
		},
		{
			name:         "deterministic edit selects gpu and recommends wasm-local",
			args:         `{"task_description":"Rename function foo to bar","context_size_estimate":2000,"latency_requirement":"batch"}`,
			wantProvider: "gpu-accelerated",
			wantClass:    "wasm-local",
		},
		{
			name:         "general task selects gpu",
			args:         `{"task_description":"Implement an auth subsystem","context_size_estimate":16000,"latency_requirement":"batch"}`,
			wantProvider: "gpu-accelerated",
			wantClass:    "gpu",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tool, mgr := newHWAwareTool(t, &recordingDispatchEmitter{})
			defer mgr.Close()

			res, err := tool.Execute(context.Background(), json.RawMessage(c.args))
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			out := decodeHWAwareResult(t, res)
			if out.JobID == "" {
				t.Fatalf("expected non-empty job id")
			}
			if out.AssignedHardwareProfile.SelectedProvider != c.wantProvider {
				t.Fatalf("selected provider = %q, want %q", out.AssignedHardwareProfile.SelectedProvider, c.wantProvider)
			}
			if out.AssignedHardwareProfile.RecommendedClass != c.wantClass {
				t.Fatalf("recommended class = %q, want %q", out.AssignedHardwareProfile.RecommendedClass, c.wantClass)
			}
			if out.AssignedHardwareProfile.CapabilityEpoch != 7 {
				t.Fatalf("capability epoch = %d, want 7", out.AssignedHardwareProfile.CapabilityEpoch)
			}
		})
	}
}

func TestHardwareAwareDispatchImmediateReturn(t *testing.T) {
	tool, mgr := newHWAwareTool(t, &recordingDispatchEmitter{})
	defer mgr.Close()

	started := time.Now()
	res, err := tool.Execute(context.Background(),
		json.RawMessage(`{"task_description":"do work","context_size_estimate":500,"latency_requirement":"realtime"}`))
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("expected immediate return, took %s", elapsed)
	}
}

func TestHardwareAwareDispatchEmitsTelemetry(t *testing.T) {
	emitter := &recordingDispatchEmitter{}
	tool, mgr := newHWAwareTool(t, emitter)
	defer mgr.Close()

	res, err := tool.Execute(context.Background(),
		json.RawMessage(`{"task_description":"do work","context_size_estimate":500,"latency_requirement":"realtime"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	if emitter.called != 1 {
		t.Fatalf("expected exactly one telemetry emission, got %d", emitter.called)
	}
	if emitter.last.SelectedProvider != "lpu-realtime" {
		t.Fatalf("telemetry selected provider = %q, want lpu-realtime", emitter.last.SelectedProvider)
	}
	if emitter.last.CapabilityEpoch != 7 {
		t.Fatalf("telemetry epoch = %d, want 7", emitter.last.CapabilityEpoch)
	}
}

func TestHardwareAwareDispatchValidation(t *testing.T) {
	cases := []struct {
		name string
		args string
	}{
		{"empty description", `{"task_description":"   ","context_size_estimate":10,"latency_requirement":"batch"}`},
		{"invalid latency", `{"task_description":"x","context_size_estimate":10,"latency_requirement":"soon"}`},
		{"bad uuid", `{"task_description":"x","context_size_estimate":10,"latency_requirement":"batch","target_workspace_uuid":"not-a-uuid"}`},
		{"quality out of range", `{"task_description":"x","context_size_estimate":10,"latency_requirement":"batch","quality_floor":2}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tool, mgr := newHWAwareTool(t, &recordingDispatchEmitter{})
			defer mgr.Close()

			res, err := tool.Execute(context.Background(), json.RawMessage(c.args))
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !res.IsError {
				t.Fatalf("expected validation error, got %+v", res)
			}
		})
	}
}

func TestHardwareAwareDispatchAllowedRoles(t *testing.T) {
	tool := NewDispatchHardwareAwareJob(nil, 300, nil, nil, dispatch.DefaultPolicyV2Config())
	roles := tool.AllowedRoles()
	if len(roles) != 1 || roles[0] != RoleOrchestrator {
		t.Fatalf("expected orchestrator-only role, got %+v", roles)
	}
}
