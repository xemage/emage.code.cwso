package sandbox

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	// ErrInvalidRequest is returned when a run request is malformed.
	ErrInvalidRequest = errors.New("invalid sandbox run request")
)

// SandboxProfile declares the requested sandbox isolation tier.
type SandboxProfile string

const (
	// ProfileDockerTrusted targets the docker-trusted baseline tier (internal tooling only).
	ProfileDockerTrusted SandboxProfile = "docker-trusted"
	// ProfileGVisorFastEphemeral targets the gVisor ephemeral tier (benign sub-agent logic).
	ProfileGVisorFastEphemeral SandboxProfile = "gvisor-fast-ephemeral"
	// ProfileFirecrackerSecure targets the Firecracker isolation tier (untrusted/LLM-generated code).
	ProfileFirecrackerSecure SandboxProfile = "firecracker-secure-isolation"
)

// ValidSandboxProfiles is the complete set of allowed caller-supplied profiles.
var ValidSandboxProfiles = map[SandboxProfile]bool{
	ProfileDockerTrusted:       true,
	ProfileGVisorFastEphemeral: true,
	ProfileFirecrackerSecure:   true,
}

// RunnerInterface is the shared execution contract for sandbox backends.
type RunnerInterface interface {
	Name() string
	Execute(ctx context.Context, req RunRequest) (RunResult, error)
	Health(ctx context.Context) error
}

// RunRequest describes a single sandboxed process execution.
type RunRequest struct {
	Name              string
	Image             string
	Command           []string
	Env               map[string]string
	WorkingDir        string
	MountWorkspace    bool
	WorkspaceDir      string
	WorkspaceWritable bool
	RootFSWritable    bool
	Timeout           time.Duration
	// SandboxProfile is the caller's requested isolation tier. The tier router
	// enforces server-side policy and may override this value.
	SandboxProfile SandboxProfile
}

// Validate checks request invariants that all runners rely on.
func (r RunRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("name is required")
	}
	if r.WorkspaceWritable && !r.MountWorkspace {
		return errors.New("workspace_writable requires mount_workspace")
	}
	if r.MountWorkspace && strings.TrimSpace(r.WorkspaceDir) == "" {
		return errors.New("workspace_dir is required when mount_workspace is true")
	}
	if r.Timeout < 0 {
		return errors.New("timeout must be >= 0")
	}
	return nil
}

// RunResult captures deterministic execution outputs.
type RunResult struct {
	ContainerID      string
	ExitCode         int
	Stdout           string
	Stderr           string
	StartedAt        time.Time
	FinishedAt       time.Time
	Cancelled        bool
	TimedOut         bool
	ResourceExceeded bool
	FailureReason    string
	// SandboxTier is the tier that was actually used for this execution.
	SandboxTier string
	// RoutingReason is an auditable code explaining why the tier was selected.
	RoutingReason string
}
