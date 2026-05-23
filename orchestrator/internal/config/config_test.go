package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Transport != "stdio" {
		t.Fatalf("expected stdio, got %s", c.Transport)
	}
	if c.SandboxRunner != "none" {
		t.Fatalf("expected sandbox runner none by default, got %s", c.SandboxRunner)
	}
	if c.HHDCapabilityRegistry {
		t.Fatal("expected capability registry disabled by default")
	}
	if c.HHDDecisionTelemetry {
		t.Fatal("expected decision telemetry disabled by default")
	}
	if c.HHDTelemetryRedaction {
		t.Fatal("expected telemetry redaction disabled by default")
	}
	if c.HHDTelemetryRequestIDMode != "hash" {
		t.Fatalf("expected default telemetry request-id mode hash, got %q", c.HHDTelemetryRequestIDMode)
	}
	if c.HHDTelemetryAnomalyNotes != "drop" {
		t.Fatalf("expected default telemetry anomaly notes mode drop, got %q", c.HHDTelemetryAnomalyNotes)
	}
	if c.HHDTelemetryRedactionSalt != "" {
		t.Fatalf("expected empty telemetry redaction salt by default, got %q", c.HHDTelemetryRedactionSalt)
	}
	if c.HHDEventMonitorEnabled {
		t.Fatal("expected event monitor disabled by default")
	}
	if c.HHDEventMonitorEBPF {
		t.Fatal("expected eBPF monitor path disabled by default")
	}
	if c.HHDEventMonitorLatencyMS != 1200 {
		t.Fatalf("expected default monitor latency threshold 1200ms, got %d", c.HHDEventMonitorLatencyMS)
	}
	if c.HHDSnapshotTTLSeconds != 30 {
		t.Fatalf("expected default snapshot ttl 30 seconds, got %d", c.HHDSnapshotTTLSeconds)
	}
	if c.HHDPolicyEngineV2 {
		t.Fatal("expected policy engine v2 disabled by default")
	}
	if c.HHDPolicyMinConfidence != 0.5 {
		t.Fatalf("expected default policy min confidence 0.5, got %v", c.HHDPolicyMinConfidence)
	}
	if c.HHDSparseQuantizedEnabled {
		t.Fatal("expected sparse/quantized assist disabled by default")
	}
	if c.HHDSparseQuantizedTradeoff != 0 {
		t.Fatalf("expected default sparse/quantized cost-latency tradeoff 0, got %v", c.HHDSparseQuantizedTradeoff)
	}
	if c.HHDQualityGuardrailMinScore != 0.98 {
		t.Fatalf("expected default quality guardrail min score 0.98, got %v", c.HHDQualityGuardrailMinScore)
	}
	if c.HHDSSMAssistEnabled {
		t.Fatal("expected SSM assist disabled by default")
	}
	if c.HHDSSMThroughputBias != 0 {
		t.Fatalf("expected default SSM throughput bias 0, got %v", c.HHDSSMThroughputBias)
	}
	if c.HHDSSMMinSequenceLength != 2048 {
		t.Fatalf("expected default SSM min sequence length 2048, got %d", c.HHDSSMMinSequenceLength)
	}
	if c.HHDSSMMaxSequenceLength != 32768 {
		t.Fatalf("expected default SSM max sequence length 32768, got %d", c.HHDSSMMaxSequenceLength)
	}
	if c.HHDSSMSequenceSensitivity != 1 {
		t.Fatalf("expected default SSM sequence sensitivity 1, got %v", c.HHDSSMSequenceSensitivity)
	}
	if c.HHDWasmScoringEnabled {
		t.Fatal("expected wasm scoring plugin disabled by default")
	}
	if c.HHDWasmScoringModuleSHA256 != "" {
		t.Fatalf("expected empty wasm scoring module sha256 by default, got %q", c.HHDWasmScoringModuleSHA256)
	}
	if c.HHDWasmScoringTrustedDir != "" {
		t.Fatalf("expected empty wasm scoring trusted dir by default, got %q", c.HHDWasmScoringTrustedDir)
	}
	if c.HHDWasmScoringTimeoutMS != 20 {
		t.Fatalf("expected default wasm scoring timeout 20ms, got %d", c.HHDWasmScoringTimeoutMS)
	}
	if c.HHDWasmScoringMemoryPages != 64 {
		t.Fatalf("expected default wasm scoring memory pages 64, got %d", c.HHDWasmScoringMemoryPages)
	}
}

