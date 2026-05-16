package sandbox

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDockerHost       = "unix:///var/run/docker.sock"
	defaultDockerImage      = "alpine:3.20"
	defaultDockerNetwork    = "none"
	defaultCPUPeriodMicros  = int64(100000)
	defaultCPUQuotaMicros   = int64(100000)
	defaultMemoryBytes      = int64(256 * 1024 * 1024)
	defaultPIDsLimit        = int64(128)
	defaultStopTimeout      = 5 * time.Second
	defaultCleanupRetries   = 3
	dockerHTTPVersionPrefix = "/v1.43"
)

type DockerRunnerConfig struct {
	Host               string
	DefaultImage       string
	DefaultCommand     []string
	Runtime            string
	NetworkMode        string
	ReadOnlyRootFS     bool
	CPUQuotaMicros     int64
	MemoryBytes        int64
	PIDsLimit          int64
	StopTimeout        time.Duration
	CleanupRetries     int
	NoNewPrivileges    bool
	DropAllCaps        bool
	CreateTimeout      time.Duration
	ListTimeout        time.Duration
	AllowWritableMount bool
}

func (c DockerRunnerConfig) withDefaults() DockerRunnerConfig {
	if strings.TrimSpace(c.Host) == "" {
		c.Host = defaultDockerHost
	}
	if strings.TrimSpace(c.DefaultImage) == "" {
		c.DefaultImage = defaultDockerImage
	}
	if c.NetworkMode == "" {
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
	if !c.NoNewPrivileges {
		c.NoNewPrivileges = true
	}
	if !c.DropAllCaps {
		c.DropAllCaps = true
	}
	if !c.ReadOnlyRootFS {
		c.ReadOnlyRootFS = true
	}
	return c
}

func (c DockerRunnerConfig) validate() error {
	if strings.EqualFold(c.NetworkMode, "host") {
		return errors.New("docker network mode host is forbidden")
	}
	if c.CPUQuotaMicros <= 0 {
		return errors.New("cpu_quota_micros must be > 0")
	}
	if c.MemoryBytes <= 0 {
		return errors.New("memory_bytes must be > 0")
	}
	if c.PIDsLimit <= 0 {
		return errors.New("pids_limit must be > 0")
	}
	if c.StopTimeout <= 0 {
		return errors.New("stop_timeout must be > 0")
	}
	if c.CleanupRetries <= 0 {
		return errors.New("cleanup_retries must be > 0")
	}
	return nil
}

type DockerRunner struct {
	client dockerAPI
	cfg    DockerRunnerConfig
	now    func() time.Time
	sleep  func(time.Duration)
}

func NewDockerRunner(cfg DockerRunnerConfig) (*DockerRunner, error) {
	resolved := cfg.withDefaults()
	if err := resolved.validate(); err != nil {
		return nil, fmt.Errorf("docker runner config: %w", err)
	}
	client, err := newDockerHTTPClient(resolved.Host)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &DockerRunner{client: client, cfg: resolved, now: time.Now, sleep: time.Sleep}, nil
}

func newDockerRunnerWithClient(cfg DockerRunnerConfig, client dockerAPI) (*DockerRunner, error) {
	resolved := cfg.withDefaults()
	if err := resolved.validate(); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.New("docker client is required")
	}
	return &DockerRunner{client: client, cfg: resolved, now: time.Now, sleep: time.Sleep}, nil
}

func (r *DockerRunner) Name() string { return "docker-trusted" }

func (r *DockerRunner) Health(ctx context.Context) error {
	return r.client.Ping(ctx)
}

