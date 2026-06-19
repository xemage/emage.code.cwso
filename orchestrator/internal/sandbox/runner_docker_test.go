package sandbox

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDockerRunnerSuccessReturnsOutputAndCleanup(t *testing.T) {
	fake := newFakeDockerAPI()
	fake.waitResult = waitResponse{StatusCode: 0}
	fake.logPayload = multiplexLogs("hello\n", "warn\n")

	r, err := newDockerRunnerWithClient(DockerRunnerConfig{DefaultImage: "alpine:3.20"}, fake)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	res, runErr := r.Execute(context.Background(), RunRequest{
		Name:    "dispatch-worker-1",
		Command: []string{"/bin/sh", "-lc", "echo hello"},
		Env: map[string]string{
			"CWSO_OBJECTIVE_PROMPT": "echo hello",
		},
	})
	if runErr != nil {
		t.Fatalf("execute: %v", runErr)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if res.Stdout != "hello\n" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
	if res.Stderr != "warn\n" {
		t.Fatalf("unexpected stderr: %q", res.Stderr)
	}
	if !containsSubsequence(fake.callLog(), []string{"create", "start", "wait", "logs", "stop", "kill", "remove", "list"}) {
		t.Fatalf("cleanup sequence mismatch: %v", fake.callLog())
	}
}

func TestDockerRunnerTimeoutTriggersCleanup(t *testing.T) {
	fake := newFakeDockerAPI()
	fake.waitBlock = make(chan struct{})
	fake.logPayload = multiplexLogs("", "")

	r, err := newDockerRunnerWithClient(DockerRunnerConfig{DefaultImage: "alpine:3.20"}, fake)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	res, runErr := r.Execute(ctx, RunRequest{Name: "dispatch-worker-timeout"})
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

func TestDockerRunnerCancellationTriggersCleanup(t *testing.T) {
	fake := newFakeDockerAPI()
	fake.waitBlock = make(chan struct{})
	fake.logPayload = multiplexLogs("", "")

	r, err := newDockerRunnerWithClient(DockerRunnerConfig{DefaultImage: "alpine:3.20"}, fake)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	res, runErr := r.Execute(ctx, RunRequest{Name: "dispatch-worker-cancel"})
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

func TestDockerRunnerAppliesSecurityAndResourceDefaults(t *testing.T) {
	fake := newFakeDockerAPI()
	fake.waitResult = waitResponse{StatusCode: 0}

	r, err := newDockerRunnerWithClient(DockerRunnerConfig{
		DefaultImage:   "alpine:3.20",
		CPUQuotaMicros: 200000,
		MemoryBytes:    536870912,
		PIDsLimit:      64,
	}, fake)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	_, runErr := r.Execute(context.Background(), RunRequest{
		Name:           "dispatch-worker-defaults",
		MountWorkspace: true,
		WorkspaceDir:   "/tmp/workspace",
	})
	if runErr != nil {
		t.Fatalf("execute: %v", runErr)
	}
	got := fake.lastCreate()
	if got.HostConfig.Privileged {
		t.Fatal("privileged mode must be false")
	}
	if got.HostConfig.NetworkMode != "none" {
		t.Fatalf("expected network none, got %q", got.HostConfig.NetworkMode)
	}
	if !got.HostConfig.ReadonlyRootfs {
		t.Fatal("expected readonly rootfs by default")
	}
	if len(got.HostConfig.CapDrop) != 1 || got.HostConfig.CapDrop[0] != "ALL" {
		t.Fatalf("expected cap drop ALL, got %+v", got.HostConfig.CapDrop)
	}
	if !containsString(got.HostConfig.SecurityOpt, "no-new-privileges:true") {
		t.Fatalf("expected no-new-privileges security opt, got %+v", got.HostConfig.SecurityOpt)
	}
	if got.HostConfig.CpuQuota != 200000 {
		t.Fatalf("expected cpu quota 200000, got %d", got.HostConfig.CpuQuota)
	}
	if got.HostConfig.Memory != 536870912 {
		t.Fatalf("expected memory 536870912, got %d", got.HostConfig.Memory)
	}
	if got.HostConfig.PidsLimit != 64 {
		t.Fatalf("expected pids limit 64, got %d", got.HostConfig.PidsLimit)
	}
	if !containsString(got.HostConfig.Binds, "/tmp/workspace:/workspace:ro") {
		t.Fatalf("expected read-only workspace bind, got %+v", got.HostConfig.Binds)
	}
}

func TestDockerRunnerRejectsHostNetwork(t *testing.T) {
	_, err := newDockerRunnerWithClient(DockerRunnerConfig{DefaultImage: "alpine", NetworkMode: "host"}, newFakeDockerAPI())
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected host network validation error, got %v", err)
	}
}

type fakeDockerAPI struct {
	mu           sync.Mutex
	createReq    dockerCreateContainerRequest
	createID     string
	infoResult   dockerInfo
	infoErr      error
	waitResult   waitResponse
	waitErr      error
	waitBlock    chan struct{}
	logPayload   []byte
	logErr       error
	listResult   []string
	listErr      error
	callSequence []string
	removeErr    error
	stopErr      error
	killErr      error
	createErr    error
	startErr     error
}

func newFakeDockerAPI() *fakeDockerAPI {
	return &fakeDockerAPI{
		createID: "container-123",
		infoResult: dockerInfo{Runtimes: map[string]json.RawMessage{
			"runc":  json.RawMessage(`{}`),
			"runsc": json.RawMessage(`{}`),
		}},
		waitResult: waitResponse{StatusCode: 0},
		listResult: nil,
	}
}

func (f *fakeDockerAPI) callLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.callSequence))
	copy(out, f.callSequence)
	return out
}

