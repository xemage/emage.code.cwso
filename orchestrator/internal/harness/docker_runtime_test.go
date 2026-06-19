package harness

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeDockerAPI struct {
	createID   string
	execStdout string
	execStderr string
	calls      []string
}

func (f *fakeDockerAPI) CreateContainer(_ context.Context, name string, _ dockerCreateRequest) (string, error) {
	f.calls = append(f.calls, "create:"+name)
	if f.createID == "" {
		f.createID = "ctr-1"
	}
	return f.createID, nil
}

func (f *fakeDockerAPI) StartContainer(_ context.Context, _ string) error {
	f.calls = append(f.calls, "start")
	return nil
}

func (f *fakeDockerAPI) StopContainer(_ context.Context, _ string, _ int) error {
	f.calls = append(f.calls, "stop")
	return nil
}

func (f *fakeDockerAPI) RemoveContainer(_ context.Context, _ string) error {
	f.calls = append(f.calls, "remove")
	return nil
}

func (f *fakeDockerAPI) CreateExec(_ context.Context, _ string, _ dockerExecCreateRequest) (string, error) {
	f.calls = append(f.calls, "exec-create")
	return "exec-1", nil
}

func (f *fakeDockerAPI) StartExec(_ context.Context, _ string) (string, string, error) {
	f.calls = append(f.calls, "exec-start")
	return f.execStdout, f.execStderr, nil
}

func (f *fakeDockerAPI) PutArchive(_ context.Context, _ string, _ string, _ io.Reader) error {
	f.calls = append(f.calls, "upload")
	return nil
}

func (f *fakeDockerAPI) GetArchive(_ context.Context, _ string, _ string) (io.ReadCloser, error) {
	f.calls = append(f.calls, "download")
	tarBody, err := buildBytesTar("remote.txt", []byte("payload"))
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(tarBody)), nil
}

func TestDockerRuntimeStartStopExec(t *testing.T) {
	fake := &fakeDockerAPI{execStdout: "ok\n"}
	runtime := newDockerRuntimeWithClient(DockerRuntimeConfig{}, fake)
	handle, err := runtime.Start(context.Background(), StartRequest{
		Name: "sess-1", Image: "alpine:3.20", Command: []string{"sleep", "3600"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Exec(context.Background(), ExecRequest{
		Handle: handle, Command: []string{"/bin/sh", "-lc", "echo ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "ok\n" {
		t.Fatalf("stdout=%q", result.Stdout)
	}
	if err := runtime.Stop(context.Background(), handle); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(fake.calls, ","), "create:sess-1") {
		t.Fatalf("calls=%v", fake.calls)
	}
}

func TestDockerRuntimeUploadDownload(t *testing.T) {
	fake := &fakeDockerAPI{}
	runtime := newDockerRuntimeWithClient(DockerRuntimeConfig{}, fake)
	handle := Handle{ContainerID: "ctr-1"}
	local := writeTempFile(t, "local.txt", "seed")
	if err := runtime.Upload(context.Background(), TransferRequest{
		Handle: handle, LocalPath: local, RemotePath: "/workspace/remote.txt",
	}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(filepath.Dir(local), "downloaded.txt")
	if err := runtime.Download(context.Background(), TransferRequest{
		Handle: handle, LocalPath: dest, RemotePath: "/workspace/remote.txt",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" {
		t.Fatalf("download=%q", string(data))
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDemuxDockerStreamSplitsStdoutStderr(t *testing.T) {
	raw := multiplexDockerLogs("hello", "warn")
	stdout, stderr := demuxDockerStream(raw)
	if stdout != "hello" || stderr != "warn" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func multiplexDockerLogs(stdout, stderr string) []byte {
	writeFrame := func(stream byte, payload string) []byte {
		size := make([]byte, 4)
		binary.BigEndian.PutUint32(size, uint32(len(payload)))
		return append(append([]byte{stream, 0, 0, 0}, size...), payload...)
	}
	var buf []byte
	buf = append(buf, writeFrame(1, stdout)...)
	buf = append(buf, writeFrame(2, stderr)...)
	return buf
}

func buildBytesTar(name string, data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), ModTime: time.Now()}
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

func TestDecodeDockerErrorIncludesMessage(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(`{"message":"bad config"}`)),
	}
	err := decodeDockerError(resp)
	if err == nil || !strings.Contains(err.Error(), "bad config") {
		t.Fatalf("err=%v", err)
	}
}
