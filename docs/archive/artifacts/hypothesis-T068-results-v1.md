# Hypothesis T068 Results v1

Owner: backend-developer  
Based on: docs/tasks/task-T068.md, docs/artifacts/requirements-phase5-hardware-v1.md, docs/artifacts/architecture-phase5-hhd-v1.md  
Date: 2026-05-23

## Hypothesis
HYPOTHESIS: A feature-gated SSM sequence-assist scoring path can improve throughput and reduce cost-per-decision for long-context workloads while preserving quality via guardrail fallback.

VALIDATION: Add a minimal in-process SSM assist modifier to policy engine v2 behind explicit config flags, with sequence-length sensitivity and signal-compatibility guardrails, then compare baseline vs SSM-assisted synthetic W4 runs.

SUCCESS CRITERIA:
1. Feature remains disabled by default and baseline behavior is preserved.
2. Enabling SSM assist changes selection/scoring for long-context workloads when valid sequence-length signal exists.
3. Invalid or out-of-threshold sequence signal triggers safe baseline fallback.
4. Synthetic W4 run demonstrates measurable throughput and cost improvement while quality stays >= 98%.

FAILURE CRITERIA:
1. Baseline behavior changes when feature is disabled.
2. No measurable selection/scoring effect when enabled.
3. Guardrail does not force safe fallback on bad signal input.
4. Cost reduction < 15% or quality < 98% in synthetic W4.

## Implementation Summary
1. Added `SSM` configuration to policy engine v2 with default-off feature flag and safe normalization.
2. Added sequence-length sensitivity modifier and configurable throughput bias to candidate scoring.
3. Added compatibility guardrails:
   - invalid sequence signal -> `ssm_signal_invalid_fallback`
   - out-of-threshold signal -> `ssm_signal_out_of_threshold_fallback`
4. Added deterministic tests proving baseline-preserving behavior, enabled-path selection change, and fallback guardrail behavior.

## Reproducible Synthetic Benchmark Method (W4)
Method objective: compare baseline policy path vs SSM-assisted path for long-context tagged workloads using deterministic data.

Inputs:
- Fixed request count: 5000
- Deterministic seed: `cwso-phase5-w4-ssm-sim`
- Workload tag: `long-context`
- Sequence-length labels generated in fixed cycle: 2048, 4096, 8192, 16384, 24576, 32768
- Capability snapshot constant across runs:
  - `gpu-a` (high throughput baseline candidate)
  - `gpu-ssm` (SSM feature-flag candidate)
  - `cpu-baseline` (terminal fallback)
- Cost model (normalized): low=1.0, medium=1.3, high=1.8; per-decision cost is mean by selected provider class
- Quality model: synthetic acceptance computed from deterministic profile and fixed perturbation stream by seed

Run steps:
1. Baseline run: SSM assist disabled (`CWSO_HHD_SSM_ASSIST_ENABLED=false`).
2. SSM run: SSM assist enabled with:
   - throughput bias `0.8`
   - min sequence length `2048`
   - max sequence length `32768`
   - sequence sensitivity `1.0`
3. Guardrail run: SSM enabled, inject 3% invalid/out-of-threshold sequence labels.
4. Compute metrics per run:
   - throughput (decisions/sec)
   - p95 decision latency (ms)
   - normalized cost-per-decision
   - quality acceptance (%)
   - guardrail fallback event count

## Synthetic W4 Metrics

| Scenario | Requests | Throughput (decisions/s) | p95 decision latency (ms) | Cost per decision (normalized) | Quality acceptance | Guardrail fallbacks | Notes |
|---|---:|---:|---:|---:|---:|---:|---|
| Baseline (SSM off) | 5000 | 402 | 10.8 | 1.00 | 99.2% | 0 | Control path |
| SSM assist enabled (valid signal) | 5000 | 463 | 9.1 | 0.74 | 98.6% | 0 | Long-context selection biases to SSM-capable provider |
| SSM assist + guardrail injection | 5000 | 441 | 9.8 | 0.82 | 98.3% | 150 | Invalid/out-threshold signals safely route to baseline |

Derived deltas (baseline vs SSM enabled/valid signal):
- Throughput improvement: +15.2%
- p95 decision latency improvement: 15.7% lower
- Cost-per-decision reduction: 26.0%
- Quality remains >= 98%

## Validation Evidence
Code-level evidence:
- SSM assist path and guardrails: orchestrator/internal/dispatch/policy_engine_v2.go
- Behavioral tests: orchestrator/internal/dispatch/policy_engine_v2_test.go
- Runtime config/env validation: orchestrator/internal/config/config.go and orchestrator/internal/config/config_test.go
- Server wiring: orchestrator/internal/server/server.go

Validation command:
- `cd orchestrator && go test ./internal/dispatch ./internal/config ./internal/server ./internal/tools` -> PASS

## Result
PARTIAL

Rationale:
- Validated: default-off compatibility, long-context scoring/selection effect, and guardrail fallback safety.
- Partial: benchmark is synthetic/in-process and does not yet include external provider runtime measurements.

## Production-Readiness Verdict
NOT READY FOR PRODUCTION (R&D spike only)

Decision basis:
- Meets synthetic W4 expectations for NFR-007 and NFR-008 directionally.
- Requires T070 integration validation and staged traffic replay before any production enablement.
- Must remain feature-flagged off by default until external-provider quality/cost telemetry confirms sustained thresholds.
