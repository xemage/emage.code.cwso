# Task Brief: T035 Phase 3 Integration Tests

**Task ID:** T035  
**Phase:** 3 (Development Integration & Validation)  
**Owner:** qa-engineer  
**Priority:** P0 (critical path)  
**Status:** pending (awaiting T034 merge)  
**Created:** 2026-05-14

---

## Objective

Implement comprehensive integration tests for Phase 3 core components: Streamable HTTP + SSE + Job Manager + Memory Broker + Telemetry Throttling. Validate end-to-end signal flow from MCP dispatch through job execution to SSE notification delivery.

## Context

**Current State:**
- Phase 3 implementation nearly complete: T030–T034 merged or in final CI
- Core components implemented but not yet validated together in integration
- Next gates: Tech Lead (T036) and Security (T037) require evidence of integration correctness

**Why This Task:**
- Integration tests are a prerequisite for release gates
- They document expected Phase 3 behavior and validate component contracts
- They reduce risk of hidden integration defects slipping to release

## Inputs

### Artifacts
- `docs/plans/plan-T035-phase3-integration-tests.md` — detailed plan
- `orchestrator/internal/transport/http_test.go` — SSE test patterns
- `orchestrator/internal/jobs/manager_test.go` — job lifecycle patterns
- `orchestrator/internal/memorybroker/broker_test.go` — broker behavior patterns
- `orchestrator/internal/eventbus/bus_test.go` — event publication patterns

### Codebase
- `orchestrator/internal/transport/http.go` — SSE endpoints, ready event, heartbeat
- `orchestrator/internal/jobs/manager.go` — job enqueue, state machine, notifications
- `orchestrator/internal/memorybroker/broker.go` — subscribe, record ingestion, throttling
- `orchestrator/internal/tools/dispatch_tools.go` — dispatch_concurrent_jobs MCP tool
- `orchestrator/internal/server/server.go` — orchestrator wiring

### Configuration
- `orchestrator/internal/config/config.go` — test-friendly config defaults
- `.gitlab-ci.yml` — CI pipeline setup (no changes needed for T035)

## Scope

### Test Functions (4 total)

1. **`TestMultiSubscriberSSEFanout`**
   - Opens two independent SSE `/mcp` GET connections
   - Publishes a sample event via eventbus
   - Verifies both subscribers receive identical JSON-RPC notification
   - Validates per-subscriber backpressure tracking
   - Duration: ~500ms, tight assertion, no job dispatch

2. **`TestJobDispatchNotificationFlow`**
   - Opens one SSE subscriber
   - Sends POST `/mcp` with valid `dispatch_concurrent_jobs` request (1 job, short timeout)
   - Job queued → running → completed, each state published to `notifications/job-state`
   - Verifies subscriber receives all three state-change notifications in order
   - No throttling in this test (job-state terminal events bypass suppression anyway)
   - Duration: ~1–2 seconds depending on worker pool

3. **`TestBrokerBackedSSEIntegration`**
   - Orchestrator configured with memory broker
   - Opens one SSE subscriber via broker subscription path
   - Publishes sample log + job-state events to broker
   - Verifies throttling applied: log events suppressed within window, terminal job-state passes through
   - Logs emitted/suppressed counter on subscriber disconnect
   - Duration: ~1 second

4. **`TestEndToEndOrchestratorSignalPath`**
   - Full orchestrator spin-up: config, broker, eventbus, transport, jobs manager, registry
   - Two concurrent SSE subscribers open
   - Four concurrent jobs dispatched via POST + dispatch tool
   - All jobs complete (deterministically, since worker pool defaults are small)
   - Memory broker accumulates all telemetry (job-state + logs)
   - Each subscriber receives independent JSON-RPC stream with correct throttling
   - Metrics logged on disconnect
   - Duration: ~3–5 seconds

### Coverage

- **Happy path:** all components working correctly
- **Backpressure:** subscriber drops tracked when buffering full
- **Throttling:** topic-aware window and terminal state bypass working
- **Ordering:** broker preserves event sequence
- **Concurrency:** no race conditions under load

### NOT in Scope

