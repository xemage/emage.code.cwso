package sandbox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	defaultFirecrackerBinary      = "firecracker"
	defaultFirecrackerKVMDevice   = "/dev/kvm"
	defaultFirecrackerVhostDevice = "/dev/vhost-net"
	defaultFirecrackerTemplateDir = "/tmp/cwso-firecracker/templates"
	defaultFirecrackerVMStateDir  = "/tmp/cwso-firecracker/vms"
)

var (
	// ErrFirecrackerUnavailable is returned when host/runtime prerequisites are missing.
	ErrFirecrackerUnavailable = errors.New("firecracker runtime unavailable")
	vmNameSanitizer           = regexp.MustCompile(`[^a-zA-Z0-9-]+`)
)

// FirecrackerRunnerConfig configures the secure-isolation runtime.
type FirecrackerRunnerConfig struct {
	BinaryPath          string
	ExecHelperPath      string
	KVMDevicePath       string
	VhostNetDevicePath  string
	SnapshotTemplateDir string
	VMStateDir          string
	DefaultCommand      []string
	CreateTimeout       time.Duration
	StopTimeout         time.Duration
	CleanupTimeout      time.Duration
	RequireVhostNet     bool
	MemoryBytes         int64
	CPUQuotaMicros      int64
}

func (c FirecrackerRunnerConfig) withDefaults() FirecrackerRunnerConfig {
	if strings.TrimSpace(c.BinaryPath) == "" {
		c.BinaryPath = defaultFirecrackerBinary
	}
	if strings.TrimSpace(c.KVMDevicePath) == "" {
		c.KVMDevicePath = defaultFirecrackerKVMDevice
	}
	if strings.TrimSpace(c.VhostNetDevicePath) == "" {
		c.VhostNetDevicePath = defaultFirecrackerVhostDevice
	}
	if strings.TrimSpace(c.SnapshotTemplateDir) == "" {
		c.SnapshotTemplateDir = defaultFirecrackerTemplateDir
	}
	if strings.TrimSpace(c.VMStateDir) == "" {
		c.VMStateDir = defaultFirecrackerVMStateDir
	}
	if c.CreateTimeout <= 0 {
		c.CreateTimeout = 10 * time.Second
	}
	if c.StopTimeout <= 0 {
		c.StopTimeout = 5 * time.Second
	}
	if c.CleanupTimeout <= 0 {
		c.CleanupTimeout = 5 * time.Second
	}
	if len(c.DefaultCommand) == 0 {
		c.DefaultCommand = []string{"/bin/sh", "-lc", "echo ${CWSO_OBJECTIVE_PROMPT:-cwso-job}"}
	}
	if c.MemoryBytes <= 0 {
		c.MemoryBytes = defaultMemoryBytes
	}
	if c.CPUQuotaMicros <= 0 {
		c.CPUQuotaMicros = defaultCPUQuotaMicros
	}
	if !c.RequireVhostNet {
		c.RequireVhostNet = true
	}
	return c
}

func (c FirecrackerRunnerConfig) validate() error {
	if strings.TrimSpace(c.BinaryPath) == "" {
		return errors.New("firecracker binary path is required")
	}
	if strings.TrimSpace(c.ExecHelperPath) == "" {
		return fmt.Errorf("%w: firecracker execution helper is required (set CWSO_FIRECRACKER_EXEC_HELPER)", ErrFirecrackerUnavailable)
	}
	if strings.TrimSpace(c.KVMDevicePath) == "" {
		return errors.New("kvm device path is required")
	}
	if strings.TrimSpace(c.SnapshotTemplateDir) == "" {
		return errors.New("snapshot_template_dir is required")
	}
	if strings.TrimSpace(c.VMStateDir) == "" {
		return errors.New("vm_state_dir is required")
	}
	if c.CreateTimeout <= 0 {
		return errors.New("create_timeout must be > 0")
	}
	if c.StopTimeout <= 0 {
		return errors.New("stop_timeout must be > 0")
	}
	if c.CleanupTimeout <= 0 {
		return errors.New("cleanup_timeout must be > 0")
	}
	if c.MemoryBytes <= 0 {
		return errors.New("memory_bytes must be > 0")
	}
	if c.CPUQuotaMicros <= 0 {
		return errors.New("cpu_quota_micros must be > 0")
	}
	return nil
}

