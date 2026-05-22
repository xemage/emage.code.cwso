# Task T069 — Event-driven monitoring spike (eBPF + fallback)

- Phase: **5 (R&D Spike)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T064 · Blocks: T070
- Status: done (2026-05-22)

## Objective
Prototype event-driven monitoring using eBPF where permitted, plus portable user-space fallback hooks for restricted environments.

## Inputs
- [docs/tasks/task-T064.md](task-T064.md)
- Security baseline and runtime constraints from existing artifacts

## Constraints
- eBPF path must be optional and capability-gated.
- No privileged-only path may become mandatory for core operation.

## Expected outputs
- `docs/artifacts/hypothesis-T069-results-v1.md`
- Instrumentation prototype and fallback implementation notes.

## Acceptance criteria
1. Both eBPF and fallback modes are demonstrated.
2. Detection-latency metrics are compared to current baseline.
3. Privilege and deployment requirements are clearly documented.

## Blocker protocol
If environment denies required capabilities, report blocker type `dependency` and proceed with fallback-only validation.

## Completion notes (2026-05-22)
- Implemented event-driven anomaly monitor spike in `orchestrator/internal/dispatch/anomaly_monitor.go` and wired it into decision telemetry via `orchestrator/internal/dispatch/telemetry.go`.
- Added portable fallback userspace hook path as default-safe signal source (`fallback-userspace`) with no privilege requirement.
- Added optional eBPF-preferred signal path (`ebpf-hook`) gated by capability checks; automatic fallback to userspace occurs when unavailable.
- Added configuration and server wiring:
	- `CWSO_HHD_EVENT_MONITOR_ENABLED`
	- `CWSO_HHD_EVENT_MONITOR_EBPF_ENABLED`
	- `CWSO_HHD_EVENT_MONITOR_LATENCY_THRESHOLD_MS`
- Added tests for fallback path, eBPF-available path, and eBPF-unavailable fallback behavior:
	- `orchestrator/internal/dispatch/anomaly_monitor_test.go`
	- `orchestrator/internal/dispatch/telemetry_test.go`
	- `orchestrator/internal/config/config_test.go`
- Produced hypothesis evidence artifact: `docs/artifacts/hypothesis-T069-results-v1.md`.

### Validation run
- `cd orchestrator && go test ./internal/dispatch ./internal/config ./internal/server ./internal/tools` -> PASS
