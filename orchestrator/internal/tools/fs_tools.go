//go:build linux

// Package tools — baseline filesystem tools (Phase 1).
//
// SECURITY: All tools accept paths that are validated to lie within the
// configured Workspace root. Symlink traversal is rejected. No write access
// outside the workspace.
//
// SECURITY (T194): pathGuard() only proves a path is safe AT THE MOMENT OF
// THE CHECK. Every caller's actual filesystem operation (open/read/write/
// mkdir/readdir) happens afterward, as a separate step — a classic
// check-then-use (TOCTOU) race. To close that window, the three callers
// below (ReadFileSync, WriteFileSync, ListDir) do not hand pathGuard's
// resolved path to a second, independently-timed, path-string-based
// os.Open/os.Stat/os.ReadFile/os.ReadDir/os.MkdirAll call. Instead they walk
// the already-verified canonical path one directory component at a time via
// openat(2), anchoring each hop to the file descriptor obtained by the
// PREVIOUS hop (see secureResolveDirs/secureOpenLeaf below), with
// O_NOFOLLOW on every hop including the final one. If a symlink is swapped
// into any path component after pathGuard's check but before this walk
// runs, the corresponding openat() call is refused by the kernel at that
// exact hop (ELOOP, or ENOTDIR when O_DIRECTORY is combined with
// O_NOFOLLOW against a symlink — verified empirically, see
// isSymlinkOpenRejection in fs_tools_test.go) — there is no second
// top-down name lookup left for an attacker to win a race against.
//
// This requires syscall.Openat/Mkdirat, which Go's standard library only
// exposes for GOOS=linux (verified: Go's darwin syscall package does not
// wrap openat(2)/mkdirat(2) under these names, and Windows has no
// equivalent at all). That matches CWSO's actual and only deployment
// target: deploy/Dockerfile.orchestrator builds on golang:1.25-alpine and
// ships on alpine:3.20 (Linux-only), and .gitlab-ci.yml / Makefile run
// `go build`/`go test` exclusively inside Linux Docker containers — no CI
// job, Makefile target, or release tooling in this repo builds for a
// non-Linux GOOS today. This `//go:build linux` constraint is therefore a
// deliberate scope decision, not an oversight — see MR description for the
// residual-risk note on hypothetical future non-Linux dev builds.
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
	"syscall"

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
// configured workspace root. Shared by pathGuard and the *at()-anchored
// helpers below so both use exactly the same trust anchor.
func absWorkspaceRoot(root string) (string, error) {
	if root == "" {
		return "", errors.New("workspace root not configured")
	}
	return filepath.Abs(root)
}

// secureResolveDirs walks the parent-directory chain of a pathGuard-
// verified, canonical path (safe) one component at a time, opening each
// directory with openat(2) using O_DIRECTORY|O_NOFOLLOW, anchored to the
// file descriptor returned by the PREVIOUS hop rather than a fresh,
// independently-timed name lookup from the top.
//
// SECURITY (T194 — closes the TOCTOU window left after pathGuard's check):
// pathGuard's returned `safe` path is, AT CHECK TIME, guaranteed to contain
// no symlink components — filepath.EvalSymlinks fully resolves every
// component for existing targets, and resolveNearestExistingAncestor
// resolves every EXISTING ancestor for new-file targets (the non-existent
// tail is, by definition, not a symlink because it doesn't exist yet).
// Under normal operation this walk is therefore a no-op verification: none
// of the openat() calls encounter a symlink, so all succeed exactly as a
// plain path-based open would have. But if a symlink is swapped into ANY
// directory component between pathGuard's check and this call — the
// precondition C015 is expected to introduce — the corresponding openat()
// call sees a name that now resolves to a symlink, and O_NOFOLLOW makes the
// KERNEL refuse it atomically (ELOOP, or ENOTDIR for the O_DIRECTORY
// combination used here — either way, a kernel-enforced rejection, not a
// second userspace check) at that exact hop. There is no second,
// separately-timed name resolution left for a racing process to win
// against: each hop is committed to the fd obtained by the previous hop,
// never to a string re-walked from absRoot a second time.
//
// If createDirs is true, an ENOENT on a hop creates that directory
// (mkdirat, tolerating a benign EEXIST race) and retries the hop once —
// this reproduces os.MkdirAll's "create missing intermediate directories"
// behavior for WriteFileSync without ever reopening from a path string. If
// the created (or already-existing) entry turns out not to be a plain,
// non-symlink directory (e.g. an attacker pre-placed a symlink or a
// regular file at that name), the retried O_NOFOLLOW|O_DIRECTORY openat()
// still correctly rejects it (ELOOP or ENOTDIR).
//
// Returns the file descriptor of the immediate parent directory of the
// final path component (caller owns it and must close it, directly or via
// secureOpenLeaf) and that final component's name.
func secureResolveDirs(absRoot, safe string, createDirs bool) (parentFd int, leaf string, err error) {
	rel, err := filepath.Rel(absRoot, safe)
	if err != nil {
		return -1, "", fmt.Errorf("resolve %q relative to workspace root: %w", safe, err)
	}
	rel = filepath.Clean(rel)
	if rel != "." && strings.HasPrefix(rel, "..") {
		// Defense in depth: should be unreachable because pathGuard already
		// verified `safe` lies inside absRoot, but never trust a second,
		// later-computed relative path implicitly.
		return -1, "", fmt.Errorf("path %q escapes workspace root", safe)
	}

	rootFd, err := syscall.Open(absRoot, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return -1, "", fmt.Errorf("open workspace root: %w", err)
	}

	if rel == "." {
		// safe IS the workspace root itself (e.g. list_dir on "."). There
		// is no separate parent to anchor to; the root fd already opened
		// above (with O_DIRECTORY, no O_NOFOLLOW needed — the root is the
		// operator-configured trust anchor, the same assumption pathGuard
		// itself makes) is both the parent and the leaf.
		return rootFd, ".", nil
	}

	parts := strings.Split(rel, string(filepath.Separator))
	dirFd := rootFd
	for i, part := range parts[:len(parts)-1] {
		const dirFlags = syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
		next, oerr := syscall.Openat(dirFd, part, dirFlags, 0)
		if oerr != nil && createDirs && errors.Is(oerr, syscall.ENOENT) {
			if merr := syscall.Mkdirat(dirFd, part, 0o755); merr != nil && !errors.Is(merr, syscall.EEXIST) {
				_ = syscall.Close(dirFd)
				return -1, "", fmt.Errorf("mkdirat %q: %w", part, merr)
			}
			next, oerr = syscall.Openat(dirFd, part, dirFlags, 0)
		}
		if oerr != nil {
			_ = syscall.Close(dirFd)
			return -1, "", fmt.Errorf("openat %q (path component %d of %q): %w", part, i, rel, oerr)
		}
		_ = syscall.Close(dirFd)
		dirFd = next
	}
	return dirFd, parts[len(parts)-1], nil
}

