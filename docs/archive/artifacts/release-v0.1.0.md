# Artifact: release-v0.1.0

## Metadata
- Producer agent: release-manager
- Task: T052
- Created: 2026-05-16
- Based on: requirements-v1.md, architecture-v1.md, checkpoint-020-phase4-t051-pass.md, completed-tasks.md

## Release Summary and Scope
v0.1.0 is the first release candidate package for CWSO PoC delivery through Phase 4 completion and security re-audit closure. Scope includes orchestrator transport + tooling foundation, Rust sidecar capabilities, async job/memory infrastructure, sandbox tier execution path, semantic merge pipeline, and conflict-matrix e2e validation.

## Included Tasks and Key Artifacts

### Foundation and architecture
- T001, T002, T003, T004
- Key artifacts: `docs/artifacts/requirements-v1.md`, `docs/artifacts/architecture-v1.md`, `docs/artifacts/security-baseline-v1.md`, `docs/decisions/ADR-001-hybrid-go-rust-split.md` through `docs/decisions/ADR-006-semantic-ast-merge.md`

### Core implementation and integration
- T005, T006, T007, T008, T009, T020, T022, T026, T029, T030, T031, T032, T033, T034, T035, T038, T040, T041, T042, T043, T044, T045, T046, T047, T048, T049
- Key artifacts: `orchestrator/internal/transport/http.go`, `orchestrator/internal/jobs/manager.go`, `orchestrator/internal/memorybroker/broker.go`, `services/cwso-git-shadow/`, `services/cwso-merge-engine/`, `scripts/phase2-integration.py`

### Validation and gate tasks
- T010, T027, T036, T037, T050, T051
- Gate evidence references: `docs/checkpoints/checkpoint-001-phase1.md`, `docs/checkpoints/checkpoint-002-phase2.md`, `docs/checkpoints/checkpoint-008-phase3-complete.md`, `docs/checkpoints/checkpoint-018-phase4-t050-conditional-pass.md`, `docs/checkpoints/checkpoint-020-phase4-t051-pass.md`

### Security remediation included in release scope
- T058, T059, T060, T061
- Summary: sidecar IPC hardening, HTTP header baseline, strict POST `/mcp` content-type gate, HS256-only clarity for current build.

## Validation Evidence Summary
- Phase 1 gate PASS with reviewed baseline architecture/security/tooling bundle.
- Phase 2 checkpoint validated sidecar-backed shadow workspace and AST flow in Docker.
- Phase 3 checkpoint recorded completed transport/job/memory stack and PASS tech-lead + security gates.
- Phase 4 quality gate advanced under CONDITIONAL_PASS with explicit tracked follow-ups.
- Security re-audit achieved PASS after remediations, with explicit command evidence in checkpoint-020:
  - `cargo test -p cwso-git-shadow -p cwso-merge-engine`: PASS
  - `go test ./internal/config ./internal/transport`: PASS

## Security Gate Verdict Reference
- Official verdict: T051 **PASS** on re-audit.
- Source: `docs/checkpoints/checkpoint-020-phase4-t051-pass.md`.

## Known Limitations and Follow-up Tasks
- Non-Linux peer-credential fallback remains permissive for sidecar auth boundaries (accepted for current Linux target scope).
- HSTS behavior depends on correct HTTPS termination in deployment environments.
- T050 conditional-pass follow-ups are still pending and should be handled immediately after this release artifact handoff:
  - T054 CI gate: merge-engine unit tests required
  - T055 Align `merge_inputs` schema/runtime contract
  - T056 ADR-006 node-level conflict detail reconciliation
  - T057 E2E policy path for sidecar reason mapping