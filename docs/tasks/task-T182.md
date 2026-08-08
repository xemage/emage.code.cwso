# Task T182 — TD-02 Introduce HTTPHandlerConfig For RunHTTP/newHTTPHandler

**ID:** T182
**Owner:** backend-developer
**Status:** pending
**Priority:** P2
**Depends on:** —
**Created:** 2026-08-08
**Based on:** docs/plans/plan-TD-remediation-v1.md

## Objective

Reduce positional parameters for `RunHTTP` and `newHTTPHandler` by introducing a required dependency struct `HTTPHandlerConfig`.

## Inputs

- `orchestrator/internal/transport/http.go`
- All call sites of `RunHTTP` in `orchestrator/**/*.go`

## Constraints

- Keep runtime behavior unchanged.
- Keep optional behavior (`HTTPOption`) intact.
- Update all call sites in one change set so build remains green.

## Steps

1. Find all call sites:
```bash
cd /home/emage/Code/emage/CWSO
rg "RunHTTP\(" orchestrator -g '*.go'
```
2. In `orchestrator/internal/transport/http.go`, add:
```go
type HTTPHandlerConfig struct {
	Log             *logging.Logger
	Bus             *eventbus.Bus
	Broker          *memorybroker.Broker
	SamplePublisher eventPublisher
	Handler         func(ctx context.Context, sess *Session, raw []byte) ([]byte, error)
}
```
3. Change signatures:
   - `RunHTTP(ctx context.Context, cfg *config.Config, hcfg HTTPHandlerConfig, opts ...HTTPOption) error`
   - `newHTTPHandler(ctx context.Context, cfg *config.Config, hcfg HTTPHandlerConfig, opts ...HTTPOption) http.Handler`
4. Replace old local variables (`log`, `bus`, `broker`, `samplePublisher`, `h`) with `hcfg.*` fields.
5. Update each call site to pass `HTTPHandlerConfig{...}`.
6. Save all modified files.

## Verification

Run:
```bash
cd /home/emage/Code/emage/CWSO/orchestrator
go build ./...
go test ./internal/transport/...
```
Expected:
- commands exit 0.

## Acceptance Criteria

1. New `HTTPHandlerConfig` exists.
2. `RunHTTP` and `newHTTPHandler` use the new signature.
3. All call sites updated.
4. `go build ./...` and transport tests pass.

## Blocker Protocol

If blocked, report exact caller(s) not migrated and compiler error text.