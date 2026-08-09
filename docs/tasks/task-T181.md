# Task T181 — TD-07 Replace Broker Close Guard With sync.Once

**ID:** T181
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** —
**Created:** 2026-08-08
**Based on:** docs/plans/plan-TD-remediation-v1.md

## Objective

Make `Broker.Close()` race-safe by replacing the `select` close guard with `sync.Once`.

## Inputs

- `orchestrator/internal/memorybroker/broker.go`

## Constraints

- Edit only `orchestrator/internal/memorybroker/broker.go`.
- Behavior must remain idempotent across repeated `Close()` calls.
- Do not change public API names.

## Steps

1. Open `orchestrator/internal/memorybroker/broker.go`.
2. In `type Broker struct`, add field:
   - `closeOnce sync.Once`
3. Replace `func (b *Broker) Close()` body with:
```go
func (b *Broker) Close() {
	b.closeOnce.Do(func() {
		close(b.closed)
		b.closeSubscribers()
		b.wg.Wait()
	})
}
```
4. Save file.

## Verification

Run:
```bash
cd /home/emage/Code/emage/CWSO/orchestrator
go build ./internal/memorybroker/...
go test ./internal/memorybroker/...
go vet ./internal/memorybroker/...
```
Expected:
- all commands exit 0.

## Acceptance Criteria

1. `Broker` struct contains `closeOnce sync.Once`.
2. Old `select` guard is removed from `Close()`.
3. Build, test, and vet pass for memorybroker package.

## Blocker Protocol

If blocked, report: failing command output, file/line, and next best fallback patch.