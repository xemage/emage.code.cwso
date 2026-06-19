// Package tools — shadow workspace tools (Phase 2).
//
// These tools front the cwso-git-shadow sidecar over a Unix Domain Socket.
// All file operations target the in-memory libgit2 ODB; nothing touches the
// host filesystem.
//
// Permission tiers per architecture-v1.md §4:
//   - create_shadow_workspace, query_ast       → orchestrator + worker
//   - read_shadow_file                         → orchestrator + worker
//   - write_shadow_file, commit_shadow         → worker only
//   - drop_shadow_workspace                    → orchestrator + worker
package tools

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/shadow"
)

// shadowTool is the embeddable base for sidecar-backed tools.
type shadowTool struct {
	client *shadow.Client
}

// --- create_shadow_workspace ---

// CreateShadowWorkspace allocates a new in-memory shadow workspace.
type CreateShadowWorkspace struct{ shadowTool }

// NewCreateShadowWorkspace constructs the tool.
func NewCreateShadowWorkspace(c *shadow.Client) *CreateShadowWorkspace {
	return &CreateShadowWorkspace{shadowTool{client: c}}
}

// Name returns the MCP tool name.
func (t *CreateShadowWorkspace) Name() string { return "create_shadow_workspace" }

// Description returns the human-readable description.
func (t *CreateShadowWorkspace) Description() string {
	return "Allocate an isolated in-memory shadow workspace backed by libgit2."
}

// InputSchema returns the JSON schema for arguments.
func (t *CreateShadowWorkspace) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"base_commit_sha": map[string]any{
				"type":        "string",
				"description": "Optional base commit OID to seed the workspace from.",
			},
		},
	}
}

// AllowedRoles lists which tiers may invoke this tool.
func (t *CreateShadowWorkspace) AllowedRoles() []Role {
	return []Role{RoleOrchestrator, RoleWorker}
}

// Execute runs the tool.
func (t *CreateShadowWorkspace) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	var p struct {
		BaseCommitSHA string `json:"base_commit_sha,omitempty"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return mcp.TextError("invalid arguments: " + err.Error()), nil
		}
	}
	params := map[string]any{"base_commit_sha": nil}
	if p.BaseCommitSHA != "" {
		params["base_commit_sha"] = p.BaseCommitSHA
	}
	var out struct {
		WorkspaceUUID string  `json:"workspace_uuid"`
		BaseTreeOID   *string `json:"base_tree_oid"`
	}
	if err := t.client.Call("create_workspace", params, &out); err != nil {
		return mcp.TextError(err.Error()), nil
	}
	b, _ := json.Marshal(out)
	return mcp.TextResult(string(b)), nil
}

// --- drop_shadow_workspace ---

// DropShadowWorkspace releases an in-memory shadow workspace.
type DropShadowWorkspace struct{ shadowTool }

// NewDropShadowWorkspace constructs the tool.
func NewDropShadowWorkspace(c *shadow.Client) *DropShadowWorkspace {
	return &DropShadowWorkspace{shadowTool{client: c}}
}

// Name returns the MCP tool name.
func (t *DropShadowWorkspace) Name() string { return "drop_shadow_workspace" }

// Description returns the human-readable description.
func (t *DropShadowWorkspace) Description() string {
	return "Release a shadow workspace and free its in-memory state."
}

// InputSchema returns the JSON schema for arguments.
func (t *DropShadowWorkspace) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"workspace_uuid": map[string]any{"type": "string"},
		},
		"required": []string{"workspace_uuid"},
	}
}

// AllowedRoles lists which tiers may invoke this tool.
func (t *DropShadowWorkspace) AllowedRoles() []Role { return []Role{RoleOrchestrator, RoleWorker} }

// Execute runs the tool.
func (t *DropShadowWorkspace) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	var p struct {
		WorkspaceUUID string `json:"workspace_uuid"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return mcp.TextError("invalid arguments: " + err.Error()), nil
	}
	if p.WorkspaceUUID == "" {
		return mcp.TextError("workspace_uuid is required"), nil
	}
	var out map[string]any
	if err := t.client.Call("drop_workspace", map[string]any{"workspace_uuid": p.WorkspaceUUID}, &out); err != nil {
		return mcp.TextError(err.Error()), nil
	}
	b, _ := json.Marshal(out)
	return mcp.TextResult(string(b)), nil
}

// --- read_shadow_file ---

// ReadShadowFile reads a file from a shadow workspace.
type ReadShadowFile struct{ shadowTool }

// NewReadShadowFile constructs the tool.
func NewReadShadowFile(c *shadow.Client) *ReadShadowFile {
	return &ReadShadowFile{shadowTool{client: c}}
}

// Name returns the MCP tool name.
func (t *ReadShadowFile) Name() string { return "read_shadow_file" }

