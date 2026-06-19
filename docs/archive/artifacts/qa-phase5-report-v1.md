# QA Phase 5 Report v1

Date: 2026-05-23  
Owner: qa-engineer  
Task: T070

## Scope and prerequisites

Scope:
- Integrated dispatch behavior validation across baseline mode, HHD policy v2, sparse/quantized assist, SSM assist, and Wasm score-adjuster fallback.
- Event-monitor fallback path validation for eBPF-preferred and userspace fallback behavior.
- Reliability validation focused on deterministic routing, fallback correctness, and no-regression repeat-run behavior.

Prerequisites:
- Phase 5 implementation tasks complete: T066, T067, T068, T069.
- Go toolchain available for orchestrator package tests.
- Deterministic local test environment with no runtime flag overrides beyond test fixtures.

## Test matrix

| Area | Scenario | Evidence test(s) | Result |
|---|---|---|---|
| Baseline mode | Policy engine disabled keeps baseline provider and baseline policy envelope | `TestPolicyEngineV2FeatureDisabledKeepsBaselinePath` | PASS |
| HHD policy v2 | Deterministic selection for identical inputs and stable fallback order | `TestPolicyEngineV2DeterministicSelectionForIdenticalInputs` | PASS |
| Sparse/quantized assist | Disabled preserves baseline behavior, enabled adjusts selection, quality guardrail auto-disables with baseline fallback | `TestPolicyEngineV2SparseQuantizedDisabledPreservesBaselineSelection`, `TestPolicyEngineV2SparseQuantizedEnabledAdjustsDecisionPath`, `TestPolicyEngineV2SparseQuantizedQualityBreachAutoDisablesPath` | PASS |
| SSM assist | Disabled preserves baseline behavior, enabled adjusts long-context route, invalid and out-of-range signals force safe fallback | `TestPolicyEngineV2SSMAssistDisabledPreservesBaselineSelection`, `TestPolicyEngineV2SSMAssistEnabledAdjustsLongContextSelection`, `TestPolicyEngineV2SSMAssistGuardrailFallbackOnInvalidSignal`, `TestPolicyEngineV2SSMAssistGuardrailFallbackOnOutOfThresholdSignal` | PASS |
| Wasm scoring fallback | Score adjuster failure fault-injection returns deterministic safe fallback reason and stable decision | `TestPolicyEngineV2PluginFailureFallsBackSafely`, `TestPolicyEngineV2FaultInjectedScoreAdjusterRemainsDeterministicAcrossRepeats` | PASS |
| Mixed backend + forced fallback | Combined backend candidate set (baseline + sparse + SSM + standard GPU) walks deterministic fallback chain to baseline | `TestPolicyEngineV2MixedBackendForcedFallbackWalksDeterministicChain` | PASS |
| Event-monitor fallback paths | Userspace fallback path, eBPF path, eBPF-unavailable fallback path | `TestDecisionAnomalyMonitorFallbackPath`, `TestDecisionAnomalyMonitorEBPFPreferredUsesHookWhenAvailable`, `TestDecisionAnomalyMonitorEBPFPreferredFallsBackWhenUnavailable`, `TestDecisionEmitterEmitsAnomalyEventsWhenMonitorEnabled` | PASS |

## Reliability KPI evidence and verdict

KPI 1: Determinism
- Evidence:
  - `TestPolicyEngineV2DeterministicSelectionForIdenticalInputs`
  - `TestPolicyEngineV2FaultInjectedScoreAdjusterRemainsDeterministicAcrossRepeats` (100 repeated selections under injected scorer fault)
- Verdict: PASS. Identical inputs produce identical decision envelopes, including under scorer-failure injection.

KPI 2: Fallback correctness
- Evidence:
  - `TestPolicyEngineV2FallbackToBaselineOnUnavailableAndFailure`
  - `TestPolicyEngineV2MixedBackendForcedFallbackWalksDeterministicChain`
  - `TestPolicyEngineV2PluginFailureFallsBackSafely`
  - `TestDecisionAnomalyMonitorEBPFPreferredFallsBackWhenUnavailable`
- Verdict: PASS. Fallback transitions are deterministic, reason codes are normalized, and baseline terminal fallback remains intact.

KPI 3: No-regression indicators (repeat runs)
- Evidence:
  - Repeated test execution for impacted packages completed with pass status.
  - New repeat-run determinism test (100 iterations) passed without variance.
- Verdict: PASS. No dispatch reliability regressions detected in impacted package test suites.

## Failure/fallback trace evidence references to tests

- Dispatch fallback on primary provider failure: `TestPolicyEngineV2FallbackToBaselineOnUnavailableAndFailure`.
- Quality guardrail forced baseline fallback: `TestPolicyEngineV2SparseQuantizedQualityBreachAutoDisablesPath`.
- SSM invalid-signal forced fallback: `TestPolicyEngineV2SSMAssistGuardrailFallbackOnInvalidSignal`.
- SSM out-of-threshold forced fallback: `TestPolicyEngineV2SSMAssistGuardrailFallbackOnOutOfThresholdSignal`.
- Wasm scorer failure fallback: `TestPolicyEngineV2PluginFailureFallsBackSafely` and `TestPolicyEngineV2FaultInjectedScoreAdjusterRemainsDeterministicAcrossRepeats`.
- Event-monitor fallback to userspace when eBPF unavailable: `TestDecisionAnomalyMonitorEBPFPreferredFallsBackWhenUnavailable`.

## Final verdict for T070

PASS.

Acceptance criteria status:
1. Regression suite passes for baseline mode and enabled feature paths: PASS.
2. Reliability KPIs equal to or better than pre-phase baseline (determinism, fallback correctness, no-regression indicators): PASS.
3. Failure and fallback behavior documented with trace evidence: PASS.
