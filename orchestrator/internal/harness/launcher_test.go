package harness

import (
	"context"
	"strings"
	"testing"
)

func TestNewLauncherRequiresProxyURL(t *testing.T) {
	_, err := NewLauncher(LauncherConfig{Runtime: NewLocalRuntime(t.TempDir())})
	if err == nil {
		t.Fatal("expected error without proxy url")
	}
}

func TestLauncherLaunchSetsProxyEnv(t *testing.T) {
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
	handle, env, err := launcher.Launch(context.Background(), LaunchRequest{
		HarnessID: IDCodex,
		SessionID: "codex-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = runtime.Stop(context.Background(), handle) }()
	if env["OPENAI_BASE_URL"] != proxy.URL() {
		t.Fatalf("env=%v", env)
	}
}

func TestLauncherRunOnceUsesAdapterCommand(t *testing.T) {
	runtime := NewLocalRuntime(t.TempDir())
	launcher, err := NewLauncher(LauncherConfig{
		Registry: DefaultRegistry(),
		Runtime:  runtime,
		ProxyURL: "http://127.0.0.1:8787",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := launcher.RunOnce(context.Background(), LaunchRequest{
		HarnessID: IDCodex,
		SessionID: "codex-run-once",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Stdout, "codex harness stub") {
		t.Fatalf("stdout=%q", result.Stdout)
	}
}
