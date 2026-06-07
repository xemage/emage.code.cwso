package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
)

type localSession struct {
	cmd *exec.Cmd
	env map[string]string
}

// LocalRuntime executes harness commands on the host (tests and dev only).
type LocalRuntime struct {
	workDir string
	seq     atomic.Uint64
	procs   map[string]*localSession
}

// NewLocalRuntime constructs a host-process runtime rooted at workDir.
func NewLocalRuntime(workDir string) *LocalRuntime {
	return &LocalRuntime{workDir: workDir, procs: make(map[string]*localSession)}
}

func (r *LocalRuntime) Name() string { return "local" }

func (r *LocalRuntime) Start(ctx context.Context, req StartRequest) (Handle, error) {
	if err := req.Validate(); err != nil {
		return Handle{}, fmt.Errorf("%w: %v", ErrInvalidRuntimeRequest, err)
	}
	id := fmt.Sprintf("local-%d", r.seq.Add(1))
	cmd := exec.CommandContext(ctx, req.Command[0], req.Command[1:]...)
	cmd.Env = envMapToSlice(req.Env)
	if dir := strings.TrimSpace(req.WorkingDir); dir != "" {
		cmd.Dir = dir
	} else if r.workDir != "" {
		cmd.Dir = r.workDir
	}
	if err := cmd.Start(); err != nil {
		return Handle{}, err
	}
	r.procs[id] = &localSession{cmd: cmd, env: req.Env}
	return Handle{ID: id, ContainerID: id}, nil
}

func (r *LocalRuntime) Stop(ctx context.Context, handle Handle) error {
	session, ok := r.procs[handle.ID]
	if !ok {
		return nil
	}
	delete(r.procs, handle.ID)
	if session.cmd.Process != nil {
		_ = session.cmd.Process.Kill()
	}
	done := make(chan error, 1)
	go func() { done <- session.cmd.Wait() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (r *LocalRuntime) Exec(ctx context.Context, req ExecRequest) (ExecResult, error) {
	if err := req.Validate(); err != nil {
		return ExecResult{}, fmt.Errorf("%w: %v", ErrInvalidRuntimeRequest, err)
	}
	cmd := exec.CommandContext(ctx, req.Command[0], req.Command[1:]...)
	if session, ok := r.procs[req.Handle.ID]; ok && len(session.env) > 0 {
		cmd.Env = envMapToSlice(session.env)
	}
	if r.workDir != "" {
		cmd.Dir = r.workDir
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return ExecResult{}, err
		}
	}
	return ExecResult{ExitCode: exitCode, Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

func (r *LocalRuntime) Upload(ctx context.Context, req TransferRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRuntimeRequest, err)
	}
	_ = ctx
	dest := filepath.Join(r.workDir, filepath.Base(req.RemotePath))
	data, err := os.ReadFile(req.LocalPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

func (r *LocalRuntime) Download(ctx context.Context, req TransferRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRuntimeRequest, err)
	}
	_ = ctx
	src := filepath.Join(r.workDir, filepath.Base(req.RemotePath))
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(req.LocalPath, data, 0o644)
}

func envMapToSlice(values map[string]string) []string {
	if len(values) == 0 {
		return os.Environ()
	}
	out := make([]string, 0, len(values))
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
}
