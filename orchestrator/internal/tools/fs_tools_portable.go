//go:build !linux

// Package tools — portable (non-Linux) fallback build for fs_tools.go
// (T194's `//go:build linux` file).
//
// WHY THIS FILE EXISTS: fs_tools.go's TOCTOU-closing mechanism (T194) needs
// syscall.Openat/Mkdirat, which Go's standard library only exposes for
// GOOS=linux. Because that file is now gated `//go:build linux` for its
// ENTIRE contents — not just the *at()-anchored logic, but ReadFileSync,
// WriteFileSync, and ListDir themselves — `internal/server/server.go`
// (which references those three types unconditionally, with no build tag)
// fails to compile on any non-Linux GOOS, and the whole T193+T194
// regression test suite silently disappears from `go test ./...` on
// non-Linux machines. This file restores buildability by providing the
// SAME exported surface (same struct shape, same Name()/Description()/
// InputSchema()/AllowedRoles()/Execute() methods, same tool names) so
// server.go compiles unmodified against either build tag.
//
// SECURITY — READ THIS BEFORE RELYING ON THIS BUILD FOR ANYTHING SECURITY-
// SENSITIVE:
//
// This build's TOCTOU guarantee is STRICTLY NARROWER than the Linux build's
// in fs_tools.go. The Linux build closes the check-then-use race with
// kernel-enforced atomicity: each path-component hop is opened via
// openat(2) anchored to the file descriptor obtained by the PREVIOUS hop,
// with O_NOFOLLOW on every hop including the leaf, so there is no second,
// separately-timed name lookup left for an attacker to win a race against.
//
// This build has no portable equivalent of openat(2)/O_NOFOLLOW-anchored
// walks available in Go's standard library across all non-Linux targets, so
// it instead RE-VERIFIES path containment and symlink-safety (via
// pathGuard, see below) a second time, IMMEDIATELY before the actual
// os.Open/os.ReadFile/os.WriteFile/os.MkdirAll/os.ReadDir call that follows
// it — minimizing, but NOT eliminating, the check-then-use window. A
// symlink swap that lands exactly inside that shortened re-verify-to-use
// gap is still THEORETICALLY POSSIBLE on this build. Do not represent this
// build as providing the same guarantee as the Linux build; it does not.
// CWSO's actual and only deployment target
// (deploy/Dockerfile.orchestrator, alpine:3.20) builds and ships the Linux
// file exclusively — this portable file exists solely so the module
// compiles, vets, and has test coverage on non-Linux developer machines; it
// is not the code path CI or production ever exercises.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/emage/cwso/orchestrator/internal/mcp"
)

// pathGuard validates that targetPath resolves inside root. Symlinks that
// escape root are rejected.
//
// This is a verbatim port of T193's fix from fs_tools.go's pathGuard: the
// symlink-resolution logic itself has no syscall.Openat/Mkdirat dependency
// and is fully portable, so it is reused here unchanged rather than
// reimplemented, to avoid regressing the T193 fix on non-Linux builds.
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
	absRoot, err := absWorkspaceRoot(root)
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

// absWorkspaceRoot validates and returns the absolute form of the
// configured workspace root. Shared by pathGuard and reverifyBeforeUse
// below so both use exactly the same trust anchor.
func absWorkspaceRoot(root string) (string, error) {
	if root == "" {
		return "", errors.New("workspace root not configured")
	}
	return filepath.Abs(root)
}

// reverifyBeforeUse re-runs pathGuard's full containment/symlink check on
// an already-pathGuard-verified `safe` path, immediately before the actual
// filesystem operation that follows it in each caller.
//
// SECURITY (T195 — the portable, NARROWER counterpart of T194's Linux
// fd-anchored fix): pathGuard's first call, earlier in each Execute method,
// only proves `safe` was free of escaping symlinks AT THAT MOMENT. Time
// then passes (argument validation, error-message formatting, etc.) before
// the actual os.Open/os.ReadFile/os.WriteFile/os.MkdirAll/os.ReadDir call.
// This function re-runs the identical check a second time, called as the
// LAST thing before that filesystem call, so any symlink swapped in during
// the (now much smaller) window between THIS check and the call right
// after it would still be caught if it were swapped in before this
// specific call — but not if it is swapped in during the residual gap
// between this function returning and the very next line executing. That
// residual gap is a real, unclosed race window on this build: unlike
// fs_tools.go's Linux openat()/O_NOFOLLOW walk, which is refused
// atomically by the kernel with no separate name lookup left to race
// against, this is two ordinary, independently-timed path-string lookups —
// narrower, not equivalent. See the package doc comment above for the full
// explanation.
func reverifyBeforeUse(workspace, safe string) (string, error) {
	return pathGuard(workspace, safe)
}

// ReadFileSync reads a UTF-8 file from inside the configured workspace.
//
// See the package doc comment above (fs_tools_portable.go) for this
// build's narrower TOCTOU guarantee relative to fs_tools.go's Linux build.
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
	// SECURITY (T195): re-verify immediately before the actual open — see
	// reverifyBeforeUse's doc comment for what this does and does not
	// guarantee on this build.
	reverified, err := reverifyBeforeUse(t.Workspace, safe)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	f, err := os.Open(reverified)
	if err != nil {
		return mcp.TextError("open: " + err.Error()), nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return mcp.TextError("stat: " + err.Error()), nil
	}
	if info.IsDir() {
		return mcp.TextError("path is a directory"), nil
	}
	const maxSize = 1 << 20 // 1 MiB cap for Phase 1
	if info.Size() > maxSize {
		return mcp.TextError(fmt.Sprintf("file exceeds %d byte cap", maxSize)), nil
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return mcp.TextError("read: " + err.Error()), nil
	}
	return mcp.TextResult(string(b)), nil
}

// WriteFileSync writes a UTF-8 file inside the workspace. Worker-only.
//
// See the package doc comment above (fs_tools_portable.go) for this
// build's narrower TOCTOU guarantee relative to fs_tools.go's Linux build.
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
	// SECURITY (T195): re-verify immediately before creating parent
	// directories and writing — see reverifyBeforeUse's doc comment for
	// what this does and does not guarantee on this build. MkdirAll and
	// WriteFile below are ordinary path-string operations (no *at()/
	// O_NOFOLLOW anchoring is available portably), so a symlink swapped in
	// between this re-verify call and either of those calls is not caught.
	reverified, err := reverifyBeforeUse(t.Workspace, safe)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	if err := os.MkdirAll(filepath.Dir(reverified), 0o755); err != nil {
		return mcp.TextError("write: " + err.Error()), nil
	}
	if err := os.WriteFile(reverified, []byte(p.Content), 0o644); err != nil {
		return mcp.TextError("write: " + err.Error()), nil
	}
	return mcp.TextResult(fmt.Sprintf("wrote %d bytes to %s", len(p.Content), p.Path)), nil
}

// ListDir lists the entries in a directory inside the workspace.
//
// See the package doc comment above (fs_tools_portable.go) for this
// build's narrower TOCTOU guarantee relative to fs_tools.go's Linux build.
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
	// SECURITY (T195): re-verify immediately before reading the directory
	// — see reverifyBeforeUse's doc comment for what this does and does
	// not guarantee on this build.
	reverified, err := reverifyBeforeUse(t.Workspace, safe)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	entries, err := os.ReadDir(reverified)
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
