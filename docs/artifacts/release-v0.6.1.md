# Artifact: release-v0.6.1

## Metadata
- Producer: orchestrator
- Created: 2026-08-08
- Based on: docs/artifacts/release-v0.6.0.md, docs/plans/plan-TD-remediation-v1.md, T180–T189
- develop tip: 540b0fe
- Prior GA tag: v0.6.0

## Latest release: v0.6.1

## Release intent

v0.6.1 is a patch release focused on **technical debt remediation** and **test quality improvements**. All code changes are refactoring and testing improvements with no changes to runtime behavior or API contracts. The Operator Dashboard (`/dashboard`, `/dashboard/status`) from v0.6.0 is unchanged. All existing MCP tools and auth flows work identically.

## Install

```bash
# Docker Compose (recommended)
export CWSO_DASHBOARD_TOKEN=<your-operator-token>
docker compose -f deploy/docker-compose.yml up --pull always

# Container registry (individual images)
docker pull registry.gitlab.com/emage/cwso/orchestrator:v0.6.1
docker pull registry.gitlab.com/emage/cwso/git-shadow:v0.6.1
docker pull registry.gitlab.com/emage/cwso/merge-engine:v0.6.1
docker pull registry.gitlab.com/emage/cwso/rollout:v0.6.1
```

## Highlights

### Code quality improvements

**Reduced function complexity** (TD-01 through TD-03, T180–T184):
- Extracted `writeBrokerSSEFrame()`, `brokerSSETelemetryDefer()`, `writeSSEFrame()` SSE helpers
- `handleBrokerSSE()` reduced from 5 to 3 parameters via `brokerSSEDeps` struct
- `handleSSE()`, `handlePOST()`, and `publishSampleEvents()` all reduced to ≤3 parameters
- Introduced `HTTPHandlerConfig` struct (5 fields) for cleaner `RunHTTP()` and `newHTTPHandler()` signatures
- All functions now comply with project standard: ≤50 lines, ≤4 parameters

**Race-free broker shutdown** (TD-07, T181):
- Replaced racy `select` guard in `Broker.Close()` with `sync.Once`
- Eliminates possibility of concurrent close and goroutine races on shutdown

**SSE connection pooling** (TD-09, T185):
- Implemented zero-count eviction in `sseConnectionStore.release()`
- Prevents stale connection leaks when clients disconnect mid-transfer

### Test quality fixes

**Fixed flaky tests** (TD-10, TD-11, T188–T189):
- `TestBrokerSSETelemetryLogOnClose`: Replaced `os.Stderr` redirect + `io.Pipe()` with `logging.NewWithWriter()` buffer injection for race-safe log capture
- Eliminated racy `time.Sleep()` by using `httptest.Server.Close()` synchronization point
- `TestRetentionEvictionOldestFirst`: Added `waitForMaxSeq()` helper to wait for ring-buffer eviction completion before asserting
- All tests now pass under `go test -race -count=5`

**New test helper** (TD-04, T186):
- `logging.NewWithWriter(levelStr string, w io.Writer)` enables deterministic test log capture without OS-level redirection
- Supports both clean-close and dropped-events scenarios for broker SSE telemetry

### What's changed

| Area | Items | Details |
|------|-------|---------|
| Go code | 9 functions | Reduced parameter counts, extracted helpers (T180–T185, T187) |
| Tests | 3 helpers | Race fixes, eviction testing, log capture (T186, T188–T189) |
| Logging | 1 API | `NewWithWriter()` for test-owned buffers (TD-10 support) |
| Documentation | 11 tasks + 1 plan | Task briefs T180–T189, technical debt remediation plan |
| Maintenance | 1 file | Removed `TECHNICAL-DEBT.md` (all 11 items resolved) |

### Breaking changes

None. All changes are internal refactoring. Runtime behavior, API contracts, and CLI signatures are unchanged.

### Migration guide

No action required. Drop-in replacement for v0.6.0.

### Testing

All internal packages tested with `go test -race`:
- `orchestrator/internal/transport`
- `orchestrator/internal/memorybroker`
- `orchestrator/internal/logging`
- `orchestrator/internal/jobs`

Flaky tests (TD-10, TD-11) now stable and passing consistently.

## Known issues

None. See [SECURITY.md](../../SECURITY.md) for security considerations.
