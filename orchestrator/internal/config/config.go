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
	JWTSecret         string   // HS256 dev key; RS256 key loaded from JWKS path (prod)
	JWTAlg            string   // "HS256" (dev) | "RS256" (prod)
	JWTIssuer         string   // "iss" claim validation
	JWTAudience       string   // "aud" claim validation
	JWKSPath          string   // path to RS256 public key file (prod)
	AllowedOrigins    []string // exact origins permitted on HTTP
	JobTimeoutSeconds int      // default async job timeout (Phase 3)
	Workspace         string   // host path that the orchestrator may serve via baseline FS tools
	ShadowSocket      string   // UDS path for the cwso-git-shadow sidecar ("" disables shadow tools)
}

// Load reads configuration, applying env-var overrides over defaults.
// configPath is reserved for future YAML loading; ignored for now.
func Load(_ string) (*Config, error) {
	// JWT secret precedence: mounted secret file > env var
	jwtSecret := ""
	// Try to read from mounted docker-compose secret (T029 remediation)
	if data, err := os.ReadFile("/run/secrets/jwt_secret"); err == nil {
		jwtSecret = strings.TrimSpace(string(data))
	}
	if jwtSecret == "" {
		jwtSecret = os.Getenv("CWSO_JWT_SECRET")
	}

	c := &Config{
		Transport:         envOr("CWSO_TRANSPORT", "stdio"),
		HTTPAddr:          envOr("CWSO_HTTP_ADDR", ":8080"),
		LogLevel:          envOr("CWSO_LOG_LEVEL", "info"),
		JWTSecret:         jwtSecret,
		JWTAlg:            envOr("CWSO_JWT_ALG", "HS256"), // HS256 for dev, RS256 for prod
		JWTIssuer:         envOr("CWSO_JWT_ISSUER", "cwso"),
		JWTAudience:       envOr("CWSO_JWT_AUDIENCE", "cwso-mcp"),
		JWKSPath:          os.Getenv("CWSO_JWKS_PATH"),
		AllowedOrigins:    splitCSV(envOr("CWSO_ALLOWED_ORIGINS", "http://localhost,http://127.0.0.1")),
		JobTimeoutSeconds: envInt("CWSO_JOB_TIMEOUT_SECONDS", 300),
		Workspace:         envOr("CWSO_WORKSPACE", "/workspace"),
		ShadowSocket:      os.Getenv("CWSO_GIT_SHADOW_SOCKET"),
	}

	if c.Transport == "http" && c.JWTSecret == "" {
		// Immutable constraint: no bypass of JWT auth, even in dev.
		return nil, fmt.Errorf("JWT secret must be set via /run/secrets/jwt_secret or CWSO_JWT_SECRET when transport=http")
	}
	if c.Transport != "stdio" && c.Transport != "http" {
		return nil, fmt.Errorf("invalid transport %q", c.Transport)
	}
	if c.JWTAlg != "HS256" && c.JWTAlg != "RS256" {
		return nil, fmt.Errorf("invalid JWT algorithm %q (must be HS256 or RS256)", c.JWTAlg)
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
