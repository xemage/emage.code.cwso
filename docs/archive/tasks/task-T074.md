# Task T074 — Telemetry minimization/redaction policy

- Phase: **5 (Security Hardening)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T073 · Blocks: —
- Status: done (2026-05-23)

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

## Completion notes (2026-05-23)
- Added configurable telemetry redaction controls to dispatch telemetry:
	- `CWSO_HHD_TELEMETRY_REDACTION_ENABLED`
	- `CWSO_HHD_TELEMETRY_REQUEST_ID_MODE` (`allow` | `hash` | `drop`)
	- `CWSO_HHD_TELEMETRY_ANOMALY_NOTES_MODE` (`allow` | `drop`)
	- `CWSO_HHD_TELEMETRY_REDACTION_SALT` (optional request-id hash salt)
- Implemented deterministic request-id hashing and optional field dropping before decision telemetry publish.
- Implemented anomaly note minimization controls in anomaly monitor publish path.
- Added config validation tests for invalid telemetry mode values.
- Added dispatch tests for request-id redaction and anomaly notes drop behavior.

### Evidence
- `docs/artifacts/hardening-telemetry-redaction-v1.md`