func (r *DockerRunner) Execute(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := req.Validate(); err != nil {
		return RunResult{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	createBody, err := r.buildCreateRequest(req)
	if err != nil {
		return RunResult{}, err
	}

	createCtx, cancelCreate := context.WithTimeout(ctx, r.cfg.CreateTimeout)
	containerID, err := r.client.CreateContainer(createCtx, req.Name, createBody)
	cancelCreate()
	if err != nil {
		return RunResult{}, err
	}

	result := RunResult{ContainerID: containerID, StartedAt: r.now()}
	if err := r.client.StartContainer(ctx, containerID); err != nil {
		cleanupErr := r.cleanup(context.Background(), req.Name, containerID)
		if cleanupErr != nil {
			return result, fmt.Errorf("start container: %w (cleanup failed: %v)", err, cleanupErr)
		}
		return result, fmt.Errorf("start container: %w", err)
	}

	waitCh := make(chan waitResponse, 1)
	waitErrCh := make(chan error, 1)
	go func() {
		waitResp, waitErr := r.client.WaitContainer(context.Background(), containerID)
		if waitErr != nil {
			waitErrCh <- waitErr
			return
		}
		waitCh <- waitResp
	}()

	select {
	case <-ctx.Done():
		result.FinishedAt = r.now()
		result.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
		result.Cancelled = errors.Is(ctx.Err(), context.Canceled)
		if !result.Cancelled && !result.TimedOut {
			result.Cancelled = true
		}
		result.Stdout, result.Stderr = r.collectLogs(containerID)
		cleanupErr := r.cleanup(context.Background(), req.Name, containerID)
		if cleanupErr != nil {
			return result, fmt.Errorf("sandbox canceled: %w (cleanup failed: %v)", ctx.Err(), cleanupErr)
		}
		return result, ctx.Err()
	case waitErr := <-waitErrCh:
		result.FinishedAt = r.now()
		result.Stdout, result.Stderr = r.collectLogs(containerID)
		cleanupErr := r.cleanup(context.Background(), req.Name, containerID)
		if cleanupErr != nil {
			return result, fmt.Errorf("wait container: %w (cleanup failed: %v)", waitErr, cleanupErr)
		}
		return result, fmt.Errorf("wait container: %w", waitErr)
	case waitResp := <-waitCh:
		result.FinishedAt = r.now()
		result.ExitCode = int(waitResp.StatusCode)
		result.Stdout, result.Stderr = r.collectLogs(containerID)
		if msg := strings.TrimSpace(waitResp.Error.Message); msg != "" {
			result.FailureReason = msg
			if strings.Contains(strings.ToLower(msg), "oom") || strings.Contains(strings.ToLower(msg), "memory") {
				result.ResourceExceeded = true
			}
		}
		cleanupErr := r.cleanup(context.Background(), req.Name, containerID)
		if cleanupErr != nil {
			return result, fmt.Errorf("cleanup failed: %w", cleanupErr)
		}
		return result, nil
	}
}

func (r *DockerRunner) buildCreateRequest(req RunRequest) (dockerCreateContainerRequest, error) {
	image := strings.TrimSpace(req.Image)
	if image == "" {
		image = r.cfg.DefaultImage
	}
	if image == "" {
		return dockerCreateContainerRequest{}, errors.New("image is required")
	}
	command := req.Command
	if len(command) == 0 {
		command = r.cfg.DefaultCommand
	}
	if len(command) == 0 {
		command = []string{"/bin/sh", "-lc", "echo ${CWSO_OBJECTIVE_PROMPT:-cwso-job}"}
	}

	env := mapToSortedEnv(req.Env)
	readOnlyRootFS := r.cfg.ReadOnlyRootFS && !req.RootFSWritable
	if req.RootFSWritable && !r.cfg.AllowWritableMount {
		return dockerCreateContainerRequest{}, errors.New("rootfs writable override is disabled")
	}
	hostConfig := dockerHostConfig{
		NetworkMode:    r.cfg.NetworkMode,
		Runtime:        strings.TrimSpace(r.cfg.Runtime),
		ReadonlyRootfs: readOnlyRootFS,
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges:true"},
		CpuPeriod:      defaultCPUPeriodMicros,
		CpuQuota:       r.cfg.CPUQuotaMicros,
		Memory:         r.cfg.MemoryBytes,
		PidsLimit:      r.cfg.PIDsLimit,
		Privileged:     false,
	}

	if req.MountWorkspace {
		mode := "ro"
		if req.WorkspaceWritable {
			mode = "rw"
		}
		hostConfig.Binds = append(hostConfig.Binds, req.WorkspaceDir+":/workspace:"+mode)
	}

	return dockerCreateContainerRequest{
		Image:      image,
		Cmd:        command,
		Env:        env,
		WorkingDir: strings.TrimSpace(req.WorkingDir),
		HostConfig: hostConfig,
	}, nil
}

func (r *DockerRunner) collectLogs(containerID string) (stdout string, stderr string) {
	body, err := r.client.ContainerLogs(context.Background(), containerID)
	if err != nil {
		return "", ""
	}
	defer body.Close()
	stdOutBuf, stdErrBuf, demuxErr := demuxDockerStream(body)
	if demuxErr != nil {
		return stdOutBuf, stdErrBuf
	}
	return stdOutBuf, stdErrBuf
}

func (r *DockerRunner) cleanup(ctx context.Context, name string, containerID string) error {
	if stopErr := r.client.StopContainer(ctx, containerID, int(r.cfg.StopTimeout.Seconds())); stopErr != nil && !isIgnorableDockerErr(stopErr) {
		return fmt.Errorf("stop: %w", stopErr)
	}
	if killErr := r.client.KillContainer(ctx, containerID); killErr != nil && !isIgnorableDockerErr(killErr) {
		return fmt.Errorf("kill: %w", killErr)
	}

	var removeErr error
	for i := 0; i < r.cfg.CleanupRetries; i++ {
		removeErr = r.client.RemoveContainer(ctx, containerID)
		if removeErr == nil || isIgnorableDockerErr(removeErr) {
			removeErr = nil
			break
		}
		r.sleep(100 * time.Millisecond)
	}
	if removeErr != nil {
		return fmt.Errorf("remove: %w", removeErr)
	}

	listCtx, cancelList := context.WithTimeout(context.Background(), r.cfg.ListTimeout)
	defer cancelList()
	containers, err := r.client.ListContainersByName(listCtx, name)
	if err != nil {
		return fmt.Errorf("verify cleanup: %w", err)
	}
	if len(containers) > 0 {
		return fmt.Errorf("verify cleanup: leaked containers remain for %q", name)
	}
	return nil
}

func mapToSortedEnv(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}

func demuxDockerStream(r io.Reader) (stdout string, stderr string, err error) {
	raw, readErr := io.ReadAll(r)
	if readErr != nil {
		return "", "", readErr
	}
	if len(raw) < 8 {
		return string(raw), "", nil
	}

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	for i := 0; i+8 <= len(raw); {
		streamType := raw[i]
		frameSize := int(binary.BigEndian.Uint32(raw[i+4 : i+8]))
		i += 8
		if frameSize < 0 || i+frameSize > len(raw) {
			return outBuf.String(), errBuf.String(), nil
		}
		frame := raw[i : i+frameSize]
		i += frameSize
		switch streamType {
		case 1:
			_, _ = outBuf.Write(frame)
		case 2:
			_, _ = errBuf.Write(frame)
		}
	}
	return outBuf.String(), errBuf.String(), nil
}

func isIgnorableDockerErr(err error) bool {
	if err == nil {
		return true
	}
	var apiErr *dockerAPIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusNotModified || apiErr.StatusCode == http.StatusConflict
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such container") || strings.Contains(msg, "is not running")
}

type dockerAPI interface {
	Ping(ctx context.Context) error
	Info(ctx context.Context) (dockerInfo, error)
	CreateContainer(ctx context.Context, name string, req dockerCreateContainerRequest) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	WaitContainer(ctx context.Context, containerID string) (waitResponse, error)
	ContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error)
	StopContainer(ctx context.Context, containerID string, timeoutSeconds int) error
	KillContainer(ctx context.Context, containerID string) error
	RemoveContainer(ctx context.Context, containerID string) error
	ListContainersByName(ctx context.Context, name string) ([]string, error)
}

type dockerHTTPClient struct {
	httpClient *http.Client
	baseURL    string
}

func newDockerHTTPClient(host string) (*dockerHTTPClient, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = defaultDockerHost
	}

	if strings.HasPrefix(host, "unix://") {
		sockPath := strings.TrimPrefix(host, "unix://")
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sockPath)
			},
		}
		return &dockerHTTPClient{httpClient: &http.Client{Transport: transport}, baseURL: "http://docker" + dockerHTTPVersionPrefix}, nil
	}

	if strings.HasPrefix(host, "tcp://") {
		host = "http://" + strings.TrimPrefix(host, "tcp://")
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		return nil, fmt.Errorf("unsupported docker host: %s", host)
	}
	return &dockerHTTPClient{httpClient: http.DefaultClient, baseURL: strings.TrimRight(host, "/") + dockerHTTPVersionPrefix}, nil
}