- External service integration (Rust shadow service not tested here)
- Complex MCP method dispatch logic (dispatch tool already unit-tested in T032)
- Advanced job orchestration features (covered in Phase 4 tasks)
- PoC-specific debt items (those have their own tracking)

## Constraints

- **Language:** Go 1.23 (same as orchestrator)
- **Framework:** `testing` package + existing test patterns (no new dependencies)
- **Environment:** in-memory only; no Docker, Kubernetes, or external services
- **Token budget:** ≤30k tokens (focused scope, no refactors)
- **Compatibility:** tests must not modify core implementation files

## Expected Output

### Deliverables

1. **`orchestrator/internal/integration/integration_test.go`** (new file)
   - Package: `integration_test`
   - Four test functions: `TestMultiSubscriber...`, `TestJobDispatch...`, `TestBroker...`, `TestEndToEnd...`
   - Each test fully self-contained (no shared state)
   - Uses helper functions for setup (defined in same file or imported from test utils)

2. **`orchestrator/internal/integration/integration.go`** (new file, optional)
   - Helper functions if needed: `newIntegrationServer()`, `connectSSE()`, `parseSSEFrame()`, etc.
   - Reuse existing patterns from `http_test.go` where possible

3. **Updated docs**
   - `docs/tasks/active-tasks.md` → mark T035 done
   - `docs/tasks/completed-tasks.md` → add T035 entry
   - `docs/checkpoints/checkpoint-007-phase3-t034-complete.md` → document T034 merge + T035 readiness

### Acceptance Criteria

- [ ] All four integration tests written and pass locally
- [ ] `go test ./internal/integration/... -race` passes without warnings
- [ ] Full `go test ./...` suite passes (no regressions)
- [ ] Each test clearly documents expected behavior via comments
- [ ] Tests are deterministic (no flake on repeated runs)
- [ ] No blocker issues identified during test execution

## Validation

**Local Validation (before commit):**
```bash
cd orchestrator
gofmt -l ./internal/integration/...
go test ./internal/integration/... -race -v
go test ./...
```

**CI Validation (on MR):**
- GitLab pipeline runs same suite
- `go:lint` and `go:test` jobs must pass
- `e2e:phase2` must pass (ensures end-to-end harness still works)

## Acceptance Self-Check (to include in MR)

When delegating, ensure agent reports:
- [ ] File structure created (`orchestrator/internal/integration/...`)
- [ ] Four test functions implemented with clear documentation
- [ ] Local `go test ./internal/integration/... -race` passes
- [ ] Full orchestrator `go test ./...` passes
- [ ] No temporary debug code or skip tags left behind
- [ ] Code follows existing test patterns and conventions

## Risks & Handling

| Risk | Severity | Handling |
|------|----------|----------|
| Job timing variance | Medium | Use context cancellation + channel sync, not sleep; generous timeouts |
| Broker retention overflow | Low | Test config sets large retention; verify no events dropped |
| SSE frame parsing breaks | Low | Use existing `readSSEFrame` helper + robustness assertions |
| CI runner slowness | Medium | 10-second timeouts per test should be ample; document if flake occurs |
| Race condition in subscriber cleanup | Low | Ensure proper goroutine closure in disconnect deferred functions |

## Blockers

**This task is blocked by:**
- T034 must be merged to develop before implementation starts

**This task unblocks:**
- T036 (Tech Lead gate)

## Related Tasks

- **T030:** Streamable HTTP + SSE (underlying transport)
- **T031:** Job Manager (job dispatch & lifecycle)
- **T033:** Memory Broker (telemetry storage)
- **T034:** Telemetry Throttling (final component)
- **T036:** Tech Lead gate (consumed T035 results)

## Notes

- Tests should be optimized for clarity over minimal LOC; document assumptions
- Each test should be independent (no shared setup/teardown state)
- Reuse helper patterns from existing `*_test.go` files where possible
- If new test utilities are needed, create `orchestrator/internal/testutil/` package
- Document edge cases discovered during implementation (e.g., broker retention limits)

---

**Created:** 2026-05-14  
**Based on:** docs/plans/plan-T035-phase3-integration-tests.md
