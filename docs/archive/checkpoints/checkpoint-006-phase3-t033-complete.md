# Checkpoint 006 — Phase 3 Progress (T033 Complete)

Date: 2026-05-14
Phase: 3 — Async + concurrency
Status: in_progress

## Completed
- T033 completed and merged to develop via MR !6.
- Event-sourced memory broker now provides ordered append-only records, bounded retention, non-blocking ingestion, and filtered query APIs.
- Integration wiring from jobs and transport sample event publishers is in place.

## Current Task State
- T032: done
- T033: done
- T034: in_progress (next execution target)
- T035: pending (depends on T034)

## Key Decisions
1. Execute T034 next to stabilize SSE telemetry signal quality before full Phase 3 integration testing.
2. Keep deterministic topic-aware throttling with terminal-event bypass to avoid losing critical state changes.
3. Reuse broker sanitization guarantees as baseline for outbound notification safety.

## Risks
- Over-throttle may hide meaningful progress.
- Under-throttle may still overwhelm clients.
- Policy complexity may create inconsistent notification cadence.

## Mitigations
- Explicit per-topic policies and bypass rules for terminal events.
- Metrics/log counters for emitted vs suppressed notifications.
- Deterministic policy tests under burst traffic.

## Next Steps
1. Implement T034 telemetry throttling and JSON-RPC notification shaping.
2. Validate burst behavior + envelope correctness in transport tests.
3. Start T035 integration matrix after T034 merge.
