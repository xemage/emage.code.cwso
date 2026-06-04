package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
	"github.com/emage/cwso/orchestrator/internal/sparse"
)

// CreateEphemeralSparseAgent instantiates a wasmtime-backed sparse micro-agent over a pinned
// skill slice (T122 / ADR-008).
type CreateEphemeralSparseAgent struct {
	client   *sparse.Client
	registry *dispatch.SparseAgentRegistry
	pub      memorybroker.Publisher
	hostCap  int
}

// NewCreateEphemeralSparseAgent wires the tool to the sparse sidecar and telemetry publisher.
func NewCreateEphemeralSparseAgent(
	client *sparse.Client,
	registry *dispatch.SparseAgentRegistry,
	pub memorybroker.Publisher,
	hostRAMCapMB int,
) *CreateEphemeralSparseAgent {
	return &CreateEphemeralSparseAgent{
		client: client, registry: registry, pub: pub, hostCap: hostRAMCapMB,
	}
}

func (t *CreateEphemeralSparseAgent) Name() string { return "create_ephemeral_sparse_agent" }

func (t *CreateEphemeralSparseAgent) Description() string {
	return "Create an ephemeral sparse Wasm micro-agent over a SHA-256-pinned skill slice. " +
		"Returns a wasm_agent_id and a cwso://agents/{id}/telemetry stream resource."
}

func (t *CreateEphemeralSparseAgent) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target_ast_node": map[string]any{
				"type":        "string",
				"description": "Optional AST node context (provenance only).",
			},
			"skill_domain": map[string]any{
				"type":        "string",
				"description": "Skill domain selecting a pinned pruned slice (required).",
			},
			"quantization": map[string]any{
				"type":        "string",
				"enum":        []string{"1.58-bit", "int4", "int8"},
				"description": "Quantization tag. Only 1.58-bit is implemented; defaults to 1.58-bit.",
			},
			"max_ram_mb": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Hard RAM cap for the agent sandbox (default 512).",
			},
		},
		"required": []string{"skill_domain"},
	}
}

func (t *CreateEphemeralSparseAgent) AllowedRoles() []Role { return []Role{RoleOrchestrator} }

func (t *CreateEphemeralSparseAgent) Execute(ctx context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	if t.client == nil || t.registry == nil {
		return mcp.TextError("sparse micro-agents are not enabled on this server"), nil
	}
	var p struct {
		TargetASTNode string `json:"target_ast_node"`
		SkillDomain   string `json:"skill_domain"`
		Quantization  string `json:"quantization"`
		MaxRAMMB      int    `json:"max_ram_mb"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return mcp.TextError("invalid arguments: " + err.Error()), nil
		}
	}
	if p.SkillDomain == "" {
		return mcp.TextError("skill_domain is required"), nil
	}
	quant, err := dispatch.NormalizeQuantization(p.Quantization)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	maxRAM := p.MaxRAMMB
	if maxRAM == 0 {
		maxRAM = 512
	}
	if maxRAM <= 0 {
		return mcp.TextError("max_ram_mb must be > 0"), nil
	}
	if t.hostCap > 0 && maxRAM > t.hostCap {
		return mcp.TextError(fmt.Sprintf("max_ram_mb %d exceeds host cap %d", maxRAM, t.hostCap)), nil
	}

	var targetNode *string
	if p.TargetASTNode != "" {
		targetNode = &p.TargetASTNode
	}

	res, err := t.client.CreateAgent(ctx, sparse.CreateAgentParams{
		SkillDomain:   p.SkillDomain,
		Quantization:  quant,
		MaxRAMMB:      uint32(maxRAM),
		TargetASTNode: targetNode,
	})
	if err != nil {
		return mcp.TextError("create agent failed: " + err.Error()), nil
	}

	rec := t.registry.Register(res.WasmAgentID, res.SkillDomain, p.TargetASTNode)
	streamURI := dispatch.AgentTelemetryURI(res.WasmAgentID)
	if rec != nil {
		streamURI = rec.StreamURI
	}

	event := dispatch.AgentTelemetryEvent{
		WasmAgentID:   res.WasmAgentID,
		SkillDomain:   res.SkillDomain,
		SliceSHA256:   res.SliceSHA256,
		State:         res.State,
		ColdStartMS:   res.ColdStartMS,
		ResidentRAMMB: res.ResidentRAMMB,
		TokensPerSec:  res.TokensPerSec,
		TargetASTNode: p.TargetASTNode,
		EmittedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	if t.pub != nil {
		_ = t.pub.Publish(dispatch.TopicAgentTelemetry, event)
	}

	out := map[string]any{
		"wasm_agent_id":   res.WasmAgentID,
		"cold_start_ms":   res.ColdStartMS,
		"resident_ram_mb": res.ResidentRAMMB,
		"stream_resource": streamURI,
		"skill_domain":    res.SkillDomain,
		"slice_sha256":    res.SliceSHA256,
		"state":           res.State,
	}
	b, _ := json.Marshal(out)
	return mcp.TextResult(string(b)), nil
}
