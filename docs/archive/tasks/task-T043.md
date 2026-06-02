# Task T043 — Firecracker runner + snapshot CoW

- Phase: **4 (Production)** · Owner: **devops-engineer** · Priority: **P0**
- Depends on: T042 · Blocks: T044
- Status: done

## Objective
Implement the Firecracker secure-isolation runner and snapshot copy-on-write execution hooks so untrusted workloads can run inside hardware-isolated microVMs with deterministic orchestration lifecycle semantics. This task provides the final runtime backend required before tier-router integration (T044).

## Inputs
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §2, §6
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-3, §FR-5, §NFR-3
- [ADR-003](../decisions/ADR-003-tiered-sandbox-strategy.md)
- [host-readiness-v1.md](../artifacts/host-readiness-v1.md)
- [degraded-mode-v1.md](../artifacts/degraded-mode-v1.md)
- [task-T041.md](./task-T041.md)
- [task-T042.md](./task-T042.md)
- [plan-T043-phase4-firecracker-runner.md](../plans/plan-T043-phase4-firecracker-runner.md)

## Constraints
- Implement under `orchestrator/internal/sandbox/` with file `runner_firecracker.go`.
- Conform to existing `RunnerInterface` with no breaking schema/API changes.
- Preserve Docker and gVisor runner behavior and test coverage.
- Firecracker path must enforce strict isolation defaults and avoid host filesystem writes outside explicit ephemeral artifacts.
- Provide deterministic lifecycle: launch -> execute -> collect -> shutdown -> cleanup.
- Include snapshot CoW abstraction/hooks for template/clone path (deterministic baseline required even if full optimization deferred).
- If host lacks KVM/Firecracker prerequisites, return explicit actionable error and avoid silent fallback.

## Expected outputs
- `orchestrator/internal/sandbox/runner_firecracker.go` — Firecracker runner implementation
- Supporting Firecracker runtime config/bootstrap wiring where needed
- Snapshot CoW lifecycle interfaces/hooks with tests
- Tests for success, timeout, cancellation, cleanup, and unavailable-runtime behavior
- Documentation updates in `sandbox/README.md` and related ops docs if required

## Acceptance criteria
1. Runner executes command via Firecracker pathway and returns stdout/stderr/exit metadata through shared result contract.
2. Timeout and cancellation perform deterministic shutdown and cleanup with no orphaned runtime artifacts.
3. Snapshot CoW hooks exist and are exercised in tests for template/clone lifecycle behavior.
4. Missing KVM/Firecracker prerequisites produce explicit actionable errors.
5. Existing Docker and gVisor paths remain green.
6. `go test ./...` in `orchestrator/` passes.
7. Output unblocks T044 without RunnerInterface rewrite.

## Blocker protocol
Same as T020. Escalate `external` blocker if target infrastructure cannot provide required KVM/Firecracker support for production runtime.
