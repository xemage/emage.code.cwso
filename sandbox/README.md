# Sandbox runners (Phase 4)

This directory will host the sandbox runner integrations:

| Runner | File | Phase |
|--------|------|-------|
| Docker (trusted) | `runner_docker.go` | 4 (T041) |
| gVisor / runsc | `runner_gvisor.go` | 4 (T042) |
| Firecracker microVM + snapshots | `runner_firecracker.go` | 4 (T043) |

A common `RunnerInterface` lives at `orchestrator/internal/sandbox/runner.go` (to be created in T041).

## Docker baseline runner (T041)

Implementation files:
- `orchestrator/internal/sandbox/runner.go`
- `orchestrator/internal/sandbox/runner_docker.go`

Runtime selection:
- `CWSO_SANDBOX_RUNNER=none` (default)
- `CWSO_SANDBOX_RUNNER=docker`
- `CWSO_SANDBOX_RUNNER=gvisor`
- `CWSO_SANDBOX_RUNNER=firecracker`

Docker baseline environment knobs:
- `CWSO_DOCKER_HOST` (default: `unix:///var/run/docker.sock`)
- `CWSO_DOCKER_IMAGE` (default: `alpine:3.20`)
- `CWSO_DOCKER_RUNTIME` (optional; default empty for Docker baseline, forced to `runsc` for gVisor runner)
- `CWSO_DOCKER_NETWORK` (default: `none`, `host` forbidden)
- `CWSO_DOCKER_CPU_QUOTA_MICROS` (default: `100000`)
- `CWSO_DOCKER_MEMORY_BYTES` (default: `268435456`)
- `CWSO_DOCKER_PIDS_LIMIT` (default: `128`)
- `CWSO_DOCKER_STOP_TIMEOUT_SECONDS` (default: `5`)

Security defaults applied on every container launch:
- non-privileged container (`Privileged=false`)
- dropped Linux capabilities (`CapDrop=["ALL"]`)
- `no-new-privileges`
- read-only root filesystem by default
- non-host networking by default (`none`)

Lifecycle semantics:
- launch (`create -> start`) and return captured `stdout`, `stderr`, and `exit_code`

## gVisor runner (T042)

Implementation file:
- `orchestrator/internal/sandbox/runner_gvisor.go`

gVisor uses Docker runtime selection with `runsc` and reuses the same
`RunnerInterface` contract and deterministic lifecycle cleanup used by Docker
baseline (`stop -> kill -> remove`, followed by leak verification by name).

gVisor-specific behavior:
- validates Docker daemon runtime registration before each execution
- returns explicit actionable errors when `runsc` is missing/misconfigured
- applies the same hardened launch defaults as Docker baseline:
	- `Privileged=false`
	- `NetworkMode=none` (host networking forbidden)
	- `CapDrop=["ALL"]`
	- `SecurityOpt=["no-new-privileges:true"]`
	- read-only root filesystem unless explicit writable override is enabled

Operational requirement for gVisor hosts:
- configure Docker runtime `runsc` (for example in `/etc/docker/daemon.json`) and restart Docker
- cancellation and timeout are deterministic: `stop -> kill -> remove`
- cleanup includes a post-remove list check to detect leaks

## Firecracker runner (T043)

Implementation file:
- `orchestrator/internal/sandbox/runner_firecracker.go`

Firecracker-specific environment knobs:
- `CWSO_FIRECRACKER_BIN` (default: `firecracker`)
- `CWSO_FIRECRACKER_EXEC_HELPER` (required when `CWSO_SANDBOX_RUNNER=firecracker`)
- `CWSO_FIRECRACKER_KVM_DEVICE` (default: `/dev/kvm`)
- `CWSO_FIRECRACKER_VHOST_DEVICE` (default: `/dev/vhost-net`)
- `CWSO_FIRECRACKER_REQUIRE_VHOST_NET` (default: `true`)
- `CWSO_FIRECRACKER_SNAPSHOT_DIR` (default: `/tmp/cwso-firecracker/templates`)
- `CWSO_FIRECRACKER_VMSTATE_DIR` (default: `/tmp/cwso-firecracker/vms`)

Lifecycle semantics:
- deterministic flow: `preflight -> ensure template -> clone snapshot -> execute -> shutdown -> cleanup -> release clone`
- timeout and cancellation enforce cleanup and clone release with explicit result flags (`timed_out`, `cancelled`)
- missing prerequisites return actionable errors:
  - missing Firecracker binary/helper
  - unavailable `/dev/kvm`
  - unavailable `/dev/vhost-net` when required

Snapshot CoW behavior:
- snapshot hooks are exposed via template/clone/release interfaces for T044 routing integration
- current baseline uses deterministic filesystem-backed template/clone metadata under the configured Firecracker state directories
