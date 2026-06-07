package harness

import (
	"archive/tar"
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
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const dockerHTTPVersionPrefix = "/v1.43"

type dockerAPI interface {
	CreateContainer(ctx context.Context, name string, req dockerCreateRequest) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, timeoutSeconds int) error
	RemoveContainer(ctx context.Context, containerID string) error
	CreateExec(ctx context.Context, containerID string, req dockerExecCreateRequest) (string, error)
	StartExec(ctx context.Context, execID string) (stdout string, stderr string, err error)
	PutArchive(ctx context.Context, containerID, remotePath string, tarBody io.Reader) error
	GetArchive(ctx context.Context, containerID, remotePath string) (io.ReadCloser, error)
}

type dockerHTTPClient struct {
	httpClient *http.Client
	baseURL    string
}

func newDockerHTTPClient(host string) (*dockerHTTPClient, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}
	if strings.HasPrefix(host, "unix://") {
		sockPath := strings.TrimPrefix(host, "unix://")
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", sockPath)
			},
		}
		return &dockerHTTPClient{
			httpClient: &http.Client{Transport: transport},
			baseURL:    "http://docker" + dockerHTTPVersionPrefix,
		}, nil
	}
	if strings.HasPrefix(host, "tcp://") {
		host = "http://" + strings.TrimPrefix(host, "tcp://")
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		return nil, fmt.Errorf("unsupported docker host: %s", host)
	}
	return &dockerHTTPClient{
		httpClient: http.DefaultClient,
		baseURL:    strings.TrimRight(host, "/") + dockerHTTPVersionPrefix,
	}, nil
}

func (c *dockerHTTPClient) request(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
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

func decodeDockerError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	msg := strings.TrimSpace(string(body))
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
		msg = payload.Message
	}
	return fmt.Errorf("docker api status=%d: %s", resp.StatusCode, msg)
}

func demuxDockerStream(raw []byte) (stdout string, stderr string) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	for i := 0; i+8 <= len(raw); {
		streamType := raw[i]
		frameSize := int(binary.BigEndian.Uint32(raw[i+4 : i+8]))
		i += 8
		if frameSize < 0 || i+frameSize > len(raw) {
			break
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
	return outBuf.String(), errBuf.String()
}

func mapToSortedEnv(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+values[key])
	}
	return out
}

func buildFileTar(localPath, remoteName string) ([]byte, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("upload expects a file path")
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	header := &tar.Header{
		Name:    filepath.ToSlash(remoteName),
		Mode:    0o644,
		Size:    int64(len(data)),
		ModTime: info.ModTime(),
	}
	if err := writer.WriteHeader(header); err != nil {
		return nil, err
	}
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func extractFirstFileFromTar(raw []byte, localPath string) error {
	reader := tar.NewReader(bytes.NewReader(raw))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("archive contained no files")
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(localPath, data, 0o644)
	}
}

type dockerCreateRequest struct {
	Image      string           `json:"Image"`
	Cmd        []string         `json:"Cmd,omitempty"`
	Env        []string         `json:"Env,omitempty"`
	WorkingDir string           `json:"WorkingDir,omitempty"`
	HostConfig dockerHostConfig `json:"HostConfig"`
}

type dockerHostConfig struct {
	Binds          []string `json:"Binds,omitempty"`
	NetworkMode    string   `json:"NetworkMode"`
	ReadonlyRootfs bool     `json:"ReadonlyRootfs"`
}

type dockerExecCreateRequest struct {
	AttachStdout bool     `json:"AttachStdout"`
	AttachStderr bool     `json:"AttachStderr"`
	Cmd          []string `json:"Cmd"`
}

func (c *dockerHTTPClient) CreateContainer(ctx context.Context, name string, req dockerCreateRequest) (string, error) {
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
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
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

func (c *dockerHTTPClient) RemoveContainer(ctx context.Context, containerID string) error {
	resp, err := c.request(ctx, http.MethodDelete, "/containers/"+containerID+"?force=1", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return decodeDockerError(resp)
	}
	return nil
}

func (c *dockerHTTPClient) CreateExec(ctx context.Context, containerID string, req dockerExecCreateRequest) (string, error) {
	resp, err := c.request(ctx, http.MethodPost, "/containers/"+containerID+"/exec", req)
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
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.ID, nil
}

func (c *dockerHTTPClient) StartExec(ctx context.Context, execID string) (string, string, error) {
	resp, err := c.request(ctx, http.MethodPost, "/exec/"+execID+"/start", map[string]bool{"Detach": false, "Tty": false})
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", decodeDockerError(resp)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	stdout, stderr := demuxDockerStream(raw)
	return stdout, stderr, nil
}

func (c *dockerHTTPClient) PutArchive(ctx context.Context, containerID, remotePath string, tarBody io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/containers/"+containerID+"/archive?path="+url.QueryEscape(remotePath), tarBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decodeDockerError(resp)
	}
	return nil
}

func (c *dockerHTTPClient) GetArchive(ctx context.Context, containerID, remotePath string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/containers/"+containerID+"/archive?path="+url.QueryEscape(remotePath), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, decodeDockerError(resp)
	}
	return resp.Body, nil
}
