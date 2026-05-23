# Task T075 — eBPF latency semantics hardening

- Phase: **5 (Security Hardening)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T073 · Blocks: —
- Status: pending

## Objective
Replace or clearly classify advisory eBPF latency semantics in anomaly monitoring output.

## Inputs
- [docs/artifacts/security-phase5-audit-v1.md](../artifacts/security-phase5-audit-v1.md)
- `orchestrator/internal/dispatch/anomaly_monitor.go`

## Constraints
- Avoid introducing false precision in security telemetry.
- Keep fallback behavior deterministic and backward compatible where feasible.

## Expected outputs
- Measured-latency implementation or explicit advisory flag semantics with tests.
- Documentation updates for operator interpretation.

## Acceptance criteria
1. eBPF latency semantics are explicit (measured or advisory-marked).
2. Behavior is validated by tests for eBPF and fallback paths.
3. Operator docs reflect the final semantics.

## Blocker protocol
If probe timestamp data is unavailable in current architecture, report blocker type `dependency` and define interim advisory contract.
