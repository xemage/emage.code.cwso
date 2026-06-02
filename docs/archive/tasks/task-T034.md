# Task T034 — Telemetry throttling + JSON-RPC notifications

- Phase: **3 (Production)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T030, T033 · Blocks: T035
- Status: in_progress

## Objective
Implement a telemetry throttling layer that transforms brokered internal events into rate-controlled JSON-RPC notifications over SSE, preserving event usefulness while preventing client overload and noisy high-frequency floods.

## Inputs
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-1.4, §FR-5.2, §NFR-1, §NFR-5
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §6
- [task-T030.md](task-T030.md)
- [task-T033.md](task-T033.md)

## Constraints
- Must preserve MCP-compatible JSON-RPC notification envelope.
- Throttling must be deterministic and topic-aware (not random dropping).
- Critical lifecycle transitions (`failed`, `cancelled`, terminal states) must bypass aggressive throttling.
- Ensure SSE backpressure behavior remains bounded and observable.
- No secrets/tokens in emitted notification payloads.
- Existing transport/eventbus behavior must remain backward-compatible.

## Expected outputs
- New telemetry throttling module in orchestrator (topic policy + rate windows)
- Integration from memory broker/event stream to SSE notification publisher
- Configurable per-topic limits (defaults suitable for Phase 3)
- Metrics/log fields for throttled/dropped/emitted counts
- Tests for:
  - rate limiting behavior by topic
  - critical-event bypass
  - envelope correctness after throttling
  - no regression to existing SSE readiness/heartbeat semantics

## Acceptance criteria
1. High-frequency event bursts are reduced according to configured limits with deterministic policy.
2. Terminal job-state notifications are always emitted (not suppressed by throttle).
3. SSE notification envelopes remain valid JSON-RPC format.
4. Throttle counters are observable in logs/metrics for emitted vs suppressed events.
5. `go test ./internal/transport/... ./internal/memorybroker/...` passes with new coverage.
6. Existing T032 dispatch tests remain green.

## Blocker protocol
Same as T020.
