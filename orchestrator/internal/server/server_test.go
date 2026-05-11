package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/transport"
)

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Transport:      "stdio",
		LogLevel:       "error",
		Workspace:      dir,
		AllowedOrigins: []string{"http://localhost"},
	}
	s, err := New(cfg, logging.New("error"))
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func TestInitialize(t *testing.T) {
	s, _ := newTestServer(t)
	raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"test","version":"0"}}}`)
	out, err := s.Handle(context.Background(), nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"protocolVersion":"2025-03-26"`) {
		t.Fatalf("expected protocolVersion in response, got: %s", out)
	}
}

func TestToolsList(t *testing.T) {
	s, _ := newTestServer(t)
	raw := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	out, err := s.Handle(context.Background(), nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"read_file_sync", "write_file_sync", "list_dir"} {
		if !strings.Contains(string(out), `"`+name+`"`) {
			t.Fatalf("expected tool %q in list, got: %s", name, out)
		}
	}
}

func TestReadFileSyncSuccess(t *testing.T) {
	s, _ := newTestServer(t)
	raw := []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file_sync","arguments":{"path":"hello.txt"}}}`)
	out, err := s.Handle(context.Background(), nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"text":"hi"`) {
		t.Fatalf("unexpected response: %s", out)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	s, _ := newTestServer(t)
	raw := []byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_file_sync","arguments":{"path":"../../etc/passwd"}}}`)
	out, err := s.Handle(context.Background(), nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "escapes workspace root") {
		t.Fatalf("expected path-guard rejection, got: %s", out)
	}
}

func TestWritePermissionDeniedForOrchestrator(t *testing.T) {
	s, _ := newTestServer(t)
	sess := &transport.Session{Role: "orchestrator"}
	raw := []byte(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"write_file_sync","arguments":{"path":"x.txt","content":"oops"}}}`)
	out, err := s.Handle(context.Background(), sess, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "permission") && !strings.Contains(string(out), "may not invoke") {
		t.Fatalf("expected permission denial, got: %s", out)
	}
}

func TestWriteAllowedForWorker(t *testing.T) {
	s, dir := newTestServer(t)
	sess := &transport.Session{Role: "worker"}
	raw := []byte(`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"write_file_sync","arguments":{"path":"out.txt","content":"new"}}}`)
	out, err := s.Handle(context.Background(), sess, raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `"error"`) {
		t.Fatalf("worker write should succeed, got: %s", out)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "out.txt"))
	if string(got) != "new" {
		t.Fatalf("file content mismatch: %q", got)
	}
}

func TestUnknownMethod(t *testing.T) {
	s, _ := newTestServer(t)
	raw := []byte(`{"jsonrpc":"2.0","id":7,"method":"does/not/exist"}`)
	out, err := s.Handle(context.Background(), nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatal(err)
	}
	if env["error"] == nil {
		t.Fatalf("expected error envelope, got: %s", out)
	}
}

func TestNotificationNoResponse(t *testing.T) {
	s, _ := newTestServer(t)
	raw := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	out, err := s.Handle(context.Background(), nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Fatalf("notification should produce no response, got: %s", out)
	}
}
