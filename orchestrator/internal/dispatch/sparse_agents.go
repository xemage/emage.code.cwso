package dispatch

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	TopicAgentTelemetry      = "agents/telemetry"
	AgentResourceScheme      = "cwso"
	AgentTelemetrySuffix     = "/telemetry"
	AgentResourcePrefix      = "cwso://agents/"
	defaultAgentQuantization = "1.58-bit"
)

// AgentTelemetryEvent is published to the broker when a sparse agent is created or refreshed.
type AgentTelemetryEvent struct {
	WasmAgentID   string  `json:"wasm_agent_id"`
	SkillDomain   string  `json:"skill_domain"`
	SliceSHA256   string  `json:"slice_sha256,omitempty"`
	State         string  `json:"state"`
	ColdStartMS   float64 `json:"cold_start_ms"`
	ResidentRAMMB float64 `json:"resident_ram_mb"`
	TokensPerSec  float64 `json:"tokens_per_sec"`
	TargetASTNode string  `json:"target_ast_node,omitempty"`
	EmittedAt     string  `json:"emitted_at"`
}

// SparseAgentRecord tracks one orchestrator-visible sparse agent for MCP resources.
type SparseAgentRecord struct {
	ID            string    `json:"wasm_agent_id"`
	SkillDomain   string    `json:"skill_domain"`
	StreamURI     string    `json:"stream_resource"`
	CreatedAt     time.Time `json:"created_at"`
	TargetASTNode string    `json:"target_ast_node,omitempty"`
}

// TelemetryURI returns the cwso:// resource URI for this agent's telemetry stream.
func AgentTelemetryURI(agentID string) string {
	return AgentResourcePrefix + agentID + AgentTelemetrySuffix
}

// ParseAgentTelemetryResourceID extracts the agent id from cwso://agents/<id>/telemetry.
func ParseAgentTelemetryResourceID(uri string) (string, bool) {
	if !strings.HasPrefix(uri, AgentResourcePrefix) || !strings.HasSuffix(uri, AgentTelemetrySuffix) {
		return "", false
	}
	inner := strings.TrimPrefix(uri, AgentResourcePrefix)
	inner = strings.TrimSuffix(inner, AgentTelemetrySuffix)
	if inner == "" || strings.ContainsAny(inner, "/?#") {
		return "", false
	}
	return inner, true
}

// AgentTelemetryFilter scopes broker records to one agent's telemetry topic + id.
type AgentTelemetryFilter struct {
	AgentID string
}

// Allow implements transport.RecordFilter.
func (f *AgentTelemetryFilter) Allow(topic string, payload []byte) bool {
	if f == nil || topic != TopicAgentTelemetry {
		return false
	}
	var env struct {
		AgentID string `json:"wasm_agent_id"`
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &env)
	}
	return env.AgentID == f.AgentID
}

// SparseAgentRegistry is a concurrency-safe store of orchestrator-tracked sparse agents.
type SparseAgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]*SparseAgentRecord
	now    func() time.Time
}

// NewSparseAgentRegistry constructs an empty registry.
func NewSparseAgentRegistry() *SparseAgentRegistry {
	return &SparseAgentRegistry{
		agents: make(map[string]*SparseAgentRecord),
		now:    time.Now,
	}
}

// Register records a newly created sparse agent.
func (r *SparseAgentRegistry) Register(id, skillDomain, targetASTNode string) *SparseAgentRecord {
	if r == nil {
		return nil
	}
	rec := &SparseAgentRecord{
		ID:            id,
		SkillDomain:   skillDomain,
		StreamURI:     AgentTelemetryURI(id),
		CreatedAt:     r.now().UTC(),
		TargetASTNode: targetASTNode,
	}
	r.mu.Lock()
	r.agents[id] = rec
	r.mu.Unlock()
	return rec
}

// Get returns the record for an agent id.
func (r *SparseAgentRegistry) Get(id string) (*SparseAgentRecord, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.agents[id]
	return rec, ok
}

// List returns all agents ordered by creation time then id.
func (r *SparseAgentRegistry) List() []*SparseAgentRecord {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	out := make([]*SparseAgentRecord, 0, len(r.agents))
	for _, rec := range r.agents {
		out = append(out, rec)
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// NormalizeQuantization validates the requested quantization tag.
func NormalizeQuantization(q string) (string, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return defaultAgentQuantization, nil
	}
	switch q {
	case "1.58-bit", "int4", "int8":
		if q != defaultAgentQuantization {
			return "", fmt.Errorf("unsupported quantization %q (only 1.58-bit is implemented)", q)
		}
		return q, nil
	default:
		return "", fmt.Errorf("unsupported quantization %q", q)
	}
}
