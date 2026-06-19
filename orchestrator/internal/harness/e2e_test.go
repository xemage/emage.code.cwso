package harness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShellCommandHarnessProxyCaptureE2E(t *testing.T) {
	ensureHTTPClient(t)

	proxy := newCaptureProxy(t)
	workDir := t.TempDir()
	runtime := NewLocalRuntime(workDir)
	launcher, err := NewLauncher(LauncherConfig{
		Registry: DefaultRegistry(),
		Runtime:  runtime,
		ProxyURL: proxy.URL(),
	})
	if err != nil {
		t.Fatal(err)
	}

	script := harnessScriptPath(t)

	handle, env, err := launcher.Launch(context.Background(), LaunchRequest{
		HarnessID: IDShellCommand,
		SessionID: "e2e-shell-command",
		Prompt:    "capture-me",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = runtime.Stop(context.Background(), handle) }()

	if env["OPENAI_BASE_URL"] != proxy.URL() {
		t.Fatalf("OPENAI_BASE_URL=%q want %q", env["OPENAI_BASE_URL"], proxy.URL())
	}

	result, err := runtime.Exec(context.Background(), ExecRequest{
		Handle:  handle,
		Command: []string{"/bin/sh", script},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("harness failed: exit=%d stderr=%q", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "ok") {
		t.Fatalf("expected upstream content in stdout: %q", result.Stdout)
	}
	if !proxy.hasChatCompletion() {
		t.Fatal("proxy did not capture /v1/chat/completions request")
	}
}

func TestLauncherRunOnceShellCommand(t *testing.T) {
	ensureHTTPClient(t)
	proxy := newCaptureProxy(t)
	runtime := NewLocalRuntime(t.TempDir())
	launcher, err := NewLauncher(LauncherConfig{
		Registry: DefaultRegistry(),
		Runtime:  runtime,
		ProxyURL: proxy.URL(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := LaunchRequest{
		HarnessID: IDShellCommand,
		SessionID: "run-once",
		Prompt:    "once",
	}
	handle, env, err := launcher.Launch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	script := harnessScriptPath(t)
	result, err := runtime.Exec(context.Background(), ExecRequest{
		Handle: handle, Command: []string{"/bin/sh", script},
	})
	_ = runtime.Stop(context.Background(), handle)
	if err != nil {
		t.Fatal(err)
	}
	if env["OPENAI_BASE_URL"] != proxy.URL() {
		t.Fatalf("env proxy url mismatch")
	}
	if !strings.Contains(result.Stdout, "ok") {
		t.Fatalf("stdout=%q", result.Stdout)
	}
}

func ensureHTTPClient(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("curl"); err == nil {
		return
	}
	if _, err := exec.LookPath("wget"); err == nil {
		return
	}
	if runtime.GOOS == "linux" {
		if err := exec.Command("apk", "add", "--no-cache", "curl").Run(); err == nil {
			return
		}
	}
	t.Skip("curl or wget required for harness e2e")
}

func harnessScriptPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	candidates := []string{
		filepath.Join(filepath.Dir(file), "testdata", "shell-command-harness.sh"),
	}
	if root := findRepoRoot(filepath.Dir(file)); root != "" {
		candidates = append(candidates, filepath.Join(root, "scripts", "shell-command-harness.sh"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Fatal("shell-command-harness.sh not found")
	return ""
}

func findRepoRoot(start string) string {
	dir := start
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "scripts", "shell-command-harness.sh")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return ""
}
