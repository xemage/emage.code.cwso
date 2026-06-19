# Phase 7 Integration QA Report — Features B + C

**Target:** Phase 7 sparse micro-agents (Feature B) + spiking AST monitors (Feature C)  
**Based on:** `task-T122.md`, `task-T123.md`, `task-T124.md`, ADR-008, `wasm-sparse-agent-design-v1.md`  
**Date:** 2026-06-04

## Hypothesis

The Phase 7 stack meets its integration budgets when wired end-to-end:

1. Sparse agent cold start `< 10 ms` (p95, warm pinned slice)
2. AST spike pipeline `0% idle CPU` (purely event-driven, no idle emissions)
3. Quality-floor breach escalates to dense GPU via HAL (`quality_guardrail_autodisable`)

## Reliability Budgets

| Budget | Target | Guard |
|--------|--------|-------|
| Cold start (sidecar, warm slice) | p95 `< 10 ms` | `cwso-sparse::agent::tests::cold_start_warm_p95_under_budget` |
| Sparse control-plane overhead | median `≤ 10 ms` | `tools.TestSparseAgentControlPlaneOverheadBudget` |
| AST spike idle emissions | 0 without writes | `dispatch.TestASTSpikePipelineZeroIdleEmissions` |
| Guardrail escalation (server) | HAL infer, no sparse agent | `server.TestSparseQualityFloorEscalationServerIntegration` |

## Integration Coverage

- **Go tools:** cold-start forwarding + control-plane budget (`sparse_agent_reliability_test.go`).
- **Go dispatch:** idle spike pipeline guard (`ast_spike_idle_test.go`).
- **Go server:** sparse tool registration + guardrail → HAL path (`server_sparse_integration_test.go`).
- **Rust sidecar:** warm-slice cold-start p95 on wasmtime instantiation (`agent.rs`).

## Verdict

**PASS** — CI pipeline #2575895437 green at `e964e1b`; merged via MR !33 (`eb4aa45`).

## Notes

- Cold-start SLO is measured on the sidecar path with a shared wasmtime engine and warm-mmap'd
  `.cwsl` slice; Go tests budget orchestrator overhead separately against an in-memory fake sidecar.
- Idle-CPU semantics are architectural (no background polling in `ASTWriteSpikeMonitor` /
  `ASTSpikeFilter`); the idle test guards against regressions that introduce timer-driven emissions.
