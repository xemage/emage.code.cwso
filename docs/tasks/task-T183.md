# Task T183 — TD-03 Reduce Internal Helper Parameter Counts

**ID:** T183
**Owner:** backend-developer
**Status:** pending
**Priority:** P2
**Depends on:** T182
**Created:** 2026-08-08
**Based on:** docs/plans/plan-TD-remediation-v1.md

## Objective

Reduce parameter counts for `handlePOST`, `handleSSE`, and `publishSampleEvents` to <= 4 by grouping dependencies into small structs.

## Inputs

- `orchestrator/internal/transport/http.go` after T182 merge

## Constraints

- Do not alter request/response semantics.
- Keep helper structs unexported.
- Keep changes scoped to `http.go` unless tests require updates.

## Steps

1. In `http.go`, add unexported structs:
   - `postHandlerDeps`
   - `sseHandlerDeps`
   - `sampleEventParams`
2. Update function signatures:
   - `handlePOST(w http.ResponseWriter, r *http.Request, deps postHandlerDeps)`
   - `handleSSE(w http.ResponseWriter, r *http.Request, deps sseHandlerDeps)`
   - `publishSampleEvents(publisher eventPublisher, log *logging.Logger, p sampleEventParams)`
3. Update internal field references to `deps.*` or `p.*`.
4. Update call sites in `newHTTPHandler`.
5. Save file.

## Verification

Run:
```bash
cd /home/emage/Code/emage/CWSO/orchestrator
go build ./internal/transport/...
go test ./internal/transport/...
```
Expected:
- build and tests pass.

## Acceptance Criteria

1. Each targeted function has <= 4 parameters.
2. `http.go` compiles with updated call sites.
3. Transport package tests pass.

## Blocker Protocol

If blocked, include the exact signature mismatch and affected call site.