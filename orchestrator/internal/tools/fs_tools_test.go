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
