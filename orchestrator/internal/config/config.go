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
	Transport                     string   // "stdio" | "http"
	HTTPAddr                      string   // ":8080"
	LogLevel                      string   // "debug" | "info" | "warn" | "error"
	JWTSecret                     string   // HS256 signing key
	JWTAlg                        string   // "HS256" (only supported algorithm in current build)
	JWTIssuer                     string   // "iss" claim validation
	JWTAudience                   string   // "aud" claim validation
	JWKSPath                      string   // path to RS256 public key file (prod)
	AllowedOrigins                []string // exact origins permitted on HTTP
	JobTimeoutSeconds             int      // default async job timeout (Phase 3)
	JobWorkers                    int      // async job worker pool size
	JobQueueSize                  int      // async job queue capacity
	Workspace                     string   // host path that the orchestrator may serve via baseline FS tools
	ShadowSocket                  string   // UDS path for the cwso-git-shadow sidecar ("" disables shadow tools)
	MergeEngineSocket             string   // UDS path for the cwso-merge-engine sidecar ("" disables merge tool)
	HALSocket                     string   // UDS path for the cwso-hal sidecar ("" keeps hardware-aware dispatch in shadow mode)
	HALCapabilitySyncSeconds      int      // interval for refreshing the capability registry from the live HAL
	SandboxRunner                 string   // "none" | "docker" | "gvisor"
	SandboxDockerHost             string   // docker host URL (unix:///var/run/docker.sock by default)
	SandboxImage                  string   // default image used by baseline docker runner
	SandboxRuntime                string   // docker runtime selector ("" for default, "runsc" for gVisor)
	SandboxNetwork                string   // default container network mode, must not be host
	SandboxCPUQuota               int64    // CPU quota in microseconds per 100ms period
	SandboxMemory                 int64    // memory limit in bytes
	SandboxPIDs                   int64    // pids limit
	SandboxStopSecs               int      // timeout for graceful stop before force-kill
	SandboxFCBin                  string   // firecracker binary name/path
	SandboxFCHelper               string   // firecracker execution helper binary path
	SandboxKVMDevice              string   // kvm device path
	SandboxVhostNet               string   // vhost-net device path
	SandboxSnapshot               string   // firecracker template snapshot directory
	SandboxVMState                string   // firecracker clone/vm state directory
	SandboxRequireVh              bool     // require vhost-net device for firecracker execution
	SandboxDegradedMode           bool     // true when Firecracker is unavailable (KVM absent); routes FC workloads to gVisor
	SandboxAllowDockerTrusted     bool     // permit docker-trusted tier in router mode (internal orchestrator use only)
	HHDCapabilityRegistry         bool     // enable hardware-dispatch capability registry and snapshot telemetry
	HHDDecisionTelemetry          bool     // enable dispatch decision telemetry emission
	HHDTelemetryRedaction         bool     // enable sensitive field minimization/redaction for dispatch telemetry
	HHDTelemetryRequestIDMode     string   // request-id handling mode: allow | hash | drop
	HHDTelemetryAnomalyNotes      string   // anomaly note handling mode: allow | drop
	HHDTelemetryRedactionSalt     string   // optional salt used when hashing request-id values
	HHDEventMonitorEnabled        bool     // enable event-driven anomaly monitor from dispatch decisions
	HHDEventMonitorEBPF           bool     // prefer eBPF signal path when capabilities permit (falls back automatically)
	HHDEventMonitorLatencyMS      int      // anomaly threshold for actual latency in milliseconds
	ASTSpikeResourcesEnabled      bool     // enable subscribe_ast_spikes tool + cwso://spikes MCP resources (Phase 7 / T117)
	ASTSpikeMonitorEnabled        bool     // enable the AST write-spike monitor+filter fed by write_shadow_file (Phase 7 / T118)
	ASTSpikePreferEBPF            bool     // prefer eBPF signal path for AST spike detection (falls back to userspace)
	ASTSpikeWindowMS              int      // sliding window for write-volume spike detection
	ASTSpikeThreshold             int      // write count within the window that constitutes a volume spike
	ASTSpikeDebounceMS            int      // suppress repeat volume spikes within this interval (0 = window)
	ASTSpikeMaxHotPaths           int      // max hot paths reported in a volume spike event
	ASTSpikeSemanticThreshold     string   // min semantic significance to emit: signature_change|symbol_added|symbol_removed|any
	ASTSpikeConflictWindowMS      int      // correlation window for cross-workspace semantic-conflict pre-warnings
	ASTSpikeSignatureTTLMS        int      // TTL for the per-symbol signature memory used by the semantic filter
	ASTSpikeMaxConflictPeers      int      // max peer workspaces listed in a conflict pre-warning
	SparseAgentsEnabled           bool     // enable create_ephemeral_sparse_agent + cwso://agents telemetry (Phase 7 / T122)
	SparseSocket                  string   // UDS path for the cwso-sparse sidecar ("" disables sparse agent tools)
	SparseHostRAMCapMB            int      // host-wide RAM cap enforced on max_ram_mb requests
	SparseQualityGuardrailEnabled bool     // enable quality-floor breach → dense GPU escalation (T123)
	RolloutRewardEnabled          bool     // enable programmatic merge SM rewards on rollout/reward topic (T136)
	RolloutAPIEnabled             bool     // enable /rollout/* Polar REST API (T137)
	RolloutSocket                 string   // UDS path for cwso-rollout sidecar (trajectory drain)
	HHDSnapshotTTLSeconds         int      // stale capability threshold for policy-facing snapshots
	HHDPolicyEngineV2             bool     // enable policy engine v2 backend selection and fallback
	HHDHardwareAwareDispatch      bool     // enable dispatch_hardware_aware_job tool + shadow provider catalog (Phase 6)
	HHDPolicyMinConfidence        float64  // minimum confidence before forcing baseline path
	HHDPolicyMaxQueueDepth        int      // queue depth normalization denominator for policy scoring
	HHDWeightHealth               float64  // policy weight for health signal
	HHDWeightReliability          float64  // policy weight for reliability signal
	HHDWeightCost                 float64  // policy weight for cost signal
	HHDWeightLatency              float64  // policy weight for latency signal
	HHDWeightQueueDepth           float64  // policy weight for queue-depth signal
	HHDWeightWorkload             float64  // policy weight for workload compatibility signal
	HHDSparseQuantizedEnabled     bool     // enable sparse/quantized assist scoring experiment
	HHDSparseQuantizedTradeoff    float64  // cost-latency tradeoff modifier in [-1, 1]
	HHDQualityGuardrailMinScore   float64  // minimum quality score before auto-disable of sparse/quantized path
	HHDSSMAssistEnabled           bool     // enable SSM sequence-assist scoring experiment
	HHDSSMThroughputBias          float64  // throughput scoring bias in [-1, 1]
	HHDSSMMinSequenceLength       int      // minimum accepted sequence-length signal for SSM assist
	HHDSSMMaxSequenceLength       int      // maximum accepted sequence-length signal for SSM assist
	HHDSSMSequenceSensitivity     float64  // scaling factor in [0, 2] for sequence-length sensitivity
	HHDWasmScoringEnabled         bool     // enable wasm scoring adjustment plugin in policy engine v2
	HHDWasmScoringModulePath      string   // filesystem path to wasm scoring module
	HHDWasmScoringModuleSHA256    string   // required sha256 hex digest for wasm module integrity verification
	HHDWasmScoringTrustedDir      string   // required trusted directory containing wasm module path
	HHDWasmScoringTimeoutMS       int      // per-call timeout budget for wasm score adjustments
	HHDWasmScoringMemoryPages     uint32   // max wasm memory pages for scoring module runtime
	HHDWasmScoringHostCalls       []string // deny-by-default host-call allowlist for wasm runtime
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
		Transport:                     envOr("CWSO_TRANSPORT", "stdio"),
		HTTPAddr:                      envOr("CWSO_HTTP_ADDR", ":8080"),
		LogLevel:                      envOr("CWSO_LOG_LEVEL", "info"),
		JWTSecret:                     jwtSecret,
		JWTAlg:                        envOr("CWSO_JWT_ALG", "HS256"),
		JWTIssuer:                     envOr("CWSO_JWT_ISSUER", "cwso"),
		JWTAudience:                   envOr("CWSO_JWT_AUDIENCE", "cwso-mcp"),
		JWKSPath:                      os.Getenv("CWSO_JWKS_PATH"),
		AllowedOrigins:                splitCSV(envOr("CWSO_ALLOWED_ORIGINS", "http://localhost,http://127.0.0.1")),
		JobTimeoutSeconds:             envInt("CWSO_JOB_TIMEOUT_SECONDS", 300),
		JobWorkers:                    envInt("CWSO_JOB_WORKERS", 4),
		JobQueueSize:                  envInt("CWSO_JOB_QUEUE_SIZE", 64),
		Workspace:                     envOr("CWSO_WORKSPACE", "/workspace"),
		ShadowSocket:                  os.Getenv("CWSO_GIT_SHADOW_SOCKET"),
		MergeEngineSocket:             os.Getenv("CWSO_MERGE_ENGINE_SOCKET"),
		HALSocket:                     os.Getenv("CWSO_HAL_SOCKET"),
		HALCapabilitySyncSeconds:      envInt("CWSO_HAL_CAPABILITY_SYNC_SECONDS", 15),
		SandboxRunner:                 envOr("CWSO_SANDBOX_RUNNER", "none"),
		SandboxDockerHost:             envOr("CWSO_DOCKER_HOST", "unix:///var/run/docker.sock"),
		SandboxImage:                  envOr("CWSO_DOCKER_IMAGE", "alpine:3.20"),
		SandboxRuntime:                os.Getenv("CWSO_DOCKER_RUNTIME"),
		SandboxNetwork:                envOr("CWSO_DOCKER_NETWORK", "none"),
		SandboxCPUQuota:               envInt64("CWSO_DOCKER_CPU_QUOTA_MICROS", 100000),
		SandboxMemory:                 envInt64("CWSO_DOCKER_MEMORY_BYTES", 268435456),
		SandboxPIDs:                   envInt64("CWSO_DOCKER_PIDS_LIMIT", 128),
		SandboxStopSecs:               envInt("CWSO_DOCKER_STOP_TIMEOUT_SECONDS", 5),
		SandboxFCBin:                  envOr("CWSO_FIRECRACKER_BIN", "firecracker"),
		SandboxFCHelper:               os.Getenv("CWSO_FIRECRACKER_EXEC_HELPER"),
		SandboxKVMDevice:              envOr("CWSO_FIRECRACKER_KVM_DEVICE", "/dev/kvm"),
		SandboxVhostNet:               envOr("CWSO_FIRECRACKER_VHOST_DEVICE", "/dev/vhost-net"),
		SandboxSnapshot:               envOr("CWSO_FIRECRACKER_SNAPSHOT_DIR", "/tmp/cwso-firecracker/templates"),
		SandboxVMState:                envOr("CWSO_FIRECRACKER_VMSTATE_DIR", "/tmp/cwso-firecracker/vms"),
		SandboxRequireVh:              envBool("CWSO_FIRECRACKER_REQUIRE_VHOST_NET", true),
		SandboxDegradedMode:           envBool("CWSO_SANDBOX_DEGRADED_MODE", false),
		SandboxAllowDockerTrusted:     envBool("CWSO_SANDBOX_ALLOW_DOCKER_TRUSTED", false),
		HHDCapabilityRegistry:         envBool("CWSO_HHD_CAPABILITY_REGISTRY_ENABLED", false),
		HHDDecisionTelemetry:          envBool("CWSO_HHD_DECISION_TELEMETRY_ENABLED", false),
		HHDTelemetryRedaction:         envBool("CWSO_HHD_TELEMETRY_REDACTION_ENABLED", false),
		HHDTelemetryRequestIDMode:     strings.ToLower(strings.TrimSpace(envOr("CWSO_HHD_TELEMETRY_REQUEST_ID_MODE", "hash"))),
		HHDTelemetryAnomalyNotes:      strings.ToLower(strings.TrimSpace(envOr("CWSO_HHD_TELEMETRY_ANOMALY_NOTES_MODE", "drop"))),
		HHDTelemetryRedactionSalt:     strings.TrimSpace(os.Getenv("CWSO_HHD_TELEMETRY_REDACTION_SALT")),
		HHDEventMonitorEnabled:        envBool("CWSO_HHD_EVENT_MONITOR_ENABLED", false),
		HHDEventMonitorEBPF:           envBool("CWSO_HHD_EVENT_MONITOR_EBPF_ENABLED", false),
		HHDEventMonitorLatencyMS:      envInt("CWSO_HHD_EVENT_MONITOR_LATENCY_THRESHOLD_MS", 1200),
		ASTSpikeResourcesEnabled:      envBool("CWSO_AST_SPIKE_RESOURCES_ENABLED", false),
		ASTSpikeMonitorEnabled:        envBool("CWSO_AST_SPIKE_MONITOR_ENABLED", false),
		ASTSpikePreferEBPF:            envBool("CWSO_AST_SPIKE_EBPF_ENABLED", false),
		ASTSpikeWindowMS:              envInt("CWSO_AST_SPIKE_WINDOW_MS", 1000),
		ASTSpikeThreshold:             envInt("CWSO_AST_SPIKE_THRESHOLD", 8),
		ASTSpikeDebounceMS:            envInt("CWSO_AST_SPIKE_DEBOUNCE_MS", 0),
		ASTSpikeMaxHotPaths:           envInt("CWSO_AST_SPIKE_MAX_HOT_PATHS", 5),
		ASTSpikeSemanticThreshold:     strings.ToLower(strings.TrimSpace(envOr("CWSO_AST_SPIKE_SEMANTIC_THRESHOLD", "signature_change"))),
		ASTSpikeConflictWindowMS:      envInt("CWSO_AST_SPIKE_CONFLICT_WINDOW_MS", 2000),
		ASTSpikeSignatureTTLMS:        envInt("CWSO_AST_SPIKE_SIGNATURE_TTL_MS", 30000),
		ASTSpikeMaxConflictPeers:      envInt("CWSO_AST_SPIKE_MAX_CONFLICT_PEERS", 8),
		SparseAgentsEnabled:           envBool("CWSO_SPARSE_AGENTS_ENABLED", false),
		SparseSocket:                  os.Getenv("CWSO_SPARSE_SOCKET"),
		SparseHostRAMCapMB:            envInt("CWSO_SPARSE_HOST_RAM_CAP_MB", 4096),
		SparseQualityGuardrailEnabled: envBool("CWSO_SPARSE_QUALITY_GUARDRAIL_ENABLED", false),
		RolloutRewardEnabled:          envBool("CWSO_ROLLOUT_REWARD_ENABLED", false),
		RolloutAPIEnabled:             envBool("CWSO_ROLLOUT_API_ENABLED", false),
		RolloutSocket:                 os.Getenv("CWSO_ROLLOUT_SOCKET"),
		HHDSnapshotTTLSeconds:         envInt("CWSO_HHD_CAPABILITY_SNAPSHOT_TTL_SECONDS", 30),
		HHDPolicyEngineV2:             envBool("CWSO_HHD_POLICY_ENGINE_V2_ENABLED", false),
		HHDHardwareAwareDispatch:      envBool("CWSO_HHD_HARDWARE_AWARE_DISPATCH_ENABLED", false),
		HHDPolicyMinConfidence:        envFloat64("CWSO_HHD_POLICY_MIN_CONFIDENCE", 0.5),
		HHDPolicyMaxQueueDepth:        envInt("CWSO_HHD_POLICY_MAX_QUEUE_DEPTH", 32),
		HHDWeightHealth:               envFloat64("CWSO_HHD_POLICY_WEIGHT_HEALTH", 0.35),
		HHDWeightReliability:          envFloat64("CWSO_HHD_POLICY_WEIGHT_RELIABILITY", 0.25),
		HHDWeightCost:                 envFloat64("CWSO_HHD_POLICY_WEIGHT_COST", 0.10),
		HHDWeightLatency:              envFloat64("CWSO_HHD_POLICY_WEIGHT_LATENCY", 0.10),
		HHDWeightQueueDepth:           envFloat64("CWSO_HHD_POLICY_WEIGHT_QUEUE_DEPTH", 0.10),
		HHDWeightWorkload:             envFloat64("CWSO_HHD_POLICY_WEIGHT_WORKLOAD", 0.10),
		HHDSparseQuantizedEnabled:     envBool("CWSO_HHD_SPARSE_QUANTIZED_ASSIST_ENABLED", false),
		HHDSparseQuantizedTradeoff:    envFloat64("CWSO_HHD_SPARSE_QUANTIZED_COST_LATENCY_TRADEOFF", 0),
		HHDQualityGuardrailMinScore:   envFloat64("CWSO_HHD_SPARSE_QUANTIZED_QUALITY_GUARDRAIL_MIN_SCORE", 0.98),
		HHDSSMAssistEnabled:           envBool("CWSO_HHD_SSM_ASSIST_ENABLED", false),
		HHDSSMThroughputBias:          envFloat64("CWSO_HHD_SSM_THROUGHPUT_BIAS", 0),
		HHDSSMMinSequenceLength:       envInt("CWSO_HHD_SSM_MIN_SEQUENCE_LENGTH", 2048),
		HHDSSMMaxSequenceLength:       envInt("CWSO_HHD_SSM_MAX_SEQUENCE_LENGTH", 32768),
		HHDSSMSequenceSensitivity:     envFloat64("CWSO_HHD_SSM_SEQUENCE_SENSITIVITY", 1),
		HHDWasmScoringEnabled:         envBool("CWSO_HHD_WASM_SCORING_ENABLED", false),
		HHDWasmScoringModulePath:      os.Getenv("CWSO_HHD_WASM_SCORING_MODULE_PATH"),
		HHDWasmScoringModuleSHA256:    strings.ToLower(strings.TrimSpace(os.Getenv("CWSO_HHD_WASM_SCORING_MODULE_SHA256"))),
		HHDWasmScoringTrustedDir:      strings.TrimSpace(os.Getenv("CWSO_HHD_WASM_SCORING_TRUSTED_DIR")),
		HHDWasmScoringTimeoutMS:       envInt("CWSO_HHD_WASM_SCORING_TIMEOUT_MS", 20),
		HHDWasmScoringMemoryPages:     uint32(envInt("CWSO_HHD_WASM_SCORING_MEMORY_LIMIT_PAGES", 64)),
		HHDWasmScoringHostCalls:       splitCSV(os.Getenv("CWSO_HHD_WASM_SCORING_HOST_CALL_ALLOWLIST")),
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
	if c.HHDSnapshotTTLSeconds <= 0 {
		return nil, fmt.Errorf("CWSO_HHD_CAPABILITY_SNAPSHOT_TTL_SECONDS must be > 0")
	}
	if c.HHDEventMonitorLatencyMS <= 0 {
		return nil, fmt.Errorf("CWSO_HHD_EVENT_MONITOR_LATENCY_THRESHOLD_MS must be > 0")
	}
	if c.ASTSpikeMonitorEnabled {
		switch c.ASTSpikeSemanticThreshold {
		case "signature_change", "symbol_added", "symbol_removed", "any":
		default:
			return nil, fmt.Errorf("CWSO_AST_SPIKE_SEMANTIC_THRESHOLD must be one of: signature_change, symbol_added, symbol_removed, any")
		}
		if c.ASTSpikeWindowMS <= 0 {
			return nil, fmt.Errorf("CWSO_AST_SPIKE_WINDOW_MS must be > 0")
		}
		if c.ASTSpikeThreshold <= 0 {
			return nil, fmt.Errorf("CWSO_AST_SPIKE_THRESHOLD must be > 0")
		}
		if c.ASTSpikeConflictWindowMS <= 0 {
			return nil, fmt.Errorf("CWSO_AST_SPIKE_CONFLICT_WINDOW_MS must be > 0")
		}
		if c.ASTSpikeSignatureTTLMS <= 0 {
			return nil, fmt.Errorf("CWSO_AST_SPIKE_SIGNATURE_TTL_MS must be > 0")
		}
	}
	if c.SparseAgentsEnabled {
		if strings.TrimSpace(c.SparseSocket) == "" {
			return nil, fmt.Errorf("CWSO_SPARSE_SOCKET must be set when CWSO_SPARSE_AGENTS_ENABLED=true")
		}
		if c.SparseHostRAMCapMB <= 0 {
			return nil, fmt.Errorf("CWSO_SPARSE_HOST_RAM_CAP_MB must be > 0")
		}
	}
	if c.HHDTelemetryRequestIDMode != "allow" && c.HHDTelemetryRequestIDMode != "hash" && c.HHDTelemetryRequestIDMode != "drop" {
		return nil, fmt.Errorf("CWSO_HHD_TELEMETRY_REQUEST_ID_MODE must be one of: allow, hash, drop")
	}
	if c.HHDTelemetryAnomalyNotes != "allow" && c.HHDTelemetryAnomalyNotes != "drop" {
		return nil, fmt.Errorf("CWSO_HHD_TELEMETRY_ANOMALY_NOTES_MODE must be one of: allow, drop")
	}
	if c.HHDPolicyMinConfidence < 0 || c.HHDPolicyMinConfidence > 1 {
		return nil, fmt.Errorf("CWSO_HHD_POLICY_MIN_CONFIDENCE must be between 0 and 1")
	}
	if c.HHDPolicyMaxQueueDepth <= 0 {
		return nil, fmt.Errorf("CWSO_HHD_POLICY_MAX_QUEUE_DEPTH must be > 0")
	}
	if c.HHDSparseQuantizedTradeoff < -1 || c.HHDSparseQuantizedTradeoff > 1 {
		return nil, fmt.Errorf("CWSO_HHD_SPARSE_QUANTIZED_COST_LATENCY_TRADEOFF must be between -1 and 1")
	}
	if c.HHDQualityGuardrailMinScore < 0 || c.HHDQualityGuardrailMinScore > 1 {
		return nil, fmt.Errorf("CWSO_HHD_SPARSE_QUANTIZED_QUALITY_GUARDRAIL_MIN_SCORE must be between 0 and 1")
	}
	if c.HHDSSMThroughputBias < -1 || c.HHDSSMThroughputBias > 1 {
		return nil, fmt.Errorf("CWSO_HHD_SSM_THROUGHPUT_BIAS must be between -1 and 1")
	}
	if c.HHDSSMMinSequenceLength <= 0 {
		return nil, fmt.Errorf("CWSO_HHD_SSM_MIN_SEQUENCE_LENGTH must be > 0")
	}
	if c.HHDSSMMaxSequenceLength <= c.HHDSSMMinSequenceLength {
		return nil, fmt.Errorf("CWSO_HHD_SSM_MAX_SEQUENCE_LENGTH must be greater than CWSO_HHD_SSM_MIN_SEQUENCE_LENGTH")
	}
	if c.HHDSSMSequenceSensitivity < 0 || c.HHDSSMSequenceSensitivity > 2 {
		return nil, fmt.Errorf("CWSO_HHD_SSM_SEQUENCE_SENSITIVITY must be between 0 and 2")
	}
	weights := []float64{
		c.HHDWeightHealth,
		c.HHDWeightReliability,
		c.HHDWeightCost,
		c.HHDWeightLatency,
		c.HHDWeightQueueDepth,
		c.HHDWeightWorkload,
	}
	total := 0.0
	for _, weight := range weights {
		if weight < 0 {
			return nil, fmt.Errorf("CWSO_HHD_POLICY_WEIGHT_* values must be >= 0")
		}
		total += weight
	}
	if total <= 0 {
		return nil, fmt.Errorf("CWSO_HHD_POLICY_WEIGHT_* values must sum to > 0")
	}
	if c.HHDWasmScoringTimeoutMS <= 0 {
		return nil, fmt.Errorf("CWSO_HHD_WASM_SCORING_TIMEOUT_MS must be > 0")
	}
	if c.HHDWasmScoringMemoryPages == 0 {
		return nil, fmt.Errorf("CWSO_HHD_WASM_SCORING_MEMORY_LIMIT_PAGES must be > 0")
	}
	if c.HHDWasmScoringEnabled && strings.TrimSpace(c.HHDWasmScoringModulePath) == "" {
		return nil, fmt.Errorf("CWSO_HHD_WASM_SCORING_MODULE_PATH must be set when CWSO_HHD_WASM_SCORING_ENABLED=true")
	}
	if c.HHDWasmScoringEnabled && c.HHDWasmScoringModuleSHA256 == "" {
		return nil, fmt.Errorf("CWSO_HHD_WASM_SCORING_MODULE_SHA256 must be set when CWSO_HHD_WASM_SCORING_ENABLED=true")
	}
	if c.HHDWasmScoringEnabled && len(c.HHDWasmScoringModuleSHA256) != 64 {
		return nil, fmt.Errorf("CWSO_HHD_WASM_SCORING_MODULE_SHA256 must be a 64-char sha256 hex digest")
	}
	if c.HHDWasmScoringEnabled {
		for _, ch := range c.HHDWasmScoringModuleSHA256 {
			if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
				return nil, fmt.Errorf("CWSO_HHD_WASM_SCORING_MODULE_SHA256 must be lowercase hex")
			}
		}
	}
	if c.HHDWasmScoringEnabled && c.HHDWasmScoringTrustedDir == "" {
		return nil, fmt.Errorf("CWSO_HHD_WASM_SCORING_TRUSTED_DIR must be set when CWSO_HHD_WASM_SCORING_ENABLED=true")
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

func envFloat64(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
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
