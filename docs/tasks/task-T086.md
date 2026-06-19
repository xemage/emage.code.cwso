# Task T086 — `dispatch_hardware_aware_job` MCP tool + schema

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T083 (GPU adapter — deferred), T085 (profiler — done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md`, `task-T085.md`

## Objective
Expose Feature A (Heterogeneous Hardware Dispatcher) as an MCP tool that profiles a
task, routes it through the deterministic policy engine to the most efficient backend,
enqueues it asynchronously, and returns the job id + assigned hardware profile immediately.

## Inputs
- `task_description` (required), `context_size_estimate` (required), `latency_requirement` (required).
- Optional: `workload_tags`, `target_workspace_uuid`, `hardware_target_hint`, `quality_floor`.
- Existing `jobs.Manager`, `PolicyEngineV2`, capability snapshot reader, decision emitter.

## Outputs
- `orchestrator/internal/tools/dispatch_hardware_aware_tools.go` (tool + helpers).
- `orchestrator/internal/tools/dispatch_hardware_aware_tools_test.go`.
- `schemas/dispatch_hardware_aware_job.json`.
- Server wiring + `CWSO_HHD_HARDWARE_AWARE_DISPATCH_ENABLED` flag (`config.go`, `server.go`).

## Acceptance Criteria
- [x] Fire-and-forget: immediate return (<100ms) with `job_id` + `assigned_hardware_profile`.
- [x] Orchestrator-only (`AllowedRoles` = orchestrator).
- [x] Deterministic routing via `PolicyEngineV2`; CPU baseline remains terminal-safe fallback.
- [x] One deterministic fallback hop on enqueue failure; structured error codes (`queue_full`, `manager_closed`).
- [x] Input validation (empty desc, bad latency, bad UUID, quality range) returns tool errors.
- [x] Decision telemetry emitted once per dispatch.
- [x] Off by default; enabled only via feature flag. `go vet` + `-race` green.

## Blocker Protocol
Report blockers with type and severity; max 2 retries before escalation.
