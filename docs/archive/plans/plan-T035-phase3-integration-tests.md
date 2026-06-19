# Plan: T035 Phase 3 Integration Tests

**Created:** 2026-05-14  
**Author:** Orchestrator  
**Status:** Draft (awaiting T034 merge + approval)

## Objective

Design and implement comprehensive Phase 3 integration tests for the full SSE + Job Manager + Memory Broker + Telemetry stack, validating:
- multi-subscriber SSE fanout with per-subscriber backpressure
- job dispatch, execution, state transitions, and notifications
- broker-backed telemetry ingestion, throttling, and subscription integrity
- end-to-end signal flow from POST `/mcp` dispatch → job lifecycle → SSE telemetry delivery

## Goal

Ensure that Phase 3 core kernel components work correctly together before gates (Tech Lead, Security) review.

## Task Dependencies

- **Blockers:** T032 (dispatch tool), T034 (telemetry throttling)
- **Consumed artifacts:**
  - `docs/tasks/task-T034.md`
  - `docs/plans/plan-T034-phase3-telemetry.md`
  - `orchestrator/internal/transport/http_test.go` (existing test patterns)
  - `orchestrator/internal/jobs/manager_test.go` (job behavior patterns)
  - `orchestrator/internal/memorybroker/broker_test.go` (broker semantics)

## Scope & Approach

### Test Suites

1. **Multi-SSE-Subscriber Fanout Tests** (transport + eventbus)
   - Two independent SSE subscribers each open long-lived connections
   - Sample event published via eventbus
   - Both subscribers receive identical JSON-RPC notifications
   - Per-subscriber backpressure tracked (dropped-event accounting)

2. **Job Dispatch → Notification Flow** (jobs + dispatch + eventbus)
   - POST `/mcp` call with valid `dispatch_concurrent_jobs` request
   - Job enqueued and state transitioned: `queued` → `running` → `completed`
   - Each state change published to `notifications/job-state` topic
   - SSE subscriber receives all job-state JSON-RPC notifications in order
   - Counters validate throttling behavior (terminal states bypass)

3. **Broker-Backed SSE Integration** (broker + transport)
   - SSE client connects, receives `ready` event
   - Sample telemetry events ingested into broker
   - Throttle policy applied (log events suppressed within window, job-state terminal events pass)
   - SSE subscriber receives only non-throttled JSON-RPC notifications
   - Broker `Len()`, `Query()` operations reflect correct state
   - Subscriber disconnect logs emitted/suppressed/dropped counters

4. **End-to-End Orchestrator Signal Path** (all components together)
   - Orchestrator spins up with config
   - Two concurrent SSE subscribers connect
   - Four concurrent jobs dispatched via POST + dispatch tool
   - Jobs execute deterministically (may complete quickly or in sequence depending on worker pool)
   - Memory broker accumulates all telemetry (job-state + logs)
   - Each SSE subscriber receives independent JSON-RPC stream with correct throttling applied
   - Subscriber metrics logged correctly on disconnect

### Test Naming & Structure

```
orchestrator/internal/integration/integration_test.go

TestMultiSubscriberSSEFanout
TestJobDispatchNotificationFlow
TestBrokerBackedSSEIntegration
TestEndToEndOrchestratorSignalPath
```

### Success Criteria

1. All tests pass in containerized Go 1.23 without race condition warnings
2. Tests cover happy path + edge cases (e.g., rapid subscriber disconnect, queue pressure)
3. Test code documents expected behavior through assertions
4. No changes required to core implementation (tests validate existing behavior)

### Known Risks

- **Timing sensitivity:** job execution speed depends on worker pool sizing; tests should not assume exact ordering without proper synchronization
- **Broker retention:** if broker retention limit is very small, some telemetry may be dropped before SSE subscriber can pull it; tests must account for this
- **Flaky assertion:** subscriber timeout in case of slow CI runner; recommend generous timeout windows

## Implementation Strategy

1. Create new integration test file in `orchestrator/internal/integration/integration_test.go` (new package)
2. Implement test fixtures: `newIntegrationServer()` with config, eventbus, broker, transport
3. Add SSE subscription helper: `connectSSE()` that opens HTTP GET /mcp and parses SSE frames
4. Add HTTP POST helper: `dispatchJobs()` that sends dispatch tool requests
5. Implement four test functions matching scope above
6. Run full suite: `go test ./internal/integration/... -race`
7. Regression test against existing suite: `go test ./...`

## Acceptance Criteria

- [ ] Integration test file created with four test functions
- [ ] All tests pass locally in containerized Go 1.23 with `-race`
- [ ] Full `go test ./...` suite passes
- [ ] No new blockers or warnings
- [ ] Tests document expected Phase 3 behavior

## Artifacts Produced

- `orchestrator/internal/integration/integration_test.go`
- Updated `orchestrator/internal/integration/integration.go` (if helper package needed)
- Updated `docs/tasks/task-T035.md` with detailed brief
- Updated `docs/tasks/active-tasks.md` and `docs/tasks/completed-tasks.md`

## Timeline

- **Estimated effort:** M (medium) — 4–6 hours including test fixture setup and debugging
- **Token budget:** ≤ 30k tokens (tight focus on integration only, not refactor)
- **Phase transition:** T035 complete → T036 (Tech Lead gate) can proceed

## Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|-----------|
| Job timing variance in tests | Medium | Use context timeouts and channel-based sync, not sleep |
| Broker retention too small for test | Low | Increase retention window in test config if needed |
| SSE frame parsing breaks on new envelope format | Low | Use existing `readSSEFrame` helper from http_test.go |
| CI runner slowness causes test flake | Medium | Generous timeout windows (5–10 seconds per test) |

## Decision Log

- **ADR-001:** Keep tests in dedicated `internal/integration/` package rather than inline with transport/broker tests to isolate orchestrator-level behavior
- **ADR-002:** Use in-memory components (no Docker/external services) for fast CI execution
- **ADR-003:** Minimal job payloads (no complex MCP methods) to focus on signal flow validation

---

**Next Steps (after approval & T034 merge):**
1. Delegate T035 implementation to `@qa-engineer`
2. Validate containerized test execution
3. Reconcile task ledger
4. Prepare T036 Tech Lead gate
