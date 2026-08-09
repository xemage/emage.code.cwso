# Task T184 — TD-01 Extract SSE Helpers And Reduce Function Length

**ID:** T184
**Owner:** backend-developer
**Status:** pending
**Priority:** P2
**Depends on:** T183
**Created:** 2026-08-08
**Based on:** docs/plans/plan-TD-remediation-v1.md

## Objective

Refactor `handleBrokerSSE` and `handleSSE` to <= 50 lines each by extracting loop-body and deferred-closure logic into helpers while preserving behavior.

## Inputs

- `orchestrator/internal/transport/http.go` after T183 merge
- `orchestrator/internal/transport/http_test.go`

## Constraints

- Preserve message ordering and heartbeat behavior.
- Preserve filter and throttle semantics.
- No functional changes, refactor only.

## Steps

1. Open `orchestrator/internal/transport/http.go`.
2. Extract from `handleBrokerSSE`:
   - `writeBrokerSSEFrame(w http.ResponseWriter, flusher http.Flusher, log *logging.Logger, rec memorybroker.Record, filter RecordFilter, throttle *telemetryThrottle) bool`
   - `brokerSSETelemetryDefer(log *logging.Logger, sub *memorybroker.Subscription, throttle *telemetryThrottle) func()`
3. Replace inline logic in the `case rec := <-sub.Messages()` block with a call to `writeBrokerSSEFrame`.
4. Replace inline deferred telemetry closure with `defer brokerSSETelemetryDefer(log, sub, throttle)()`.
5. Extract from `handleSSE`:
   - `writeSSEFrame(w http.ResponseWriter, flusher http.Flusher, log *logging.Logger, msg eventbus.Message, filter RecordFilter) bool`
6. Replace inline message write block in `handleSSE` with `writeSSEFrame` call.
7. Ensure both target functions are <= 50 lines.
8. Save file.

## Verification

Run:
```bash
cd /home/emage/Code/emage/CWSO/orchestrator
go test ./internal/transport/...
```
Then verify line counts quickly:
```bash
cd /home/emage/Code/emage/CWSO
awk '/^func handleSSE\(|^func handleBrokerSSE\(/, /^}/ {print NR ":" $0}' orchestrator/internal/transport/http.go
```
Expected:
- tests pass.
- both functions are <= 50 lines.

## Acceptance Criteria

1. Both `handleSSE` and `handleBrokerSSE` are <= 50 lines.
2. New helpers exist with exact signatures above.
3. Existing transport tests pass without behavior regressions.

## Blocker Protocol

If blocked, report which extracted helper changed behavior and include failing test name.