// FirecrackerRunner executes untrusted workloads through a firecracker helper.
type FirecrackerRunner struct {
	cfg      FirecrackerRunnerConfig
	runtime  firecrackerRuntime
	snapshot firecrackerSnapshotHooks
	now      func() time.Time
}

func NewFirecrackerRunner(cfg FirecrackerRunnerConfig) (*FirecrackerRunner, error) {
	resolved := cfg.withDefaults()
	if err := resolved.validate(); err != nil {
		return nil, fmt.Errorf("firecracker runner config: %w", err)
	}
	return &FirecrackerRunner{
		cfg:      resolved,
		runtime:  newFirecrackerBinaryRuntime(),
		snapshot: newFirecrackerFilesystemSnapshotHooks(resolved.SnapshotTemplateDir, resolved.VMStateDir),
		now:      time.Now,
	}, nil
}

func newFirecrackerRunnerWithDeps(cfg FirecrackerRunnerConfig, runtime firecrackerRuntime, snapshot firecrackerSnapshotHooks) (*FirecrackerRunner, error) {
	resolved := cfg.withDefaults()
	if err := resolved.validate(); err != nil {
		return nil, err
	}
	if runtime == nil {
		return nil, errors.New("firecracker runtime is required")
	}
	if snapshot == nil {
		return nil, errors.New("firecracker snapshot hooks are required")
	}
	return &FirecrackerRunner{cfg: resolved, runtime: runtime, snapshot: snapshot, now: time.Now}, nil
}

func (r *FirecrackerRunner) Name() string { return "firecracker-secure-isolation" }

func (r *FirecrackerRunner) Health(ctx context.Context) error {
	if err := r.runtime.Preflight(ctx, r.cfg); err != nil {
		return err
	}
	return nil
}

