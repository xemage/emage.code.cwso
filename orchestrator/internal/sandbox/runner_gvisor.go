package sandbox

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	defaultGVisorRuntime = "runsc"
)

// GVisorRunnerConfig configures runsc-backed execution via Docker runtime selection.
type GVisorRunnerConfig struct {
	Host               string
	DefaultImage       string
	DefaultCommand     []string
	Runtime            string
	NetworkMode        string
	CPUQuotaMicros     int64
	MemoryBytes        int64
	PIDsLimit          int64
	StopTimeout        time.Duration
	CleanupRetries     int
	CreateTimeout      time.Duration
	ListTimeout        time.Duration
	AllowWritableMount bool
}

func (c GVisorRunnerConfig) withDefaults() GVisorRunnerConfig {
	if strings.TrimSpace(c.Runtime) == "" {
		c.Runtime = defaultGVisorRuntime
	}
	if strings.TrimSpace(c.NetworkMode) == "" {
		c.NetworkMode = defaultDockerNetwork
	}
	if c.CPUQuotaMicros <= 0 {
		c.CPUQuotaMicros = defaultCPUQuotaMicros
	}
	if c.MemoryBytes <= 0 {
		c.MemoryBytes = defaultMemoryBytes
	}
	if c.PIDsLimit <= 0 {
		c.PIDsLimit = defaultPIDsLimit
	}
	if c.StopTimeout <= 0 {
		c.StopTimeout = defaultStopTimeout
	}
	if c.CleanupRetries <= 0 {
		c.CleanupRetries = defaultCleanupRetries
	}
	if c.CreateTimeout <= 0 {
		c.CreateTimeout = 10 * time.Second
	}
	if c.ListTimeout <= 0 {
		c.ListTimeout = 3 * time.Second
	}
	return c
}

func (c GVisorRunnerConfig) validate() error {
	if strings.TrimSpace(c.Runtime) == "" {
		return errorsMissingRuntimeConfig
	}
	if strings.EqualFold(c.NetworkMode, "host") {
		return fmt.Errorf("gvisor network mode host is forbidden")
	}
	if c.CPUQuotaMicros <= 0 {
		return fmt.Errorf("cpu_quota_micros must be > 0")
	}
	if c.MemoryBytes <= 0 {
		return fmt.Errorf("memory_bytes must be > 0")
	}
	if c.PIDsLimit <= 0 {
		return fmt.Errorf("pids_limit must be > 0")
	}
	if c.StopTimeout <= 0 {
		return fmt.Errorf("stop_timeout must be > 0")
	}
	if c.CleanupRetries <= 0 {
		return fmt.Errorf("cleanup_retries must be > 0")
	}
	return nil
}

var errorsMissingRuntimeConfig = fmt.Errorf("gvisor runtime is required")

// GVisorRunner executes workloads with gVisor (runsc) using Docker runtime integration.
type GVisorRunner struct {
	docker  *DockerRunner
	runtime string
}

func NewGVisorRunner(cfg GVisorRunnerConfig) (*GVisorRunner, error) {
	resolved := cfg.withDefaults()
	if err := resolved.validate(); err != nil {
		return nil, fmt.Errorf("gvisor runner config: %w", err)
	}

	dockerRunner, err := NewDockerRunner(dockerConfigFromGVisor(resolved))
	if err != nil {
		return nil, fmt.Errorf("gvisor docker client: %w", err)
	}
	return &GVisorRunner{docker: dockerRunner, runtime: resolved.Runtime}, nil
}

func newGVisorRunnerWithClient(cfg GVisorRunnerConfig, client dockerAPI) (*GVisorRunner, error) {
	resolved := cfg.withDefaults()
	if err := resolved.validate(); err != nil {
		return nil, err
	}
	dockerRunner, err := newDockerRunnerWithClient(dockerConfigFromGVisor(resolved), client)
	if err != nil {
		return nil, err
	}
	return &GVisorRunner{docker: dockerRunner, runtime: resolved.Runtime}, nil
}

func dockerConfigFromGVisor(c GVisorRunnerConfig) DockerRunnerConfig {
	return DockerRunnerConfig{
		Host:               c.Host,
		DefaultImage:       c.DefaultImage,
		DefaultCommand:     c.DefaultCommand,
		Runtime:            c.Runtime,
		NetworkMode:        c.NetworkMode,
		ReadOnlyRootFS:     true,
		CPUQuotaMicros:     c.CPUQuotaMicros,
		MemoryBytes:        c.MemoryBytes,
		PIDsLimit:          c.PIDsLimit,
		StopTimeout:        c.StopTimeout,
		CleanupRetries:     c.CleanupRetries,
		NoNewPrivileges:    true,
		DropAllCaps:        true,
		CreateTimeout:      c.CreateTimeout,
		ListTimeout:        c.ListTimeout,
		AllowWritableMount: c.AllowWritableMount,
	}
}

func (r *GVisorRunner) Name() string { return "gvisor-fast-ephemeral" }

func (r *GVisorRunner) Health(ctx context.Context) error {
	if err := r.docker.Health(ctx); err != nil {
		return err
	}
	if err := r.ensureRuntime(ctx); err != nil {
		return err
	}
	return nil
}

func (r *GVisorRunner) Execute(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := r.ensureRuntime(ctx); err != nil {
		return RunResult{}, err
	}
	result, err := r.docker.Execute(ctx, req)
	if err != nil && isRuntimeSelectionError(err, r.runtime) {
		return result, wrapRuntimeUnavailableError(r.runtime, err)
	}
	return result, err
}

func (r *GVisorRunner) ensureRuntime(ctx context.Context) error {
	info, err := r.docker.client.Info(ctx)
	if err != nil {
		return fmt.Errorf("query docker runtime info: %w", err)
	}
	if info.HasRuntime(r.runtime) {
		return nil
	}
	if strings.TrimSpace(info.DefaultRuntime) == r.runtime {
		return nil
	}
	return missingRuntimeError(r.runtime, info.DefaultRuntime, info.RuntimeNames())
}

func missingRuntimeError(runtime string, defaultRuntime string, available []string) error {
	availableLabel := "none"
	if len(available) > 0 {
		availableLabel = strings.Join(available, ",")
	}
	defaultRuntime = strings.TrimSpace(defaultRuntime)
	if defaultRuntime == "" {
		defaultRuntime = "unknown"
	}
	return fmt.Errorf("gvisor runtime %q is not available in Docker daemon (default=%q, available=%s). Configure runsc under Docker runtimes (for example in /etc/docker/daemon.json) and restart Docker", runtime, defaultRuntime, availableLabel)
}

func wrapRuntimeUnavailableError(runtime string, err error) error {
	return fmt.Errorf("gvisor runtime %q is unavailable or misconfigured: %w", runtime, err)
}

func isRuntimeSelectionError(err error, runtime string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	runtime = strings.ToLower(strings.TrimSpace(runtime))
	if runtime != "" && strings.Contains(msg, runtime) {
		return true
	}
	if strings.Contains(msg, "unknown runtime") {
		return true
	}
	if strings.Contains(msg, "runtime") && strings.Contains(msg, "not found") {
		return true
	}
	if strings.Contains(msg, "invalid runtime") {
		return true
	}
	return false
}