func TestLoadHTTPRequiresJWTSecret(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "http")
	t.Setenv("CWSO_JWT_SECRET", "")
	if _, err := Load(""); err == nil {
		t.Fatal("expected error when http transport has no JWT secret")
	}
}

func TestLoadInvalidTransport(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "carrier-pigeon")
	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid transport error")
	}
}

func TestLoadInvalidSandboxRunner(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_SANDBOX_RUNNER", "nspawn")
	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid sandbox runner error")
	}
}

func TestLoadDockerSandboxRejectsHostNetwork(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_SANDBOX_RUNNER", "docker")
	t.Setenv("CWSO_DOCKER_NETWORK", "host")
	if _, err := Load(""); err == nil {
		t.Fatal("expected host network rejection")
	}
}

func TestLoadGVisorRunnerDefaultsRuntimeToRunsc(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_SANDBOX_RUNNER", "gvisor")
	t.Setenv("CWSO_DOCKER_RUNTIME", "")

	c, err := Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if c.SandboxRuntime != "runsc" {
		t.Fatalf("expected gvisor runtime runsc, got %q", c.SandboxRuntime)
	}
}

func TestLoadGVisorRejectsHostNetwork(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_SANDBOX_RUNNER", "gvisor")
	t.Setenv("CWSO_DOCKER_NETWORK", "host")
	if _, err := Load(""); err == nil {
		t.Fatal("expected host network rejection")
	}
}

func TestLoadFirecrackerRequiresExecHelper(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_SANDBOX_RUNNER", "firecracker")
	t.Setenv("CWSO_FIRECRACKER_EXEC_HELPER", "")

	if _, err := Load(""); err == nil {
		t.Fatal("expected firecracker helper validation error")
	}
}

func TestLoadFirecrackerRunnerAccepted(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_SANDBOX_RUNNER", "firecracker")
	t.Setenv("CWSO_FIRECRACKER_EXEC_HELPER", "/usr/local/bin/cwso-firecracker-helper")

	c, err := Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if c.SandboxRunner != "firecracker" {
		t.Fatalf("expected firecracker runner, got %q", c.SandboxRunner)
	}
	if c.SandboxFCHelper == "" {
		t.Fatal("expected firecracker helper path to be set")
	}
}

func TestLoadMergeEngineSocketFromEnv(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_MERGE_ENGINE_SOCKET", "/run/cwso/merge-engine.sock")

	c, err := Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if c.MergeEngineSocket != "/run/cwso/merge-engine.sock" {
		t.Fatalf("expected merge engine socket, got %q", c.MergeEngineSocket)
	}
}

func TestLoadRejectsRS256InCurrentBuild(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_JWT_ALG", "RS256")

	if _, err := Load(""); err == nil {
		t.Fatal("expected RS256 to be rejected in current build")
	}
}

func TestLoadRejectsInvalidHHDSnapshotTTL(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_CAPABILITY_SNAPSHOT_TTL_SECONDS", "0")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid HHD snapshot ttl rejection")
	}
}

func TestLoadRejectsInvalidPolicyMinConfidence(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_POLICY_MIN_CONFIDENCE", "1.2")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid HHD policy min confidence rejection")
	}
}

func TestLoadRejectsInvalidSparseQuantizedTradeoff(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_SPARSE_QUANTIZED_COST_LATENCY_TRADEOFF", "1.2")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid sparse/quantized tradeoff rejection")
	}
}

func TestLoadRejectsInvalidSparseQuantizedQualityGuardrail(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_SPARSE_QUANTIZED_QUALITY_GUARDRAIL_MIN_SCORE", "-0.1")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid sparse/quantized quality guardrail rejection")
	}
}

