package tools

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/jobs"
	"github.com/emage/cwso/orchestrator/internal/mcp"
)

// DispatchHardwareAwareJob implements Feature A (Heterogeneous Hardware
// Dispatcher). It profiles an incoming task, routes it through the
// deterministic policy engine to the most efficient registered backend,
// enqueues the work asynchronously, and returns the job id plus the assigned
// hardware profile immediately (fire-and-forget).
//
// Routing is deterministic and lives entirely in the Go control plane; the
// model never coordinates backend selection. The CPU baseline remains the
// terminal-safe fallback.
type DispatchHardwareAwareJob struct {
	manager        *jobs.Manager
	defaultTimeout time.Duration
	emitter        dispatchDecisionEmitter
	snapshots      capabilitySnapshotReader
	policyEngine   *dispatch.PolicyEngineV2
}

// NewDispatchHardwareAwareJob constructs the hardware-aware dispatch tool bound
// to the async job manager, capability snapshot reader, and policy engine.
func NewDispatchHardwareAwareJob(
	manager *jobs.Manager,
	defaultTimeoutSeconds int,
	emitter dispatchDecisionEmitter,
	snapshots capabilitySnapshotReader,
	policyCfg dispatch.PolicyV2Config,
) *DispatchHardwareAwareJob {
	if defaultTimeoutSeconds <= 0 {
		defaultTimeoutSeconds = defaultDispatchTimeoutSeconds
	}
	return &DispatchHardwareAwareJob{
		manager:        manager,
		defaultTimeout: time.Duration(defaultTimeoutSeconds) * time.Second,
		emitter:        emitter,
		snapshots:      snapshots,
		policyEngine:   dispatch.NewPolicyEngineV2(policyCfg),
	}
}

// Name returns the MCP tool name.
func (t *DispatchHardwareAwareJob) Name() string { return "dispatch_hardware_aware_job" }

// Description returns the human-readable description.
func (t *DispatchHardwareAwareJob) Description() string {
	return "Profile a task and dispatch it asynchronously to the most efficient hardware backend " +
		"(LPU/GPU/SSM/Wasm-local) via deterministic policy routing. Returns a job_id and the assigned " +
		"hardware profile immediately."
}

// InputSchema returns the JSON schema for arguments.
func (t *DispatchHardwareAwareJob) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_description":      map[string]any{"type": "string", "minLength": 1},
			"context_size_estimate": map[string]any{"type": "integer", "minimum": 0},
			"latency_requirement": map[string]any{
				"type": "string",
				"enum": []string{dispatch.LatencyRealtime, dispatch.LatencyBatch},
			},
			"workload_tags": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"target_workspace_uuid": map[string]any{"type": "string", "format": "uuid"},
			"hardware_target_hint": map[string]any{
				"type": "string",
				"enum": []string{"auto", "lpu", "gpu", "photonic_sim", "edge_npu", "wasm_local"},
			},
			"quality_floor": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
		"required":             []string{"task_description", "context_size_estimate", "latency_requirement"},
		"additionalProperties": false,
	}
}

// AllowedRoles lists which tiers may invoke this tool.
func (t *DispatchHardwareAwareJob) AllowedRoles() []Role { return []Role{RoleOrchestrator} }

type hwAwareArgs struct {
	TaskDescription     string   `json:"task_description"`
	ContextSizeEstimate int      `json:"context_size_estimate"`
	LatencyRequirement  string   `json:"latency_requirement"`
	WorkloadTags        []string `json:"workload_tags,omitempty"`
	TargetWorkspaceUUID string   `json:"target_workspace_uuid,omitempty"`
	HardwareTargetHint  string   `json:"hardware_target_hint,omitempty"`
	QualityFloor        *float64 `json:"quality_floor,omitempty"`
}

type assignedHardwareProfile struct {
	SelectedProvider    string   `json:"selected_provider"`
	RankedFallbackChain []string `json:"ranked_fallback_chain"`
	PolicyVersion       string   `json:"policy_version"`
	CapabilityEpoch     uint64   `json:"capability_epoch"`
	Confidence          float64  `json:"confidence"`
	ReasonCode          string   `json:"reason_code"`
	RecommendedClass    string   `json:"recommended_class"`
	WorkloadTags        []string `json:"workload_tags"`
}

type hwAwareResult struct {
	JobID                   string                  `json:"job_id"`
	AssignedHardwareProfile assignedHardwareProfile `json:"assigned_hardware_profile"`
}

