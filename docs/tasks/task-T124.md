# Task T124 — Phase 7 integration QA (Feature B + C)

**Status:** in_review  
**Owner:** qa-engineer  
**Priority:** P0  
**Depends on:** T118, T123  
**Roadmap mapping:** Feature B/C placeholder T098 → active T124  
**Based on:** `task-T122.md`, `task-T123.md`, `docs/artifacts/wasm-sparse-agent-design-v1.md`, ADR-008

## Objective

Validate Phase 7 Feature B (sparse micro-agents) and Feature C (spiking AST monitors) integration
budgets end-to-end:

- **Cold start** `< 10 ms` (p95, warm pinned slice)
- **0% idle CPU** for the AST spike pipeline (event-driven, no polling emissions)
- **Quality-floor escalation** routes to dense GPU via HAL when guardrail breaches

## Reliability Budgets (verified)

| Budget | Target | Test |
|--------|--------|------|
| Sparse agent cold start (sidecar measurement, warm slice) | p95 `< 10 ms` | `cwso-sparse`: `cold_start_warm_p95_under_budget` |
| Sparse agent control-plane overhead (tool → UDS → registry) | median `≤ 10 ms` | `TestSparseAgentControlPlaneOverheadBudget` |
| AST spike pipeline idle emissions | 0 broker records without writes | `TestASTSpikePipelineZeroIdleEmissions` |
| Quality-floor escalation (server path) | `escalated: true`, HAL `infer`, no sparse agent | `TestSparseQualityFloorEscalationServerIntegration` |

## Integration Coverage

- `internal/tools`: cold-start reporting + control-plane budget guards (`sparse_agent_reliability_test.go`).
- `internal/dispatch`: idle spike pipeline guard (`ast_spike_idle_test.go`).
- `internal/server`: sparse agent tool registration + guardrail → HAL escalation
  (`server_sparse_integration_test.go`).
- `services/cwso-sparse`: warm-slice cold-start p95 budget on wasmtime instantiation path.

## Acceptance Criteria

- [x] Cold start p95 `< 10 ms` on warm slice (Rust sidecar path).
- [x] AST spike monitor + filter emit nothing when idle (0% idle CPU semantics).
- [x] Quality-floor breach escalates via server-integrated guardrail to dense HAL backend.
- [x] `go test -race ./...`, `cargo test --release -p cwso-sparse`, gofmt, vet clean.

## Notes

- Cold-start SLO is measured on the sidecar's wasmtime instantiation path with a shared engine and
  warm-mmap'd `.cwsl` slice (ADR-008 validation). Control-plane overhead is separately budgeted at
  the Go tool layer against an in-memory fake sidecar (isolates orchestrator cost from wasm compile).
- Real deployment SLOs under load belong to deployment-time benchmarking, not unit CI.