func (r *FirecrackerRunner) Execute(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := req.Validate(); err != nil {
		return RunResult{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if err := r.runtime.Preflight(ctx, r.cfg); err != nil {
		return RunResult{}, err
	}

	execCtx := ctx
	cancelExec := func() {}
	if req.Timeout > 0 {
		execCtx, cancelExec = context.WithTimeout(ctx, req.Timeout)
	}
	defer cancelExec()

	vmID := buildFirecrackerVMID(req.Name)
	template, clone, vm, err := r.prepareVM(execCtx, req, vmID)
	if err != nil {
		return RunResult{}, err
	}
	_ = template

	result := RunResult{
		ContainerID: vm.ID(),
		StartedAt:   r.now(),
	}

	type execOutcome struct {
		res firecrackerExecResult
		err error
	}
	execDone := make(chan execOutcome, 1)
	go func() {
		runRes, runErr := vm.Execute(execCtx, firecrackerExecSpec{
			Command:           resolveFirecrackerCommand(req.Command, r.cfg.DefaultCommand),
			Env:               mapToSortedEnv(req.Env),
			WorkingDir:        strings.TrimSpace(req.WorkingDir),
			MountWorkspace:    req.MountWorkspace,
			WorkspaceDir:      strings.TrimSpace(req.WorkspaceDir),
			WorkspaceWritable: req.WorkspaceWritable,
			RootFSWritable:    req.RootFSWritable,
			Timeout:           req.Timeout,
			Image:             strings.TrimSpace(req.Image),
		})
		execDone <- execOutcome{res: runRes, err: runErr}
	}()

	select {
	case <-execCtx.Done():
		result.FinishedAt = r.now()
		result.TimedOut = errors.Is(execCtx.Err(), context.DeadlineExceeded)
		result.Cancelled = errors.Is(execCtx.Err(), context.Canceled)
		if !result.TimedOut && !result.Cancelled {
			result.Cancelled = true
		}
		cleanupErr := r.finalize(vm, clone)
		if cleanupErr != nil {
			return result, fmt.Errorf("firecracker execution canceled: %w (cleanup failed: %v)", execCtx.Err(), cleanupErr)
		}
		return result, execCtx.Err()
	case outcome := <-execDone:
		result.FinishedAt = r.now()
		result.ExitCode = outcome.res.ExitCode
		result.Stdout = outcome.res.Stdout
		result.Stderr = outcome.res.Stderr
		result.ResourceExceeded = outcome.res.ResourceExceeded
		result.FailureReason = strings.TrimSpace(outcome.res.FailureReason)
		cleanupErr := r.finalize(vm, clone)
		if outcome.err != nil {
			if cleanupErr != nil {
				return result, fmt.Errorf("execute firecracker workload: %w (cleanup failed: %v)", outcome.err, cleanupErr)
			}
			return result, fmt.Errorf("execute firecracker workload: %w", outcome.err)
		}
		if cleanupErr != nil {
			return result, fmt.Errorf("cleanup firecracker workload: %w", cleanupErr)
		}
		return result, nil
	}
}

func (r *FirecrackerRunner) prepareVM(ctx context.Context, req RunRequest, vmID string) (FirecrackerSnapshotTemplate, FirecrackerSnapshotClone, firecrackerVM, error) {
	template, err := r.snapshot.EnsureTemplate(ctx, templateKeyForRequest(req))
	if err != nil {
		return FirecrackerSnapshotTemplate{}, FirecrackerSnapshotClone{}, nil, fmt.Errorf("prepare firecracker template: %w", err)
	}

	clone, err := r.snapshot.CloneTemplate(ctx, template, vmID)
	if err != nil {
		return FirecrackerSnapshotTemplate{}, FirecrackerSnapshotClone{}, nil, fmt.Errorf("clone firecracker snapshot template: %w", err)
	}

	createCtx, cancel := context.WithTimeout(ctx, r.cfg.CreateTimeout)
	defer cancel()
	vm, err := r.runtime.Launch(createCtx, firecrackerLaunchSpec{VMID: vmID, Template: template, Clone: clone, Request: req, Config: r.cfg})
	if err != nil {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), r.cfg.CleanupTimeout)
		releaseErr := r.snapshot.ReleaseClone(releaseCtx, clone)
		releaseCancel()
		if releaseErr != nil {
			return FirecrackerSnapshotTemplate{}, FirecrackerSnapshotClone{}, nil, fmt.Errorf("launch firecracker vm: %w (release clone failed: %v)", err, releaseErr)
		}
		return FirecrackerSnapshotTemplate{}, FirecrackerSnapshotClone{}, nil, fmt.Errorf("launch firecracker vm: %w", err)
	}
	return template, clone, vm, nil
}

func (r *FirecrackerRunner) finalize(vm firecrackerVM, clone FirecrackerSnapshotClone) error {
	stopCtx, stopCancel := context.WithTimeout(context.Background(), r.cfg.StopTimeout)
	shutdownErr := vm.Shutdown(stopCtx)
	stopCancel()

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), r.cfg.CleanupTimeout)
	cleanupErr := vm.Cleanup(cleanupCtx)
	cleanupCancel()

	releaseCtx, releaseCancel := context.WithTimeout(context.Background(), r.cfg.CleanupTimeout)
	releaseErr := r.snapshot.ReleaseClone(releaseCtx, clone)
	releaseCancel()

	if shutdownErr == nil && cleanupErr == nil && releaseErr == nil {
		return nil
	}
	return errors.Join(shutdownErr, cleanupErr, releaseErr)
}

func resolveFirecrackerCommand(candidate []string, fallback []string) []string {
	if len(candidate) > 0 {
		return candidate
	}
	if len(fallback) > 0 {
		return fallback
	}
	return []string{"/bin/sh", "-lc", "echo ${CWSO_OBJECTIVE_PROMPT:-cwso-job}"}
}

