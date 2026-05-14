# Task T033 — Event-sourced memory broker

- Phase: **3 (Production)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T031 · Blocks: T034, T035
- Status: pending

## Objective
Introduce an event-sourced memory broker in the orchestrator kernel that records job and tool lifecycle events into an append-only in-memory log with bounded retention and query APIs, forming the canonical event source for telemetry throttling (T034) and phase-3 integration validation (T035).

## Inputs
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-5.2, §NFR-5
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §3, §6
- [task-T031.md](task-T031.md)
- [task-T032.md](task-T032.md)

## Constraints
- Event ingestion must be non-blocking for job execution path.
- Use append-only event model with monotonic sequence numbers.
- Bounded retention in memory (ring buffer) with deterministic eviction behavior.
- Expose read API for recent events by filters (topic, job_id, time window).
- Preserve existing SSE/eventbus behavior; broker augments, not replaces, current event flow.
- Do not log sensitive secrets or payloads that violate security constraints.

## Expected outputs
- New package: `orchestrator/internal/memorybroker/`
  - append-only event record type
  - in-memory ring buffer store
  - subscribe/read interfaces for consumers
- Integration wiring from jobs/dispatch/eventbus producers into broker ingestion path
- Query/read helper for downstream telemetry and integration tests
- Tests for:
  - sequence ordering
  - retention/eviction under load
  - concurrent ingest + reads (race-safe)
  - filtered queries correctness

## Acceptance criteria
1. Broker assigns strictly increasing sequence IDs for ingested events.
2. Retention cap is enforced with deterministic oldest-first eviction.
3. Concurrent producers and readers pass race-enabled tests without data corruption.
4. Filtered reads by topic and job ID return consistent ordered results.
5. `go test ./internal/memorybroker/... -race` passes.
6. Existing T031/T032 tests remain green.

## Blocker protocol
Same as T020.
