//go:build linux

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
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

// ---------------------------------------------------------------------
// T194 — TOCTOU-window regression tests for secureResolveDirs/secureOpenLeaf.
//
// A true TOCTOU race (a symlink swapped in DURING the walk) is inherently
// hard to prove deterministically with a plain unit test. The tests below
// instead prove the necessary-but-not-sufficient property the T194 brief
// asks for: the new *at()-anchored, O_NOFOLLOW code path is reachable and
// correctly rejects a symlink that is ALREADY in place before the call —
// at both the intermediate-directory level (secureResolveDirs) and the
// leaf level (secureOpenLeaf). See fs_tools.go's secureResolveDirs doc
// comment for the written reasoning proof of why this generalizes to the
// actual race window (each hop commits to a previously-obtained fd, so a
// symlink swapped in at ANY point before a given hop — whether that's
// "before pathGuard even ran" as in these tests, or "in the race window
// between pathGuard and this call" as in production — is refused by that
// hop's O_NOFOLLOW openat() the same way).
// ---------------------------------------------------------------------

// isSymlinkOpenRejection reports whether err is the kernel's way of saying
// "refused to follow a symlink because O_NOFOLLOW was set". Combining
// O_DIRECTORY with O_NOFOLLOW against a symlink surfaces ENOTDIR on this
// kernel (verified empirically: the O_DIRECTORY type check on the raw
// dirent — which is a symlink, not a directory — is what fires, rather
// than a distinct ELOOP path); O_NOFOLLOW alone against a symlink surfaces
// ELOOP. Both are correct, kernel-enforced rejections of the symlink; only
// the errno differs by flag combination, not the security property being
// asserted.
func isSymlinkOpenRejection(err error) bool {
	return errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.ENOTDIR)
}

// TestSecureResolveDirsRejectsSymlinkComponent proves the *at-anchored walk
// itself — independent of pathGuard — refuses to traverse a symlink in an
// intermediate path component via O_NOFOLLOW, surfacing ELOOP or ENOTDIR
// depending on flag combination (see isSymlinkOpenRejection).
func TestSecureResolveDirsRejectsSymlinkComponent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}

	// Bypass pathGuard entirely and feed secureResolveDirs a path whose
	// intermediate component is a live symlink, to prove the walk's own
	// O_NOFOLLOW enforcement is reachable and correct on its own terms.
	target := filepath.Join(root, "link", "pwned.txt")
	parentFd, _, err := secureResolveDirs(absRoot, target, false)
	if err == nil {
		_ = syscall.Close(parentFd)
		t.Fatal("expected secureResolveDirs to reject a symlink path component")
	}
	if !isSymlinkOpenRejection(err) {
		t.Fatalf("expected symlink rejection (ELOOP/ENOTDIR) for symlink component, got: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(outside, "pwned.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no file to exist outside workspace, stat err = %v", statErr)
	}
}

// TestSecureResolveDirsCreateDirsRejectsSymlinkPlantedAtDirName proves the
// createDirs=true path (used by WriteFileSync to reproduce os.MkdirAll
// behavior) does not silently tolerate a pre-existing symlink at a
// directory-component name: mkdirat's EEXIST is swallowed (as intended, to
// tolerate a benign concurrent mkdir), but the retried O_NOFOLLOW openat()
// still rejects the symlink.
func TestSecureResolveDirsCreateDirsRejectsSymlinkPlantedAtDirName(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}

	target := filepath.Join(root, "link", "sub", "pwned.txt")
	parentFd, _, err := secureResolveDirs(absRoot, target, true)
	if err == nil {
		_ = syscall.Close(parentFd)
		t.Fatal("expected secureResolveDirs(createDirs=true) to reject a symlink path component")
	}
	if !isSymlinkOpenRejection(err) {
		t.Fatalf("expected symlink rejection (ELOOP/ENOTDIR) for symlink component, got: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(outside, "sub")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no directory to exist outside workspace, stat err = %v", statErr)
	}
}

