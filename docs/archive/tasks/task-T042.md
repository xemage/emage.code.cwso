# Task T042 — gVisor runner

- Phase: **4 (Production)** · Owner: **devops-engineer** · Priority: **P0**
- Depends on: T041 · Blocks: T043
- Status: done

## Objective
Implement the gVisor sandbox runner as the fast-ephemeral isolated runtime tier for Phase 4. The implementation must follow the shared sandbox execution contract, preserve secure defaults introduced in T041, and operate correctly in degraded mode when Firecracker is unavailable.

## Inputs
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §3, §6
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-3, §FR-5, §NFR-6
- [ADR-003](../decisions/ADR-003-tiered-sandbox-strategy.md)
- [degraded-mode-v1.md](../artifacts/degraded-mode-v1.md)
- [task-T041.md](./task-T041.md)
- [plan-T042-phase4-gvisor-runner.md](../plans/plan-T042-phase4-gvisor-runner.md)

## Constraints
- Implement under `orchestrator/internal/sandbox/` with file `runner_gvisor.go`.
- Must satisfy existing `RunnerInterface` from `runner.go` without contract changes.
- Do not break Docker baseline (`runner_docker.go`) or existing dispatch tool schemas.
- Use gVisor runtime (`runsc`) through Docker runtime integration; forbid privileged mode and host networking.
- Apply secure defaults on every launch: dropped caps, no-new-privileges, read-only rootfs unless explicit override.
- Enforce resource limits and deterministic cleanup: stop -> kill -> remove.
- Surface clear actionable errors when `runsc` is unavailable/misconfigured.

## Expected outputs
- `orchestrator/internal/sandbox/runner_gvisor.go` — gVisor runner implementation
- Config/bootstrap wiring updates for selecting gVisor runtime
- Tests for success, timeout, cancellation, cleanup, and missing-runtime failure path
- Documentation updates in `sandbox/README.md` (and related ops docs if needed)

## Acceptance criteria
1. Runner executes a sandboxed command under gVisor and returns stdout/stderr/exit code.
2. Timeout and cancellation paths remove containers deterministically with no leaks.
3. Security defaults are enforced on all runs and validated in tests.
4. Missing `runsc` runtime returns explicit, non-ambiguous error guidance.
5. Existing Docker baseline path still passes tests.
6. `go test ./...` in `orchestrator/` passes.
7. Task output unblocks T043 with no interface rewrite.

## Blocker protocol
Same as T020. Escalate `external` blocker if target environment cannot install/enable `runsc` despite documented prerequisites.

## Completion notes (2026-05-15)
- Implemented `orchestrator/internal/sandbox/runner_gvisor.go` using Docker runtime integration with `runsc`.
- Added explicit runtime preflight checks and actionable missing/misconfigured runtime errors.
- Added gVisor tests for success, timeout, cancellation, deterministic cleanup, and missing runtime behavior.
- Verified baseline compatibility by running `go test ./...` in `orchestrator/` (all packages passed).