func (f *fakeDockerAPI) lastCreate() dockerCreateContainerRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createReq
}

func (f *fakeDockerAPI) record(op string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callSequence = append(f.callSequence, op)
}

func (f *fakeDockerAPI) Ping(context.Context) error {
	f.record("ping")
	return nil
}

func (f *fakeDockerAPI) Info(context.Context) (dockerInfo, error) {
	f.record("info")
	if f.infoErr != nil {
		return dockerInfo{}, f.infoErr
	}
	return f.infoResult, nil
}

func (f *fakeDockerAPI) CreateContainer(_ context.Context, _ string, req dockerCreateContainerRequest) (string, error) {
	f.record("create")
	f.mu.Lock()
	f.createReq = req
	f.mu.Unlock()
	if f.createErr != nil {
		return "", f.createErr
	}
	return f.createID, nil
}

func (f *fakeDockerAPI) StartContainer(context.Context, string) error {
	f.record("start")
	return f.startErr
}

func (f *fakeDockerAPI) WaitContainer(context.Context, string) (waitResponse, error) {
	f.record("wait")
	if f.waitBlock != nil {
		<-f.waitBlock
	}
	return f.waitResult, f.waitErr
}

func (f *fakeDockerAPI) ContainerLogs(context.Context, string) (io.ReadCloser, error) {
	f.record("logs")
	if f.logErr != nil {
		return nil, f.logErr
	}
	return io.NopCloser(bytes.NewReader(f.logPayload)), nil
}

func (f *fakeDockerAPI) StopContainer(context.Context, string, int) error {
	f.record("stop")
	return f.stopErr
}

func (f *fakeDockerAPI) KillContainer(context.Context, string) error {
	f.record("kill")
	return f.killErr
}

func (f *fakeDockerAPI) RemoveContainer(context.Context, string) error {
	f.record("remove")
	return f.removeErr
}

func (f *fakeDockerAPI) ListContainersByName(context.Context, string) ([]string, error) {
	f.record("list")
	return f.listResult, f.listErr
}

func multiplexLogs(stdout, stderr string) []byte {
	buf := &bytes.Buffer{}
	if stdout != "" {
		buf.WriteByte(1)
		buf.Write([]byte{0, 0, 0})
		size := make([]byte, 4)
		binary.BigEndian.PutUint32(size, uint32(len(stdout)))
		buf.Write(size)
		buf.WriteString(stdout)
	}
	if stderr != "" {
		buf.WriteByte(2)
		buf.Write([]byte{0, 0, 0})
		size := make([]byte, 4)
		binary.BigEndian.PutUint32(size, uint32(len(stderr)))
		buf.Write(size)
		buf.WriteString(stderr)
	}
	return buf.Bytes()
}

func containsSubsequence(haystack []string, needle []string) bool {
	if len(needle) == 0 {
		return true
	}
	index := 0
	for _, item := range haystack {
		if item == needle[index] {
			index++
			if index == len(needle) {
				return true
			}
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, item := range values {
		if item == want {
			return true
		}
	}
	return false
}
