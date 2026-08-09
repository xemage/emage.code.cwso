# Task T185 — TD-09 Evict Zero-Count SSE Connection Entries

**ID:** T185
**Owner:** backend-developer
**Status:** pending
**Priority:** P2
**Depends on:** T184
**Created:** 2026-08-08
**Based on:** docs/plans/plan-TD-remediation-v1.md

## Objective

Prevent unbounded growth in `sseConns` by deleting map entries when per-IP connection count returns to zero.

## Inputs

- `orchestrator/internal/transport/http.go`
- `orchestrator/internal/transport/http_test.go`

## Constraints

- Keep acquire limit behavior unchanged.
- Add a focused unit test for eviction.
- Keep implementation thread-safe.

## Steps

1. Open `orchestrator/internal/transport/http.go`.
2. Replace `release(ip string)` with:
```go
func (s *sseConnectionStore) release(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conns[ip] <= 1 {
		delete(s.conns, ip)
		return
	}
	s.conns[ip]--
}
```
3. Open `orchestrator/internal/transport/http_test.go`.
4. Add `TestSSEConnectionStoreEviction` that:
   - acquires `1.2.3.4`
   - releases `1.2.3.4`
   - asserts map key is absent
5. Save files.

## Verification

Run:
```bash
cd /home/emage/Code/emage/CWSO/orchestrator
go test ./internal/transport/... -run TestSSEConnectionStoreEviction
go test ./internal/transport/...
```
Expected:
- focused test passes.
- full transport package tests pass.

## Acceptance Criteria

1. `release()` deletes zero-count entries.
2. New eviction test exists and passes.
3. No regressions in transport tests.

## Blocker Protocol

If blocked, report race/failure details and include proposed lock-safe alternative.