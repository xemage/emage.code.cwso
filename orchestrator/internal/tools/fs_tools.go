// Package tools — baseline filesystem tools (Phase 1).
//
// SECURITY: All tools accept paths that are validated to lie within the
// configured Workspace root. Symlink traversal is rejected. No write access
// outside the workspace.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emage/cwso/orchestrator/internal/mcp"
)

// pathGuard validates that targetPath resolves inside root. Symlinks that
// escape root are rejected.
func pathGuard(root, targetPath string) (string, error) {
	if root == "" {
		return "", errors.New("workspace root not configured")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	candidate := targetPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(absRoot, candidate)
	}
	clean := filepath.Clean(candidate)
	rel, err := filepath.Rel(absRoot, clean)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q escapes workspace root", targetPath)
	}
	// Resolve symlinks if target exists; reject if it escapes.
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		relResolved, err := filepath.Rel(absRoot, resolved)
		if err != nil || strings.HasPrefix(relResolved, "..") {
			return "", fmt.Errorf("path %q symlinks outside workspace root", targetPath)
		}
		return resolved, nil
	}
	return clean, nil
}

// ReadFileSync reads a UTF-8 file from inside the configured workspace.
type ReadFileSync struct{ Workspace string }

// Name returns the tool name.
func (t *ReadFileSync) Name() string { return "read_file_sync" }

// Description returns the human-readable tool description.
func (t *ReadFileSync) Description() string {
	return "Synchronously read a UTF-8 file from inside the configured workspace root."
}

// InputSchema returns the JSON Schema for tool input.
func (t *ReadFileSync) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Workspace-relative or absolute path inside workspace."},
		},
		"required": []string{"path"},
	}
}

// AllowedRoles lists which permission tiers may invoke this tool.
func (t *ReadFileSync) AllowedRoles() []Role { return []Role{RoleOrchestrator, RoleWorker} }

// Execute runs the tool.
func (t *ReadFileSync) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return mcp.TextError("invalid arguments: " + err.Error()), nil
	}
	if p.Path == "" {
		return mcp.TextError("path is required"), nil
	}
	safe, err := pathGuard(t.Workspace, p.Path)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	const maxSize = 1 << 20 // 1 MiB cap for Phase 1
	info, err := os.Stat(safe)
	if err != nil {
		return mcp.TextError("stat: " + err.Error()), nil
	}
	if info.IsDir() {
		return mcp.TextError("path is a directory"), nil
	}
	if info.Size() > maxSize {
		return mcp.TextError(fmt.Sprintf("file exceeds %d byte cap", maxSize)), nil
	}
	b, err := os.ReadFile(safe)
	if err != nil {
		return mcp.TextError("read: " + err.Error()), nil
	}
	return mcp.TextResult(string(b)), nil
}

// WriteFileSync writes a UTF-8 file inside the workspace. Worker-only.
type WriteFileSync struct{ Workspace string }

// Name returns the tool name.
func (t *WriteFileSync) Name() string { return "write_file_sync" }

// Description returns the human-readable tool description.
func (t *WriteFileSync) Description() string {
	return "Synchronously write a UTF-8 file inside the configured workspace root. Worker-tier only."
}

// InputSchema returns the JSON Schema for tool input.
func (t *WriteFileSync) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		},
		"required": []string{"path", "content"},
	}
}

// AllowedRoles lists which permission tiers may invoke this tool.
func (t *WriteFileSync) AllowedRoles() []Role { return []Role{RoleWorker} }

// Execute runs the tool.
func (t *WriteFileSync) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return mcp.TextError("invalid arguments: " + err.Error()), nil
	}
	if p.Path == "" {
		return mcp.TextError("path is required"), nil
	}
	safe, err := pathGuard(t.Workspace, p.Path)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	if err := os.MkdirAll(filepath.Dir(safe), 0o755); err != nil {
		return mcp.TextError("mkdir: " + err.Error()), nil
	}
	if err := os.WriteFile(safe, []byte(p.Content), 0o644); err != nil {
		return mcp.TextError("write: " + err.Error()), nil
	}
	return mcp.TextResult(fmt.Sprintf("wrote %d bytes to %s", len(p.Content), p.Path)), nil
}

// ListDir lists the entries in a directory inside the workspace.
type ListDir struct{ Workspace string }

// Name returns the tool name.
func (t *ListDir) Name() string { return "list_dir" }

// Description returns the human-readable tool description.
func (t *ListDir) Description() string {
	return "List entries of a directory inside the configured workspace root."
}

// InputSchema returns the JSON Schema for tool input.
func (t *ListDir) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Workspace-relative directory (defaults to root)."},
		},
	}
}

// AllowedRoles lists which permission tiers may invoke this tool.
func (t *ListDir) AllowedRoles() []Role { return []Role{RoleOrchestrator, RoleWorker} }

// Execute runs the tool.
func (t *ListDir) Execute(_ context.Context, args json.RawMessage) (*mcp.ToolCallResult, error) {
	var p struct {
		Path string `json:"path"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return mcp.TextError("invalid arguments: " + err.Error()), nil
		}
	}
	if p.Path == "" {
		p.Path = "."
	}
	safe, err := pathGuard(t.Workspace, p.Path)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	entries, err := os.ReadDir(safe)
	if err != nil {
		return mcp.TextError("readdir: " + err.Error()), nil
	}
	var b strings.Builder
	for _, e := range entries {
		kind := "file"
		if e.IsDir() {
			kind = "dir"
		}
		fmt.Fprintf(&b, "%s\t%s\n", kind, e.Name())
	}
	return mcp.TextResult(b.String()), nil
}
