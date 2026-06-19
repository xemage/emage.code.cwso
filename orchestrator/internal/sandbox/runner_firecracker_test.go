package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFirecrackerRunnerSuccessReturnsOutputAndCleansUp(t *testing.T) {
	vm := &fakeFirecrackerVM{id: "vm-123", execResult: firecrackerExecResult{ExitCode: 0, Stdout: "ok\n"}}
	runtime := &fakeFirecrackerRuntime{vm: vm}
	hooks := &fakeSnapshotHooks{}

	runner, err := newFirecrackerRunnerWithDeps(testFirecrackerConfig(), runtime, hooks)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	res, runErr := runner.Execute(context.Background(), RunRequest{Name: "dispatch-firecracker-success"})
	if runErr != nil {
		t.Fatalf("execute: %v", runErr)
	}
	if res.ContainerID != "vm-123" {
		t.Fatalf("expected vm id vm-123, got %q", res.ContainerID)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if res.Stdout != "ok\n" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
	if !containsSubsequence(vm.calls(), []string{"execute", "shutdown", "cleanup"}) {
		t.Fatalf("unexpected vm call sequence: %v", vm.calls())
	}
	if !containsSubsequence(hooks.calls(), []string{"ensure", "clone", "release"}) {
		t.Fatalf("unexpected snapshot hook sequence: %v", hooks.calls())
	}
}

func TestFirecrackerRunnerTimeoutTriggersDeterministicCleanup(t *testing.T) {
	vm := &fakeFirecrackerVM{id: "vm-timeout", blockExecute: true}
	runtime := &fakeFirecrackerRuntime{vm: vm}
	hooks := &fakeSnapshotHooks{}

	runner, err := newFirecrackerRunnerWithDeps(testFirecrackerConfig(), runtime, hooks)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	res, runErr := runner.Execute(ctx, RunRequest{Name: "dispatch-firecracker-timeout"})
	if !errors.Is(runErr, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", runErr)
	}
	if !res.TimedOut {
		t.Fatal("expected timed_out=true")
	}
	if !containsSubsequence(vm.calls(), []string{"shutdown", "cleanup"}) {
		t.Fatalf("expected vm cleanup on timeout, got %v", vm.calls())
	}
	if !containsSubsequence(hooks.calls(), []string{"release"}) {
		t.Fatalf("expected clone release on timeout, got %v", hooks.calls())
	}
}

func TestFirecrackerRunnerCancellationTriggersDeterministicCleanup(t *testing.T) {
	vm := &fakeFirecrackerVM{id: "vm-cancel", blockExecute: true}
	runtime := &fakeFirecrackerRuntime{vm: vm}
	hooks := &fakeSnapshotHooks{}

	runner, err := newFirecrackerRunnerWithDeps(testFirecrackerConfig(), runtime, hooks)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	res, runErr := runner.Execute(ctx, RunRequest{Name: "dispatch-firecracker-cancel"})
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", runErr)
	}
	if !res.Cancelled {
		t.Fatal("expected cancelled=true")
	}
	if !containsSubsequence(vm.calls(), []string{"shutdown", "cleanup"}) {
		t.Fatalf("expected vm cleanup on cancel, got %v", vm.calls())
	}
	if !containsSubsequence(hooks.calls(), []string{"release"}) {
		t.Fatalf("expected clone release on cancel, got %v", hooks.calls())
	}
}

func TestFirecrackerRunnerMissingPrerequisitesReturnsActionableError(t *testing.T) {
	runtime := &fakeFirecrackerRuntime{preflightErr: fmt.Errorf("%w: KVM device \"/dev/kvm\" is not available. Enable KVM virtualization", ErrFirecrackerUnavailable)}
	hooks := &fakeSnapshotHooks{}

	runner, err := newFirecrackerRunnerWithDeps(testFirecrackerConfig(), runtime, hooks)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	_, runErr := runner.Execute(context.Background(), RunRequest{Name: "dispatch-firecracker-prereq"})
	if runErr == nil {
		t.Fatal("expected runtime unavailable error")
	}
	if !strings.Contains(runErr.Error(), "/dev/kvm") || !strings.Contains(strings.ToLower(runErr.Error()), "enable kvm") {
		t.Fatalf("expected actionable KVM error, got %v", runErr)
	}
	if runtime.launchCalls != 0 {
		t.Fatalf("launch should not be called when preflight fails, got %d", runtime.launchCalls)
	}
}

func TestFirecrackerRunnerLaunchFailureReleasesClone(t *testing.T) {
	runtime := &fakeFirecrackerRuntime{launchErr: errors.New("launch failed")}
	hooks := &fakeSnapshotHooks{}

	runner, err := newFirecrackerRunnerWithDeps(testFirecrackerConfig(), runtime, hooks)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	_, runErr := runner.Execute(context.Background(), RunRequest{Name: "dispatch-firecracker-launch-fail"})
	if runErr == nil {
		t.Fatal("expected launch failure")
	}
	if !containsSubsequence(hooks.calls(), []string{"ensure", "clone", "release"}) {
		t.Fatalf("expected clone release after launch failure, got %v", hooks.calls())
	}
}

