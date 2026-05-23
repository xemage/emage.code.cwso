# Artifact: hardening-wasm-integrity-v1

## Metadata
- Producer agent: backend-developer
- Created: 2026-05-23
- Based on: docs/tasks/task-T073.md, docs/artifacts/security-phase5-audit-v1.md

## Objective
Close security finding F-071-01 by enforcing Wasm module integrity and trusted-path controls for policy engine scoring plugins.

## Controls implemented

### 1) Mandatory SHA-256 pin when enabled
When `CWSO_HHD_WASM_SCORING_ENABLED=true`, configuration now requires:
- `CWSO_HHD_WASM_SCORING_MODULE_SHA256` (64-char lowercase hex digest)

Loader behavior:
- Reads module bytes.
- Computes SHA-256 hash.
- Fails module initialization if expected and actual hashes differ.

### 2) Mandatory trusted module directory when enabled
When `CWSO_HHD_WASM_SCORING_ENABLED=true`, configuration now requires:
- `CWSO_HHD_WASM_SCORING_TRUSTED_DIR`

Loader behavior:
- Resolves symlinks for trusted directory and module path.
- Verifies module path is contained within trusted directory.
- Fails initialization when module path escapes trusted directory.

## Files changed
- `orchestrator/internal/config/config.go`
- `orchestrator/internal/config/config_test.go`
- `orchestrator/internal/server/server.go`
- `orchestrator/internal/dispatch/wasm_scoring_plugin.go`
- `orchestrator/internal/dispatch/wasm_scoring_plugin_test.go`

## Test evidence
- Runtime tests:
  - trusted-directory escape rejection
  - SHA-256 mismatch rejection
- Config validation tests:
  - enabled mode rejects missing SHA-256
  - enabled mode rejects invalid SHA-256
  - enabled mode rejects missing trusted dir

## Residual risk and follow-up
- This hardening enforces hash pinning and path trust, but does not introduce signature-based provenance. If stronger provenance is needed, evaluate signature verification as a future enhancement.

## Verdict
F-071-01 mitigated for current deployment model (hash pin + trusted path enforced, fail-closed behavior active).
