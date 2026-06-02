# Task T067 — Sparse and quantized assist spike

- Phase: **5 (R&D Spike)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T065 · Blocks: T070
- Status: done (2026-05-23)

## Objective
Evaluate sparse/quantized assist modules for targeted decision paths and measure cost-latency-quality tradeoffs.

## Inputs
- [docs/tasks/task-T065.md](task-T065.md)
- Benchmark definitions from T062

## Constraints
- Must remain behind feature flags.
- Quality guardrails must auto-disable degraded paths.

## Expected outputs
- `docs/artifacts/hypothesis-T067-results-v1.md`
- Experimental adapter implementation or integration notes.

## Acceptance criteria
1. Report includes baseline vs experiment metrics (latency, cost, quality).
2. Auto-disable condition is defined and validated in at least one failure case.
3. Recommendation states validated/invalidated with evidence.

## Blocker protocol
If model/provider access is unavailable, report blocker type `external` and include a synthetic benchmark fallback.

## Completion notes (2026-05-23)
- Implemented a sparse/quantized assist spike in `orchestrator/internal/dispatch/policy_engine_v2.go` behind explicit feature configuration (`SparseQuantized.Enabled`, default `false`).
- Added a configurable cost-latency tradeoff scoring modifier (`CostLatencyTradeoff`) applied only to tagged sparse/quantized providers and targeted workloads.
- Added quality guardrail threshold (`QualityGuardrailMinScore`) with process-lifetime auto-disable and immediate baseline fallback on breach (`quality_guardrail_autodisable`).
- Added test evidence in `orchestrator/internal/dispatch/policy_engine_v2_test.go` for:
	- feature-disabled baseline-preserving behavior,
	- feature-enabled scoring/decision path adjustment,
	- quality breach auto-disable fallback and persistent disable behavior.
- Wired runtime config and validation in `orchestrator/internal/config/config.go` and `orchestrator/internal/server/server.go`.
- Produced synthetic benchmark artifact `docs/artifacts/hypothesis-T067-results-v1.md` with baseline vs experiment methodology, metrics, and recommendation.
- Validation run:
	- `cd orchestrator && go test ./internal/dispatch ./internal/config ./internal/server ./internal/tools`
	- Result: pass.
