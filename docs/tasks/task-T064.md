# Task T064 — Capability discovery and telemetry fabric

- Phase: **5 (Implementation)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T063 · Blocks: T065, T069
- Status: done (2026-05-22)

## Objective
Implement capability discovery and telemetry collection needed for real-time dispatch decisions.

## Inputs
- [docs/tasks/task-T063.md](task-T063.md)
- `docs/artifacts/architecture-phase5-hhd-v1.md` (from T063)

## Constraints
- Telemetry overhead must remain bounded and configurable.
- Capability snapshots must support deterministic replay for debugging.

## Expected outputs
- Capability registry implementation and schema updates.
- `docs/artifacts/capability-telemetry-spec-v1.md`

## Acceptance criteria
1. Registry captures backend capabilities and health status.
2. Telemetry emits signals required by policy engine scoring.
3. Replayable traces are available for at least one benchmark scenario.

## Blocker protocol
If required platform counters are not available, report blocker type `dependency` and implement fallback metric adapters.

## Completion notes (2026-05-22)
- Implemented capability registry MVP in `orchestrator/internal/dispatch/capability_registry.go` with:
	- validated provider capability schema fields
	- deterministic snapshot ordering by `provider_id`
	- monotonic `epoch` for replay alignment
	- stale-health projection (`healthy` -> `degraded`) based on configurable TTL
- Implemented telemetry fabric MVP in `orchestrator/internal/dispatch/telemetry.go` with:
	- capability snapshot topic: `dispatch/capabilities`
	- decision event topic: `dispatch/decision`
	- decision envelope fields needed by policy and audit traces
- Wired server/tool integration (feature-gated, default-off):
	- config flags in `orchestrator/internal/config/config.go`
	- server wiring in `orchestrator/internal/server/server.go`
	- dispatch tool emission hook in `orchestrator/internal/tools/dispatch_tools.go`
- Added/updated tests:
	- `orchestrator/internal/dispatch/capability_registry_test.go`
	- `orchestrator/internal/dispatch/telemetry_test.go`
	- `orchestrator/internal/tools/dispatch_tools_test.go`
	- `orchestrator/internal/config/config_test.go`
- Added artifact:
	- `docs/artifacts/capability-telemetry-spec-v1.md`

### Validation run
- `cd orchestrator && go test ./internal/dispatch ./internal/tools ./internal/config ./internal/server` -> PASS
