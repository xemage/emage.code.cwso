package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/emage/cwso/orchestrator/internal/jobs"
	"github.com/emage/cwso/orchestrator/internal/mcp"
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
	manager        *jobs.Manager
	defaultTimeout time.Duration
	maxBatch       int
	runFuncBuilder func(dispatchJobSpec) func(context.Context) error
}

// NewDispatchConcurrentJobs constructs a dispatch tool bound to the async job manager.
func NewDispatchConcurrentJobs(manager *jobs.Manager, defaultTimeoutSeconds, maxBatch int) *DispatchConcurrentJobs {
	if defaultTimeoutSeconds <= 0 {
		defaultTimeoutSeconds = defaultDispatchTimeoutSeconds
	}
	if maxBatch <= 0 {
		maxBatch = defaultDispatchMaxBatch
	}
	return &DispatchConcurrentJobs{
		manager:        manager,
		defaultTimeout: time.Duration(defaultTimeoutSeconds) * time.Second,
		maxBatch:       maxBatch,
		runFuncBuilder: defaultRunFunc,
	}
}

func defaultRunFunc(_ dispatchJobSpec) func(context.Context) error {
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

		job, err := t.manager.Enqueue(jobs.Request{
			Name:    buildDispatchJobName(spec),
			Timeout: timeout,
			Run:     t.runFuncBuilder(spec),
		})
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
	return mcp.TextResult(string(b)), nil
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
