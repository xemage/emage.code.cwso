# Artifact: hardening-ebpf-latency-semantics-v1

## Metadata
- Producer agent: backend-developer
- Created: 2026-05-24
- Based on: docs/tasks/task-T075.md, docs/artifacts/security-phase5-audit-v1.md, docs/artifacts/hypothesis-T069-results-v1.md

## Objective
Close security finding F-071-03 by removing false precision from eBPF-path anomaly latency output and making advisory semantics explicit to operators.

## Problem statement
Previous spike behavior emitted a constant `detection_latency_ms=2` in `ebpf-hook` mode with `detection_latency_mode=estimated`.
This could be misinterpreted as measured timing fidelity.

## Final semantics contract

| Signal path | `detection_latency_mode` | `detection_latency_ms` | `detection_latency_is_advisory` | Operator interpretation |
|---|---|---|---|---|
| `ebpf-hook` | `advisory` | `0` | `true` | Value is non-authoritative placeholder; do not treat as measured latency. |
| `fallback-userspace` with parseable `emitted_at` | `measured` | measured delta | `false` | Process-local measured latency from decision emit to anomaly detection. |
| `fallback-userspace` without parseable `emitted_at` | `estimated` | `0` | `false` | Timing unavailable; estimated placeholder for compatibility. |

## Implementation summary
- Added explicit field: `detection_latency_is_advisory` to anomaly telemetry payload.
- Updated latency derivation logic for `ebpf-hook` path:
  - Removed fixed 2ms estimate.
  - Emit advisory mode + advisory marker.
- Preserved fallback path behavior and compatibility.

## Files changed
- `orchestrator/internal/dispatch/anomaly_monitor.go`
- `orchestrator/internal/dispatch/anomaly_monitor_test.go`
- `docs/tasks/task-T075.md`
- `docs/tasks/active-tasks.md`

## Test evidence
- `go test ./internal/dispatch ./internal/config ./internal/server ./internal/tools`
- Updated tests:
  - `TestDecisionAnomalyMonitorEBPFPreferredUsesHookWhenAvailable`
  - `TestDecisionAnomalyMonitorFallbackPath`
  - `TestDecisionAnomalyMonitorEBPFPreferredFallsBackWhenUnavailable`

## Residual risk and follow-up
- Advisory semantics prevent false precision but do not provide measured kernel-probe latency.
- A future eBPF probe-timestamp integration can replace advisory output with measured kernel-path values.

## Verdict
F-071-03 mitigated for current architecture via explicit advisory semantics and operator-visible contract.
