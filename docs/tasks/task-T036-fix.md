# Task T036-fix — Tech Lead Gate Remediation

**Status:** in_progress  
**Owner:** backend-developer  
**Priority:** P0  
**Depends on:** T036 FAIL verdict  
**Created:** 2026-05-15

---

## Objective

Fix the HIGH and MEDIUM findings from the T036 Phase 3 Tech Lead Gate to unblock re-review and gate approval.

## Required Fixes

### F-01 (HIGH) — Goroutine leak in `Subscription.pump()` — MUST FIX

**Files:** `orchestrator/internal/eventbus/bus.go`, `orchestrator/internal/memorybroker/broker.go`

**Problem:** `pump()` goroutine dequeues a record from the buffered internal channel, then blocks on an unbuffered `s.out <-` send. If the SSE handler has already exited (called `sub.Close()`), no reader exists and the goroutine blocks permanently.

**Fix:** Make the `s.out` send select on the subscription's done channel:
```go
select {
case s.out <- msg:
case <-s.done:
    return
}
```
`s.done` must be a `chan struct{}` closed by `sub.Close()`. Review both `eventbus` and `memorybroker` subscription implementations.

### F-03 (MEDIUM) — `rateLimiterStore` unbounded memory growth — MUST FIX

**File:** `orchestrator/internal/transport/http.go`

**Problem:** `getLimiter()` never evicts entries from the IP→limiter map.

**Fix:** Add a TTL-based eviction loop (e.g. clean entries not accessed in the last 5 minutes). Use a `lastSeen time.Time` field per entry, and a background goroutine that runs every minute cleaning stale entries. The goroutine must be tied to the handler's context for clean shutdown.

### F-11 (MEDIUM) — Flaky integration test timeout — MUST FIX

**File:** `orchestrator/internal/integration/integration_test.go`

**Problem:** `TestIntegrationEndToEndSignalPath` calls `readSSEFrame(t, reader1, 100*time.Millisecond)` inside a deadline loop. `readSSEFrame` calls `t.Fatalf` on timeout, which immediately terminates the test. The outer deadline loop is never retried.

**Fix:** Change `readSSEFrame` to return a `(data string, ok bool)` pair and return `("", false)` on timeout instead of calling `t.Fatalf`. Update `TestIntegrationEndToEndSignalPath` and `TestIntegrationBrokerBackedSSEThrottling` to use the new non-fatal signature where timeout is expected to be benign.

### F-02, F-04, F-05, F-06 (MEDIUM/LOW) — Function length and parameter count — TRACK AS DEBT

These are style findings. They do not need to be fixed before gate re-approval — add `<!-- POC-DEBT -->` comments and register in `TECHNICAL-DEBT.md`.

### F-08, F-09, F-10 (LOW) — Best-effort gaps — TRACK AS DEBT

Register in `TECHNICAL-DEBT.md` only.

## Expected Outputs

1. Fixed `orchestrator/internal/eventbus/bus.go` — pump goroutine is leak-free
2. Fixed `orchestrator/internal/memorybroker/broker.go` — pump goroutine is leak-free
3. Fixed `orchestrator/internal/transport/http.go` — rateLimiterStore has TTL eviction
4. Fixed `orchestrator/internal/integration/integration_test.go` — readSSEFrame non-fatal variant
5. `TECHNICAL-DEBT.md` updated with F-02, F-04, F-05, F-06, F-08, F-09, F-10 entries
6. `go test ./... -race` passes
7. All 4 integration tests still pass

## Acceptance Criteria

- [ ] `go test ./... -race` green in golang:1.23 container
- [ ] No goroutine leak detectable via `-count=100` stress run of integration tests
- [ ] `TECHNICAL-DEBT.md` has all LOW/MEDIUM-style findings registered
- [ ] No secrets introduced
- [ ] Commit message: `fix(transport): T036-fix goroutine leak, rate-limiter eviction, integration test robustness`
