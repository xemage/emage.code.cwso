# Task T085 — Profiling Layer: tensor_tag derivation + workload mapping

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T082 (soft — implemented ahead of Rust HAL in shadow mode)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md`, `docs/plans/plan-cwso-nextgen-phase6plus.md`

## Objective
Provide a deterministic, pure-function profiler that maps an incoming task to a
"tensor tag" workload profile so the policy engine can route to the most efficient
backend without any model-side coordination.

## Inputs
- Task description, estimated context size, latency requirement (`realtime` | `batch`).
- Routing matrix from the Next-Gen blueprint (Feature A).

## Outputs
- `orchestrator/internal/dispatch/profiler.go`:
  - `WorkloadProfile{ Tags, RecommendedClass, ContextSizeEstimate, LatencyRequirement, RequestLabels }`
  - `ProfileTask(desc, contextSize, latency) WorkloadProfile`
  - `NormalizeLatencyRequirement(raw) (string, error)`
- `orchestrator/internal/dispatch/profiler_test.go` (routing matrix + determinism).

## Acceptance Criteria
- [x] Pure/deterministic: identical inputs → identical profile.
- [x] Routing matrix: large-context→SSM, realtime+small→LPU, batch+mechanical-edit→Wasm-local, else→GPU.
- [x] Large-context wins over realtime to avoid OOM on latency-optimized backends.
- [x] Negative context clamped to 0; unknown latency rejected.
- [x] `go test ./internal/dispatch/... -race` passes.

## Blocker Protocol
Report blockers with type (`technical|dependency|unclear_requirements|external`) and
severity (`critical|major|minor`); max 2 retries before escalation.
