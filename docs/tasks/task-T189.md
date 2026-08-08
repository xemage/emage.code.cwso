# Task T189 — TD-11 Investigate and Fix TestRetentionEvictionOldestFirst Flakiness

**ID:** T189
**Owner:** qa-engineer
**Status:** pending
**Priority:** P2
**Depends on:** —
**Created:** 2026-08-08
**Based on:** TECHNICAL-DEBT.md TD-11

## Objective

Identify why `TestRetentionEvictionOldestFirst` in `broker_test.go` returns `[1,2,3]`
instead of expected `[3,4,5]` under parallel test load, and fix the test or the broker
eviction logic accordingly.

## Inputs

- `orchestrator/internal/memorybroker/broker_test.go`
- `orchestrator/internal/memorybroker/broker.go`

## Investigation Steps

1. Read the full `TestRetentionEvictionOldestFirst` test in `broker_test.go`.
2. Determine whether the test makes a timing or ordering assumption that is not guaranteed.
3. Run with `-v` and `-count=20` in isolation to reproduce the failure:
   ```bash
   cd /home/emage/Code/emage/CWSO/orchestrator
   go test -count=20 -run TestRetentionEvictionOldestFirst ./internal/memorybroker/...
   ```
4. If the test is correct and the broker is wrong: fix the broker eviction logic.
5. If the test has a bad assumption: fix the test to be deterministic.

## Constraints

- Do not change public API of the Broker.
- Fix must make the test pass reliably under `-race -count=20` without `time.Sleep` hacks.

## Verification

```bash
cd /home/emage/Code/emage/CWSO/orchestrator
go test -race -count=20 -run TestRetentionEvictionOldestFirst ./internal/memorybroker/...
go test -race -count=1 ./internal/memorybroker/...
```
Expected: zero failures across all runs.

## Acceptance Criteria

1. Root cause identified and documented in execution notes.
2. Test passes under `-race -count=20` consistently.
3. Full memorybroker package tests pass.

## Blocker Protocol

Report: type, severity, exact failure reproduction steps, proposed fix.