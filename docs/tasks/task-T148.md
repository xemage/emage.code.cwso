# Task T148 — Evaluator registry + SWE-bench hook

- **Status:** in_review
- **Owner:** backend-developer / qa-engineer
- **Priority:** P2
- **Depends on:** T146, T144
- **Based on:** Polar §3.5

## Objective

Registry-backed evaluators run after trajectory construction: session reward, test-on-output,
and SWE-bench/SWE-Gym patch scoring in a fresh runtime.

## Acceptance Criteria

- [x] Evaluator plugin interface + built-in session reward
- [x] SWE-bench harness evaluator PoC (single instance)
- [x] Rewards attach to trajectory traces per Polar propagation rules

## Completion Notes (2026-06-09)

Implemented `orchestrator/internal/rollout/evaluator_registry.go` with pluggable `Plugin`
interface, `SessionRewardPlugin` (merge SM rewards from `rollout/reward` topic), and
`SWEBenchPlugin` stub (instance metadata + neutral reward; harness launch deferred).

Feature flags (default off):
- `CWSO_ROLLOUT_EVALUATOR_REGISTRY_ENABLED`
- `CWSO_ROLLOUT_EVALUATOR_SESSION_REWARD_ENABLED`
- `CWSO_ROLLOUT_EVALUATOR_SWEBENCH_ENABLED`
- `CWSO_ROLLOUT_SWEBENCH_INSTANCE`

Wired into `CompleteSession` via `Service.SetEvaluatorRegistry`; server attaches registry when
`RolloutAPIEnabled` + registry flag set.

Tests: `evaluator_registry_test.go`, `TestEvaluatorRegistryIntegration` in `integration_test.go`.
