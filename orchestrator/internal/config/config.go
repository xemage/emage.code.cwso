// Package config holds runtime configuration loaded from env vars and optional YAML.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration. Secrets MUST come from env vars or
// mounted files; never from source-controlled defaults.
type Config struct {
	Transport         string   // "stdio" | "http"
	HTTPAddr          string   // ":8080"
	LogLevel          string   // "debug" | "info" | "warn" | "error"
	JWTSecret         string   // HS256 dev key; production should use RS256 + mounted key file
	AllowedOrigins    []string // exact origins permitted on HTTP
	JobTimeoutSeconds int      // default async job timeout (Phase 3)
	Workspace         string   // host path that the orchestrator may serve via baseline FS tools
	ShadowSocket      string   // UDS path for the cwso-git-shadow sidecar ("" disables shadow tools)
}

// Load reads configuration, applying env-var overrides over defaults.
// configPath is reserved for future YAML loading; ignored for now.
func Load(_ string) (*Config, error) {
	c := &Config{
		Transport:         envOr("CWSO_TRANSPORT", "stdio"),
		HTTPAddr:          envOr("CWSO_HTTP_ADDR", ":8080"),
		LogLevel:          envOr("CWSO_LOG_LEVEL", "info"),
		JWTSecret:         os.Getenv("CWSO_JWT_SECRET"),
		AllowedOrigins:    splitCSV(envOr("CWSO_ALLOWED_ORIGINS", "http://localhost,http://127.0.0.1")),
		JobTimeoutSeconds: envInt("CWSO_JOB_TIMEOUT_SECONDS", 300),
		Workspace:         envOr("CWSO_WORKSPACE", "/workspace"),
		ShadowSocket:      os.Getenv("CWSO_GIT_SHADOW_SOCKET"),
	}

	if c.Transport == "http" && c.JWTSecret == "" {
		// Immutable constraint: no bypass of JWT auth, even in dev.
		return nil, fmt.Errorf("CWSO_JWT_SECRET must be set when transport=http")
	}
	if c.Transport != "stdio" && c.Transport != "http" {
		return nil, fmt.Errorf("invalid transport %q", c.Transport)
	}
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
