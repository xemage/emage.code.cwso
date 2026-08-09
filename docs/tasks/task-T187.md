# Task T187 — TD-03 Residual: Reduce handleBrokerSSE to ≤4 Parameters

**ID:** T187
**Owner:** backend-developer
**Status:** pending
**Priority:** P2
**Depends on:** —
**Created:** 2026-08-08
**Based on:** docs/plans/plan-TD-remediation-v1.md (Condition 2 from tech-lead CONDITIONAL_PASS gate 2026-08-08)

## Objective

`handleBrokerSSE` at `orchestrator/internal/transport/http.go:459` has 5 positional
parameters (`w, r, log, broker, filter`), violating the ≤4-parameter project standard.
Wrap its non-HTTP dependencies into an unexported `brokerSSEDeps` struct and update the
sole call site in `handleSSE`.

## Inputs

- `orchestrator/internal/transport/http.go`

## Constraints

- Edit only `orchestrator/internal/transport/http.go`.
- `brokerSSEDeps` must be unexported.
- Do not change `handleBrokerSSE` runtime behavior.

## Steps

1. Add struct after the existing `sseHandlerDeps` definition:
```go
// brokerSSEDeps groups the dependencies for the broker-backed SSE path.
type brokerSSEDeps struct {
	log    *logging.Logger
	broker *memorybroker.Broker
	filter RecordFilter
}
```
2. Change `handleBrokerSSE` signature to:
```go
func handleBrokerSSE(w http.ResponseWriter, r *http.Request, deps brokerSSEDeps) {
```
3. Replace all `log`, `broker`, `filter` references inside `handleBrokerSSE` with `deps.log`, `deps.broker`, `deps.filter`.
4. Update the call site in `handleSSE`:
```go
handleBrokerSSE(w, r, brokerSSEDeps{log: deps.log, broker: deps.broker, filter: filter})
```
5. Save file.

## Verification

```bash
cd /home/emage/Code/emage/CWSO/orchestrator
go build ./internal/transport/...
go test -race -count=1 ./internal/transport/...
```
Expected: both commands exit 0.

## Acceptance Criteria

1. `brokerSSEDeps` struct exists and is unexported.
2. `handleBrokerSSE` has exactly 3 parameters.
3. All internal field references updated.
4. Build and race tests pass.

## Blocker Protocol

Report: blocker type, severity, exact failing step, and proposed fix.