package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/hal"
	"github.com/emage/cwso/orchestrator/internal/jobs"
)

// SparseAgentGuardrail wires quality-floor breach detection to dense GPU escalation (T123).
type SparseAgentGuardrail struct {
	Enabled   bool
	MinScore  float64
	Policy    dispatch.PolicyV2Config
	Snapshots capabilitySnapshotReader
	Jobs      *jobs.Manager
	HAL       halInferrer
	Timeout   time.Duration
}

func (g *SparseAgentGuardrail) breach(qualityFloor *float64) bool {
	if g == nil || !g.Enabled {
		return false
	}
	return dispatch.QualityGuardrailBreached(qualityFloor, g.MinScore)
}

func (g *SparseAgentGuardrail) escalate(
	ctx context.Context,
	qualityFloor *float64,
	taskDescription string,
) (map[string]any, error) {
	if g == nil || g.Snapshots == nil {
		return nil, fmt.Errorf("sparse quality guardrail is not configured")
	}
	snapshot := g.Snapshots.Snapshot()
	engine := dispatch.EscalationEngineForDenseGPU(g.Policy)
	decision := dispatch.SelectDenseGPUEscalation(engine, snapshot)

	out := map[string]any{
		"escalated":             true,
		"reason_code":           dispatch.ReasonQualityGuardrailAutodisable,
		"quality_floor":         *qualityFloor,
		"selected_provider":     decision.SelectedProvider,
		"ranked_fallback_chain": decision.RankedFallbackChain,
		"policy_version":        decision.PolicyVersion,
		"capability_epoch":      decision.CapabilityEpoch,
	}
	if g.Jobs == nil || g.HAL == nil {
		return out, nil
	}

	prompt := strings.TrimSpace(taskDescription)
	if prompt == "" {
		prompt = "sparse-agent-quality-floor-escalation"
	}
	inferReq := hal.InferenceRequest{
		WorkloadTags:  []string{"inference-heavy", "deterministic-edit"},
		Prompt:        prompt,
		ContextTokens: 0,
		LatencyClass:  dispatch.LatencyBatch,
	}
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	providerID := decision.SelectedProvider
	job, err := g.Jobs.Enqueue(jobs.Request{
		Name:    "sparse-escalation:" + providerID,
		Timeout: timeout,
		RunResult: func(runCtx context.Context) (string, error) {
			res, inferErr := g.HAL.Infer(runCtx, providerID, decision.RankedFallbackChain, inferReq)
			if inferErr != nil {
				return "", inferErr
			}
			summary := hwAwareJobResult{
				ServedBy:      res.ServedBy,
				FallbackCount: res.FallbackCount,
				TokensOut:     res.Completion.TokensOut,
				Deterministic: res.Completion.Deterministic,
				Output:        res.Completion.Output,
			}
			b, mErr := json.Marshal(summary)
			if mErr != nil {
				return "", mErr
			}
			return string(b), nil
		},
	})
	if err != nil {
		return nil, err
	}
	out["job_id"] = job.ID
	return out, nil
}
