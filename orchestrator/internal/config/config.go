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
	Transport                 string   // "stdio" | "http"
	HTTPAddr                  string   // ":8080"
	LogLevel                  string   // "debug" | "info" | "warn" | "error"
	JWTSecret                 string   // HS256 signing key
	JWTAlg                    string   // "HS256" (only supported algorithm in current build)
	JWTIssuer                 string   // "iss" claim validation
	JWTAudience               string   // "aud" claim validation
	JWKSPath                  string   // path to RS256 public key file (prod)
	AllowedOrigins            []string // exact origins permitted on HTTP
	JobTimeoutSeconds         int      // default async job timeout (Phase 3)
	JobWorkers                int      // async job worker pool size
	JobQueueSize              int      // async job queue capacity
	Workspace                 string   // host path that the orchestrator may serve via baseline FS tools
	ShadowSocket              string   // UDS path for the cwso-git-shadow sidecar ("" disables shadow tools)
	MergeEngineSocket         string   // UDS path for the cwso-merge-engine sidecar ("" disables merge tool)
	SandboxRunner             string   // "none" | "docker" | "gvisor"
	SandboxDockerHost         string   // docker host URL (unix:///var/run/docker.sock by default)
	SandboxImage              string   // default image used by baseline docker runner
	SandboxRuntime            string   // docker runtime selector ("" for default, "runsc" for gVisor)
	SandboxNetwork            string   // default container network mode, must not be host
	SandboxCPUQuota           int64    // CPU quota in microseconds per 100ms period
	SandboxMemory             int64    // memory limit in bytes
	SandboxPIDs               int64    // pids limit
	SandboxStopSecs           int      // timeout for graceful stop before force-kill
	SandboxFCBin              string   // firecracker binary name/path
	SandboxFCHelper           string   // firecracker execution helper binary path
	SandboxKVMDevice          string   // kvm device path
	SandboxVhostNet           string   // vhost-net device path
	SandboxSnapshot           string   // firecracker template snapshot directory
	SandboxVMState            string   // firecracker clone/vm state directory
	SandboxRequireVh          bool     // require vhost-net device for firecracker execution
	SandboxDegradedMode       bool     // true when Firecracker is unavailable (KVM absent); routes FC workloads to gVisor
	SandboxAllowDockerTrusted bool     // permit docker-trusted tier in router mode (internal orchestrator use only)
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
		Transport:                 envOr("CWSO_TRANSPORT", "stdio"),
		HTTPAddr:                  envOr("CWSO_HTTP_ADDR", ":8080"),
		LogLevel:                  envOr("CWSO_LOG_LEVEL", "info"),
		JWTSecret:                 jwtSecret,
		JWTAlg:                    envOr("CWSO_JWT_ALG", "HS256"),
		JWTIssuer:                 envOr("CWSO_JWT_ISSUER", "cwso"),
		JWTAudience:               envOr("CWSO_JWT_AUDIENCE", "cwso-mcp"),
		JWKSPath:                  os.Getenv("CWSO_JWKS_PATH"),
		AllowedOrigins:            splitCSV(envOr("CWSO_ALLOWED_ORIGINS", "http://localhost,http://127.0.0.1")),
		JobTimeoutSeconds:         envInt("CWSO_JOB_TIMEOUT_SECONDS", 300),
		JobWorkers:                envInt("CWSO_JOB_WORKERS", 4),
		JobQueueSize:              envInt("CWSO_JOB_QUEUE_SIZE", 64),
		Workspace:                 envOr("CWSO_WORKSPACE", "/workspace"),
		ShadowSocket:              os.Getenv("CWSO_GIT_SHADOW_SOCKET"),
		MergeEngineSocket:         os.Getenv("CWSO_MERGE_ENGINE_SOCKET"),
		SandboxRunner:             envOr("CWSO_SANDBOX_RUNNER", "none"),
		SandboxDockerHost:         envOr("CWSO_DOCKER_HOST", "unix:///var/run/docker.sock"),
		SandboxImage:              envOr("CWSO_DOCKER_IMAGE", "alpine:3.20"),
		SandboxRuntime:            os.Getenv("CWSO_DOCKER_RUNTIME"),
		SandboxNetwork:            envOr("CWSO_DOCKER_NETWORK", "none"),
		SandboxCPUQuota:           envInt64("CWSO_DOCKER_CPU_QUOTA_MICROS", 100000),
		SandboxMemory:             envInt64("CWSO_DOCKER_MEMORY_BYTES", 268435456),
		SandboxPIDs:               envInt64("CWSO_DOCKER_PIDS_LIMIT", 128),
		SandboxStopSecs:           envInt("CWSO_DOCKER_STOP_TIMEOUT_SECONDS", 5),
		SandboxFCBin:              envOr("CWSO_FIRECRACKER_BIN", "firecracker"),
		SandboxFCHelper:           os.Getenv("CWSO_FIRECRACKER_EXEC_HELPER"),
		SandboxKVMDevice:          envOr("CWSO_FIRECRACKER_KVM_DEVICE", "/dev/kvm"),
		SandboxVhostNet:           envOr("CWSO_FIRECRACKER_VHOST_DEVICE", "/dev/vhost-net"),
		SandboxSnapshot:           envOr("CWSO_FIRECRACKER_SNAPSHOT_DIR", "/tmp/cwso-firecracker/templates"),
		SandboxVMState:            envOr("CWSO_FIRECRACKER_VMSTATE_DIR", "/tmp/cwso-firecracker/vms"),
		SandboxRequireVh:          envBool("CWSO_FIRECRACKER_REQUIRE_VHOST_NET", true),
		SandboxDegradedMode:       envBool("CWSO_SANDBOX_DEGRADED_MODE", false),
		SandboxAllowDockerTrusted: envBool("CWSO_SANDBOX_ALLOW_DOCKER_TRUSTED", false),
	}

	if c.Transport == "http" && c.JWTSecret == "" {
		// Immutable constraint: no bypass of JWT auth, even in dev.
		return nil, fmt.Errorf("JWT secret must be set via /run/secrets/jwt_secret or CWSO_JWT_SECRET when transport=http")
	}
	if c.Transport != "stdio" && c.Transport != "http" {
		return nil, fmt.Errorf("invalid transport %q", c.Transport)
	}
	if c.JWTAlg != "HS256" {
		return nil, fmt.Errorf("invalid JWT algorithm %q (must be HS256 in current build)", c.JWTAlg)
	}
	if c.JobWorkers <= 0 {
		return nil, fmt.Errorf("CWSO_JOB_WORKERS must be > 0")
	}
	if c.JobQueueSize <= 0 {
		return nil, fmt.Errorf("CWSO_JOB_QUEUE_SIZE must be > 0")
	}
	if c.SandboxRunner != "none" && c.SandboxRunner != "docker" && c.SandboxRunner != "gvisor" && c.SandboxRunner != "firecracker" && c.SandboxRunner != "router" {
		return nil, fmt.Errorf("CWSO_SANDBOX_RUNNER must be one of: none, docker, gvisor, firecracker, router")
	}
	if c.SandboxRunner == "docker" || c.SandboxRunner == "gvisor" || c.SandboxRunner == "firecracker" {
		if strings.TrimSpace(c.SandboxImage) == "" {
			return nil, fmt.Errorf("CWSO_DOCKER_IMAGE must be set when CWSO_SANDBOX_RUNNER=%s", c.SandboxRunner)
		}
		if strings.EqualFold(c.SandboxNetwork, "host") {
			return nil, fmt.Errorf("CWSO_DOCKER_NETWORK=host is forbidden")
		}
		if c.SandboxCPUQuota <= 0 {
			return nil, fmt.Errorf("CWSO_DOCKER_CPU_QUOTA_MICROS must be > 0")
		}
		if c.SandboxMemory <= 0 {
			return nil, fmt.Errorf("CWSO_DOCKER_MEMORY_BYTES must be > 0")
		}
		if c.SandboxPIDs <= 0 {
			return nil, fmt.Errorf("CWSO_DOCKER_PIDS_LIMIT must be > 0")
		}
		if c.SandboxStopSecs <= 0 {
			return nil, fmt.Errorf("CWSO_DOCKER_STOP_TIMEOUT_SECONDS must be > 0")
		}
		if (c.SandboxRunner == "gvisor" || c.SandboxRunner == "router") && strings.TrimSpace(c.SandboxRuntime) == "" {
			c.SandboxRuntime = "runsc"
		}
		if c.SandboxRunner == "firecracker" {
			if strings.TrimSpace(c.SandboxFCHelper) == "" {
				return nil, fmt.Errorf("CWSO_FIRECRACKER_EXEC_HELPER must be set when CWSO_SANDBOX_RUNNER=firecracker")
			}
			if strings.TrimSpace(c.SandboxFCBin) == "" {
				return nil, fmt.Errorf("CWSO_FIRECRACKER_BIN must not be empty when CWSO_SANDBOX_RUNNER=firecracker")
			}
			if strings.TrimSpace(c.SandboxKVMDevice) == "" {
				return nil, fmt.Errorf("CWSO_FIRECRACKER_KVM_DEVICE must not be empty when CWSO_SANDBOX_RUNNER=firecracker")
			}
			if strings.TrimSpace(c.SandboxSnapshot) == "" || strings.TrimSpace(c.SandboxVMState) == "" {
				return nil, fmt.Errorf("CWSO_FIRECRACKER_SNAPSHOT_DIR and CWSO_FIRECRACKER_VMSTATE_DIR must be set when CWSO_SANDBOX_RUNNER=firecracker")
			}
		}
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

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
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
