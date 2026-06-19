# Task T066 — Wasm micro-agent runtime integration

- Phase: **5 (Implementation)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T065 · Blocks: T070
- Status: done (2026-05-22)

## Objective
Integrate a safe Wasm micro-agent runtime for hot-swappable orchestration extensions (for example policy adapters or post-processors).

## Inputs
- [docs/tasks/task-T065.md](task-T065.md)
- `docs/artifacts/architecture-phase5-hhd-v1.md` (from T063)

## Constraints
- Host-call surface must be allowlisted and auditable.
- Runtime must support explicit resource limits (CPU/memory/time).

## Expected outputs
- Wasm runtime integration module.
- Operator configuration docs for module loading and rollback.

## Acceptance criteria
1. At least one micro-agent extension executes in staging with sandbox limits.
2. Module load/disable is controlled by configuration and feature flags.
3. Security review artifacts list host capabilities exposed to Wasm.

## Blocker protocol
If runtime constraints cannot be enforced, report blocker type `technical` and halt promotion until controls are in place.

## Completion notes (2026-05-22)
- Added Wasm scoring runtime integration in `orchestrator/internal/dispatch/wasm_scoring_plugin.go` using wazero with:
	- feature-gated module enable/disable behavior
	- per-call timeout limits
	- memory limit pages
	- deny-by-default host-call allowlist enforcement
- Integrated practical plugin hook in policy scoring flow (`orchestrator/internal/dispatch/policy_engine_v2.go`) via optional `ScoreAdjuster`.
- Preserved backward compatibility and safe fallback:
	- plugin disabled -> baseline policy scoring path unchanged
	- plugin runtime/load/call failure -> built-in scoring path selected with `plugin_failed_fallback` reason code
- Added targeted tests covering disabled, enabled-success, and failure-fallback scenarios:
	- `orchestrator/internal/dispatch/policy_engine_v2_test.go`
	- `orchestrator/internal/dispatch/wasm_scoring_plugin_test.go`
	- `orchestrator/internal/config/config_test.go`
- Added operator-facing note: `docs/artifacts/wasm-scoring-runtime-ops-v1.md`.
