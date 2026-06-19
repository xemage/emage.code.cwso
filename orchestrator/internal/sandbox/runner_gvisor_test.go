package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGVisorRunnerSuccessReturnsOutputAndCleanup(t *testing.T) {
	fake := newFakeDockerAPI()
	fake.waitResult = waitResponse{StatusCode: 0}
	fake.logPayload = multiplexLogs("ok\n", "")

	r, err := newGVisorRunnerWithClient(GVisorRunnerConfig{DefaultImage: "alpine:3.20"}, fake)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	res, runErr := r.Execute(context.Background(), RunRequest{Name: "dispatch-gvisor-success"})
	if runErr != nil {
		t.Fatalf("execute: %v", runErr)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if res.Stdout != "ok\n" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
	if !containsSubsequence(fake.callLog(), []string{"info", "create", "start", "wait", "logs", "stop", "kill", "remove", "list"}) {
		t.Fatalf("cleanup sequence mismatch: %v", fake.callLog())
	}

	createReq := fake.lastCreate()
	if createReq.HostConfig.Runtime != "runsc" {
		t.Fatalf("expected runtime runsc, got %q", createReq.HostConfig.Runtime)
	}
	if createReq.HostConfig.Privileged {
		t.Fatal("privileged mode must be false")
	}
	if createReq.HostConfig.NetworkMode != "none" {
		t.Fatalf("expected network none, got %q", createReq.HostConfig.NetworkMode)
	}
	if !createReq.HostConfig.ReadonlyRootfs {
		t.Fatal("expected readonly rootfs by default")
	}
	if len(createReq.HostConfig.CapDrop) != 1 || createReq.HostConfig.CapDrop[0] != "ALL" {
		t.Fatalf("expected cap drop ALL, got %+v", createReq.HostConfig.CapDrop)
	}
	if !containsString(createReq.HostConfig.SecurityOpt, "no-new-privileges:true") {
		t.Fatalf("expected no-new-privileges, got %+v", createReq.HostConfig.SecurityOpt)
	}
}

func TestGVisorRunnerTimeoutTriggersCleanup(t *testing.T) {
	fake := newFakeDockerAPI()
	fake.waitBlock = make(chan struct{})

	r, err := newGVisorRunnerWithClient(GVisorRunnerConfig{DefaultImage: "alpine:3.20"}, fake)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	res, runErr := r.Execute(ctx, RunRequest{Name: "dispatch-gvisor-timeout"})
	if !errors.Is(runErr, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", runErr)
	}
	if !res.TimedOut {
		t.Fatal("expected timed_out=true")
	}
	if !containsSubsequence(fake.callLog(), []string{"stop", "kill", "remove", "list"}) {
		t.Fatalf("expected cleanup calls after timeout, got %v", fake.callLog())
	}
}

func TestGVisorRunnerCancellationTriggersCleanup(t *testing.T) {
	fake := newFakeDockerAPI()
	fake.waitBlock = make(chan struct{})

	r, err := newGVisorRunnerWithClient(GVisorRunnerConfig{DefaultImage: "alpine:3.20"}, fake)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	res, runErr := r.Execute(ctx, RunRequest{Name: "dispatch-gvisor-cancel"})
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected canceled, got %v", runErr)
	}
	if !res.Cancelled {
		t.Fatal("expected cancelled=true")
	}
	if !containsSubsequence(fake.callLog(), []string{"stop", "kill", "remove", "list"}) {
		t.Fatalf("expected cleanup calls after cancellation, got %v", fake.callLog())
	}
}

func TestGVisorRunnerMissingRuntimeFailsWithActionableError(t *testing.T) {
	fake := newFakeDockerAPI()
	fake.infoResult = dockerInfo{DefaultRuntime: "runc", Runtimes: map[string]json.RawMessage{"runc": json.RawMessage(`{}`)}}

	r, err := newGVisorRunnerWithClient(GVisorRunnerConfig{DefaultImage: "alpine:3.20"}, fake)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	_, runErr := r.Execute(context.Background(), RunRequest{Name: "dispatch-gvisor-missing-runtime"})
	if runErr == nil {
		t.Fatal("expected runtime error")
	}
	if !strings.Contains(runErr.Error(), "runsc") || !strings.Contains(runErr.Error(), "daemon.json") {
		t.Fatalf("expected actionable runtime error, got %v", runErr)
	}
	if containsSubsequence(fake.callLog(), []string{"create"}) {
		t.Fatalf("container should not be created when runtime is unavailable, calls: %v", fake.callLog())
	}
}

func TestGVisorRunnerWrapsMisconfiguredRuntimeCreateError(t *testing.T) {
	fake := newFakeDockerAPI()
	fake.createErr = errors.New("docker api status=500: unknown runtime specified runsc")

	r, err := newGVisorRunnerWithClient(GVisorRunnerConfig{DefaultImage: "alpine:3.20"}, fake)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	_, runErr := r.Execute(context.Background(), RunRequest{Name: "dispatch-gvisor-runtime-create-fail"})
	if runErr == nil {
		t.Fatal("expected runtime selection error")
	}
	if !strings.Contains(runErr.Error(), "unavailable or misconfigured") {
		t.Fatalf("expected wrapped runtime error, got %v", runErr)
	}
}
