# Task T186 — TD-04 Add Dedicated Unit Test For Broker SSE Deferred Telemetry

**ID:** T186
**Owner:** qa-engineer
**Status:** pending
**Priority:** P2
**Depends on:** T184
**Created:** 2026-08-08
**Based on:** docs/plans/plan-TD-remediation-v1.md

## Objective

Add a dedicated test for the deferred telemetry-close path in `handleBrokerSSE`, covering both no-drop and dropped-event branches.

## Inputs

- `orchestrator/internal/transport/http_test.go`
- `orchestrator/internal/transport/http.go`
- `orchestrator/internal/memorybroker/broker.go`

## Constraints

- Use real HTTP SSE server path, not a mocked direct helper call.
- Assert ready-event is observed before close.
- Keep test deterministic.

## Steps

1. Open `orchestrator/internal/transport/http_test.go`.
2. Add `TestBrokerSSETelemetryLogOnClose` with two subtests:
   - `clean_close_logs_info`
   - `dropped_events_logs_warn`
3. Use existing `newSSETestServer` helper to stand up the endpoint.
4. For each subtest:
   - open SSE GET request
   - read first event and assert it contains `event: ready`
   - trigger close path
5. For dropped-event subtest:
   - construct broker with `memorybroker.WithSubscriberQueueDepth(1)`
   - ingest enough events to trigger subscriber drop accounting
6. Capture log output and assert:
   - clean close contains `SSE telemetry stream closed` and telemetry counts
   - drop path contains `SSE telemetry stream closed` and `dropped_events`
7. Save file.

## Verification

Run:
```bash
cd /home/emage/Code/emage/CWSO/orchestrator
go test -race ./internal/transport/... -run TestBrokerSSETelemetryLogOnClose
go test ./internal/transport/...
```
Expected:
- race test passes.
- full transport tests pass.

## Acceptance Criteria

1. New test exists with two subtests (clean and drop).
2. Ready event assertion is present.
3. Log assertions cover both info/warn branches.
4. `go test -race` focused run passes.

## Blocker Protocol

If blocked, provide the flaky point (read timing/log capture), failing assertion, and deterministic fallback approach.