func templateKeyForRequest(req RunRequest) string {
	if v := strings.TrimSpace(req.Image); v != "" {
		return v
	}
	if v := strings.TrimSpace(req.WorkingDir); v != "" {
		return v
	}
	return "default"
}

func buildFirecrackerVMID(name string) string {
	clean := vmNameSanitizer.ReplaceAllString(strings.TrimSpace(name), "-")
	clean = strings.Trim(clean, "-")
	if clean == "" {
		clean = "job"
	}
	if len(clean) > 40 {
		clean = clean[:40]
	}
	return fmt.Sprintf("fc-%s-%d", clean, time.Now().UnixNano())
}

type firecrackerRuntime interface {
	Preflight(ctx context.Context, cfg FirecrackerRunnerConfig) error
	Launch(ctx context.Context, spec firecrackerLaunchSpec) (firecrackerVM, error)
}

type firecrackerLaunchSpec struct {
	VMID     string
	Template FirecrackerSnapshotTemplate
	Clone    FirecrackerSnapshotClone
	Request  RunRequest
	Config   FirecrackerRunnerConfig
}

type firecrackerVM interface {
	ID() string
	Execute(ctx context.Context, spec firecrackerExecSpec) (firecrackerExecResult, error)
	Shutdown(ctx context.Context) error
	Cleanup(ctx context.Context) error
}

type firecrackerExecSpec struct {
	Command           []string
	Env               []string
	WorkingDir        string
	MountWorkspace    bool
	WorkspaceDir      string
	WorkspaceWritable bool
	RootFSWritable    bool
	Timeout           time.Duration
	Image             string
}

type firecrackerExecResult struct {
	ExitCode         int
	Stdout           string
	Stderr           string
	ResourceExceeded bool
	FailureReason    string
}

// FirecrackerSnapshotTemplate identifies a reusable snapshot template.
type FirecrackerSnapshotTemplate struct {
	ID   string
	Path string
}

// FirecrackerSnapshotClone identifies a clone derived from a template.
type FirecrackerSnapshotClone struct {
	ID   string
	Path string
}

type firecrackerSnapshotHooks interface {
	EnsureTemplate(ctx context.Context, key string) (FirecrackerSnapshotTemplate, error)
	CloneTemplate(ctx context.Context, template FirecrackerSnapshotTemplate, vmID string) (FirecrackerSnapshotClone, error)
	ReleaseClone(ctx context.Context, clone FirecrackerSnapshotClone) error
}

type firecrackerBinaryRuntime struct {
	lookPath  func(string) (string, error)
	stat      func(string) (os.FileInfo, error)
	runHelper func(ctx context.Context, helperPath string, payload firecrackerHelperPayload) (firecrackerExecResult, error)
}

func newFirecrackerBinaryRuntime() *firecrackerBinaryRuntime {
	return &firecrackerBinaryRuntime{
		lookPath: exec.LookPath,
		stat:     os.Stat,
		runHelper: func(ctx context.Context, helperPath string, payload firecrackerHelperPayload) (firecrackerExecResult, error) {
			return runFirecrackerHelper(ctx, helperPath, payload)
		},
	}
}

