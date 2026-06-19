package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/tools"
	"github.com/emage/cwso/orchestrator/internal/transport"
)

type errTool struct{}

func (t *errTool) Name() string { return "err_tool" }

func (t *errTool) Description() string { return "always errors" }

func (t *errTool) InputSchema() map[string]any { return map[string]any{"type": "object"} }

func (t *errTool) AllowedRoles() []tools.Role {
	return []tools.Role{tools.RoleOrchestrator, tools.RoleWorker}
}

func (t *errTool) Execute(_ context.Context, _ json.RawMessage) (*mcp.ToolCallResult, error) {
	return nil, errors.New("boom")
}

type nilTool struct{}

func (t *nilTool) Name() string { return "nil_tool" }

func (t *nilTool) Description() string { return "returns nil result" }

func (t *nilTool) InputSchema() map[string]any { return map[string]any{"type": "object"} }

func (t *nilTool) AllowedRoles() []tools.Role { return []tools.Role{tools.RoleOrchestrator} }

func (t *nilTool) Execute(_ context.Context, _ json.RawMessage) (*mcp.ToolCallResult, error) {
	return nil, nil
}

func TestHandleParseErrorAndToolBranches(t *testing.T) {
	s, _ := newTestServer(t)

	out, err := s.Handle(context.Background(), nil, []byte(`{`))
	if err != nil {
		t.Fatalf("parse error request should still marshal response, got err=%v", err)
	}
	if !strings.Contains(string(out), "parse error") {
		t.Fatalf("expected parse error response, got %s", string(out))
	}

	unknownToolReq := []byte(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"does_not_exist","arguments":{}}}`)
	out, err = s.Handle(context.Background(), nil, unknownToolReq)
	if err != nil {
		t.Fatalf("unknown tool request: %v", err)
	}
	if !strings.Contains(string(out), "tool not found") {
		t.Fatalf("expected tool-not-found response, got %s", string(out))
	}

	if err := s.Registry().Register(&errTool{}); err != nil {
		t.Fatalf("register err tool: %v", err)
	}
	errToolReq := []byte(`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"err_tool","arguments":{}}}`)
	out, err = s.Handle(context.Background(), &transport.Session{Role: "worker"}, errToolReq)
	if err != nil {
		t.Fatalf("err tool request: %v", err)
	}
	if !strings.Contains(string(out), "boom") {
		t.Fatalf("expected tool execution error response, got %s", string(out))
	}

	if err := s.Registry().Register(&nilTool{}); err != nil {
		t.Fatalf("register nil tool: %v", err)
	}
	nilToolReq := []byte(`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"nil_tool","arguments":{}}}`)
	out, err = s.Handle(context.Background(), &transport.Session{Role: "orchestrator"}, nilToolReq)
	if err != nil {
		t.Fatalf("nil tool request: %v", err)
	}
	if !strings.Contains(string(out), "tool returned nil") {
		t.Fatalf("expected nil-result internal error, got %s", string(out))
	}
}

func TestRunUnsupportedTransportAndGetters(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.Transport = "unsupported"

	if got := s.Registry(); got == nil {
		t.Fatal("registry getter returned nil")
	}
	if got := s.Jobs(); got == nil {
		t.Fatal("jobs getter returned nil")
	}
	if got := s.Memory(); got == nil {
		t.Fatal("memory getter returned nil")
	}

	err := s.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unsupported transport") {
		t.Fatalf("expected unsupported transport error, got %v", err)
	}
}
