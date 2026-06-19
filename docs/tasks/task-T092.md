# Task T092 — Hardware-aware job result retrieval

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P2
- **Depends on:** T089 (done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A) — follow-up
- **Based on:** `docs/artifacts/gate-phase6-feature-a-2026-06-02.md`, `task-T087.md`

## Objective
Capture the HAL completion produced by a hardware-aware job and expose it through the job
lifecycle, so a caller that dispatched fire-and-forget can retrieve *which backend served* and
*what was produced* via `Manager.Get` or the job-state notification stream.

## Changes
- `jobs.Manager`:
  - New body variant `RunResult func(context.Context) (string, error)` on `jobs.Request`
    (exactly one of `Run` / `RunResult` must be set; enforced in `Enqueue`).
  - `Job` gained a `Result string` field; the `RunResult` payload is stored on completion via
    a `transitionWithResult` helper.
  - `publishTransition` includes `Result` in the SSE job-state payload.
- `dispatch_hardware_aware_job` now dispatches via `RunResult`, marshaling a compact
  `hwAwareJobResult` summary: `served_by`, `fallback_count`, `tokens_out`, `deterministic`,
  `output`. Shadow mode (no HAL client) preserves the context-respecting no-op with empty result.

## Acceptance Criteria
- [x] Job completion payload is captured into `Job.Result` and surfaced via `Manager.Get`.
- [x] `Run` and `RunResult` are mutually exclusive (rejected with `ErrInvalidJob`).
- [x] Live hardware-aware dispatch stores the backend identity + output in the result.
- [x] `go test -race ./...`, `gofmt`, `go vet` clean.

## Tests
- `jobs.TestLifecycleRunResultCaptured`, `jobs.TestEnqueueRejectsBothRunAndRunResult`.
- `tools.TestHardwareAwareDispatchCapturesJobResult` — asserts the completed job carries the
  policy-selected `served_by` (`lpu-realtime`) and the produced output.

## Notes / Follow-ups
- The result is published on the existing job-state notification; a dedicated streaming
  resource for incremental token output is out of scope for this follow-up.
