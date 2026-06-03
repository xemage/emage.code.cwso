package tools

import (
	"context"
	"encoding/json"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/mcp"
)

// SubscribeASTSpikes registers an AST write-spike subscription and returns a streamable
// resource handle (T117 / roadmap T097). The stream itself is delivered over the SSE
// transport scoped to the returned subscription id; events are gated to the requested
// semantic threshold, path glob, and workspace scope.
type SubscribeASTSpikes struct {
	registry *dispatch.SpikeSubscriptionRegistry
}

// NewSubscribeASTSpikes constructs the tool around a subscription registry.
func NewSubscribeASTSpikes(registry *dispatch.SpikeSubscriptionRegistry) *SubscribeASTSpikes {
	return &SubscribeASTSpikes{registry: registry}
}

// Name returns the MCP tool name.
func (t *SubscribeASTSpikes) Name() string { return "subscribe_ast_spikes" }

// Description returns the human-readable description.
func (t *SubscribeASTSpikes) Description() string {
	return "Subscribe to semantic AST write-spike events. Returns a cwso:// stream resource " +
		"that emits SSE notifications only when a write crosses the requested semantic threshold."
}

// InputSchema returns the JSON schema for arguments.
func (t *SubscribeASTSpikes) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Optional file path or glob (path.Match syntax) to scope the stream. Empty matches all.",
			},
			"semantic_threshold": map[string]any{
				"type":        "string",
				"enum":        []string{"signature_change", "symbol_added", "symbol_removed", "any"},
				"description": "Minimum semantic significance to emit. Defaults to signature_change.",
			},
			"workspace_scope": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional workspace UUIDs to scope the stream. Empty matches all.",
			},
		},
	}
}

// AllowedRoles lists which tiers may invoke this tool. Both orchestrator and worker agents
// may subscribe to conflict pre-warnings for their own workspaces.
func (t *SubscribeASTSpikes) AllowedRoles() []Role { return []Role{RoleOrchestrator, RoleWorker} }

// Execute registers the subscription and returns its handle.
func (t *SubscribeASTSpikes) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	if t.registry == nil {
		return mcp.TextError("ast spike subscriptions are not enabled on this server"), nil
	}
	var p struct {
		Path              string   `json:"path"`
		SemanticThreshold string   `json:"semantic_threshold"`
		WorkspaceScope    []string `json:"workspace_scope"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return mcp.TextError("invalid arguments: " + err.Error()), nil
		}
	}
	sub, err := t.registry.Create(p.Path, dispatch.SpikeKind(p.SemanticThreshold), p.WorkspaceScope)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	out := map[string]any{
		"subscription_id":    sub.ID,
		"stream_resource":    sub.URI(),
		"semantic_threshold": string(sub.SemanticThreshold),
		"path":               sub.Path,
		"workspace_scope":    sub.WorkspaceScope,
		"topics":             dispatch.ASTSpikeTopics(),
		"transport_hint":     "open GET /mcp?subscription=" + sub.ID + " (SSE) for the live stream, or resources/read for a snapshot",
	}
	b, _ := json.Marshal(out)
	return mcp.TextResult(string(b)), nil
}
