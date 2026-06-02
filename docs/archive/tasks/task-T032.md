# Task T032 — dispatch_concurrent_jobs tool

- Phase: **3 (Production)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T031 · Blocks: T035
- Status: in_progress

## Objective
Expose a new MCP tool, `dispatch_concurrent_jobs`, that accepts a batch of job specs and returns immediately with job IDs and accepted/rejected results, without blocking caller execution, while scheduling accepted jobs on the T031 async job manager.

## Inputs
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-1.3, §FR-5.1
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §4, §6
- [task-T031.md](task-T031.md)
- [ADR-002-streamable-http-transport.md](../decisions/ADR-002-streamable-http-transport.md)

## Constraints
- Must not block the caller waiting for job completion.
- Tool output must include deterministic per-item acceptance/rejection status.
- Validate all user-provided job specs server-side (type, required fields, limits).
- Enforce permission model (planning-tier tool; orchestrator role only).
- Queue overload must be surfaced as partial or full rejection, never panic.
- Publish lifecycle events through existing job manager/event bus.

## Expected outputs
- Tool implementation in orchestrator tool registry:
  - `dispatch_concurrent_jobs` accepts array of job requests
  - returns stable response with `job_id` for accepted jobs
- Integration with `internal/jobs` manager from T031
- Input schema and validation logic for batch requests
- Unit tests for:
  - immediate return behavior
  - mixed accepted/rejected results under queue pressure
  - permission enforcement
  - invalid input handling

## Acceptance criteria
1. `dispatch_concurrent_jobs` returns synchronously with accepted/rejected entries and never waits for completion.
2. Accepted jobs are enqueued and observable via job manager state transitions.
3. Over-capacity batches return deterministic rejections (e.g., queue_full) for failed enqueues.
4. Non-orchestrator roles are denied invocation by server-side permission checks.
5. `go test ./internal/tools/... ./internal/server/...` passes with new coverage.
6. Existing T030/T031 tests remain green.

## Blocker protocol
Same as T020.
