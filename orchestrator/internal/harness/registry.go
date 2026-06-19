// Package harness launches Polar-style coding harnesses against cwso-rollout (T144).
package harness

import (
	"errors"
	"fmt"
	"strings"
)

// ID identifies a supported external coding harness adapter.
type ID string

const (
	IDShellCommand ID = "shell-command"
	IDCodex        ID = "codex"
	IDClaudeCode   ID = "claude_code"
	IDQwenCode     ID = "qwen_code"
)

// AdapterConfig describes how to launch a harness unchanged except for model base_url.
type AdapterConfig struct {
	ID          ID
	DisplayName string
	Image       string
	Command     []string
	BaseURLEnv  map[string]string
	ExtraEnv    map[string]string
}

// Registry holds harness adapter definitions.
type Registry struct {
	adapters map[ID]AdapterConfig
}

// DefaultRegistry returns built-in Polar harness adapters (shell-command is the reference PoC).
func DefaultRegistry() *Registry {
	registry := &Registry{adapters: make(map[ID]AdapterConfig)}
	for _, cfg := range []AdapterConfig{
		{
			ID:          IDShellCommand,
			DisplayName: "Shell command (reference)",
			Image:       "alpine:3.20",
			Command: []string{
				"/bin/sh", "-lc",
				`apk add --no-cache curl >/dev/null 2>&1; ` +
					`curl -sS "${OPENAI_BASE_URL}/v1/chat/completions" ` +
					`-H "Content-Type: application/json" ` +
					`-H "Authorization: Bearer ${OPENAI_API_KEY:-cwso-harness}" ` +
					`-d "{\"model\":\"gpt-4\",\"messages\":[{\"role\":\"user\",\"content\":\"${CWSO_HARNESS_PROMPT:-ping}\"}],\"stream\":false}"`,
			},
			BaseURLEnv: map[string]string{"openai": "OPENAI_BASE_URL"},
			ExtraEnv:   map[string]string{"OPENAI_API_KEY": "cwso-harness"},
		},
		{
			ID:          IDCodex,
			DisplayName: "OpenAI Codex CLI",
			Image:       "alpine:3.20",
			Command:     []string{"/bin/sh", "-lc", `echo "codex harness stub; set OPENAI_BASE_URL=${OPENAI_BASE_URL}"`},
			BaseURLEnv:  map[string]string{"openai": "OPENAI_BASE_URL"},
		},
		{
			ID:          IDClaudeCode,
			DisplayName: "Claude Code CLI",
			Image:       "alpine:3.20",
			Command:     []string{"/bin/sh", "-lc", `echo "claude_code harness stub; set ANTHROPIC_BASE_URL=${ANTHROPIC_BASE_URL}"`},
			BaseURLEnv:  map[string]string{"anthropic": "ANTHROPIC_BASE_URL"},
		},
		{
			ID:          IDQwenCode,
			DisplayName: "Qwen Code CLI",
			Image:       "alpine:3.20",
			Command:     []string{"/bin/sh", "-lc", `echo "qwen_code harness stub; set OPENAI_BASE_URL=${OPENAI_BASE_URL}"`},
			BaseURLEnv:  map[string]string{"openai": "OPENAI_BASE_URL"},
		},
	} {
		registry.adapters[cfg.ID] = cfg
	}
	return registry
}

// Get returns the adapter config for id.
func (r *Registry) Get(id ID) (AdapterConfig, error) {
	if r == nil {
		return AdapterConfig{}, errors.New("harness registry is nil")
	}
	cfg, ok := r.adapters[id]
	if !ok {
		return AdapterConfig{}, fmt.Errorf("unknown harness adapter %q", id)
	}
	return cfg, nil
}

// List returns all registered adapters sorted by ID.
func (r *Registry) List() []AdapterConfig {
	if r == nil {
		return nil
	}
	out := make([]AdapterConfig, 0, len(r.adapters))
	for _, cfg := range r.adapters {
		out = append(out, cfg)
	}
	sortAdapters(out)
	return out
}

// LaunchEnv builds harness environment with proxy base_url injected.
func (c AdapterConfig) LaunchEnv(proxyURL string, extra map[string]string) map[string]string {
	proxyURL = strings.TrimRight(strings.TrimSpace(proxyURL), "/")
	env := make(map[string]string, len(c.ExtraEnv)+len(c.BaseURLEnv)+len(extra)+1)
	for key, value := range c.ExtraEnv {
		env[key] = value
	}
	for _, key := range c.BaseURLEnv {
		env[key] = proxyURL
	}
	for key, value := range extra {
		env[key] = value
	}
	return env
}

func sortAdapters(items []AdapterConfig) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if string(items[j].ID) < string(items[i].ID) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
