package harness

import (
	"context"
	"fmt"
	"strings"
)

const defaultIdleCommand = "/bin/sh -c 'trap exit TERM; while :; do sleep 3600; done'"

// LauncherConfig wires registry + runtime + rollout proxy URL.
type LauncherConfig struct {
	Registry *Registry
	Runtime  Runtime
	ProxyURL string
}

// Launcher starts harness adapters with model base_url pointed at cwso-rollout.
type Launcher struct {
	registry *Registry
	runtime  Runtime
	proxyURL string
}

// NewLauncher constructs a harness launcher.
func NewLauncher(cfg LauncherConfig) (*Launcher, error) {
	if cfg.Registry == nil {
		cfg.Registry = DefaultRegistry()
	}
	if cfg.Runtime == nil {
		return nil, fmt.Errorf("harness runtime is required")
	}
	proxyURL := strings.TrimSpace(cfg.ProxyURL)
	if proxyURL == "" {
		return nil, fmt.Errorf("rollout proxy url is required")
	}
	return &Launcher{
		registry: cfg.Registry,
		runtime:  cfg.Runtime,
		proxyURL: strings.TrimRight(proxyURL, "/"),
	}, nil
}

// LaunchRequest describes a harness session launch.
type LaunchRequest struct {
	HarnessID    ID
	SessionID    string
	WorkspaceDir string
	Prompt       string
	ExtraEnv     map[string]string
}

// Launch starts the harness runtime with proxy-directed env vars.
func (l *Launcher) Launch(ctx context.Context, req LaunchRequest) (Handle, map[string]string, error) {
	cfg, err := l.registry.Get(req.HarnessID)
	if err != nil {
		return Handle{}, nil, err
	}
	env := cfg.LaunchEnv(l.proxyURL, req.ExtraEnv)
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		env["CWSO_HARNESS_PROMPT"] = prompt
	}
	name := strings.TrimSpace(req.SessionID)
	if name == "" {
		name = "cwso-harness"
	}
	startReq := StartRequest{
		Name:    name,
		Image:   cfg.Image,
		Command: []string{"/bin/sh", "-lc", defaultIdleCommand},
		Env:     env,
	}
	if dir := strings.TrimSpace(req.WorkspaceDir); dir != "" {
		startReq.Binds = []string{dir + ":/workspace:rw"}
		startReq.WorkingDir = "/workspace"
	}
	handle, err := l.runtime.Start(ctx, startReq)
	if err != nil {
		return Handle{}, nil, err
	}
	return handle, env, nil
}

// RunOnce launches, executes the adapter command, then stops the runtime.
func (l *Launcher) RunOnce(ctx context.Context, req LaunchRequest) (ExecResult, error) {
	cfg, err := l.registry.Get(req.HarnessID)
	if err != nil {
		return ExecResult{}, err
	}
	handle, _, err := l.Launch(ctx, req)
	if err != nil {
		return ExecResult{}, err
	}
	defer func() { _ = l.runtime.Stop(context.Background(), handle) }()
	return l.runtime.Exec(ctx, ExecRequest{Handle: handle, Command: cfg.Command})
}
