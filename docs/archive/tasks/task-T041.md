# Task T041 — Docker baseline runner

- Phase: **4 (Production)** · Owner: **devops-engineer** · Priority: **P0**
- Depends on: T040 · Blocks: T042, T044
- Status: done

## Objective
Implement the baseline Docker sandbox runner used by the orchestrator for trusted execution in Phase 4. This runner must provide reliable lifecycle controls, strict resource constraints, secure-by-default container settings, and deterministic cleanup semantics so downstream tiered runtimes (gVisor, Firecracker) can build on a stable execution contract.

## Inputs
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §3, §6
- [ADR-003](../decisions/ADR-003-tiered-sandbox-strategy.md)
- [host-readiness-v1.md](../artifacts/host-readiness-v1.md)
- [degraded-mode-v1.md](../artifacts/degraded-mode-v1.md)
- [plan-T041-phase4-docker-baseline.md](../plans/plan-T041-phase4-docker-baseline.md)

## Constraints
- Implement under `orchestrator/internal/sandbox/` with a shared `RunnerInterface` in `runner.go`.
- Docker runner file name: `runner_docker.go`.
- No privileged containers; no host networking; no writable host mounts except explicit workspace mount policy.
- Default runtime security flags must include dropped Linux capabilities and `no-new-privileges`.
- Enforce resource limits per job: CPU quota, memory, pids, and wall-clock timeout.
- Cancellation must be cooperative and forceful: send stop, then kill, then remove.
- Preserve existing Phase 3 dispatch interfaces and avoid breaking `dispatch_concurrent_jobs` behavior.

## Expected outputs
- `orchestrator/internal/sandbox/runner.go` — common runner interface + config types
- `orchestrator/internal/sandbox/runner_docker.go` — Docker baseline implementation
- Runtime wiring updates in orchestrator config/bootstrap code to enable baseline selection
- Tests:
  - unit tests for config validation and lifecycle transitions
  - integration tests for successful run, timeout, cancellation, and cleanup on failure
- Documentation updates in `sandbox/README.md` (and any relevant ops docs) for baseline runner usage

## Acceptance criteria
1. Runner can launch a containerized job and return stdout/stderr and exit code successfully.
2. Timeout path stops and removes container without leaks (verified via Docker listing check in test).
3. Cancellation path behaves deterministically: stop -> kill -> remove, with bounded retries.
4. Security defaults applied on every launch: non-privileged, dropped caps, `no-new-privileges`, and read-only rootfs unless explicit override.
5. Resource limits are enforced and surfaced in logs/metrics when exceeded.
6. `go test` for sandbox runner packages passes locally and in CI.
7. Task output unblocks T042 without API/contract rewrites.

## Blocker protocol
Same as T020. Escalate `external` blocker if Docker daemon/runtime capability is unavailable in target environment.

## Completion notes (2026-05-15)
- Added shared sandbox contract and Docker baseline runner under `orchestrator/internal/sandbox/`.
- Wired optional runner bootstrap via orchestrator config/server (`CWSO_SANDBOX_RUNNER=docker`).
- Preserved dispatch request/response schema and Phase 3 async contracts.
- Added runner tests for success, timeout, cancellation, security defaults, and cleanup verification.
- Updated `sandbox/README.md` with runtime knobs and security/lifecycle behavior.

Validation summary:
- `go test ./...` from `orchestrator/` passes locally including sandbox package tests.