// Description returns the human-readable description.
func (t *ReadShadowFile) Description() string {
	return "Read a file from a shadow workspace (in-memory libgit2 ODB). UTF-8 expected."
}

// InputSchema returns the JSON schema for arguments.
func (t *ReadShadowFile) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"workspace_uuid": map[string]any{"type": "string"},
			"path":           map[string]any{"type": "string"},
		},
		"required": []string{"workspace_uuid", "path"},
	}
}

// AllowedRoles lists which tiers may invoke this tool.
func (t *ReadShadowFile) AllowedRoles() []Role { return []Role{RoleOrchestrator, RoleWorker} }

// Execute runs the tool.
func (t *ReadShadowFile) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	var p struct {
		WorkspaceUUID string `json:"workspace_uuid"`
		Path          string `json:"path"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return mcp.TextError("invalid arguments: " + err.Error()), nil
	}
	if p.WorkspaceUUID == "" || p.Path == "" {
		return mcp.TextError("workspace_uuid and path are required"), nil
	}
	var out struct {
		ContentB64 string `json:"content_b64"`
		Size       int    `json:"size"`
	}
	if err := t.client.Call("read_file", map[string]any{
		"workspace_uuid": p.WorkspaceUUID,
		"path":           p.Path,
	}, &out); err != nil {
		return mcp.TextError(err.Error()), nil
	}
	dec, err := base64.StdEncoding.DecodeString(out.ContentB64)
	if err != nil {
		return mcp.TextError("decode content: " + err.Error()), nil
	}
	return mcp.TextResult(string(dec)), nil
}

// --- write_shadow_file ---

// WriteShadowFile writes a file into a shadow workspace.
//
// When an observer is attached (T118), each successful write also emits a
// dispatch.WriteEvent so the AST write-spike monitor + semantic filter (T115/T116) observe
// live edits. This is the in-process write-event feeder for the spiking AST monitors.
type WriteShadowFile struct {
	shadowTool
	observer dispatch.WriteEventSink
}

// NewWriteShadowFile constructs the tool without a write-event feeder.
func NewWriteShadowFile(c *shadow.Client) *WriteShadowFile {
	return &WriteShadowFile{shadowTool: shadowTool{client: c}}
}

// NewWriteShadowFileWithObserver constructs the tool wired to a write-event sink so
// successful writes feed the AST spike monitors.
func NewWriteShadowFileWithObserver(c *shadow.Client, observer dispatch.WriteEventSink) *WriteShadowFile {
	return &WriteShadowFile{shadowTool: shadowTool{client: c}, observer: observer}
}

// Name returns the MCP tool name.
func (t *WriteShadowFile) Name() string { return "write_shadow_file" }

// Description returns the human-readable description.
func (t *WriteShadowFile) Description() string {
	return "Write a file into a shadow workspace's in-memory ODB. Worker-tier only."
}

// InputSchema returns the JSON schema for arguments.
func (t *WriteShadowFile) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"workspace_uuid": map[string]any{"type": "string"},
			"path":           map[string]any{"type": "string"},
			"content":        map[string]any{"type": "string", "description": "UTF-8 content."},
		},
		"required": []string{"workspace_uuid", "path", "content"},
	}
}

// AllowedRoles lists which tiers may invoke this tool.
func (t *WriteShadowFile) AllowedRoles() []Role { return []Role{RoleWorker} }

// Execute runs the tool.
func (t *WriteShadowFile) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	var p struct {
		WorkspaceUUID string `json:"workspace_uuid"`
		Path          string `json:"path"`
		Content       string `json:"content"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return mcp.TextError("invalid arguments: " + err.Error()), nil
	}
	if p.WorkspaceUUID == "" || p.Path == "" {
		return mcp.TextError("workspace_uuid and path are required"), nil
	}
	enc := base64.StdEncoding.EncodeToString([]byte(p.Content))
	var out struct {
		BlobOID string `json:"blob_oid"`
		Size    int    `json:"size"`
	}
	if err := t.client.Call("write_file", map[string]any{
		"workspace_uuid": p.WorkspaceUUID,
		"path":           p.Path,
		"content_b64":    enc,
	}, &out); err != nil {
		return mcp.TextError(err.Error()), nil
	}
	t.feedWriteEvent(p.WorkspaceUUID, p.Path, p.Content)
	return mcp.TextResult(fmt.Sprintf("wrote %d bytes (blob %s)", out.Size, out.BlobOID)), nil
}