// TestSecureOpenLeafRejectsSymlinkLeaf proves the final, leaf-level
// openat() hop also applies O_NOFOLLOW independently of the directory
// walk: a pre-existing symlink AT the leaf name itself (pointing either
// inside or outside the workspace — the kernel refuses to follow it either
// way, which is the point: no second, separately-timed resolution decides
// this) is rejected, not silently opened through.
func TestSecureOpenLeafRejectsSymlinkLeaf(t *testing.T) {
	root := t.TempDir()

	realFile := filepath.Join(root, "real.txt")
	if err := os.WriteFile(realFile, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write real file: %v", err)
	}
	leafLink := filepath.Join(root, "leaf.txt")
	if err := os.Symlink(realFile, leafLink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}

	// Bypass pathGuard and feed secureOpenLeaf a path whose leaf component
	// is a live symlink (even one that would resolve inside the
	// workspace), to prove the leaf-level O_NOFOLLOW hop independently
	// refuses it.
	f, err := secureOpenLeaf(absRoot, leafLink, syscall.O_RDONLY, 0, false)
	if err == nil {
		_ = f.Close()
		t.Fatal("expected secureOpenLeaf to reject a symlink leaf")
	}
	if !isSymlinkOpenRejection(err) {
		t.Fatalf("expected symlink rejection (ELOOP/ENOTDIR) for symlink leaf, got: %v", err)
	}
}

// TestSecureOpenLeafAllowsRealFileAndDir is the positive-path baseline:
// once a path has no symlink components at all (the normal, pathGuard-
// resolved case), secureOpenLeaf must behave exactly like a plain open —
// no regression in the legitimate case.
func TestSecureOpenLeafAllowsRealFileAndDir(t *testing.T) {
	root := t.TempDir()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}

	filePath := filepath.Join(root, "plain.txt")
	if err := os.WriteFile(filePath, []byte("plain"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	f, err := secureOpenLeaf(absRoot, filePath, syscall.O_RDONLY, 0, false)
	if err != nil {
		t.Fatalf("expected clean file open to succeed, got: %v", err)
	}
	defer f.Close()
	b := make([]byte, 5)
	if _, rerr := f.Read(b); rerr != nil {
		t.Fatalf("read opened file: %v", rerr)
	}
	if string(b) != "plain" {
		t.Fatalf("unexpected content %q", string(b))
	}

	newPath := filepath.Join(root, "nested", "new.txt")
	wf, err := secureOpenLeaf(absRoot, newPath, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, 0o644, true)
	if err != nil {
		t.Fatalf("expected new-file create-through-missing-dir to succeed, got: %v", err)
	}
	if _, werr := wf.Write([]byte("x")); werr != nil {
		t.Fatalf("write: %v", werr)
	}
	if cerr := wf.Close(); cerr != nil {
		t.Fatalf("close: %v", cerr)
	}
	stored, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(stored) != "x" {
		t.Fatalf("unexpected content %q", string(stored))
	}
}

// TestWriteFileSyncRaceAgainstSymlinkSwapNeverEscapesWorkspace is a
// best-effort, non-flaky-by-design race test: it does not assert that the
// race window is hit on any particular iteration (that would be flaky by
// construction), only that across many iterations of a concurrent
// symlink-swap attempt, write_file_sync NEVER results in a file written
// outside the workspace root. This is supporting evidence alongside the
// deterministic tests above and the written reasoning proof in
// secureResolveDirs's doc comment — not the sole proof.
func TestWriteFileSyncRaceAgainstSymlinkSwapNeverEscapesWorkspace(t *testing.T) {
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

	// Racer: repeatedly try to replace "race" with a symlink pointing
	// outside the workspace, then restore it as a plain directory, in a
	// tight loop for the duration of the test.
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
		// A rejection (symlink escape or O_NOFOLLOW failure) is fine; the
		// only unacceptable outcome is a file actually landing outside the
		// workspace.
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
		t.Fatal("write_file_sync wrote a file outside the workspace during concurrent symlink swap")
	}
	// "No escape observed" across `iterations` racing attempts is
	// supporting empirical evidence; see secureResolveDirs's doc comment
	// for the non-probabilistic argument for why escapes are structurally
	// impossible here, not just empirically rare.
}
