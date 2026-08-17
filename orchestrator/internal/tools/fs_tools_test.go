package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathGuardValidatesRootAndTraversal(t *testing.T) {
	if _, err := pathGuard("", "a.txt"); err == nil {
		t.Fatal("expected error for empty workspace root")
	}

	root := t.TempDir()
	safe, err := pathGuard(root, "nested/file.txt")
	if err != nil {
		t.Fatalf("unexpected error for in-root path: %v", err)
	}
	if !strings.HasPrefix(safe, root) {
		t.Fatalf("expected safe path inside root, got %q", safe)
	}

	if _, err := pathGuard(root, "../outside.txt"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestPathGuardRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	linkPath := filepath.Join(root, "link.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if _, err := pathGuard(root, "link.txt"); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

// TestPathGuardRejectsSymlinkEscapeForNewFile is the regression test for
// T193: a NEW (non-existent) file reached through a symlinked intermediate
// directory that points outside the workspace root must be rejected, not
// silently allowed through because filepath.EvalSymlinks errors on a
// non-existent leaf.
func TestPathGuardRejectsSymlinkEscapeForNewFile(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()

	escapeLink := filepath.Join(root, "escape")
	if err := os.Symlink(outsideDir, escapeLink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	// "pwned.txt" does not exist yet anywhere — this is the new-file write
	// case that used to fall through to `return clean, nil` unchecked.
	if _, err := pathGuard(root, "escape/pwned.txt"); err == nil {
		t.Fatal("expected new-file write through escaping symlink to be rejected")
	}

	if _, err := os.Stat(filepath.Join(outsideDir, "pwned.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected no file to exist outside workspace, stat err = %v", err)
	}
}

// TestPathGuardRejectsSymlinkEscapeForDeeplyNestedNewFile exercises the
// ancestor-walk with more than one non-existent tail component, to make
// sure the fix isn't only correct for a single missing path segment.
func TestPathGuardRejectsSymlinkEscapeForDeeplyNestedNewFile(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()

	escapeLink := filepath.Join(root, "escape")
	if err := os.Symlink(outsideDir, escapeLink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	// Neither "sub", "deep", nor "pwned.txt" exist anywhere yet.
	if _, err := pathGuard(root, "escape/sub/deep/pwned.txt"); err == nil {
		t.Fatal("expected deeply-nested new-file write through escaping symlink to be rejected")
	}

	if _, err := os.Stat(filepath.Join(outsideDir, "sub")); !os.IsNotExist(err) {
		t.Fatalf("expected no directory to exist outside workspace, stat err = %v", err)
	}
}

// TestPathGuardAllowsNewFileThroughInWorkspaceSymlink proves the fix does
// not break the legitimate case: a symlink that stays inside the workspace
// must still allow writes to new files reached through it.
func TestPathGuardAllowsNewFileThroughInWorkspaceSymlink(t *testing.T) {
	root := t.TempDir()

	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("mkdir real: %v", err)
	}
	linkedDir := filepath.Join(root, "linked")
	if err := os.Symlink(realDir, linkedDir); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	safe, err := pathGuard(root, "linked/new.txt")
	if err != nil {
		t.Fatalf("expected in-workspace symlink new-file write to be allowed, got: %v", err)
	}
	if !strings.HasPrefix(safe, root) {
		t.Fatalf("expected resolved path inside root, got %q", safe)
	}
}

// TestPathGuardAllowsPlainNewFileNoSymlinks is the baseline no-symlink
// regression check for new-file writes with multiple non-existent
// intermediate directories.
func TestPathGuardAllowsPlainNewFileNoSymlinks(t *testing.T) {
	root := t.TempDir()

	safe, err := pathGuard(root, "a/b/c/new.txt")
	if err != nil {
		t.Fatalf("expected plain nested new-file write to be allowed, got: %v", err)
	}
	if !strings.HasPrefix(safe, root) {
		t.Fatalf("expected resolved path inside root, got %q", safe)
	}
}

func TestReadFileSyncExecuteBranches(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	dirPath := filepath.Join(root, "adir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}

	tooBigPath := filepath.Join(root, "big.bin")
	tooBig := make([]byte, (1<<20)+1)
	if err := os.WriteFile(tooBigPath, tooBig, 0o644); err != nil {
		t.Fatalf("write big fixture: %v", err)
	}

	tool := &ReadFileSync{Workspace: root}

	res, err := tool.Execute(context.Background(), json.RawMessage(`{`))
	if err != nil {
		t.Fatalf("execute invalid json: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "invalid arguments") {
		t.Fatalf("expected invalid argument error, got %+v", res)
	}

	res, err = tool.Execute(context.Background(), json.RawMessage(`{"path":""}`))
	if err != nil {
		t.Fatalf("execute empty path: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "path is required") {
		t.Fatalf("expected path required error, got %+v", res)
	}

	res, err = tool.Execute(context.Background(), json.RawMessage(`{"path":"adir"}`))
	if err != nil {
		t.Fatalf("execute directory path: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "path is a directory") {
		t.Fatalf("expected directory error, got %+v", res)
	}

	res, err = tool.Execute(context.Background(), json.RawMessage(`{"path":"big.bin"}`))
	if err != nil {
		t.Fatalf("execute big file path: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "file exceeds") {
		t.Fatalf("expected size cap error, got %+v", res)
	}

	res, err = tool.Execute(context.Background(), json.RawMessage(`{"path":"hello.txt"}`))
	if err != nil {
		t.Fatalf("execute success path: %v", err)
	}
	if res.IsError || res.Content[0].Text != "hello" {
		t.Fatalf("expected successful read result, got %+v", res)
	}
}

func TestWriteFileSyncAndListDirExecute(t *testing.T) {
	root := t.TempDir()
	writeTool := &WriteFileSync{Workspace: root}

	res, err := writeTool.Execute(context.Background(), json.RawMessage(`{"path":""}`))
	if err != nil {
		t.Fatalf("execute empty path: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "path is required") {
		t.Fatalf("expected path required error, got %+v", res)
	}

	res, err = writeTool.Execute(context.Background(), json.RawMessage(`{"path":"nested/out.txt","content":"abc"}`))
	if err != nil {
		t.Fatalf("execute write success: %v", err)
	}
	if res.IsError || !strings.Contains(res.Content[0].Text, "wrote 3 bytes") {
		t.Fatalf("expected successful write, got %+v", res)
	}

	stored, err := os.ReadFile(filepath.Join(root, "nested", "out.txt"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(stored) != "abc" {
		t.Fatalf("unexpected written content %q", string(stored))
	}

	if err := os.Mkdir(filepath.Join(root, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write root file: %v", err)
	}

	listTool := &ListDir{Workspace: root}
	res, err = listTool.Execute(context.Background(), json.RawMessage(`{`))
	if err != nil {
		t.Fatalf("execute invalid list args: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "invalid arguments") {
		t.Fatalf("expected invalid args error, got %+v", res)
	}

	res, err = listTool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("execute list success: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected successful list, got %+v", res)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "dir\tsubdir") || !strings.Contains(text, "file\tfile.txt") {
		t.Fatalf("unexpected list output: %q", text)
	}
}

// TestWriteFileSyncRejectsNewFileThroughEscapingSymlink is the end-to-end
// regression test for T193 at the tool-execution layer (not just pathGuard
// directly): a write_file_sync call targeting a new file reached through a
// symlinked intermediate directory that points outside the configured
// workspace must be rejected, and — critically — must not actually write
// anything to disk outside the workspace.
func TestWriteFileSyncRejectsNewFileThroughEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()

	escapeLink := filepath.Join(root, "escape")
	if err := os.Symlink(outsideDir, escapeLink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	writeTool := &WriteFileSync{Workspace: root}
	res, err := writeTool.Execute(context.Background(), json.RawMessage(`{"path":"escape/pwned.txt","content":"owned"}`))
	if err != nil {
		t.Fatalf("execute write through escaping symlink: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected write through escaping symlink to be rejected, got %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "symlinks outside workspace root") {
		t.Fatalf("expected symlink-escape error message, got %+v", res)
	}

	// Check the filesystem directly, not just the returned error: no file
	// must exist outside the workspace root regardless of what the error
	// message claims.
	outsidePath := filepath.Join(outsideDir, "pwned.txt")
	if _, err := os.Stat(outsidePath); !os.IsNotExist(err) {
		t.Fatalf("expected no file written outside workspace at %q, stat err = %v", outsidePath, err)
	}

	entries, err := os.ReadDir(outsideDir)
	if err != nil {
		t.Fatalf("readdir outside: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected outside directory to remain empty, found %d entries", len(entries))
	}
}

// TestWriteFileSyncAllowsNewFileThroughInWorkspaceSymlink confirms the fix
// does not break a legitimate write reached through a symlink that stays
// entirely inside the workspace.
func TestWriteFileSyncAllowsNewFileThroughInWorkspaceSymlink(t *testing.T) {
	root := t.TempDir()

	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("mkdir real: %v", err)
	}
	linkedDir := filepath.Join(root, "linked")
	if err := os.Symlink(realDir, linkedDir); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	writeTool := &WriteFileSync{Workspace: root}
	res, err := writeTool.Execute(context.Background(), json.RawMessage(`{"path":"linked/new.txt","content":"abc"}`))
	if err != nil {
		t.Fatalf("execute write through in-workspace symlink: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected write through in-workspace symlink to succeed, got %+v", res)
	}

	stored, err := os.ReadFile(filepath.Join(realDir, "new.txt"))
	if err != nil {
		t.Fatalf("read written file via real dir: %v", err)
	}
	if string(stored) != "abc" {
		t.Fatalf("unexpected written content %q", string(stored))
	}
}
