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
