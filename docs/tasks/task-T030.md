# Task T030 — Streamable HTTP full-duplex SSE

- Phase: **3 (Production)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T029 · Blocks: T031, T034
- Status: in_progress

## Objective
Promote the SSE endpoint from heartbeat-only to a full server→client notification stream. After this task, server-side events (job lifecycle, AST index updates, telemetry) can be pushed unidirectionally to MCP clients without the LLM polling.

## Inputs
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-1.4, §FR-5
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §6
- [ADR-002](../decisions/ADR-002-streamable-http-transport.md)
- MCP spec 2025-03-26 (Streamable HTTP transport)

## Constraints
- Must not break existing POST `/mcp` behavior (Phase 1 acceptance must still pass).
- SSE event format strictly per MCP spec: each event is a JSON-RPC notification envelope.
- Per-connection backpressure: drop low-priority events if client falls behind by > 1 MiB.
- Connection auth identical to POST: JWT Bearer required; Origin allow-listed.
- Heartbeats every 15s as keep-alive; documented `event: ready` on connect.
- New package: `orchestrator/internal/eventbus/` exposing `Publish(topic, payload)` and a per-session `Subscribe()`.

## Expected outputs
- `orchestrator/internal/eventbus/bus.go` (in-memory pub/sub, per-subscriber bounded channel)
- Refactor of [orchestrator/internal/transport/http.go](../../orchestrator/internal/transport/http.go) `handleSSE` to subscribe and stream
- New `notifications/log` and `notifications/job-state` event topics
- Tests: connect SSE, publish event, assert client receives within 100 ms; backpressure test asserts drop counter
- Integration test using a Go HTTP client that opens SSE while POSTing tool calls

## Acceptance criteria
1. SSE client receives a published `notifications/job-state` event end-to-end p95 < 100 ms (NFR-1).
2. Two concurrent SSE subscribers each receive every published event (broadcast semantics).
3. A slow client triggers backpressure and increments a `dropped_events` metric (visible in logs); fast client is unaffected.
4. SSE stream survives 5-minute idle period thanks to heartbeats; client receives at least 19 heartbeats.
5. `go test ./internal/eventbus/... -race` and `./internal/transport/...` PASS.
6. Phase 1 e2e smoke test still passes unchanged.

## Blocker protocol
Same as T020.
