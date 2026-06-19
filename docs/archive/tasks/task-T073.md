# Task T073 — Wasm module integrity verification hardening

- Phase: **5 (Security Hardening)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T071 · Blocks: T074, T075
- Status: done (2026-05-23)

## Objective
Implement mandatory integrity and path trust controls for Wasm scoring modules used by policy engine v2.

## Inputs
- [docs/tasks/task-T071.md](task-T071.md)
- [docs/artifacts/security-phase5-audit-v1.md](../artifacts/security-phase5-audit-v1.md)
- [docs/artifacts/wasm-scoring-runtime-ops-v1.md](../artifacts/wasm-scoring-runtime-ops-v1.md)

## Constraints
- Enforce fail-closed integrity checks before module compilation.
- Preserve baseline fallback behavior when plugin initialization fails.
- Keep host-call allowlist deny-by-default.

## Expected outputs
- Code changes in Wasm scorer + config/server wiring.
- Updated tests for integrity and trusted-path enforcement.
- `docs/artifacts/hardening-wasm-integrity-v1.md`

## Acceptance criteria
1. Wasm scoring enabled mode requires configured module SHA-256 pin.
2. Wasm module path is restricted to configured trusted directory.
3. Runtime rejects module load on hash mismatch with explicit error.
4. Tests cover hash mismatch and trusted-path escape cases.

## Blocker protocol
If hardening breaks compatibility in CI or deploy environments, report blocker type `technical` with migration notes.

## Completion notes (2026-05-23)
- Added required config controls for enabled Wasm scoring:
  - `CWSO_HHD_WASM_SCORING_MODULE_SHA256`
  - `CWSO_HHD_WASM_SCORING_TRUSTED_DIR`
- Added runtime checks in `orchestrator/internal/dispatch/wasm_scoring_plugin.go`:
  - trusted directory containment validation (with symlink-resolved paths)
  - SHA-256 integrity verification before Wasm compile/instantiate
- Wired new controls from config to runtime in `orchestrator/internal/server/server.go`.
- Added tests for:
  - module outside trusted directory rejection
  - module SHA-256 mismatch rejection
  - config validation for missing/invalid SHA-256 and missing trusted dir.

### Evidence
- `docs/artifacts/hardening-wasm-integrity-v1.md`
