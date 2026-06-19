# Hypothesis T069 Results v1

Owner: backend-developer  
Based on: docs/tasks/task-T069.md, docs/artifacts/capability-telemetry-spec-v1.md, docs/artifacts/architecture-phase5-hhd-v1.md  
Date: 2026-05-22

## Hypothesis
HYPOTHESIS: Event-driven anomaly detection can be integrated into dispatch telemetry with a portable default path and an optional eBPF path, while preserving baseline behavior when disabled.

VALIDATION: Implement a minimal real monitor in orchestrator decision telemetry, add capability-gated eBPF preference with automatic fallback, and verify behavior with targeted tests.

SUCCESS CRITERIA:
1. Default path works without elevated privileges.
2. eBPF path is optional and capability-gated.
3. Baseline behavior remains unchanged when feature is disabled.
4. Detection-latency signal can be emitted for anomaly events.

FAILURE CRITERIA:
1. Monitoring requires privileged mode by default.
2. Feature changes baseline dispatch behavior when disabled.
3. No test evidence for fallback/eBPF-gated behavior.

## Method
1. Added event-driven anomaly monitor triggered from dispatch decision events.
2. Implemented two signal paths:
   - fallback-userspace: portable hook-based path (default)
   - ebpf-hook: optional preferred path when capabilities permit
3. Added capability gate for eBPF path and automatic fallback to userspace when unavailable.
4. Added focused tests for fallback mode, eBPF available mode, and eBPF unavailable fallback mode.
5. Ran targeted Go tests for modified packages.

## Scenarios and Evidence

| Scenario | Config | Expected signal path | Observed result | Evidence |
|---|---|---|---|---|
| Default fallback mode | monitor enabled, eBPF disabled | fallback-userspace | PASS | orchestrator/internal/dispatch/anomaly_monitor_test.go (TestDecisionAnomalyMonitorFallbackPath) |
| eBPF preferred and available | monitor enabled, eBPF enabled, checker=true | ebpf-hook | PASS | orchestrator/internal/dispatch/anomaly_monitor_test.go (TestDecisionAnomalyMonitorEBPFPreferredUsesHookWhenAvailable) |
| eBPF preferred but unavailable | monitor enabled, eBPF enabled, checker=false | fallback-userspace with reason note | PASS | orchestrator/internal/dispatch/anomaly_monitor_test.go (TestDecisionAnomalyMonitorEBPFPreferredFallsBackWhenUnavailable) |
| Event integration | decision emitted via DecisionEmitter | anomaly event emitted | PASS | orchestrator/internal/dispatch/telemetry_test.go (TestDecisionEmitterEmitsAnomalyEventsWhenMonitorEnabled) |

## Detection-Latency Comparison (Spike)

The spike emits `detection_latency_ms` with measured values in fallback mode and an estimated value in eBPF mode (integration hook stub).

| Path | Source | Detection-latency result | Notes |
|---|---|---|---|
| fallback-userspace | measured from decision emitted_at to anomaly detected_at | measured in tests (non-negative, environment/runtime-dependent) | Portable and unprivileged |
| ebpf-hook | estimated constant in spike prototype | 2 ms estimate | Integration hook only; real kernel instrumentation deferred |

Interpretation:
- Userspace fallback provides immediate, portable monitoring with practical low-latency detection tied to process scheduling.
- eBPF path is represented as a capability-gated hook in this spike; measured kernel-path latency requires follow-up implementation on privileged Linux hosts.

## Privilege Requirements
- Default mode (`fallback-userspace`): no elevated privileges required.
- Optional eBPF-preferred mode (`ebpf-hook`): requires Linux plus root or equivalent capabilities (`CAP_BPF`/`CAP_PERFMON`) and bpffs availability.
- If eBPF requirements are not met, monitor auto-falls back to userspace path.

## Validation Commands
- `cd orchestrator && go test ./internal/dispatch ./internal/config ./internal/server ./internal/tools` -> PASS

## Result
PARTIAL

Rationale:
- Validated: portable default path, capability-gated eBPF preference, default-safe behavior, tests and wiring complete.
- Partial: eBPF path is an integration hook/stub with estimated latency, not a full kernel probe implementation.

## Recommendation
Go forward with fallback userspace monitoring for Phase 5 integration and reliability QA (T070). Keep eBPF mode behind capability/feature flag until a privileged host validation pass adds real probe attachment and measured kernel-path latency benchmarks.
