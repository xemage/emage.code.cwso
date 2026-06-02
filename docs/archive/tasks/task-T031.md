# Task T031 — Async job runner pool

- Phase: **3 (Production)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T030 · Blocks: T032, T033
- Status: in_progress

## Objective
Introduce an internal asynchronous job manager with a bounded worker pool so the orchestrator can enqueue and execute background jobs concurrently without blocking the transport thread. This is the execution substrate for `dispatch_concurrent_jobs` and event-sourced memory work in T032/T033.

## Inputs
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-5, §NFR-1
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §2, §6
- [ADR-002-streamable-http-transport.md](../decisions/ADR-002-streamable-http-transport.md)
- [task-T030.md](task-T030.md)

## Constraints
- Preserve current synchronous POST `/mcp` behavior for existing tools.
- Bounded queue and bounded worker concurrency (no unbounded goroutine fanout).
- Job lifecycle states must be explicit and immutable transitions:
  - `queued -> running -> completed|failed|cancelled`
- Support cancellation by job ID via context cancellation.
- Publish lifecycle events through the existing event bus for SSE consumers.
- All external inputs to job creation/cancellation must be validated server-side.

## Expected outputs
- New package: `orchestrator/internal/jobs/`
  - pool manager with worker count config
  - enqueue API and status lookup API
  - cancellation API
  - lifecycle event publication hooks
- Integration wiring from router/transport layer where needed for internal use (without introducing T032 public tool yet)
- Tests:
  - queue enqueue/dequeue and state transitions
  - bounded concurrency behavior
  - cancellation path
  - race-safe execution (`-race`)

## Acceptance criteria
1. Job pool runs multiple jobs concurrently up to configured worker limit and no more.
2. Jobs transition through valid lifecycle states only; invalid transitions are rejected.
3. Cancelling a running or queued job marks it `cancelled` and publishes corresponding event.
4. Under overload, queue limit behavior is deterministic (reject or backpressure by design) and tested.
5. `go test ./internal/jobs/... -race` passes.
6. Existing transport tests and T030 SSE tests remain green.

## Blocker protocol
Same as T020.