func TestFirecrackerRunnerSnapshotCloneHookErrorBubblesUp(t *testing.T) {
	runtime := &fakeFirecrackerRuntime{vm: &fakeFirecrackerVM{id: "vm-ignored"}}
	hooks := &fakeSnapshotHooks{cloneErr: errors.New("template clone unavailable")}

	runner, err := newFirecrackerRunnerWithDeps(testFirecrackerConfig(), runtime, hooks)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	_, runErr := runner.Execute(context.Background(), RunRequest{Name: "dispatch-firecracker-clone-fail"})
	if runErr == nil {
		t.Fatal("expected clone hook error")
	}
	if !strings.Contains(runErr.Error(), "clone firecracker snapshot template") {
		t.Fatalf("expected snapshot clone error, got %v", runErr)
	}
	if runtime.launchCalls != 0 {
		t.Fatalf("launch should not be called when clone hook fails, got %d", runtime.launchCalls)
	}
}

func testFirecrackerConfig() FirecrackerRunnerConfig {
	return FirecrackerRunnerConfig{
		BinaryPath:          "firecracker",
		ExecHelperPath:      "fc-helper",
		KVMDevicePath:       "/dev/kvm",
		VhostNetDevicePath:  "/dev/vhost-net",
		SnapshotTemplateDir: "/tmp/cwso-firecracker/templates",
		VMStateDir:          "/tmp/cwso-firecracker/vms",
		CreateTimeout:       5 * time.Second,
		StopTimeout:         2 * time.Second,
		CleanupTimeout:      2 * time.Second,
		MemoryBytes:         268435456,
		CPUQuotaMicros:      100000,
	}
}

type fakeFirecrackerRuntime struct {
	preflightErr error
	launchErr    error
	vm           *fakeFirecrackerVM
	launchCalls  int
}

func (f *fakeFirecrackerRuntime) Preflight(context.Context, FirecrackerRunnerConfig) error {
	return f.preflightErr
}

func (f *fakeFirecrackerRuntime) Launch(_ context.Context, _ firecrackerLaunchSpec) (firecrackerVM, error) {
	f.launchCalls++
	if f.launchErr != nil {
		return nil, f.launchErr
	}
	if f.vm == nil {
		f.vm = &fakeFirecrackerVM{id: "vm-default"}
	}
	return f.vm, nil
}

type fakeFirecrackerVM struct {
	id           string
	blockExecute bool
	execResult   firecrackerExecResult
	execErr      error
	shutdownErr  error
	cleanupErr   error
	mu           sync.Mutex
	callLog      []string
}

func (v *fakeFirecrackerVM) ID() string { return v.id }

func (v *fakeFirecrackerVM) Execute(ctx context.Context, _ firecrackerExecSpec) (firecrackerExecResult, error) {
	v.record("execute")
	if v.blockExecute {
		<-ctx.Done()
		return firecrackerExecResult{}, ctx.Err()
	}
	return v.execResult, v.execErr
}

func (v *fakeFirecrackerVM) Shutdown(_ context.Context) error {
	v.record("shutdown")
	return v.shutdownErr
}

func (v *fakeFirecrackerVM) Cleanup(_ context.Context) error {
	v.record("cleanup")
	return v.cleanupErr
}

func (v *fakeFirecrackerVM) record(entry string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.callLog = append(v.callLog, entry)
}

func (v *fakeFirecrackerVM) calls() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]string, len(v.callLog))
	copy(out, v.callLog)
	return out
}

type fakeSnapshotHooks struct {
	ensureErr  error
	cloneErr   error
	releaseErr error
	mu         sync.Mutex
	callLog    []string
}

func (h *fakeSnapshotHooks) EnsureTemplate(_ context.Context, _ string) (FirecrackerSnapshotTemplate, error) {
	h.record("ensure")
	if h.ensureErr != nil {
		return FirecrackerSnapshotTemplate{}, h.ensureErr
	}
	return FirecrackerSnapshotTemplate{ID: "tmpl-1", Path: "/tmp/cwso-firecracker/templates/tmpl-1"}, nil
}

func (h *fakeSnapshotHooks) CloneTemplate(_ context.Context, _ FirecrackerSnapshotTemplate, vmID string) (FirecrackerSnapshotClone, error) {
	h.record("clone")
	if h.cloneErr != nil {
		return FirecrackerSnapshotClone{}, h.cloneErr
	}
	return FirecrackerSnapshotClone{ID: vmID, Path: "/tmp/cwso-firecracker/vms/" + vmID}, nil
}

func (h *fakeSnapshotHooks) ReleaseClone(_ context.Context, _ FirecrackerSnapshotClone) error {
	h.record("release")
	return h.releaseErr
}

func (h *fakeSnapshotHooks) record(entry string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callLog = append(h.callLog, entry)
}

func (h *fakeSnapshotHooks) calls() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.callLog))
	copy(out, h.callLog)
	return out
}
