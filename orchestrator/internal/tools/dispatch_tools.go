package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/jobs"
	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/sandbox"
)

const (
	defaultDispatchTimeoutSeconds = 300
	defaultDispatchMaxBatch       = 64
	maxTimeoutSeconds             = 3600
	maxFieldLength                = 512
)

// DispatchConcurrentJobs accepts a planning batch and enqueues accepted jobs.
// The tool returns immediately with deterministic per-item outcomes.
type DispatchConcurrentJobs struct {
	manager         *jobs.Manager
	defaultTimeout  time.Duration
	maxBatch        int
	runner          sandbox.RunnerInterface
	workspaceRoot   string
	runFuncBuilder  func(dispatchJobSpec, string) func(context.Context) error
	emitter         dispatchDecisionEmitter
	snapshots       capabilitySnapshotReader
	policyEngine    *dispatch.PolicyEngineV2
	policyV2Enabled bool
}

type dispatchDecisionEmitter interface {
	EmitDecision(event dispatch.DecisionEvent) error
}

type capabilitySnapshotReader interface {
	Snapshot() dispatch.CapabilitySnapshot
}

// NewDispatchConcurrentJobs constructs a dispatch tool bound to the async job manager.
func NewDispatchConcurrentJobs(manager *jobs.Manager, defaultTimeoutSeconds, maxBatch int) *DispatchConcurrentJobs {
	return NewDispatchConcurrentJobsWithRunner(manager, defaultTimeoutSeconds, maxBatch, nil, "")
}

// NewDispatchConcurrentJobsWithRunner constructs a dispatch tool and optionally
// wires asynchronous jobs to a sandbox runner.
func NewDispatchConcurrentJobsWithRunner(manager *jobs.Manager, defaultTimeoutSeconds, maxBatch int, runner sandbox.RunnerInterface, workspaceRoot string) *DispatchConcurrentJobs {
	return NewDispatchConcurrentJobsWithDispatchPolicy(
		manager,
		defaultTimeoutSeconds,
		maxBatch,
		runner,
		workspaceRoot,
		nil,
		nil,
		dispatch.DefaultPolicyV2Config(),
	)
}

// NewDispatchConcurrentJobsWithTelemetry constructs a dispatch tool and optionally
// wires decision telemetry enriched with capability snapshot metadata.
func NewDispatchConcurrentJobsWithTelemetry(
	manager *jobs.Manager,
	defaultTimeoutSeconds, maxBatch int,
	runner sandbox.RunnerInterface,
	workspaceRoot string,
	emitter dispatchDecisionEmitter,
	snapshots capabilitySnapshotReader,
) *DispatchConcurrentJobs {
	return NewDispatchConcurrentJobsWithDispatchPolicy(
		manager,
		defaultTimeoutSeconds,
		maxBatch,
		runner,
		workspaceRoot,
		emitter,
		snapshots,
		dispatch.DefaultPolicyV2Config(),
	)
}

