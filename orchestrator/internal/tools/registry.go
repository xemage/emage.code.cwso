// Package tools defines the Tool interface and registry used by the MCP router.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/emage/cwso/orchestrator/internal/mcp"
)

// Role enumerates permission tiers per architecture-v1.md §4.
type Role string

const (
	RoleOrchestrator Role = "orchestrator"
	RoleWorker       Role = "worker"
)

// Tool is a single MCP tool implementation.
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	// AllowedRoles defines which permission tiers may invoke this tool.
	AllowedRoles() []Role
	// Execute runs the tool with raw JSON args and returns a tool result.
	Execute(ctx context.Context, args json.RawMessage) (*mcp.ToolCallResult, error)
}

// Registry holds registered tools and enforces permission tiers.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{tools: make(map[string]Tool)} }

// Register adds a tool. Duplicate names return an error.
func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name()]; exists {
		return fmt.Errorf("tool %q already registered", t.Name())
	}
	r.tools[t.Name()] = t
	return nil
}

// List returns all registered tools sorted by name (deterministic for testing).
func (r *Registry) List() []mcp.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]mcp.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, mcp.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return out
}

// Get returns a tool by name, or nil.
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Authorized reports whether the given role may invoke the named tool.
// Returns (tool, true) on success; (nil, false) if not found or denied.
func (r *Registry) Authorized(name string, role Role) (Tool, bool) {
	t := r.Get(name)
	if t == nil {
		return nil, false
	}
	for _, allowed := range t.AllowedRoles() {
		if allowed == role {
			return t, true
		}
	}
	return t, false
}
