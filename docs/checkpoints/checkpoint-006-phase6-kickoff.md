# Checkpoint 006 — Phase 6 Kickoff (Hardware-Aware Dispatch, Go control plane)

## Phase summary
Phase 6 (Hardware Abstraction & Real Backends, Feature A — Heterogeneous Hardware
Dispatcher) is underway. The Go control-plane half landed on
`feature/T086-hardware-aware-dispatch`: a deterministic workload profiler, a new
orchestrator-only `dispatch_hardware_aware_job` MCP tool, its JSON schema, server wiring
behind a feature flag, and a shadow provider catalog routed through the existing
`PolicyEngineV2`. The tool is fire-and-forget and returns `job_id` +
`assigned_hardware_profile` immediately. All Go packages build and pass `go vet` and
`go test ./... -race` in the `golang:1.23-alpine` container. Live backend execution
(Rust `cwso-hal` adapters) is not yet implemented, so job bodies run as
context-respecting no-ops in shadow mode. Acceptance criteria for the Go-side critical
path (T085, T086) are met; T087/T088 are partially complete pending the Rust HAL.

## Completed tasks (this phase)
| ID | Title | Owner | Outcome |
|----|-------|-------|---------|
| T080 | Phase 6 requirements + benchmark targets | product-owner | Captured in blueprint + plan |
| T081 | HAL design (`InferenceBackend` + `dispatch.provider/v2`) | solution-architect | Designed in blueprint Feature A |
| T085 | Profiling layer (`ProfileTask` / `WorkloadProfile`) | backend-developer | Implemented + unit-tested |
| T086 | `dispatch_hardware_aware_job` MCP tool + schema | backend-developer | Implemented + unit-tested |

## Open / carried over
| ID | Title | Owner | Status | Notes |
|----|-------|-------|--------|-------|
| T082 | Rust `cwso-hal` crate + CPU adapter | backend-developer | pending | Critical path for live execution |
| T083 | GPU adapter (vLLM/TensorRT-LLM) | backend-developer | pending | Depends on T082 |
| T084 | LPU adapter (Groq-style) | backend-developer | pending | Depends on T082 |
| T087 | Wire policy engine to live adapters | backend-developer | in_progress | Shadow mode done; blocked on T082 |
| T088 | Integration + reliability QA | qa-engineer | in_progress | Go unit/race green; benchmarks pending adapters |
| T089 | Tech-Lead + Security gate | tech-lead / security-engineer | pending | After T088 |

## Key decisions
- Implement the Go control plane first (profiler + MCP tool + policy wiring) in **shadow
  mode** so routing, fallback, telemetry, and tests are validated before the Rust HAL
  exists. CPU baseline remains the terminal-safe fallback.
- New surface ships **off by default** behind `CWSO_HHD_HARDWARE_AWARE_DISPATCH_ENABLED`
  (consistent with existing HHD feature-flag discipline).
- Routing authority stays server-side: `hardware_target_hint` is advisory only; the
  deterministic `PolicyEngineV2` makes the selection.

## Artifacts produced
- `orchestrator/internal/dispatch/profiler.go` (+ test)
- `orchestrator/internal/tools/dispatch_hardware_aware_tools.go` (+ test)
- `schemas/dispatch_hardware_aware_job.json`
- `orchestrator/internal/server/server.go` (shadow provider seed + tool registration)
- `orchestrator/internal/config/config.go` (`HHDHardwareAwareDispatch` flag)
- `docs/tasks/active-tasks.md`, `task-T085.md`, `task-T086.md`, `task-T087.md`

## Blockers (active)
| ID | Type | Severity | Owner | Reported | Status |
|----|------|----------|-------|----------|--------|
| T087-B1 | dependency | major | backend-developer | 2026-06-02 | Live execution blocked on Rust `cwso-hal` (T082-T084) |

## Token usage
| Phase | Budget | Spent | % |
|-------|--------|-------|---|
| Implementation (Phase 6 partial) | 120k | ~unmetered (local) | — |

## Next steps
- Phase: 6 (continue).
- Tasks: **T082** (Rust `cwso-hal` crate + `InferenceBackend` trait + CPU adapter) is the
  next critical-path item, then T083/T084 adapters, then close T087/T088.
- Inputs to delegate forward: this checkpoint, `cwso-nextgen-blueprint-v1.md` (Feature A),
  `task-T087.md`.

## Compression note
This checkpoint is the canonical handoff for continuing Phase 6. The next agent receives
**only**: this checkpoint + its task brief + referenced artifact versions.
