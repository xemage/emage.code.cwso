# Checkpoint 005 — Phase 3 Progress (T032 Complete)

Date: 2026-05-14
Phase: 3 — Async + concurrency
Status: in_progress

## Completed
- T032 completed and merged to develop via MR !5.
- Dispatch tool now returns immediate per-item accepted/rejected outcomes and enqueues accepted jobs to T031 job manager.
- CI passed and auto-merge completed.

## Current Task State
- T031: done
- T032: done
- T033: pending (next critical-path task)
- T034: pending (depends on T033)

## Key Decisions
1. Prioritize T033 next to establish canonical event log source before telemetry throttling (T034).
2. Maintain non-blocking execution paths by keeping broker ingestion lightweight and bounded.
3. Standardize event ordering and retention semantics now to reduce T034/T035 integration churn.

## Risks
- Event broker contention impacting dispatch/job throughput.
- Inconsistent event schema across producers.
- Over-retention causing memory pressure.

## Mitigations
- Bounded ring-buffer with deterministic eviction.
- Central envelope type and producer helper API.
- Concurrency tests with race detector and retention stress tests.

## Next Steps
1. Approve and execute T033 implementation.
2. Integrate broker outputs into T034 telemetry throttling path.
3. Prepare T035 integration test matrix once T033/T034 converge.