// Execute profiles the task, selects a backend, enqueues the job, and returns
// immediately. It never blocks on job completion.
func (t *DispatchHardwareAwareJob) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	startedAt := time.Now()
	if t.manager == nil {
		return mcp.TextError("job manager is not configured"), nil
	}
	if t.snapshots == nil || t.policyEngine == nil {
		return mcp.TextError("hardware-aware dispatch is not configured"), nil
	}

	var p hwAwareArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return mcp.TextError("invalid arguments: " + err.Error()), nil
	}
	if !isSaneField(p.TaskDescription) {
		return mcp.TextError("task_description is required"), nil
	}
	if p.ContextSizeEstimate < 0 {
		return mcp.TextError("context_size_estimate must be >= 0"), nil
	}
	latency, err := dispatch.NormalizeLatencyRequirement(p.LatencyRequirement)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	if p.TargetWorkspaceUUID != "" && !looksLikeUUID(p.TargetWorkspaceUUID) {
		return mcp.TextError("target_workspace_uuid must be a valid UUID"), nil
	}
	if p.QualityFloor != nil && (*p.QualityFloor < 0 || *p.QualityFloor > 1) {
		return mcp.TextError("quality_floor must be between 0 and 1"), nil
	}

	profile := dispatch.ProfileTask(p.TaskDescription, p.ContextSizeEstimate, latency)
	tags := mergeWorkloadTags(profile.Tags, p.WorkloadTags)

	snapshot := t.snapshots.Snapshot()
	decision := t.policyEngine.Select(snapshot, dispatch.PolicyInput{
		WorkloadTags:  tags,
		QualityScore:  p.QualityFloor,
		RequestLabels: profile.RequestLabels,
	})
	if decision.CapabilityEpoch == 0 {
		decision.CapabilityEpoch = snapshot.Epoch
	}

	job, enqErr := t.enqueue(p, decision.SelectedProvider)
	if enqErr != nil {
		// One deterministic fallback hop before surfacing a structured error.
		fallback := t.policyEngine.FallbackOnFailure(decision, decision.SelectedProvider, classifyProviderFailure(enqErr))
		if fb := strings.TrimSpace(fallback.SelectedProvider); fb != "" && fb != decision.SelectedProvider {
			decision = fallback
			job, enqErr = t.enqueue(p, decision.SelectedProvider)
		}
	}
	if enqErr != nil {
		code := "enqueue_failed"
		switch {
		case errors.Is(enqErr, jobs.ErrQueueFull):
			code = "queue_full"
		case errors.Is(enqErr, jobs.ErrClosed):
			code = "manager_closed"
		}
		return mcp.TextError("dispatch rejected: " + code), nil
	}

	out := hwAwareResult{
		JobID: job.ID,
		AssignedHardwareProfile: assignedHardwareProfile{
			SelectedProvider:    decision.SelectedProvider,
			RankedFallbackChain: decision.RankedFallbackChain,
			PolicyVersion:       decision.PolicyVersion,
			CapabilityEpoch:     decision.CapabilityEpoch,
			Confidence:          decision.Confidence,
			ReasonCode:          decision.ReasonCode,
			RecommendedClass:    profile.RecommendedClass,
			WorkloadTags:        tags,
		},
	}
	b, _ := json.Marshal(out)
	_ = t.emitHardwareAwareTelemetry(decision, time.Since(startedAt))
	return mcp.TextResult(string(b)), nil
}

func (t *DispatchHardwareAwareJob) enqueue(p hwAwareArgs, providerID string) (jobs.Job, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		providerID = dispatch.DefaultBaselineProviderID
	}
	return t.manager.Enqueue(jobs.Request{
		Name:    buildHardwareAwareJobName(p, providerID),
		Timeout: t.defaultTimeout,
		Run:     hardwareAwareRunFunc(providerID),
	})
}

func (t *DispatchHardwareAwareJob) emitHardwareAwareTelemetry(decision dispatch.PolicyDecision, elapsed time.Duration) error {
	if t == nil || t.emitter == nil {
		return nil
	}
	selectedProvider := strings.TrimSpace(decision.SelectedProvider)
	if selectedProvider == "" {
		selectedProvider = dispatch.DefaultBaselineProviderID
	}
	chain := decision.RankedFallbackChain
	if len(chain) == 0 {
		chain = []string{dispatch.DefaultBaselineProviderID}
	}
	fallbackCount := 0
	if len(chain) > 1 {
		fallbackCount = len(chain) - 1
	}
	policyVersion := strings.TrimSpace(decision.PolicyVersion)
	if policyVersion == "" {
		policyVersion = "cpu-baseline-default"
	}
	reasonCode := strings.TrimSpace(decision.ReasonCode)
	if reasonCode == "" {
		reasonCode = "selected"
	}
	return t.emitter.EmitDecision(dispatch.DecisionEvent{
		PolicyVersion:         policyVersion,
		CapabilityEpoch:       decision.CapabilityEpoch,
		SelectedProvider:      selectedProvider,
		FallbackChain:         chain,
		FallbackCount:         fallbackCount,
		ReasonCode:            reasonCode,
		Confidence:            decision.Confidence,
		EstimatedLatencyMS:    int(elapsed.Milliseconds()),
		ActualLatencyMS:       int(elapsed.Milliseconds()),
		FeatureFlagsApplied:   []string{"hhd.hardware_aware_dispatch.mcp"},
		QualityGuardrailState: "not-evaluated",
	})
}

// mergeWorkloadTags combines profiler-derived tags with caller-supplied tags,
// deduplicating case-insensitively and returning a stable sorted order so the
// policy decision is replay-deterministic.
func mergeWorkloadTags(profileTags, explicit []string) []string {
	combined := make([]string, 0, len(profileTags)+len(explicit))
	combined = append(combined, profileTags...)
	for _, tag := range explicit {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			combined = append(combined, trimmed)
		}
	}
	seen := make(map[string]struct{}, len(combined))
	out := make([]string, 0, len(combined))
	for _, tag := range combined {
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func buildHardwareAwareJobName(p hwAwareArgs, providerID string) string {
	ws := strings.TrimSpace(p.TargetWorkspaceUUID)
	if ws == "" {
		ws = "ephemeral"
	}
	return "hw-dispatch:" + strings.TrimSpace(providerID) + ":" + ws
}

// hardwareAwareRunFunc returns the job body. In the current shadow-mode rollout
// (blueprint Stage B) execution is a context-respecting no-op: the decision is
// logged and the job lifecycle is exercised, but no live provider call is made
// until real HAL adapters land (Phase 6 T082-T084).
func hardwareAwareRunFunc(providerID string) func(context.Context) error {
	_ = providerID
	return func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
}