// secureOpenLeaf opens the final path component of a pathGuard-verified,
// canonical path using openat(2) anchored to the parent directory fd
// obtained from secureResolveDirs, with O_NOFOLLOW applied to this final
// hop too — so a symlink swapped into the leaf name itself (not just an
// intermediate directory) after pathGuard's check is refused by the kernel
// the same way. See secureResolveDirs for the full reasoning.
func secureOpenLeaf(absRoot, safe string, flags int, mode uint32, createDirs bool) (*os.File, error) {
	parentFd, leaf, err := secureResolveDirs(absRoot, safe, createDirs)
	if err != nil {
		return nil, err
	}
	if leaf == "." {
		// safe IS the workspace root; secureResolveDirs already opened it
		// with O_DIRECTORY above. There is no distinct leaf to O_NOFOLLOW
		// open relative to itself.
		return os.NewFile(uintptr(parentFd), safe), nil
	}
	fd, oerr := syscall.Openat(parentFd, leaf, flags|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, mode)
	_ = syscall.Close(parentFd)
	if oerr != nil {
		return nil, oerr
	}
	return os.NewFile(uintptr(fd), safe), nil
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
	absRoot, err := absWorkspaceRoot(t.Workspace)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	const maxSize = 1 << 20 // 1 MiB cap for Phase 1
	// SECURITY (T194): open via the *at()-anchored, O_NOFOLLOW walk instead
	// of a second, independently-timed os.Stat/os.ReadFile by path string —
	// see secureResolveDirs/secureOpenLeaf for why this closes the
	// check-then-use window left after pathGuard's check above.
	f, err := secureOpenLeaf(absRoot, safe, syscall.O_RDONLY, 0, false)
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
	absRoot, err := absWorkspaceRoot(t.Workspace)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	// SECURITY (T194): create missing parent directories and open the leaf
	// via the *at()-anchored, O_NOFOLLOW walk instead of a second,
	// independently-timed os.MkdirAll/os.WriteFile by path string — see
	// secureResolveDirs/secureOpenLeaf for why this closes the
	// check-then-use window left after pathGuard's check above. createDirs
	// is true here to reproduce os.MkdirAll's "create missing intermediate
	// directories" behavior.
	const writeFlags = syscall.O_WRONLY | syscall.O_CREAT | syscall.O_TRUNC
	f, err := secureOpenLeaf(absRoot, safe, writeFlags, 0o644, true)
	if err != nil {
		return mcp.TextError("write: " + err.Error()), nil
	}
	if _, werr := f.Write([]byte(p.Content)); werr != nil {
		_ = f.Close()
		return mcp.TextError("write: " + werr.Error()), nil
	}
	if cerr := f.Close(); cerr != nil {
		return mcp.TextError("write: " + cerr.Error()), nil
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
	absRoot, err := absWorkspaceRoot(t.Workspace)
	if err != nil {
		return mcp.TextError(err.Error()), nil
	}
	// SECURITY (T194): open via the *at()-anchored, O_NOFOLLOW walk instead
	// of a second, independently-timed os.ReadDir by path string — see
	// secureResolveDirs/secureOpenLeaf for why this closes the
	// check-then-use window left after pathGuard's check above.
	const listFlags = syscall.O_RDONLY | syscall.O_DIRECTORY
	f, err := secureOpenLeaf(absRoot, safe, listFlags, 0, false)
	if err != nil {
		return mcp.TextError("readdir: " + err.Error()), nil
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
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
