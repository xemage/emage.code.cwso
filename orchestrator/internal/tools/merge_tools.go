package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"

	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/mergeengine"
)

const (
	mergeOpThreeWay = "merge_three_way"
)

// MergeConcurrentResults merges concurrent workspace outputs through the
// cwso-merge-engine sidecar and returns stable, structured outcomes.
type MergeConcurrentResults struct {
	client *mergeengine.Client
}

// NewMergeConcurrentResults constructs the tool.
func NewMergeConcurrentResults(c *mergeengine.Client) *MergeConcurrentResults {
	return &MergeConcurrentResults{client: c}
}

// Name returns the MCP tool name.
func (t *MergeConcurrentResults) Name() string { return "merge_concurrent_results" }

// Description returns the human-readable description.
func (t *MergeConcurrentResults) Description() string {
	return "Merge concurrent shadow results via semantic AST-aware merge engine."
}

// InputSchema returns the JSON schema for arguments.
func (t *MergeConcurrentResults) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source_workspace_uuids": map[string]any{
				"type":     "array",
				"minItems": 2,
				"items":    map[string]any{"type": "string", "format": "uuid"},
			},
			"target_branch_ref": map[string]any{"type": "string", "default": "main"},
			"auto_resolve_heuristic": map[string]any{
				"type":    "string",
				"enum":    []string{"ast_semantic_only", "prefer_theirs", "prefer_ours", "fail_rapidly_on_conflict"},
				"default": "ast_semantic_only",
			},
			"merge_inputs": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":           map[string]any{"type": "string", "minLength": 1},
						"language":       map[string]any{"type": "string", "enum": []string{"go", "rust", "python", "typescript"}},
						"base_content":   map[string]any{"type": "string"},
						"ours_content":   map[string]any{"type": "string"},
						"theirs_content": map[string]any{"type": "string"},
					},
					"required":             []string{"path", "language", "base_content", "ours_content", "theirs_content"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"source_workspace_uuids"},
		"additionalProperties": false,
	}
}

// AllowedRoles lists which tiers may invoke this tool.
func (t *MergeConcurrentResults) AllowedRoles() []Role { return []Role{RoleOrchestrator} }

type mergeConcurrentArgs struct {
	SourceWorkspaceUUIDs []string     `json:"source_workspace_uuids"`
	TargetBranchRef      string       `json:"target_branch_ref"`
	AutoResolveHeuristic string       `json:"auto_resolve_heuristic"`
	MergeInputs          []mergeInput `json:"merge_inputs"`
}

type mergeInput struct {
	Path          string `json:"path"`
	Language      string `json:"language"`
	BaseContent   string `json:"base_content"`
	OursContent   string `json:"ours_content"`
	TheirsContent string `json:"theirs_content"`
}

type mergeResultItem struct {
	Path          string `json:"path"`
	Language      string `json:"language"`
	Status        string `json:"status"`
	ReasonCode    string `json:"reason_code"`
	MergedContent string `json:"merged_content,omitempty"`
	Message       string `json:"message,omitempty"`
}

type mergeConcurrentOutput struct {
	Outcome              string            `json:"outcome"`
	TargetBranchRef      string            `json:"target_branch_ref"`
	AutoResolveHeuristic string            `json:"auto_resolve_heuristic"`
	SourceWorkspaceUUIDs []string          `json:"source_workspace_uuids"`
	MergedCount          int               `json:"merged_count"`
	ConflictCount        int               `json:"conflict_count"`
	FailureCount         int               `json:"failure_count"`
	Results              []mergeResultItem `json:"results"`
}

type mergeEngineSuccess struct {
	MergedB64 string `json:"merged_b64"`
}