// NewDispatchConcurrentJobsWithDispatchPolicy constructs a dispatch tool and optionally
// enables policy engine v2 scoring/fallback when configured.
func NewDispatchConcurrentJobsWithDispatchPolicy(
	manager *jobs.Manager,
	defaultTimeoutSeconds, maxBatch int,
	runner sandbox.RunnerInterface,
	workspaceRoot string,
	emitter dispatchDecisionEmitter,
	snapshots capabilitySnapshotReader,
	policyCfg dispatch.PolicyV2Config,
) *DispatchConcurrentJobs {
	if defaultTimeoutSeconds <= 0 {
		defaultTimeoutSeconds = defaultDispatchTimeoutSeconds
	}
	if maxBatch <= 0 {
		maxBatch = defaultDispatchMaxBatch
	}
	runBuilder := defaultRunFunc
	if runner != nil {
		runBuilder = func(spec dispatchJobSpec, providerID string) func(context.Context) error {
			return func(ctx context.Context) error {
				req := sandbox.RunRequest{
					Name: strings.ReplaceAll(buildDispatchJobName(spec), ":", "-"),
					Env: map[string]string{
						"CWSO_AGENT_ROLE":        strings.TrimSpace(spec.AgentRole),
						"CWSO_OBJECTIVE_PROMPT":  spec.ObjectivePrompt,
						"CWSO_TARGET_WORKSPACE":  spec.TargetWorkspaceUUID,
						"CWSO_DISPATCH_JOB_NAME": buildDispatchJobName(spec),
						"CWSO_DISPATCH_PROVIDER": strings.TrimSpace(providerID),
					},
					SandboxProfile: sandbox.SandboxProfile(spec.SandboxProfile),
				}
				if strings.TrimSpace(workspaceRoot) != "" {
					req.MountWorkspace = true
					req.WorkspaceDir = filepath.Clean(workspaceRoot)
				}
				_, err := runner.Execute(ctx, req)
				return err
			}
		}
	}
	return &DispatchConcurrentJobs{
		manager:         manager,
		defaultTimeout:  time.Duration(defaultTimeoutSeconds) * time.Second,
		maxBatch:        maxBatch,
		runner:          runner,
		workspaceRoot:   workspaceRoot,
		runFuncBuilder:  runBuilder,
		emitter:         emitter,
		snapshots:       snapshots,
		policyEngine:    dispatch.NewPolicyEngineV2(policyCfg),
		policyV2Enabled: policyCfg.Enabled,
	}
}

func defaultRunFunc(_ dispatchJobSpec, _ string) func(context.Context) error {
	return func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
}

// Name returns the MCP tool name.
func (t *DispatchConcurrentJobs) Name() string { return "dispatch_concurrent_jobs" }

// Description returns the human-readable description.
func (t *DispatchConcurrentJobs) Description() string {
	return "Dispatch planning-tier concurrent jobs asynchronously and return accepted/rejected results immediately."
}

// InputSchema returns the JSON schema for arguments.
func (t *DispatchConcurrentJobs) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"jobs": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"agent_role":            map[string]any{"type": "string"},
						"objective_prompt":      map[string]any{"type": "string", "minLength": 1},
						"target_workspace_uuid": map[string]any{"type": "string", "format": "uuid"},
						"sandbox_profile": map[string]any{
							"type": "string",
							"enum": []string{
								string(sandbox.ProfileDockerTrusted),
								string(sandbox.ProfileGVisorFastEphemeral),
								string(sandbox.ProfileFirecrackerSecure),
							},
							"description": "Requested sandbox isolation tier. Server policy enforces routing; callers cannot escalate to docker-trusted.",
						},
					},
					"required":             []string{"agent_role", "objective_prompt", "target_workspace_uuid"},
					"additionalProperties": false,
				},
			},
			"execution_timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": defaultDispatchTimeoutSeconds},
		},
		"required":             []string{"jobs"},
		"additionalProperties": false,
	}
}

// AllowedRoles lists which tiers may invoke this tool.
func (t *DispatchConcurrentJobs) AllowedRoles() []Role { return []Role{RoleOrchestrator} }

type dispatchArgs struct {
	Jobs                    []dispatchJobSpec `json:"jobs"`
	ExecutionTimeoutSeconds *int              `json:"execution_timeout_seconds,omitempty"`
}

type dispatchJobSpec struct {
	AgentRole           string `json:"agent_role"`
	ObjectivePrompt     string `json:"objective_prompt"`
	TargetWorkspaceUUID string `json:"target_workspace_uuid"`
	SandboxProfile      string `json:"sandbox_profile,omitempty"`
}

type dispatchBatchResult struct {
	Requested int                  `json:"requested"`
	Accepted  int                  `json:"accepted"`
	Rejected  int                  `json:"rejected"`
	Results   []dispatchItemResult `json:"results"`
}

type dispatchItemResult struct {
	Index  int    `json:"index"`
	Status string `json:"status"`
	JobID  string `json:"job_id,omitempty"`
	Code   string `json:"code,omitempty"`
}

