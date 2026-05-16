# Task T044 — Sandbox tier router

- Phase: **4 (Production)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T043 · Blocks: T045
- Status: done

## Objective
Implement the orchestrator sandbox tier router that selects `docker-trusted`, `gvisor-fast-ephemeral`, or `firecracker-secure-isolation` per workload based on trust policy and host/runtime readiness. Routing decisions must be enforced server-side to prevent caller privilege escalation.

## Inputs
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §2, §6
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-3, §FR-5, §NFR-3
- [ADR-003](../decisions/ADR-003-tiered-sandbox-strategy.md)
- [host-readiness-v1.md](../artifacts/host-readiness-v1.md)
- [degraded-mode-v1.md](../artifacts/degraded-mode-v1.md)
- [task-T041.md](./task-T041.md)
- [task-T042.md](./task-T042.md)
- [task-T043.md](./task-T043.md)
- [plan-T044-phase4-sandbox-tier-router.md](../plans/plan-T044-phase4-sandbox-tier-router.md)

## Constraints
- Enforce routing policy server-side; callers cannot self-escalate to higher-privilege tiers.
- Preserve existing dispatch/request schemas unless strictly required; avoid breaking API contracts.
- Integrate runner availability checks and degraded-mode rules for Firecracker-unavailable hosts.
- Maintain deterministic behavior and explicit reason codes for routing outcomes.
- Unknown or invalid profiles must be rejected with clear errors (default-deny).
- Keep existing Docker/gVisor/Firecracker runner implementations and tests green.

## Expected outputs
- Router implementation integrated in orchestrator execution path
- Policy mapping for trust levels and profile overrides/validation
- Structured telemetry/log fields for routing decisions and fallback reason
- Tests for policy enforcement, degraded fallback, and non-escalation guarantees
- Documentation updates where routing behavior is described

## Acceptance criteria
1. Trusted/internal workloads route to Docker baseline when policy permits.
2. Benign ephemeral workloads route to gVisor by policy.
3. Untrusted workloads route to Firecracker when available; otherwise explicit degraded behavior per policy.
4. Caller attempts to escalate sandbox tier are rejected or overridden by server policy.
5. Routing decisions include auditable reason metadata.
6. `go test ./...` in `orchestrator/` passes.
7. Output unblocks T045 without changing `RunnerInterface` contract.

## Blocker protocol
Same as T020. Escalate `external` blocker if environment policy mandates Firecracker but required runtime/host capability is unavailable.
