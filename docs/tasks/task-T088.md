# Task T088 — Phase 6 integration + reliability QA

- **Status:** in_review
- **Owner:** qa-engineer
- **Priority:** P0
- **Depends on:** T087 (done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A)
- **Based on:** `task-T087.md`, `docs/artifacts/cwso-nextgen-blueprint-v1.md`

## Objective
Validate the hardware-aware dispatch path end-to-end and guard the Phase 6 reliability
budgets: **dispatch overhead ≤ 10 ms** and **fallback ≤ 2.0 s**.

## Reliability Budgets (verified)
| Budget | Target | Measured (local) | Test |
|--------|--------|------------------|------|
| Dispatch overhead (profile + policy select + enqueue; the synchronous work the model waits on) | ≤ 10 ms median | median ≈ 4 µs, p95 ≈ 21 µs over 300 iters | `TestDispatchOverheadBudget` |
| Fallback end-to-end (selected backend → cpu-baseline, job completes) | ≤ 2.0 s | ≈ 51 ms | `TestFallbackLatencyBudget` |
| Failure propagation (all backends down → job fails fast, reason preserved) | ≤ 2.0 s, no hang | sub-ms | `TestDispatchFailurePropagatesWithinBudget` |

## Integration Coverage
- `internal/server`: `TestHardwareAwareDispatchLiveHALIntegration` exercises the full
  `config → server → hal.Client → UDS` path against a fake `cwso-hal` UDS server:
  asserts the tool is registered when `HHD*` flags are on, that a `tools/call` returns a
  `job_id`, and that the live HAL receives an `infer` with the policy-selected provider
  (`lpu-realtime`) and the faithfully forwarded prompt.
- `internal/hal`: client round-trip, structured-error decode, dial-failure (from T087).
- `internal/tools`: live-execution forwarding + shadow-mode no-Infer (from T087) plus the
  reliability budgets above.
- Full suite passes under `go test -race ./...`, `gofmt -l` clean, `go vet ./...` clean.

## Reliability Defect Found & Fixed
The QA gate uncovered a latent bug in `jobs.Manager.runRecord`: it called `r.cancel()`
**before** inspecting `r.ctx.Err()`, so the post-run cancel made `ctx.Err()` report
`Canceled` for *every* errored job. Consequence: genuine job failures were misclassified
as `cancelled` and the real error reason was discarded.

- **Fix:** capture `ctxErr := r.ctx.Err()` before `r.cancel()`; classify as cancelled only
  on a genuine context cancellation / deadline, otherwise `failed` with the original error.
- **Regression guard:** `jobs.TestLifecycleFailedPreservesError`.
- **Severity:** major (reliability/observability) · **Type:** technical.

## Acceptance Criteria
- [x] Dispatch overhead ≤ 10 ms (median).
- [x] Fallback path completes ≤ 2.0 s end-to-end.
- [x] Failures propagate (no hang) and preserve the error reason.
- [x] Server-level integration test covers the live HAL execution path.
- [x] `go test -race ./...`, `gofmt`, `go vet` all clean.

## Notes / Follow-ups
- Reliability budgets are measured against an in-memory fake HAL, so they isolate
  control-plane overhead from real provider latency (the intended QA target). Real-backend
  SLOs belong to deployment-time load testing, not unit CI.
- Capability-snapshot live-sync remains the deferred Phase 6 follow-up from T087 (out of
  scope for this gate).
