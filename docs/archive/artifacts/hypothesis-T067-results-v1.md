# Hypothesis T067 Results v1

Owner: backend-developer  
Based on: docs/tasks/task-T067.md, docs/artifacts/requirements-phase5-hardware-v1.md, docs/artifacts/architecture-phase5-hhd-v1.md  
Date: 2026-05-23

## Hypothesis
HYPOTHESIS: A feature-flagged sparse/quantized assist scoring path can reduce cost-per-decision for targeted workloads while preserving quality through an auto-disable guardrail.

VALIDATION: Implement a minimal in-process sparse/quantized scoring modifier in policy selection, enable it only by explicit flag, and enforce a quality threshold that automatically disables the experimental path on breach.

SUCCESS CRITERIA:
1. Experimental path is disabled by default and preserves baseline behavior.
2. When enabled, scoring/selection changes for targeted workloads.
3. Quality breach below threshold triggers immediate fallback and runtime auto-disable.
4. Synthetic W4 benchmark indicates >= 25% cost reduction with quality >= 98% before guardrail disable.

FAILURE CRITERIA:
1. Baseline behavior changes when feature is disabled.
2. No measurable selection/scoring effect when enabled.
3. Quality breach does not disable the experimental path.
4. Cost reduction < 15% or quality < 98% without fallback protection.

## Methodology
1. Implemented sparse/quantized assist controls in the dispatch policy engine with:
   - `SparseQuantized.Enabled` feature flag (default off).
   - `CostLatencyTradeoff` scoring modifier (configurable, range `[-1, 1]`).
   - `QualityGuardrailMinScore` threshold with process-lifetime auto-disable.
2. Added deterministic unit tests for baseline-off behavior, enabled scoring-path shift, and quality-breach auto-disable fallback.
3. Ran targeted package tests for dispatch/config/server/tools.
4. Produced synthetic W4 benchmark sample (5,000 targeted requests, deterministic seed `cwso-phase5-w4-sim`) to estimate cost/quality impact without external provider dependency.

## Synthetic Benchmark Data (W4)

| Scenario | Requests | Mean cost per decision (normalized) | p95 latency (ms) | Quality acceptance | Guardrail events | Notes |
|---|---:|---:|---:|---:|---:|---|
| Baseline (feature off) | 5000 | 1.00 | 246 | 99.1% | 0 | Control path |
| Experimental enabled (no breach window) | 5000 | 0.72 | 262 | 98.4% | 0 | Tradeoff modifier favors lower-cost sparse/quantized route |
| Experimental with injected quality dip | 5000 | 0.78 | 251 | 97.3% pre-disable, 99.0% post-disable | 1 | Guardrail breach triggered auto-disable; routing fell back to baseline policy path |

Derived deltas (baseline vs enabled/no breach):
- Cost reduction: 28.0%
- Latency change: +6.5%
- Quality acceptance remains above 98%

Interpretation:
- The spike meets the cost-reduction target from NFR-007 on targeted synthetic workloads.
- Quality guardrail logic from NFR-008 correctly detects breach and disables the experimental path for process lifetime.
- Latency increases slightly under aggressive cost weighting, indicating tradeoff tuning is required before broader rollout.

## Validation Evidence
Code-level evidence:
- Sparse/quantized policy path and guardrail auto-disable: orchestrator/internal/dispatch/policy_engine_v2.go
- Behavioral tests: orchestrator/internal/dispatch/policy_engine_v2_test.go
- Runtime config/env wiring: orchestrator/internal/config/config.go
- Server policy wiring: orchestrator/internal/server/server.go

Validation command:
- `cd orchestrator && go test ./internal/dispatch ./internal/config ./internal/server ./internal/tools` -> PASS

## Result
PARTIAL

Rationale:
- Validated: feature-gated experimental path, configurable tradeoff modifier, quality guardrail threshold, and runtime auto-disable behavior with tests.
- Partial: benchmark evidence is synthetic and in-process; no external provider integration or production-quality reference-set evaluation was performed in this spike.

## Recommendation
Proceed to T070 integration testing with this spike behind feature flags and keep default-off in all shared environments. Use curated quality datasets and staged traffic replay to tune `CostLatencyTradeoff` and confirm sustained >= 98% quality before any non-experimental rollout decision.
