package rollout

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// EvaluatorID identifies a registered post-run evaluator plugin (Polar §3.5, T148).
type EvaluatorID string

const (
	EvaluatorSessionReward EvaluatorID = "session-reward"
	EvaluatorSWEBench      EvaluatorID = "swe-bench"
)

// EvalRequest carries a completed trajectory into evaluator plugins.
type EvalRequest struct {
	TaskID    string
	SessionID string
	Spec      TaskSpec
	Group     *TrajectoryGroup
	TimedOut  bool
}

// EvalResult is one evaluator's score for a session.
type EvalResult struct {
	EvaluatorID EvaluatorID       `json:"evaluator_id"`
	Reward      float64           `json:"reward"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Plugin scores a trajectory after construction (registry-backed evaluators, T148).
type Plugin interface {
	ID() EvaluatorID
	Enabled() bool
	Evaluate(ctx context.Context, req EvalRequest) (EvalResult, error)
}

// Registry holds pluggable post-run evaluators.
type Registry struct {
	mu      sync.RWMutex
	enabled bool
	plugins map[EvaluatorID]Plugin
}

// RegistryConfig wires built-in evaluators from orchestrator config (T148).
type RegistryConfig struct {
	Enabled              bool
	SessionRewardEnabled bool
	SWEBenchEnabled      bool
	SWEBenchInstance     string
	Rewards              RewardReader
	Client               *Client
}

// NewRegistry constructs an evaluator registry. Disabled registries are no-ops.
func NewRegistry(cfg RegistryConfig) *Registry {
	registry := &Registry{
		enabled: cfg.Enabled,
		plugins: make(map[EvaluatorID]Plugin),
	}
	if !cfg.Enabled {
		return registry
	}
	registry.Register(NewSessionRewardPlugin(cfg.SessionRewardEnabled, cfg.Rewards))
	registry.Register(NewSWEBenchPlugin(SWEBenchConfig{
		Enabled:    cfg.SWEBenchEnabled,
		InstanceID: cfg.SWEBenchInstance,
		Client:     cfg.Client,
	}))
	return registry
}

// Enabled reports whether the registry runs evaluators.
func (r *Registry) Enabled() bool {
	return r != nil && r.enabled
}

// Register adds a plugin. Duplicate IDs return an error.
func (r *Registry) Register(plugin Plugin) error {
	if r == nil {
		return fmt.Errorf("evaluator registry is nil")
	}
	if plugin == nil {
		return fmt.Errorf("evaluator plugin is nil")
	}
	id := plugin.ID()
	if id == "" {
		return fmt.Errorf("evaluator id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[id]; exists {
		return fmt.Errorf("evaluator %q already registered", id)
	}
	r.plugins[id] = plugin
	return nil
}

// List returns registered plugin IDs sorted for deterministic tests.
func (r *Registry) List() []EvaluatorID {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]EvaluatorID, 0, len(r.plugins))
	for id := range r.plugins {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Evaluate runs enabled plugins and returns their results.
func (r *Registry) Evaluate(ctx context.Context, req EvalRequest) ([]EvalResult, error) {
	if !r.Enabled() {
		return nil, nil
	}
	r.mu.RLock()
	plugins := make([]Plugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		if plugin.Enabled() {
			plugins = append(plugins, plugin)
		}
	}
	r.mu.RUnlock()

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].ID() < plugins[j].ID()
	})

	out := make([]EvalResult, 0, len(plugins))
	for _, plugin := range plugins {
		result, err := plugin.Evaluate(ctx, req)
		if err != nil {
			return out, fmt.Errorf("evaluator %q: %w", plugin.ID(), err)
		}
		if result.EvaluatorID == "" {
			result.EvaluatorID = plugin.ID()
		}
		out = append(out, result)
	}
	return out, nil
}

// ApplyEvaluations runs plugins and attaches rewards/metadata to the trajectory group.
func (r *Registry) ApplyEvaluations(ctx context.Context, req EvalRequest) (*TrajectoryGroup, error) {
	group := req.Group
	if group == nil {
		return nil, nil
	}
	results, err := r.Evaluate(ctx, req)
	if err != nil {
		return group, err
	}
	AttachEvaluations(group, results)
	return group, nil
}

// AttachEvaluations propagates evaluator scores onto a trajectory group (Polar §3.5).
func AttachEvaluations(group *TrajectoryGroup, results []EvalResult) {
	if group == nil || len(results) == 0 {
		return
	}
	if group.Metadata == nil {
		group.Metadata = make(map[string]string, len(results)*2)
	}
	for _, result := range results {
		group.Rewards = append(group.Rewards, result.Reward)
		prefix := "evaluator." + string(result.EvaluatorID)
		group.Metadata[prefix+".reward"] = fmt.Sprintf("%g", result.Reward)
		for key, value := range result.Metadata {
			group.Metadata[prefix+"."+key] = value
		}
	}
}