func TestLoadRejectsInvalidSSMThroughputBias(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_SSM_THROUGHPUT_BIAS", "1.2")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid SSM throughput bias rejection")
	}
}

func TestLoadRejectsInvalidSSMSequenceRange(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_SSM_MIN_SEQUENCE_LENGTH", "4096")
	t.Setenv("CWSO_HHD_SSM_MAX_SEQUENCE_LENGTH", "4096")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid SSM sequence range rejection")
	}
}

func TestLoadRejectsInvalidSSMSequenceSensitivity(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_SSM_SEQUENCE_SENSITIVITY", "2.2")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid SSM sequence sensitivity rejection")
	}
}

func TestLoadRejectsInvalidHHDEventMonitorLatencyThreshold(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_EVENT_MONITOR_LATENCY_THRESHOLD_MS", "0")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid event monitor latency threshold rejection")
	}
}

func TestLoadRejectsInvalidTelemetryRequestIDMode(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_TELEMETRY_REQUEST_ID_MODE", "mask")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid telemetry request-id mode rejection")
	}
}

func TestLoadRejectsInvalidTelemetryAnomalyNotesMode(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_TELEMETRY_ANOMALY_NOTES_MODE", "hash")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid telemetry anomaly notes mode rejection")
	}
}

func TestLoadRejectsWasmScoringEnabledWithoutModulePath(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_WASM_SCORING_ENABLED", "true")
	t.Setenv("CWSO_HHD_WASM_SCORING_MODULE_PATH", "")
	t.Setenv("CWSO_HHD_WASM_SCORING_MODULE_SHA256", strings.Repeat("a", 64))
	t.Setenv("CWSO_HHD_WASM_SCORING_TRUSTED_DIR", "/tmp")

	if _, err := Load(""); err == nil {
		t.Fatal("expected module path validation error when wasm scoring is enabled")
	}
}

func TestLoadRejectsWasmScoringEnabledWithoutModuleSHA256(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_WASM_SCORING_ENABLED", "true")
	t.Setenv("CWSO_HHD_WASM_SCORING_MODULE_PATH", "/tmp/plugin.wasm")
	t.Setenv("CWSO_HHD_WASM_SCORING_MODULE_SHA256", "")
	t.Setenv("CWSO_HHD_WASM_SCORING_TRUSTED_DIR", "/tmp")

	if _, err := Load(""); err == nil {
		t.Fatal("expected module sha256 validation error when wasm scoring is enabled")
	}
}

func TestLoadRejectsInvalidWasmScoringModuleSHA256(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_WASM_SCORING_ENABLED", "true")
	t.Setenv("CWSO_HHD_WASM_SCORING_MODULE_PATH", "/tmp/plugin.wasm")
	t.Setenv("CWSO_HHD_WASM_SCORING_MODULE_SHA256", "xyz")
	t.Setenv("CWSO_HHD_WASM_SCORING_TRUSTED_DIR", "/tmp")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid module sha256 rejection")
	}
}

func TestLoadRejectsWasmScoringEnabledWithoutTrustedDir(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_WASM_SCORING_ENABLED", "true")
	t.Setenv("CWSO_HHD_WASM_SCORING_MODULE_PATH", "/tmp/plugin.wasm")
	t.Setenv("CWSO_HHD_WASM_SCORING_MODULE_SHA256", strings.Repeat("a", 64))
	t.Setenv("CWSO_HHD_WASM_SCORING_TRUSTED_DIR", "")

	if _, err := Load(""); err == nil {
		t.Fatal("expected trusted dir validation error when wasm scoring is enabled")
	}
}

func TestLoadRejectsInvalidWasmScoringTimeout(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_WASM_SCORING_TIMEOUT_MS", "0")

	if _, err := Load(""); err == nil {
		t.Fatal("expected wasm scoring timeout validation error")
	}
}

func TestLoadRejectsInvalidWasmScoringMemoryPages(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_WASM_SCORING_MEMORY_LIMIT_PAGES", "0")

	if _, err := Load(""); err == nil {
		t.Fatal("expected wasm scoring memory pages validation error")
	}
}
