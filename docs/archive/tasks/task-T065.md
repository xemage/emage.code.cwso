# Task T065 — Dispatch policy engine v2

- Phase: **5 (Implementation)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T063, T064 · Blocks: T066, T067, T068
- Status: done (2026-05-22)

## Objective
Build policy engine v2 that selects execution backends using capability and telemetry signals, with deterministic fallback behavior.

## Inputs
- [docs/tasks/task-T063.md](task-T063.md)
- [docs/tasks/task-T064.md](task-T064.md)

## Constraints
- Preserve existing behavior when feature flags are disabled.
- Fallback to baseline CPU path must complete within configured SLO.

## Expected outputs
- Dispatch policy engine implementation.
- Config and schema updates for policy tuning.

## Acceptance criteria
1. Policy selection is deterministic for identical inputs.
2. Fallback to baseline path occurs within 2 seconds on simulated provider failure.
3. Existing e2e scenarios pass with feature flags disabled.

## Blocker protocol
If policy signal quality is insufficient, report blocker type `technical` and propose temporary weighting defaults with evidence.

## Completion notes (2026-05-22)
- Implemented policy engine v2 in `orchestrator/internal/dispatch/policy_engine_v2.go` with deterministic scoring tuple and stable fallback-chain generation ending in `cpu-baseline`.
- Added policy fallback logic for provider failure/unavailable via `FallbackOnFailure`, and integrated selection + fallback behavior into dispatch flow in `orchestrator/internal/tools/dispatch_tools.go`.
- Added configurable policy controls in runtime config (`CWSO_HHD_POLICY_ENGINE_V2_ENABLED`, weights, confidence, queue-depth threshold) with safe defaults and input validation.
- Preserved baseline behavior when feature flags are disabled (`cpu-baseline-default` path).
- Added/updated tests for deterministic selection, baseline fallback behavior, and disabled-feature compatibility.
- Validation run:
	- `cd orchestrator && go test ./internal/dispatch ./internal/tools ./internal/config ./internal/server`
	- Result: pass.
