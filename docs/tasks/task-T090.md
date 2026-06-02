# Task T090 — Thread job context into `hal.Client.Infer`

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T089 (done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A) — follow-up
- **Based on:** `docs/artifacts/gate-phase6-feature-a-2026-06-02.md`, `task-T087.md`

## Objective
Propagate the job `context.Context` through the Go HAL client so a cancelled or timed-out
hardware-aware job aborts the in-flight HAL request instead of blocking on UDS I/O until the
fixed `ioTimeout` ceiling.

## Changes
- `hal.Client.Call` now takes `ctx context.Context` as its first argument.
  - Returns early if `ctx` is already done.
  - Bounds the connection deadline by the smaller of the fixed `ioTimeout` and `ctx.Deadline()`.
  - Spawns a watcher that closes the connection on `ctx.Done()`, so a blocked `writeFrame` /
    `readFrame` returns promptly; the context error (not the I/O error) is then returned.
- `hal.Client.Infer(ctx, providerID, fallbackChain, req)` threads the caller's context.
  `Stat` / `Capabilities` pass `context.Background()` (no external caller context yet).
- `dispatch_hardware_aware_job` passes the job context (from `jobs.Manager`) into `Infer`, so
  job cancellation/timeout cancels the HAL call.

## Acceptance Criteria
- [x] `hal.Client.Infer` accepts and honors a `context.Context`.
- [x] Cancellation unblocks in-flight I/O and returns `context.Canceled`.
- [x] An already-expired deadline returns `context.DeadlineExceeded` without dialing through.
- [x] `go test -race ./...`, `gofmt`, `go vet` clean.

## Tests
- `hal.TestClientInferContextCancelled` — server accepts but never replies; ctx cancel returns
  `context.Canceled` in well under 1s.
- `hal.TestClientInferContextDeadline` — pre-expired deadline returns `context.DeadlineExceeded`.

## Notes / Follow-ups
- Active health probing (T091) and TLS guidance (T093) are tracked separately as Rust-side
  follow-ups.
