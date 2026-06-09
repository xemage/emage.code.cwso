# Task T146 — Gateway async staging + partial trace recovery

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T132, T144
- **Based on:** Polar §3.3 INIT/RUNNING/POSTRUN pools, timeout partial traces

## Objective

Implement gateway worker pools (INIT, READY buffer, RUNNING, POSTRUN) so runtime prep and
evaluation do not block GPU-bound harness execution; recover partial trajectories on timeout.

## Acceptance Criteria

- [x] Stage-isolated worker pools in cwso-rollout or orchestrator gateway layer
- [x] Evaluator prewarm begins during agent run when configured
- [x] Timeout still emits POSTRUN with partial captures + terminal status

## Completion Notes (2026-06-07)

Implemented `orchestrator/internal/rollout/gateway.go` with INIT → READY → RUNNING → POSTRUN
staged worker pools. Evaluator prewarm via `StubEvaluator` (sidecar probe when `CWSO_ROLLOUT_SOCKET`
set; no-op stub otherwise). Timeout path runs POSTRUN and stores partial trajectories via
`ApplySessionOutcome` with `TaskFailed` + `session timeout` error.

Feature flags (default off):
- `CWSO_ROLLOUT_GATEWAY_STAGING_ENABLED`
- `CWSO_ROLLOUT_EVALUATOR_PREWARM_ENABLED`
- Pool sizing / timeout: `CWSO_ROLLOUT_GATEWAY_*`

Tests: `gateway_test.go`, `integration_test.go` (`TestGatewayTimeoutPartialTraceRecovery`).