func (c *dockerHTTPClient) Ping(ctx context.Context) error {
	resp, err := c.request(ctx, http.MethodGet, "/_ping", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decodeDockerError(resp)
	}
	return nil
}

type dockerInfo struct {
	DefaultRuntime string                     `json:"DefaultRuntime"`
	Runtimes       map[string]json.RawMessage `json:"Runtimes"`
}

func (i dockerInfo) RuntimeNames() []string {
	if len(i.Runtimes) == 0 {
		return nil
	}
	names := make([]string, 0, len(i.Runtimes))
	for name := range i.Runtimes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (i dockerInfo) HasRuntime(runtime string) bool {
	runtime = strings.TrimSpace(runtime)
	if runtime == "" {
		return false
	}
	_, ok := i.Runtimes[runtime]
	return ok
}

func (c *dockerHTTPClient) Info(ctx context.Context) (dockerInfo, error) {
	resp, err := c.request(ctx, http.MethodGet, "/info", nil)
	if err != nil {
		return dockerInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return dockerInfo{}, decodeDockerError(resp)
	}
	var payload dockerInfo
	if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr != nil {
		return dockerInfo{}, decodeErr
	}
	if payload.Runtimes == nil {
		payload.Runtimes = map[string]json.RawMessage{}
	}
	return payload, nil
}

func (c *dockerHTTPClient) CreateContainer(ctx context.Context, name string, req dockerCreateContainerRequest) (string, error) {
	q := url.Values{}
	q.Set("name", name)
	resp, err := c.request(ctx, http.MethodPost, "/containers/create?"+q.Encode(), req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", decodeDockerError(resp)
	}
	var payload struct {
		ID string `json:"Id"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr != nil {
		return "", decodeErr
	}
	if strings.TrimSpace(payload.ID) == "" {
		return "", errors.New("docker create returned empty id")
	}
	return payload.ID, nil
}

func (c *dockerHTTPClient) StartContainer(ctx context.Context, containerID string) error {
	resp, err := c.request(ctx, http.MethodPost, "/containers/"+containerID+"/start", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		return decodeDockerError(resp)
	}
	return nil
}

type waitResponse struct {
	StatusCode int64 `json:"StatusCode"`
	Error      struct {
		Message string `json:"Message"`
	} `json:"Error"`
}

func (c *dockerHTTPClient) WaitContainer(ctx context.Context, containerID string) (waitResponse, error) {
	resp, err := c.request(ctx, http.MethodPost, "/containers/"+containerID+"/wait?condition=not-running", nil)
	if err != nil {
		return waitResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return waitResponse{}, decodeDockerError(resp)
	}
	var payload waitResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr != nil {
		return waitResponse{}, decodeErr
	}
	return payload, nil
}

func (c *dockerHTTPClient) ContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	path := "/containers/" + containerID + "/logs?stdout=1&stderr=1"
	resp, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, decodeDockerError(resp)
	}
	return resp.Body, nil
}

func (c *dockerHTTPClient) StopContainer(ctx context.Context, containerID string, timeoutSeconds int) error {
	resp, err := c.request(ctx, http.MethodPost, "/containers/"+containerID+"/stop?t="+strconv.Itoa(timeoutSeconds), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		return decodeDockerError(resp)
	}
	return nil
}

func (c *dockerHTTPClient) KillContainer(ctx context.Context, containerID string) error {
	resp, err := c.request(ctx, http.MethodPost, "/containers/"+containerID+"/kill", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		return decodeDockerError(resp)
	}
	return nil
}

func (c *dockerHTTPClient) RemoveContainer(ctx context.Context, containerID string) error {
	resp, err := c.request(ctx, http.MethodDelete, "/containers/"+containerID+"?force=1&v=1", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return decodeDockerError(resp)
	}
	return nil
}

func (c *dockerHTTPClient) ListContainersByName(ctx context.Context, name string) ([]string, error) {
	filter := map[string][]string{"name": {"^/" + name + "$"}}
	b, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}
	path := "/containers/json?all=1&filters=" + url.QueryEscape(string(b))
	resp, reqErr := c.request(ctx, http.MethodGet, path, nil)
	if reqErr != nil {
		return nil, reqErr
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, decodeDockerError(resp)
	}
	var payload []struct {
		ID string `json:"Id"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr != nil {
		return nil, decodeErr
	}
	out := make([]string, 0, len(payload))
	for _, item := range payload {
		if strings.TrimSpace(item.ID) != "" {
			out = append(out, item.ID)
		}
	}
	return out, nil
}

func (c *dockerHTTPClient) request(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

type dockerAPIError struct {
	StatusCode int
	Message    string
}

func (e *dockerAPIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("docker api status=%d", e.StatusCode)
	}
	return fmt.Sprintf("docker api status=%d: %s", e.StatusCode, e.Message)
}

func decodeDockerError(resp *http.Response) error {
	if resp == nil {
		return errors.New("docker response is nil")
	}
	body, _ := io.ReadAll(resp.Body)
	msg := strings.TrimSpace(string(body))
	if msg != "" {
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
			msg = payload.Message
		}
	}
	return &dockerAPIError{StatusCode: resp.StatusCode, Message: msg}
}

type dockerCreateContainerRequest struct {
	Image      string           `json:"Image"`
	Cmd        []string         `json:"Cmd,omitempty"`
	Env        []string         `json:"Env,omitempty"`
	WorkingDir string           `json:"WorkingDir,omitempty"`
	HostConfig dockerHostConfig `json:"HostConfig"`
}

type dockerHostConfig struct {
	Binds          []string `json:"Binds,omitempty"`
	NetworkMode    string   `json:"NetworkMode"`
	Runtime        string   `json:"Runtime,omitempty"`
	ReadonlyRootfs bool     `json:"ReadonlyRootfs"`
	CapDrop        []string `json:"CapDrop,omitempty"`
	SecurityOpt    []string `json:"SecurityOpt,omitempty"`
	CpuPeriod      int64    `json:"CpuPeriod,omitempty"`
	CpuQuota       int64    `json:"CpuQuota,omitempty"`
	Memory         int64    `json:"Memory,omitempty"`
	PidsLimit      int64    `json:"PidsLimit,omitempty"`
	Privileged     bool     `json:"Privileged"`
}
