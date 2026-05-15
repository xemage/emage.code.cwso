package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/mcp"
)

type fakeTool struct {
	name        string
	description string
	schema      map[string]any
	roles       []Role
	result      *mcp.ToolCallResult
	err         error
}

func (t *fakeTool) Name() string { return t.name }

func (t *fakeTool) Description() string { return t.description }

func (t *fakeTool) InputSchema() map[string]any { return t.schema }

func (t *fakeTool) AllowedRoles() []Role { return t.roles }

func (t *fakeTool) Execute(_ context.Context, _ json.RawMessage) (*mcp.ToolCallResult, error) {
	return t.result, t.err
}

func TestRegistryRegisterGetListAndDuplicate(t *testing.T) {
	r := NewRegistry()
	toolA := &fakeTool{name: "alpha", description: "first", schema: map[string]any{"type": "object"}, roles: []Role{RoleOrchestrator}}
	toolB := &fakeTool{name: "beta", description: "second", schema: map[string]any{"type": "object"}, roles: []Role{RoleWorker}}

	if err := r.Register(toolA); err != nil {
		t.Fatalf("register alpha: %v", err)
	}
	if err := r.Register(toolB); err != nil {
		t.Fatalf("register beta: %v", err)
	}
	if err := r.Register(toolA); err == nil {
		t.Fatal("expected duplicate registration error")
	}

	if got := r.Get("alpha"); got == nil || got.Name() != "alpha" {
		t.Fatalf("unexpected get result: %#v", got)
	}
	if got := r.Get("missing"); got != nil {
		t.Fatalf("expected nil for missing tool, got %#v", got)
	}

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 tools in list, got %d", len(list))
	}
	seen := map[string]bool{}
	for _, item := range list {
		seen[item.Name] = true
	}
	if !seen["alpha"] || !seen["beta"] {
		t.Fatalf("expected both tools in list, got %+v", list)
	}
}

func TestRegistryAuthorized(t *testing.T) {
	r := NewRegistry()
	tool := &fakeTool{name: "read", roles: []Role{RoleWorker}}
	if err := r.Register(tool); err != nil {
		t.Fatalf("register: %v", err)
	}

	got, ok := r.Authorized("read", RoleWorker)
	if !ok || got == nil {
		t.Fatalf("expected worker to be authorized, got ok=%v tool=%#v", ok, got)
	}

	got, ok = r.Authorized("read", RoleOrchestrator)
	if ok {
		t.Fatal("expected orchestrator to be denied")
	}
	if got == nil || got.Name() != "read" {
		t.Fatalf("expected denied lookup to still return tool, got %#v", got)
	}

	got, ok = r.Authorized("missing", RoleWorker)
	if ok || got != nil {
		t.Fatalf("expected missing tool to return (nil,false), got (%#v,%v)", got, ok)
	}
}