func (r *firecrackerBinaryRuntime) Preflight(_ context.Context, cfg FirecrackerRunnerConfig) error {
	if _, err := r.lookPath(cfg.BinaryPath); err != nil {
		return fmt.Errorf("%w: firecracker binary %q not found in PATH. Install Firecracker on the host and set CWSO_FIRECRACKER_BIN", ErrFirecrackerUnavailable, cfg.BinaryPath)
	}
	if _, err := r.lookPath(cfg.ExecHelperPath); err != nil {
		return fmt.Errorf("%w: firecracker execution helper %q not found. Install/configure helper and set CWSO_FIRECRACKER_EXEC_HELPER", ErrFirecrackerUnavailable, cfg.ExecHelperPath)
	}
	if _, err := r.stat(cfg.KVMDevicePath); err != nil {
		return fmt.Errorf("%w: KVM device %q is not available. Enable KVM virtualization and verify host readiness probe", ErrFirecrackerUnavailable, cfg.KVMDevicePath)
	}
	if cfg.RequireVhostNet {
		if _, err := r.stat(cfg.VhostNetDevicePath); err != nil {
			return fmt.Errorf("%w: vhost-net device %q is not available. Load vhost_net kernel module or run degraded gvisor mode", ErrFirecrackerUnavailable, cfg.VhostNetDevicePath)
		}
	}
	return nil
}

func (r *firecrackerBinaryRuntime) Launch(_ context.Context, spec firecrackerLaunchSpec) (firecrackerVM, error) {
	return &firecrackerHelperVM{
		vmID:       spec.VMID,
		helperPath: spec.Config.ExecHelperPath,
		payload: firecrackerHelperPayload{
			VMID:              spec.VMID,
			TemplateID:        spec.Template.ID,
			TemplatePath:      spec.Template.Path,
			CloneID:           spec.Clone.ID,
			ClonePath:         spec.Clone.Path,
			MountWorkspace:    spec.Request.MountWorkspace,
			WorkspaceDir:      strings.TrimSpace(spec.Request.WorkspaceDir),
			WorkspaceWritable: spec.Request.WorkspaceWritable,
			RootFSWritable:    spec.Request.RootFSWritable,
			MemoryBytes:       spec.Config.MemoryBytes,
			CPUQuotaMicros:    spec.Config.CPUQuotaMicros,
		},
		runHelper: r.runHelper,
	}, nil
}

type firecrackerHelperVM struct {
	vmID       string
	helperPath string
	payload    firecrackerHelperPayload
	runHelper  func(ctx context.Context, helperPath string, payload firecrackerHelperPayload) (firecrackerExecResult, error)
}

func (v *firecrackerHelperVM) ID() string { return v.vmID }

func (v *firecrackerHelperVM) Execute(ctx context.Context, spec firecrackerExecSpec) (firecrackerExecResult, error) {
	payload := v.payload
	payload.Command = spec.Command
	payload.Env = spec.Env
	payload.WorkingDir = spec.WorkingDir
	payload.TimeoutMillis = spec.Timeout.Milliseconds()
	payload.Image = spec.Image
	return v.runHelper(ctx, v.helperPath, payload)
}

func (v *firecrackerHelperVM) Shutdown(_ context.Context) error { return nil }

func (v *firecrackerHelperVM) Cleanup(_ context.Context) error { return nil }

type firecrackerHelperPayload struct {
	VMID              string   `json:"vm_id"`
	TemplateID        string   `json:"template_id"`
	TemplatePath      string   `json:"template_path"`
	CloneID           string   `json:"clone_id"`
	ClonePath         string   `json:"clone_path"`
	Command           []string `json:"command"`
	Env               []string `json:"env"`
	WorkingDir        string   `json:"working_dir,omitempty"`
	Image             string   `json:"image,omitempty"`
	MountWorkspace    bool     `json:"mount_workspace"`
	WorkspaceDir      string   `json:"workspace_dir,omitempty"`
	WorkspaceWritable bool     `json:"workspace_writable"`
	RootFSWritable    bool     `json:"rootfs_writable"`
	TimeoutMillis     int64    `json:"timeout_millis,omitempty"`
	MemoryBytes       int64    `json:"memory_bytes"`
	CPUQuotaMicros    int64    `json:"cpu_quota_micros"`
}

type firecrackerHelperResponse struct {
	ExitCode         int    `json:"exit_code"`
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
	FailureReason    string `json:"failure_reason,omitempty"`
	ResourceExceeded bool   `json:"resource_exceeded,omitempty"`
}

