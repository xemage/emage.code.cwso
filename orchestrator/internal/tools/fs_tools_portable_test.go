//go:build !linux

package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------
// T195 — portable (!linux) counterpart to fs_tools_test.go.
//
// This file exists so that `go test ./internal/tools/...` on a non-Linux
// GOOS actually exercises symlink-rejection coverage for
// fs_tools_portable.go, instead of silently running zero tests (the gap
// this task closes — see fs_tools_portable.go's package doc comment and
// docs/tasks/task-T195.md for the full context). It mirrors the T193/T194
// regression cases from fs_tools_test.go as closely as the portable
// implementation allows; see individual test comments for where portable
// coverage intentionally differs from the Linux build's stricter,
// fd-anchored guarantees.
// ---------------------------------------------------------------------

func TestPortablePathGuardValidatesRootAndTraversal(t *testing.T) {
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

// TestPortablePathGuardRejectsSymlinkEscapeAtLeaf is the "symlink-at-leaf"
// rejection case: an EXISTING file reached via a leaf that is itself a
// symlink pointing outside the workspace must be rejected.
func TestPortablePathGuardRejectsSymlinkEscapeAtLeaf(t *testing.T) {
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
		t.Fatal("expected symlink escape at leaf to be rejected")
	}
}

// TestPortablePathGuardRejectsSymlinkEscapeAtComponentForNewFile is the
// "symlink-at-component" rejection case for a NEW (non-existent) file: an
// intermediate directory component that is a symlink pointing outside the
// workspace must be rejected, not silently allowed through because
// filepath.EvalSymlinks errors on a non-existent leaf. Regression coverage
// for T193's fix, ported to the portable build.
func TestPortablePathGuardRejectsSymlinkEscapeAtComponentForNewFile(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()

	escapeLink := filepath.Join(root, "escape")
	if err := os.Symlink(outsideDir, escapeLink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if _, err := pathGuard(root, "escape/pwned.txt"); err == nil {
		t.Fatal("expected new-file write through escaping symlink component to be rejected")
	}

	if _, err := os.Stat(filepath.Join(outsideDir, "pwned.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected no file to exist outside workspace, stat err = %v", err)
	}
}

// TestPortablePathGuardRejectsSymlinkEscapeForDeeplyNestedNewFile exercises
// the ancestor-walk with more than one non-existent tail component.
func TestPortablePathGuardRejectsSymlinkEscapeForDeeplyNestedNewFile(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()

	escapeLink := filepath.Join(root, "escape")
	if err := os.Symlink(outsideDir, escapeLink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if _, err := pathGuard(root, "escape/sub/deep/pwned.txt"); err == nil {
		t.Fatal("expected deeply-nested new-file write through escaping symlink to be rejected")
	}

	if _, err := os.Stat(filepath.Join(outsideDir, "sub")); !os.IsNotExist(err) {
		t.Fatalf("expected no directory to exist outside workspace, stat err = %v", err)
	}
}

// TestPortablePathGuardAllowsNewFileThroughInWorkspaceSymlink proves the
// fix does not break the legitimate case: a symlink that stays inside the
// workspace must still allow writes to new files reached through it.
func TestPortablePathGuardAllowsNewFileThroughInWorkspaceSymlink(t *testing.T) {
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

// TestPortablePathGuardAllowsPlainNewFileNoSymlinks is the baseline
// no-symlink regression check for new-file writes with multiple
// non-existent intermediate directories.
func TestPortablePathGuardAllowsPlainNewFileNoSymlinks(t *testing.T) {
	root := t.TempDir()

	safe, err := pathGuard(root, "a/b/c/new.txt")
	if err != nil {
		t.Fatalf("expected plain nested new-file write to be allowed, got: %v", err)
	}
	if !strings.HasPrefix(safe, root) {
		t.Fatalf("expected resolved path inside root, got %q", safe)
	}
}

func TestPortableReadFileSyncExecuteBranches(t *testing.T) {
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

// TestPortableReadFileSyncRejectsLeafSymlinkEscape is the tool-execution-
// layer "symlink-at-leaf" rejection case: reading a path whose leaf is a
// symlink pointing outside the workspace must fail, and must not leak the
// outside file's contents.
func TestPortableReadFileSyncRejectsLeafSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("top secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	linkPath := filepath.Join(root, "link.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	tool := &ReadFileSync{Workspace: root}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"link.txt"}`))
	if err != nil {
		t.Fatalf("execute read through escaping symlink: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected read through escaping symlink to be rejected, got %+v", res)
	}
	if strings.Contains(res.Content[0].Text, "top secret") {
		t.Fatalf("expected outside file contents not to leak, got %+v", res)
	}
}

func TestPortableWriteFileSyncAndListDirExecute(t *testing.T) {
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

// TestPortableWriteFileSyncRejectsNewFileThroughEscapingSymlink is the
// end-to-end regression test for T193 at the tool-execution layer (not
// just pathGuard directly), ported to the portable build: a
// write_file_sync call targeting a new file reached through a symlinked
// intermediate directory that points outside the configured workspace
// must be rejected, and — critically — must not actually write anything to
// disk outside the workspace.
func TestPortableWriteFileSyncRejectsNewFileThroughEscapingSymlink(t *testing.T) {
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

// TestPortableWriteFileSyncRejectsLeafSymlinkEscape is the tool-execution-
// layer "symlink-at-leaf" rejection case for writes: an EXISTING leaf that
// is a symlink pointing outside the workspace must be rejected before any
// write is attempted through it.
func TestPortableWriteFileSyncRejectsLeafSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "victim.txt")
	if err := os.WriteFile(outsideFile, []byte("original"), 0o644); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	linkPath := filepath.Join(root, "link.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	writeTool := &WriteFileSync{Workspace: root}
	res, err := writeTool.Execute(context.Background(), json.RawMessage(`{"path":"link.txt","content":"owned"}`))
	if err != nil {
		t.Fatalf("execute write through escaping leaf symlink: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected write through escaping leaf symlink to be rejected, got %+v", res)
	}

	stored, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatalf("read outside fixture: %v", err)
	}
	if string(stored) != "original" {
		t.Fatalf("expected outside file to remain untouched, got %q", string(stored))
	}
}

// TestPortableWriteFileSyncAllowsNewFileThroughInWorkspaceSymlink confirms
// the fix does not break a legitimate write reached through a symlink that
// stays entirely inside the workspace.
func TestPortableWriteFileSyncAllowsNewFileThroughInWorkspaceSymlink(t *testing.T) {
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

// TestPortableWriteFileSyncRaceAgainstSymlinkSwapNeverEscapesWorkspace is a
// best-effort, non-flaky-by-design race test, ported from fs_tools_test.go.
// It does not assert the race window is hit on any particular iteration —
// this build's window is real and unclosed (see fs_tools_portable.go's
// package doc comment), so an escape here would not be surprising in
// principle. What it asserts is the same as the Linux counterpart: across
// many iterations of a concurrent symlink-swap attempt on this machine and
// workload, write_file_sync did not observably write outside the
// workspace. This is supporting empirical evidence for the "narrowed, not
// closed" claim, not proof the window is unreachable.
func TestPortableWriteFileSyncRaceAgainstSymlinkSwapNeverEscapesWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race stress test in -short mode")
	}

	root := t.TempDir()
	outside := t.TempDir()
	targetDir := filepath.Join(root, "race")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir race dir: %v", err)
	}

	writeTool := &WriteFileSync{Workspace: root}

	const iterations = 200
	var (
		stop   int32
		wg     sync.WaitGroup
		escape int32
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		linkPath := filepath.Join(root, "race")
		for atomic.LoadInt32(&stop) == 0 {
			_ = os.Remove(linkPath)
			_ = os.Symlink(outside, linkPath)
			_ = os.Remove(linkPath)
			_ = os.Mkdir(linkPath, 0o755)
		}
	}()

	for i := 0; i < iterations; i++ {
		payload := json.RawMessage(`{"path":"race/pwned.txt","content":"owned"}`)
		res, err := writeTool.Execute(context.Background(), payload)
		if err != nil {
			t.Fatalf("execute write: %v", err)
		}
		_ = res
		if _, statErr := os.Stat(filepath.Join(outside, "pwned.txt")); statErr == nil {
			atomic.StoreInt32(&escape, 1)
			break
		}
		_ = os.RemoveAll(filepath.Join(root, "race", "pwned.txt"))
	}

	atomic.StoreInt32(&stop, 1)
	wg.Wait()

	if atomic.LoadInt32(&escape) != 0 {
		t.Fatal("write_file_sync (portable build) wrote a file outside the workspace during concurrent symlink swap — this build's narrower re-verify-before-use window was hit; see fs_tools_portable.go's doc comment, this is a known, documented limitation of this build, not a surprise, but should prompt re-checking whether the window needs to be narrowed further")
	}
}
