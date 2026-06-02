# Artifact: hardening-telemetry-redaction-v1

## Metadata
- Producer agent: backend-developer
- Created: 2026-05-23
- Based on: docs/tasks/task-T074.md, docs/artifacts/security-phase5-audit-v1.md

## Objective
Close security finding F-071-02 by enforcing configurable telemetry minimization/redaction controls for dispatch decision and anomaly telemetry.

## Policy matrix

| Field | Topic | Classification | Mode `allow` | Mode `hash` | Mode `drop` |
|---|---|---|---|---|---|
| `request_id` | `dispatch/decision` | sensitive correlation identifier | emitted as-is | deterministic SHA-256 short hash (`sha256:<24hex>`) | omitted from payload |
| `notes` | `dispatch/anomaly` | environment-derived diagnostic text | emitted as-is | n/a | omitted from payload |

## Controls implemented

### 1) Configurable request-id minimization
- Added `CWSO_HHD_TELEMETRY_REQUEST_ID_MODE` with allowed values: `allow`, `hash`, `drop`.
- Redaction is active only when `CWSO_HHD_TELEMETRY_REDACTION_ENABLED=true`.
- Hash mode is deterministic and supports optional salting via `CWSO_HHD_TELEMETRY_REDACTION_SALT`.

### 2) Configurable anomaly-note minimization
- Added `CWSO_HHD_TELEMETRY_ANOMALY_NOTES_MODE` with allowed values: `allow`, `drop`.
- Drop mode removes environment-derived fallback notes from anomaly payloads.

### 3) Fail-fast configuration validation
- Startup validation rejects unsupported telemetry mode values.
- Invalid mode values fail config load before server initialization.

## Files changed
- `orchestrator/internal/dispatch/telemetry.go`
- `orchestrator/internal/dispatch/anomaly_monitor.go`
- `orchestrator/internal/dispatch/telemetry_test.go`
- `orchestrator/internal/dispatch/anomaly_monitor_test.go`
- `orchestrator/internal/config/config.go`
- `orchestrator/internal/config/config_test.go`
- `orchestrator/internal/server/server.go`

## Test evidence
- `go test ./internal/dispatch ./internal/config ./internal/server ./internal/tools`
- Added test coverage for:
  - request-id redaction/hash behavior at decision publish time
  - anomaly notes dropping under redaction policy
  - config validation for unsupported telemetry redaction mode values

## Residual risk and follow-up
- Hash mode remains pseudonymous, not anonymous. Operators should use a non-empty redaction salt in shared environments.
- If stronger unlinkability is required, consider rotating salt with bounded retention windows.

## Verdict
F-071-02 mitigated for current deployment model (configurable deterministic redaction with test coverage and startup validation).