// Execute validates and dispatches a batch without waiting for job completion.
func (t *DispatchConcurrentJobs) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	startedAt := time.Now()
	if t.manager == nil {
		return mcp.TextError("job manager is not configured"), nil
	}

	var p dispatchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return mcp.TextError("invalid arguments: " + err.Error()), nil
	}

	if len(p.Jobs) == 0 {
		return mcp.TextError("jobs must contain at least 1 item"), nil
	}
	if len(p.Jobs) > t.maxBatch {
		return mcp.TextError(fmt.Sprintf("jobs exceeds max batch size of %d", t.maxBatch)), nil
	}

	timeout, timeoutErr := t.resolveTimeout(p.ExecutionTimeoutSeconds)
	if timeoutErr != nil {
		return mcp.TextError(timeoutErr.Error()), nil
	}

	selection := dispatch.PolicyDecision{
		PolicyVersion:       "cpu-baseline-default",
		SelectedProvider:    dispatch.DefaultBaselineProviderID,
		RankedFallbackChain: []string{dispatch.DefaultBaselineProviderID},
		Confidence:          1,
		ReasonCode:          "feature_disabled",
	}
	if t.policyEnabled() {
		snapshot := t.snapshots.Snapshot()
		selection = t.policyEngine.Select(snapshot, dispatch.PolicyInput{WorkloadTags: deriveBatchWorkloadTags(p.Jobs)})
		if selection.CapabilityEpoch == 0 {
			selection.CapabilityEpoch = snapshot.Epoch
		}
	}

	activeProvider := selection.SelectedProvider

	out := dispatchBatchResult{Requested: len(p.Jobs), Results: make([]dispatchItemResult, 0, len(p.Jobs))}
	for i, spec := range p.Jobs {
		if err := validateDispatchSpec(spec); err != nil {
			out.Rejected++
			out.Results = append(out.Results, dispatchItemResult{
				Index:  i,
				Status: "rejected",
				Code:   "invalid_spec",
			})
			continue
		}

		job, err := t.enqueue(spec, timeout, activeProvider)
		if err != nil && t.policyEnabled() {
			fallbackDecision := t.policyEngine.FallbackOnFailure(selection, activeProvider, classifyProviderFailure(err))
			fallbackProvider := strings.TrimSpace(fallbackDecision.SelectedProvider)
			if fallbackProvider != "" && fallbackProvider != activeProvider {
				selection = fallbackDecision
				activeProvider = fallbackProvider
				job, err = t.enqueue(spec, timeout, activeProvider)
			}
		}
		if err == nil {
			out.Accepted++
			out.Results = append(out.Results, dispatchItemResult{
				Index:  i,
				Status: "accepted",
				JobID:  job.ID,
			})
			continue
		}

		code := "enqueue_failed"
		switch {
		case errors.Is(err, jobs.ErrQueueFull):
			code = "queue_full"
		case errors.Is(err, jobs.ErrClosed):
			code = "manager_closed"
		}
		out.Rejected++
		out.Results = append(out.Results, dispatchItemResult{
			Index:  i,
			Status: "rejected",
			Code:   code,
		})
	}

	b, _ := json.Marshal(out)
	_ = t.emitDecisionTelemetry(out, selection, time.Since(startedAt))
	return mcp.TextResult(string(b)), nil
}

