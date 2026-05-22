package config

import (
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
	if c.HHDWasmScoringEnabled {
		t.Fatal("expected wasm scoring plugin disabled by default")
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

func TestLoadRejectsInvalidHHDEventMonitorLatencyThreshold(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_EVENT_MONITOR_LATENCY_THRESHOLD_MS", "0")

	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid event monitor latency threshold rejection")
	}
}

func TestLoadRejectsWasmScoringEnabledWithoutModulePath(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HHD_WASM_SCORING_ENABLED", "true")
	t.Setenv("CWSO_HHD_WASM_SCORING_MODULE_PATH", "")

	if _, err := Load(""); err == nil {
		t.Fatal("expected module path validation error when wasm scoring is enabled")
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
