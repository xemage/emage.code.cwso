package harness

import (
	"context"
	"errors"
	"strings"
)

var (
	// ErrInvalidRuntimeRequest is returned when a runtime request fails validation.
	ErrInvalidRuntimeRequest = errors.New("invalid harness runtime request")
)

// Handle identifies a running harness container or process.
type Handle struct {
	ID          string
	ContainerID string
}

// StartRequest starts a long-lived harness runtime (Polar §3.2.2).
type StartRequest struct {
	Name       string
	Image      string
	Command    []string
	Env        map[string]string
	WorkingDir string
	Binds      []string
}

// Validate checks start request invariants.
func (r StartRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(r.Image) == "" {
		return errors.New("image is required")
	}
	if len(r.Command) == 0 {
		return errors.New("command is required")
	}
	return nil
}

// ExecRequest runs a command inside a started runtime.
type ExecRequest struct {
	Handle  Handle
	Command []string
}

// Validate checks exec request invariants.
func (r ExecRequest) Validate() error {
	if strings.TrimSpace(r.Handle.ContainerID) == "" && strings.TrimSpace(r.Handle.ID) == "" {
		return errors.New("handle is required")
	}
	if len(r.Command) == 0 {
		return errors.New("command is required")
	}
	return nil
}

// ExecResult captures command output from Exec.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// TransferRequest moves a file between host and runtime.
type TransferRequest struct {
	Handle     Handle
	LocalPath  string
	RemotePath string
}

// Validate checks transfer request invariants.
func (r TransferRequest) Validate() error {
	if strings.TrimSpace(r.Handle.ContainerID) == "" {
		return errors.New("container handle is required")
	}
	if strings.TrimSpace(r.LocalPath) == "" || strings.TrimSpace(r.RemotePath) == "" {
		return errors.New("local and remote paths are required")
	}
	return nil
}

// Runtime is the Polar-style harness execution surface (Docker first).
type Runtime interface {
	Name() string
	Start(ctx context.Context, req StartRequest) (Handle, error)
	Stop(ctx context.Context, handle Handle) error
	Exec(ctx context.Context, req ExecRequest) (ExecResult, error)
	Upload(ctx context.Context, req TransferRequest) error
	Download(ctx context.Context, req TransferRequest) error
}
