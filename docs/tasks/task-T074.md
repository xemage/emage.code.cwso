# Task T074 — Telemetry minimization/redaction policy

- Phase: **5 (Security Hardening)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T073 · Blocks: —
- Status: pending

## Objective
Define and enforce redaction/minimization controls for dispatch decision and anomaly telemetry payloads.

## Inputs
- [docs/artifacts/security-phase5-audit-v1.md](../artifacts/security-phase5-audit-v1.md)
- `orchestrator/internal/dispatch/telemetry.go`
- `orchestrator/internal/dispatch/anomaly_monitor.go`

## Constraints
- Preserve operational observability while reducing sensitive field exposure.
- Redaction policy must be configurable and test-covered.

## Expected outputs
- Telemetry redaction/minimization implementation + tests.
- Security hardening artifact update with policy matrix.

## Acceptance criteria
1. Sensitive telemetry fields are redacted or dropped under policy.
2. Policy behavior is deterministic and test covered.
3. No regression in anomaly detection event emission.

## Blocker protocol
If required fields are ambiguous for operators, report blocker type `unclear_requirements` with proposed field classification draft.