func (t *DispatchConcurrentJobs) emitDecisionTelemetry(out dispatchBatchResult, selection dispatch.PolicyDecision, elapsed time.Duration) error {
	if t == nil || t.emitter == nil {
		return nil
	}

	capabilityEpoch := uint64(0)
	if selection.CapabilityEpoch > 0 {
		capabilityEpoch = selection.CapabilityEpoch
	} else if t.snapshots != nil {
		capabilityEpoch = t.snapshots.Snapshot().Epoch
	}

	reasonCode := strings.TrimSpace(selection.ReasonCode)
	if reasonCode == "" || reasonCode == "selected" {
		reasonCode = "accepted"
		if out.Accepted == 0 {
			reasonCode = "rejected"
		} else if out.Rejected > 0 {
			reasonCode = "accepted_partial"
		}
	}

	policyVersion := strings.TrimSpace(selection.PolicyVersion)
	if policyVersion == "" {
		policyVersion = "cpu-baseline-default"
	}

	selectedProvider := strings.TrimSpace(selection.SelectedProvider)
	if selectedProvider == "" {
		selectedProvider = dispatch.DefaultBaselineProviderID
	}

	fallbackChain := selection.RankedFallbackChain
	if len(fallbackChain) == 0 {
		fallbackChain = []string{dispatch.DefaultBaselineProviderID}
	}
	fallbackCount := 0
	if len(fallbackChain) > 1 {
		fallbackCount = len(fallbackChain) - 1
	}

	return t.emitter.EmitDecision(dispatch.DecisionEvent{
		PolicyVersion:         policyVersion,
		CapabilityEpoch:       capabilityEpoch,
		SelectedProvider:      selectedProvider,
		FallbackChain:         fallbackChain,
		FallbackCount:         fallbackCount,
		ReasonCode:            reasonCode,
		Confidence:            selection.Confidence,
		EstimatedLatencyMS:    int(elapsed.Milliseconds()),
		ActualLatencyMS:       int(elapsed.Milliseconds()),
		FeatureFlagsApplied:   []string{"hhd.decision_telemetry.mcp_dispatch"},
		QualityGuardrailState: "not-evaluated",
	})
}

func (t *DispatchConcurrentJobs) policyEnabled() bool {
	if t == nil || t.policyEngine == nil || t.snapshots == nil {
		return false
	}
	return t.policyV2Enabled
}

func (t *DispatchConcurrentJobs) enqueue(spec dispatchJobSpec, timeout time.Duration, providerID string) (jobs.Job, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		providerID = dispatch.DefaultBaselineProviderID
	}
	return t.manager.Enqueue(jobs.Request{
		Name:    buildDispatchJobName(spec),
		Timeout: timeout,
		Run:     t.runFuncBuilder(spec, providerID),
	})
}

func deriveBatchWorkloadTags(jobs []dispatchJobSpec) []string {
	tags := make([]string, 0, len(jobs)+1)
	tags = append(tags, "default")
	for _, spec := range jobs {
		profile := strings.TrimSpace(strings.ToLower(spec.SandboxProfile))
		if profile != "" {
			tags = append(tags, profile)
		}
	}
	return tags
}

func classifyProviderFailure(err error) string {
	switch {
	case errors.Is(err, jobs.ErrQueueFull):
		return "capacity_exhausted"
	case errors.Is(err, jobs.ErrClosed):
		return "unavailable"
	default:
		return "provider_failure"
	}
}

func (t *DispatchConcurrentJobs) resolveTimeout(raw *int) (time.Duration, error) {
	if raw == nil {
		return t.defaultTimeout, nil
	}
	if *raw <= 0 {
		return 0, errors.New("execution_timeout_seconds must be >= 1")
	}
	if *raw > maxTimeoutSeconds {
		return 0, fmt.Errorf("execution_timeout_seconds must be <= %d", maxTimeoutSeconds)
	}
	return time.Duration(*raw) * time.Second, nil
}

func validateDispatchSpec(spec dispatchJobSpec) error {
	if !isSaneField(spec.AgentRole) {
		return errors.New("agent_role is required")
	}
	if !isSaneField(spec.ObjectivePrompt) {
		return errors.New("objective_prompt is required")
	}
	if !looksLikeUUID(spec.TargetWorkspaceUUID) {
		return errors.New("target_workspace_uuid must be a valid UUID")
	}
	if spec.SandboxProfile != "" && !sandbox.ValidSandboxProfiles[sandbox.SandboxProfile(spec.SandboxProfile)] {
		return fmt.Errorf("sandbox_profile %q is not a valid tier", spec.SandboxProfile)
	}
	return nil
}

func isSaneField(v string) bool {
	v = strings.TrimSpace(v)
	return v != "" && len(v) <= maxFieldLength
}

func buildDispatchJobName(spec dispatchJobSpec) string {
	role := strings.TrimSpace(spec.AgentRole)
	ws := strings.TrimSpace(spec.TargetWorkspaceUUID)
	return "dispatch:" + role + ":" + ws
}

func looksLikeUUID(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i, c := range v {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