// feedWriteEvent notifies the AST spike monitors of a successful write. The symbol surface
// is approximated by the file path and the signature by a content hash: this gives the
// volume monitor real write events and lets the semantic filter detect content changes and
// cross-workspace edits to the same file. AST-symbol-level extraction (via query_ast) is a
// later refinement; until then "symbol = file path" is a deliberate, documented PoC choice.
func (t *WriteShadowFile) feedWriteEvent(workspace, filePath, content string) {
	if t.observer == nil {
		return
	}
	_ = t.observer.ObserveWrite(dispatch.WriteEvent{
		Workspace:     workspace,
		Path:          filePath,
		Language:      languageFromPath(filePath),
		At:            time.Now().UTC(),
		Symbol:        filePath,
		SignatureHash: contentSignature(content),
	})
}

// languageFromPath maps a file extension to a tree-sitter language tag (matching the
// languages query_ast supports). Unknown extensions return "".
func languageFromPath(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	default:
		return ""
	}
}

func contentSignature(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// --- commit_shadow ---

// CommitShadow creates a commit from the workspace's current state.
type CommitShadow struct{ shadowTool }

// NewCommitShadow constructs the tool.
func NewCommitShadow(c *shadow.Client) *CommitShadow { return &CommitShadow{shadowTool{client: c}} }

// Name returns the MCP tool name.
func (t *CommitShadow) Name() string { return "commit_shadow" }

// Description returns the human-readable description.
func (t *CommitShadow) Description() string {
	return "Commit the shadow workspace's staged files into the bare repo. Returns commit and tree OIDs."
}

// InputSchema returns the JSON schema for arguments.
func (t *CommitShadow) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"workspace_uuid": map[string]any{"type": "string"},
			"message":        map[string]any{"type": "string"},
		},
		"required": []string{"workspace_uuid", "message"},
	}
}

// AllowedRoles lists which tiers may invoke this tool.
func (t *CommitShadow) AllowedRoles() []Role { return []Role{RoleWorker} }

// Execute runs the tool.
func (t *CommitShadow) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	var p struct {
		WorkspaceUUID string `json:"workspace_uuid"`
		Message       string `json:"message"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return mcp.TextError("invalid arguments: " + err.Error()), nil
	}
	if p.WorkspaceUUID == "" || p.Message == "" {
		return mcp.TextError("workspace_uuid and message are required"), nil
	}
	var out struct {
		TreeOID   string `json:"tree_oid"`
		CommitOID string `json:"commit_oid"`
	}
	if err := t.client.Call("commit", map[string]any{
		"workspace_uuid": p.WorkspaceUUID,
		"message":        p.Message,
	}, &out); err != nil {
		return mcp.TextError(err.Error()), nil
	}
	b, _ := json.Marshal(out)
	return mcp.TextResult(string(b)), nil
}

// --- query_ast ---

// QueryAST runs a tree-sitter query against a file in a shadow workspace.
type QueryAST struct{ shadowTool }

// NewQueryAST constructs the tool.
func NewQueryAST(c *shadow.Client) *QueryAST { return &QueryAST{shadowTool{client: c}} }

// Name returns the MCP tool name.
func (t *QueryAST) Name() string { return "query_ast" }

// Description returns the human-readable description.
func (t *QueryAST) Description() string {
	return "Run a tree-sitter AST query against a file in a shadow workspace. Supports Go, Python, Rust, and TypeScript."
}

// InputSchema returns the JSON schema for arguments.
func (t *QueryAST) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"workspace_uuid": map[string]any{"type": "string"},
			"path":           map[string]any{"type": "string"},
			"query_type": map[string]any{
				"type": "string",
				"enum": []string{"find_definition", "find_references", "extract_signature", "list_exports", "detect_entrypoints"},
			},
			"target_symbol": map[string]any{"type": "string"},
		},
		"required": []string{"workspace_uuid", "path", "query_type"},
	}
}

// AllowedRoles lists which tiers may invoke this tool.
func (t *QueryAST) AllowedRoles() []Role { return []Role{RoleOrchestrator, RoleWorker} }

// Execute runs the tool.
func (t *QueryAST) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	var p struct {
		WorkspaceUUID string `json:"workspace_uuid"`
		Path          string `json:"path"`
		QueryType     string `json:"query_type"`
		TargetSymbol  string `json:"target_symbol"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return mcp.TextError("invalid arguments: " + err.Error()), nil
	}
	if p.WorkspaceUUID == "" || p.Path == "" || p.QueryType == "" {
		return mcp.TextError("workspace_uuid, path, query_type are required"), nil
	}
	var out json.RawMessage
	if err := t.client.Call("query_ast", map[string]any{
		"workspace_uuid": p.WorkspaceUUID,
		"path":           p.Path,
		"query_type":     p.QueryType,
		"target_symbol":  p.TargetSymbol,
	}, &out); err != nil {
		return mcp.TextError(err.Error()), nil
	}
	return mcp.TextResult(string(out)), nil
}