func runFirecrackerHelper(ctx context.Context, helperPath string, payload firecrackerHelperPayload) (firecrackerExecResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return firecrackerExecResult{}, fmt.Errorf("marshal firecracker helper payload: %w", err)
	}

	cmd := exec.CommandContext(ctx, helperPath)
	cmd.Stdin = bytes.NewReader(body)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errLabel := strings.TrimSpace(stderr.String())
		if errLabel == "" {
			errLabel = strings.TrimSpace(stdout.String())
		}
		if errLabel == "" {
			errLabel = "no diagnostics"
		}
		return firecrackerExecResult{}, fmt.Errorf("firecracker execution helper failed: %w (%s)", err, errLabel)
	}

	var resp firecrackerHelperResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return firecrackerExecResult{}, fmt.Errorf("firecracker execution helper returned invalid JSON: %w", err)
	}
	return firecrackerExecResult{
		ExitCode:         resp.ExitCode,
		Stdout:           resp.Stdout,
		Stderr:           resp.Stderr,
		FailureReason:    resp.FailureReason,
		ResourceExceeded: resp.ResourceExceeded,
	}, nil
}

type firecrackerFilesystemSnapshotHooks struct {
	templateDir string
	cloneRoot   string
}

func newFirecrackerFilesystemSnapshotHooks(templateDir string, cloneRoot string) *firecrackerFilesystemSnapshotHooks {
	return &firecrackerFilesystemSnapshotHooks{templateDir: filepath.Clean(templateDir), cloneRoot: filepath.Clean(cloneRoot)}
}

func (h *firecrackerFilesystemSnapshotHooks) EnsureTemplate(_ context.Context, key string) (FirecrackerSnapshotTemplate, error) {
	hash := sha256.Sum256([]byte(strings.TrimSpace(key)))
	id := hex.EncodeToString(hash[:16])
	path, err := safeChild(h.templateDir, id)
	if err != nil {
		return FirecrackerSnapshotTemplate{}, err
	}
	if err := os.MkdirAll(path, 0o750); err != nil {
		return FirecrackerSnapshotTemplate{}, fmt.Errorf("create snapshot template dir: %w", err)
	}
	return FirecrackerSnapshotTemplate{ID: id, Path: path}, nil
}

func (h *firecrackerFilesystemSnapshotHooks) CloneTemplate(_ context.Context, template FirecrackerSnapshotTemplate, vmID string) (FirecrackerSnapshotClone, error) {
	path, err := safeChild(h.cloneRoot, vmID)
	if err != nil {
		return FirecrackerSnapshotClone{}, err
	}
	if err := os.MkdirAll(path, 0o750); err != nil {
		return FirecrackerSnapshotClone{}, fmt.Errorf("create snapshot clone dir: %w", err)
	}
	if writeErr := os.WriteFile(filepath.Join(path, "template.ref"), []byte(template.Path), 0o640); writeErr != nil {
		return FirecrackerSnapshotClone{}, fmt.Errorf("record snapshot template reference: %w", writeErr)
	}
	return FirecrackerSnapshotClone{ID: vmID, Path: path}, nil
}

func (h *firecrackerFilesystemSnapshotHooks) ReleaseClone(_ context.Context, clone FirecrackerSnapshotClone) error {
	if strings.TrimSpace(clone.Path) == "" {
		return nil
	}
	target := filepath.Clean(clone.Path)
	if !strings.HasPrefix(target, h.cloneRoot+string(filepath.Separator)) && target != h.cloneRoot {
		return fmt.Errorf("clone path %q escapes clone root %q", target, h.cloneRoot)
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove snapshot clone path: %w", err)
	}
	return nil
}

func safeChild(root string, child string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("root path is required")
	}
	root = filepath.Clean(root)
	target := filepath.Clean(filepath.Join(root, child))
	if !strings.HasPrefix(target, root+string(filepath.Separator)) && target != root {
		return "", fmt.Errorf("resolved path %q escapes root %q", target, root)
	}
	return target, nil
}
