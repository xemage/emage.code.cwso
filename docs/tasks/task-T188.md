# Task T188 — TD-10 Fix SSE Telemetry Test Stderr-Capture Race

**ID:** T188
**Owner:** qa-engineer
**Status:** pending
**Priority:** P2
**Depends on:** —
**Created:** 2026-08-08
**Based on:** TECHNICAL-DEBT.md TD-10

## Objective

Replace the `os.Pipe()` + `os.Stderr` redirect in `TestBrokerSSETelemetryLogOnClose` with
`logging.NewWithWriter` injection so each subtest writes to its own `bytes.Buffer` and is
immune to parallel-test stderr noise.

## Inputs

- `orchestrator/internal/transport/http_sse_telemetry_test.go`
- `orchestrator/internal/logging/logger.go` — `NewWithWriter` already exists

## Constraints

- Edit only `http_sse_telemetry_test.go`.
- Do not change test assertions — only the log-capture mechanism.
- Both subtests must continue to use a real `httptest.Server` (not a direct helper call).

## Steps

1. Open `orchestrator/internal/transport/http_sse_telemetry_test.go`.
2. In `clean_close_logs_info` subtest:
   a. Remove the `os.Pipe()` / `os.Stderr` redirect block.
   b. Add `var logBuf bytes.Buffer` before server creation.
   c. Replace `logging.New("debug")` with `logging.NewWithWriter("debug", &logBuf)`.
   d. After closing `resp.Body` add `time.Sleep(100*time.Millisecond)` then read from `logBuf.String()`.
3. In `dropped_events_logs_warn` subtest:
   a. Remove the `os.Pipe()` / `os.Stderr` redirect block.
   b. Add `var logBuf bytes.Buffer` before server creation.
   c. Replace `logging.New("debug")` with `logging.NewWithWriter("debug", &logBuf)`.
   d. After closing `resp.Body` add `time.Sleep(100*time.Millisecond)` then read from `logBuf.String()`.
4. Remove any now-unused `r`, `w`, `oldStderr` variables and `io.Copy` calls.
5. Save file.

## Verification

```bash
cd /home/emage/Code/emage/CWSO/orchestrator
go test -race -count=5 -run TestBrokerSSETelemetryLogOnClose ./internal/transport/...
go test -race -count=1 ./internal/transport/...
```
Expected: all runs pass without flakiness.

## Acceptance Criteria

1. `os.Pipe()` and `os.Stderr` redirect removed from both subtests.
2. `logging.NewWithWriter` used for log capture.
3. Test passes reliably under `-race -count=5`.
4. Full transport package tests pass.

## Blocker Protocol

Report: type, severity, exact failure, proposed fix.