# Changelog

All notable changes to this project are documented in this file.

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