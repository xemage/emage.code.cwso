package harness

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// DockerRuntimeConfig configures the Docker harness runtime.
type DockerRuntimeConfig struct {
	Host        string
	NetworkMode string
	StopTimeout time.Duration
}

// DockerRuntime implements Runtime via the Docker HTTP API.
type DockerRuntime struct {
	client dockerAPI
	cfg    DockerRuntimeConfig
}

// NewDockerRuntime constructs a Docker-backed harness runtime.
func NewDockerRuntime(cfg DockerRuntimeConfig) (*DockerRuntime, error) {
	client, err := newDockerHTTPClient(cfg.Host)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.NetworkMode) == "" {
		cfg.NetworkMode = "bridge"
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = 5 * time.Second
	}
	return &DockerRuntime{client: client, cfg: cfg}, nil
}

func newDockerRuntimeWithClient(cfg DockerRuntimeConfig, client dockerAPI) *DockerRuntime {
	if strings.TrimSpace(cfg.NetworkMode) == "" {
		cfg.NetworkMode = "bridge"
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = 5 * time.Second
	}
	return &DockerRuntime{client: client, cfg: cfg}
}

func (r *DockerRuntime) Name() string { return "docker" }

func (r *DockerRuntime) Start(ctx context.Context, req StartRequest) (Handle, error) {
	if err := req.Validate(); err != nil {
		return Handle{}, fmt.Errorf("%w: %v", ErrInvalidRuntimeRequest, err)
	}
	body := dockerCreateRequest{
		Image:      req.Image,
		Cmd:        req.Command,
		Env:        mapToSortedEnv(req.Env),
		WorkingDir: strings.TrimSpace(req.WorkingDir),
		HostConfig: dockerHostConfig{
			Binds:          req.Binds,
			NetworkMode:    r.cfg.NetworkMode,
			ReadonlyRootfs: false,
		},
	}
	containerID, err := r.client.CreateContainer(ctx, req.Name, body)
	if err != nil {
		return Handle{}, err
	}
	if err := r.client.StartContainer(ctx, containerID); err != nil {
		_ = r.client.RemoveContainer(context.Background(), containerID)
		return Handle{}, err
	}
	return Handle{ID: req.Name, ContainerID: containerID}, nil
}

func (r *DockerRuntime) Stop(ctx context.Context, handle Handle) error {
	if strings.TrimSpace(handle.ContainerID) == "" {
		return fmt.Errorf("%w: container id is required", ErrInvalidRuntimeRequest)
	}
	timeout := int(r.cfg.StopTimeout.Seconds())
	if timeout <= 0 {
		timeout = 5
	}
	_ = r.client.StopContainer(ctx, handle.ContainerID, timeout)
	return r.client.RemoveContainer(ctx, handle.ContainerID)
}

func (r *DockerRuntime) Exec(ctx context.Context, req ExecRequest) (ExecResult, error) {
	if err := req.Validate(); err != nil {
		return ExecResult{}, fmt.Errorf("%w: %v", ErrInvalidRuntimeRequest, err)
	}
	execID, err := r.client.CreateExec(ctx, req.Handle.ContainerID, dockerExecCreateRequest{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          req.Command,
	})
	if err != nil {
		return ExecResult{}, err
	}
	stdout, stderr, err := r.client.StartExec(ctx, execID)
	if err != nil {
		return ExecResult{}, err
	}
	return ExecResult{ExitCode: 0, Stdout: stdout, Stderr: stderr}, nil
}

func (r *DockerRuntime) Upload(ctx context.Context, req TransferRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRuntimeRequest, err)
	}
	remoteDir := filepath.ToSlash(filepath.Dir(req.RemotePath))
	remoteName := filepath.Base(req.RemotePath)
	tarBody, err := buildFileTar(req.LocalPath, remoteName)
	if err != nil {
		return err
	}
	return r.client.PutArchive(ctx, req.Handle.ContainerID, remoteDir, bytes.NewReader(tarBody))
}

func (r *DockerRuntime) Download(ctx context.Context, req TransferRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRuntimeRequest, err)
	}
	body, err := r.client.GetArchive(ctx, req.Handle.ContainerID, req.RemotePath)
	if err != nil {
		return err
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	return extractFirstFileFromTar(raw, req.LocalPath)
}
