# Checkpoint 004 — Phase 3 Progress (T031 Complete)

Date: 2026-05-14
Phase: 3 — Async + concurrency
Status: in_progress

## Completed
- T031 completed and merged to develop via MR !4.
- T031 delivered bounded async job manager, lifecycle FSM, cancellation path, and job-state event publication hooks.
- CI merged on green.

## Current Task State
- T030: done
- T031: done
- T032: in_progress (next critical-path task)
- T033/T034: pending (parallel/converging prerequisites for T035)

## Key Decisions
1. Start T032 immediately because it is now the lowest-ID unblocked P0 task on the critical path.
2. Keep dispatch behavior enqueue-only with immediate acknowledgment to satisfy FR-5.1.
3. Use per-item acceptance/rejection output for deterministic overload handling.

## Risks
- Batch input abuse (oversized payloads / queue saturation).
- Accidental blocking semantics in dispatch implementation.
- Role permission drift for planning-tier tool invocation.

## Mitigations
- Strict schema and batch-size validation.
- Explicit non-blocking tests and queue-pressure tests.
- Server-side authorization checks plus negative-role tests.

## Next Steps
1. Implement and test T032 (`dispatch_concurrent_jobs`).
2. Align with T033/T034 event model for T035 integration gate readiness.
