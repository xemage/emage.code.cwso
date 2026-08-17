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
//
// SECURITY: for targets that already exist on disk, filepath.EvalSymlinks
// resolves the full path and we check the resolved path against root. For
// targets that do NOT yet exist (e.g. a new file being written for the
// first time), EvalSymlinks cannot resolve the non-existent leaf, so we
// instead resolve symlinks on the nearest EXISTING ancestor directory and
// re-verify the non-existent tail rejoined onto that resolved ancestor.
// Without this, a symlinked intermediate directory pointing outside root
// would let a new-file write escape the workspace unchecked.
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
		if !withinWorkspace(absRoot, resolved) {
			return "", fmt.Errorf("path %q symlinks outside workspace root", targetPath)
		}
		return resolved, nil
	}
	// The target itself doesn't exist yet (EvalSymlinks requires every path
	// component, including the leaf, to exist). Resolve symlinks on the
	// nearest existing ancestor directory instead, then rejoin the
	// non-existent tail components and re-verify the rejoined path is still
	// inside the workspace before trusting it.
	resolvedAncestor, tail, err := resolveNearestExistingAncestor(absRoot, clean)
	if err != nil {
		return "", fmt.Errorf("path %q: %w", targetPath, err)
	}
	if !withinWorkspace(absRoot, resolvedAncestor) {
		return "", fmt.Errorf("path %q symlinks outside workspace root", targetPath)
	}
	joined := filepath.Clean(filepath.Join(append([]string{resolvedAncestor}, tail...)...))
	if !withinWorkspace(absRoot, joined) {
		return "", fmt.Errorf("path %q symlinks outside workspace root", targetPath)
	}
	return joined, nil
}

// withinWorkspace reports whether resolved lies inside (or equals) absRoot.
// Both arguments must already be absolute, cleaned paths.
func withinWorkspace(absRoot, resolved string) bool {
	rel, err := filepath.Rel(absRoot, resolved)
	return err == nil && !strings.HasPrefix(rel, "..")
}

// resolveNearestExistingAncestor walks p upward (via filepath.Dir) until it
// finds an ancestor directory that exists on disk, then resolves symlinks
// on that ancestor via filepath.EvalSymlinks. It returns the resolved
// ancestor path plus the non-existent tail path components (in
// root-to-leaf order) that the caller must rejoin onto the resolved
// ancestor and re-check against the workspace root.
//
// The walk is bounded by absRoot: p is assumed to already lie lexically
// inside absRoot (pathGuard checks this before calling in), so the walk
// will reach absRoot if nothing more specific exists. If absRoot itself
// does not exist, or EvalSymlinks fails on an existing ancestor for a
// reason other than the path not existing (e.g. a permission error), this
// returns an error rather than silently falling back to an unresolved,
// unchecked path.
func resolveNearestExistingAncestor(absRoot, p string) (string, []string, error) {
	var tail []string
	cur := p
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return resolved, tail, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", nil, err
		}
		if cur == absRoot {
			return "", nil, fmt.Errorf("workspace root %q does not exist", absRoot)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached the filesystem root without ever hitting absRoot.
			// Should be unreachable given pathGuard's earlier lexical
			// containment check, but guard against it defensively rather
			// than looping forever or resolving against something outside
			// the workspace.
			return "", nil, fmt.Errorf("no existing ancestor found for %q", p)
		}
		tail = append([]string{filepath.Base(cur)}, tail...)
		cur = parent
	}
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
