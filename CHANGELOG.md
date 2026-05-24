# Changelog

All notable changes to this project are documented in this file.

## v0.2.0-rc1 - 2026-05-24

### Phase 5 Hardening Closure
- Closed all security hardening follow-ups from Phase 5 conditional pass:
  - T073: Wasm module integrity verification (SHA-256 pin + trusted path)
  - T074: Telemetry minimization/redaction policy (request ID and anomaly notes)
  - T075: eBPF latency semantics hardening (explicit advisory signaling)
- Updated dispatch telemetry and anomaly contracts to reduce false precision and
  sensitive-field exposure while preserving deterministic fallback behavior.

### Operations and Documentation
- Expanded hardware-aware operator guidance in README with:
  - mandatory Wasm integrity controls (`CWSO_HHD_WASM_SCORING_MODULE_SHA256`,
    `CWSO_HHD_WASM_SCORING_TRUSTED_DIR`)
  - telemetry redaction controls (`CWSO_HHD_TELEMETRY_*`)
  - explicit advisory interpretation for `ebpf-hook` latency fields.
- Added release-candidate readiness artifact for v0.2.0-rc1.

### CI / Gates
- Release-candidate validation reached green pipeline on `develop` after the
  final hardening changes (`2548879153`).
- No open active tasks remain in Phase 5 scope.

## v0.1.1 - 2026-05-22

### Release Blockers Closed
- Closed all tracked post-v0.1.0 release blockers from the Phase 4 conditional pass:
  - T054: merge-engine unit-test CI gate requirement
  - T055: `merge_inputs` schema/runtime alignment
  - T056: ADR-006 reconciliation for node-level conflict-detail scope
  - T057: e2e policy-path validation for sidecar reason mapping
- Reconciled task board state to reflect blocker completion and current non-blocking deferrals.

### Documentation
- Updated [README.md](README.md) with a clearer "What CWSO is" overview and a
  practical "How to use CWSO" section covering startup, auth, MCP invocation,
  and validation commands.
- Added [release-v0.1.1 artifact](docs/artifacts/release-v0.1.1.md) with scope,
  validation, and release readiness summary.

### CI / Gates
- Release-ready baseline confirmed on `develop` with green lint/build/test/e2e
  pipeline status prior to release packaging.

## v0.1.0 - 2026-05-16

### Added
- Phase 1 foundation (T001-T011): requirements and architecture baselines, security baseline, Go orchestrator MCP server core, baseline filesystem tools, Streamable HTTP transport skeleton, and HS256 + Origin controls.
- Phase 2 shadow workspace + AST (T020, T022, T026, T028, T029): Rust `cwso-git-shadow` sidecar, UDS shadow client/tools, end-to-end integration harness, and PoC debt remediation pass.
- Phase 3 transport + concurrency (T030-T038): full-duplex SSE transport, async job runner pool, concurrent dispatch tool, event-sourced memory broker, telemetry throttling, and completed tech-lead/security gates.
- Phase 4 sandbox + merge pipeline (T040-T050): Docker/gVisor/Firecracker runner path, sandbox tier router, Rust merge engine, AST semantic merge flow, conflict-matrix escalation, and matrix-aware swarm e2e suite.

### Security
- Security gate T051 re-audit passed after remediation completion (see checkpoint-020).
- T058 hardened sidecar IPC socket permissions and Linux peer authorization.
- T059 added baseline HTTP security headers in transport middleware.
- T060 enforced `application/json` Content-Type for `POST /mcp`.
- T061 removed RS256 ambiguity by constraining current build/runtime to HS256.

### Testing and Validation
- Phase 1 review gate: PASS (checkpoint-001).
- Phase 2 integration validation: PASS for sidecar + shadow workspace + AST flows (checkpoint-002).
- Phase 3 tech-lead and security gates: PASS (checkpoint-008).
- Phase 4 quality gate: CONDITIONAL_PASS with tracked follow-up items (checkpoint-018).
- Security re-audit gate: PASS (checkpoint-020), with evidence:
  - `cargo test -p cwso-git-shadow -p cwso-merge-engine` (Rust sidecars): PASS.
  - `go test ./internal/config ./internal/transport` (orchestrator): PASS.

### Notes / Known Residual Risk
- Non-Linux peer-credential fallback remains permissive; acceptable for current Linux deployment scope, but must be revisited if portability scope expands.
- HSTS effectiveness depends on HTTPS termination configuration in deployment.
- T050 follow-up conditions remain tracked as open work for post-v0.1.0 hardening/alignment: T054, T055, T056, T057.