func (t *MergeConcurrentResults) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	var p mergeConcurrentArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return mcp.TextError("invalid arguments: " + err.Error()), nil
	}
	if len(p.SourceWorkspaceUUIDs) < 2 {
		return mcp.TextError("source_workspace_uuids requires at least 2 items"), nil
	}
	if len(p.MergeInputs) == 0 {
		return mcp.TextError("merge_inputs requires at least 1 item for IPC merge execution"), nil
	}
	if p.TargetBranchRef == "" {
		p.TargetBranchRef = "main"
	}
	if p.AutoResolveHeuristic == "" {
		p.AutoResolveHeuristic = "ast_semantic_only"
	}

	// Ensure deterministic ordering regardless of caller argument ordering.
	inputs := make([]mergeInput, len(p.MergeInputs))
	copy(inputs, p.MergeInputs)
	sort.SliceStable(inputs, func(i, j int) bool {
		if inputs[i].Path == inputs[j].Path {
			return inputs[i].Language < inputs[j].Language
		}
		return inputs[i].Path < inputs[j].Path
	})

	out := mergeConcurrentOutput{
		TargetBranchRef:      p.TargetBranchRef,
		AutoResolveHeuristic: p.AutoResolveHeuristic,
		SourceWorkspaceUUIDs: p.SourceWorkspaceUUIDs,
		Results:              make([]mergeResultItem, 0, len(inputs)),
	}

	for _, input := range inputs {
		item := mergeResultItem{Path: input.Path, Language: input.Language}
		if input.Path == "" || input.Language == "" {
			item.Status = "error"
			item.ReasonCode = "invalid_input"
			item.Message = "path and language are required"
			out.FailureCount++
			out.Results = append(out.Results, item)
			continue
		}

		engineResult, err := t.mergeThreeWay(input)
		if err != nil {
			mapToolMergeError(&item, err)
			if item.Status == "conflict" {
				out.ConflictCount++
			} else {
				out.FailureCount++
			}
			out.Results = append(out.Results, item)
			continue
		}

		mergedBytes, decodeErr := base64.StdEncoding.DecodeString(engineResult.MergedB64)
		if decodeErr != nil {
			item.Status = "error"
			item.ReasonCode = "invalid_engine_payload"
			item.Message = "merge-engine returned invalid merged payload"
			out.FailureCount++
			out.Results = append(out.Results, item)
			continue
		}

		item.Status = "merged"
		item.ReasonCode = "semantic_merge_success"
		item.MergedContent = string(mergedBytes)
		out.MergedCount++
		out.Results = append(out.Results, item)
	}

	switch {
	case out.FailureCount > 0:
		out.Outcome = "error"
	case out.ConflictCount > 0:
		out.Outcome = "conflict"
	default:
		out.Outcome = "success"
	}

	b, _ := json.Marshal(out)
	return mcp.TextResult(string(b)), nil
}

func (t *MergeConcurrentResults) mergeThreeWay(input mergeInput) (mergeEngineSuccess, error) {
	params := map[string]any{
		"language":   input.Language,
		"base_b64":   base64.StdEncoding.EncodeToString([]byte(input.BaseContent)),
		"ours_b64":   base64.StdEncoding.EncodeToString([]byte(input.OursContent)),
		"theirs_b64": base64.StdEncoding.EncodeToString([]byte(input.TheirsContent)),
	}
	var out mergeEngineSuccess
	err := t.client.Call(mergeOpThreeWay, params, &out)
	return out, err
}

func mapToolMergeError(item *mergeResultItem, err error) {
	var sidecarErr *mergeengine.SidecarError
	if errors.As(err, &sidecarErr) {
		switch sidecarErr.Code {
		case "unimplemented_conflict":
			item.Status = "conflict"
			item.ReasonCode = "semantic_conflict"
			item.Message = sidecarErr.Message
		case "invalid_input":
			item.Status = "error"
			item.ReasonCode = "invalid_input"
			item.Message = "merge-engine rejected input"
		default:
			item.Status = "error"
			item.ReasonCode = "merge_engine_error"
			item.Message = sidecarErr.Message
		}
		return
	}

	item.Status = "error"
	item.ReasonCode = "merge_engine_unavailable"
	item.Message = "merge-engine IPC call failed"
}
