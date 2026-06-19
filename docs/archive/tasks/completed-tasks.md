# Completed Tasks

Append-only log. Entries move here after the orchestrator marks a task `done`.

| ID | Title | Owner | Done on | Outcome / artifact |
|----|-------|-------|---------|--------------------|
| T001 | Requirements distillation | product-owner | 2026-05-10 | [requirements-v1.md](../artifacts/requirements-v1.md) |
| T002 | Architecture v1 + 6 ADRs | solution-architect | 2026-05-10 | [architecture-v1.md](../artifacts/architecture-v1.md), [ADR-001](../decisions/ADR-001-hybrid-go-rust-split.md)..[ADR-006](../decisions/ADR-006-semantic-ast-merge.md) |
| T003 | Monorepo scaffold + Docker dev env | devops-engineer | 2026-05-10 | [Makefile](../../Makefile), [deploy/](../../deploy/), [README.md](../../README.md) |
| T004 | Security baseline | security-engineer | 2026-05-10 | [security-baseline-v1.md](../artifacts/security-baseline-v1.md), [SECURITY.md](../../SECURITY.md) |
| T005 | Go MCP server core | backend-developer | 2026-05-10 | [orchestrator/internal/mcp](../../orchestrator/internal/mcp), [orchestrator/internal/server](../../orchestrator/internal/server) |
| T006 | Baseline FS tools | backend-developer | 2026-05-10 | [orchestrator/internal/tools/fs_tools.go](../../orchestrator/internal/tools/fs_tools.go) |
| T007 | Streamable HTTP transport | backend-developer | 2026-05-10 | [orchestrator/internal/transport/http.go](../../orchestrator/internal/transport/http.go) |
| T008 | JWT + Origin validation | backend-developer | 2026-05-10 | HS256 verifier + Origin allow-list (same file) |
| T009 | Phase 1 test harness | qa-engineer | 2026-05-10 | `go test -race` all PASS; e2e HTTP smoke validated |
| T010 | Phase 1 review gate | tech-lead | 2026-05-10 | **VERDICT: PASS** ([checkpoint](../checkpoints/checkpoint-001-phase1.md)) |
| T011 | Phase 1 debt scorecard | backend-developer | 2026-05-10 | [POC-DEBT-SCORECARD-phase1.md](../../POC-DEBT-SCORECARD-phase1.md) |
| T020 | cwso-git-shadow Rust crate | backend-developer | 2026-05-11 | [services/cwso-git-shadow](../../services/cwso-git-shadow/) (8/8 tests PASS) |
| T022 | Shadow workspace MCP tools + Go UDS client | backend-developer | 2026-05-11 | [orchestrator/internal/shadow](../../orchestrator/internal/shadow/), [shadow_tools.go](../../orchestrator/internal/tools/shadow_tools.go) |
| T026 | Phase 2 integration test | qa-engineer | 2026-05-11 | [scripts/phase2-integration.py](../../scripts/phase2-integration.py) — **PASS** end-to-end in Docker |
| T027 | Phase 2 Tech Lead review gate | tech-lead | 2026-05-11 | **VERDICT: CONDITIONAL_PASS** ([gate-T027-phase2-techlead.md](../checkpoints/gate-T027-phase2-techlead.md)); 7 conditions folded into T029 + new T028a |
| T028 | Phase 2 debt scorecard | backend-developer | 2026-05-11 | [POC-DEBT-SCORECARD-phase2.md](../../POC-DEBT-SCORECARD-phase2.md) (8 items) |
| T029 | PoC-debt remediation pass | backend-developer | 2026-05-12 | MR !2 merged to `develop` (JWT library migration, mounted secret loading, Rust+TypeScript grammars, POST `/mcp` rate limiting, CI/e2e alignment fixes) |
| T030 | Streamable HTTP full duplex SSE | backend-developer | 2026-05-14 | MR !3 merged to `develop` with bounded in-memory event bus, SSE JSON-RPC notifications, fanout/drop tests, and transport integration updates |
| T031 | Async job runner pool | backend-developer | 2026-05-14 | MR !4 merged to `develop` with bounded job manager, lifecycle FSM, cancellation, and notifications/job-state event hooks |
| T032 | dispatch_concurrent_jobs tool | backend-developer | 2026-05-14 | MR !5 merged to `develop` with orchestrator-only dispatch tool, deterministic per-item acceptance/rejection, queue-pressure handling, and tool/server tests |
| T033 | Event-sourced memory broker | backend-developer | 2026-05-14 | MR !6 merged to `develop` with bounded append-only broker, non-blocking ingestion, filtered query APIs, and jobs/transport integration wiring |
| T048 | Conflict matrix escalation | backend-developer | 2026-05-16 | Deterministic conflict-matrix escalation implemented across merge-engine and orchestrator with additive class/reason metadata and passing Go/Rust validation ([task-T048.md](./task-T048.md), [plan-T048-phase4-conflict-matrix-escalation.md](../plans/plan-T048-phase4-conflict-matrix-escalation.md)) |
| T049 | Phase 4 swarm e2e suite | qa-engineer | 2026-05-16 | Added matrix-aware Phase 4 e2e coverage and CI jobs with deterministic escalation assertions; baseline flows preserved ([task-T049.md](./task-T049.md), [plan-T049-phase4-swarm-e2e-suite.md](../plans/plan-T049-phase4-swarm-e2e-suite.md)) |
| T050 | Phase 4 Tech Lead gate | tech-lead | 2026-05-16 | **VERDICT: CONDITIONAL_PASS**. Proceed to T051 with tracked conditions: T054 (merge-engine CI tests), T055 (schema/runtime contract alignment), T056 (ADR-006 node-level conflict detail reconciliation), T057 (sidecar policy-path e2e). |
| T058 | Harden sidecar socket permissions and peer auth | backend-developer | 2026-05-16 | Sidecar UDS permissions tightened to `0660`, Linux `SO_PEERCRED` allowlisting enforced from `CWSO_IPC_ALLOWED_UIDS`/`CWSO_IPC_ALLOWED_GIDS`, and negative unauthorized-peer tests added for both sidecars. Validation: `docker run --rm -v "$PWD":/workspace -w /workspace/services rust:1.86-slim cargo test -p cwso-git-shadow -p cwso-merge-engine` PASS. |
| T059 | Add baseline HTTP security headers | backend-developer | 2026-05-16 | Added `Content-Security-Policy`, `Strict-Transport-Security`, and `X-XSS-Protection` in HTTP security middleware while preserving existing headers and POST `Cache-Control`. Validation: `go test ./internal/transport` PASS. |
| T060 | Enforce POST /mcp Content-Type | backend-developer | 2026-05-16 | Added strict `application/json` media-type gate for `POST /mcp` with `415 Unsupported Media Type` on invalid content types; added transport regression test. Validation: `go test ./internal/transport` PASS. |
| T061 | Clarify/implement RS256 support path | backend-developer | 2026-05-16 | Removed RS256-ready ambiguity by constraining config/runtime to HS256-only in current build, updated compose comment, and added config test rejecting RS256. Validation: `go test ./internal/config ./internal/transport` PASS. |
| T051 | OWASP Top-10 security audit | security-engineer | 2026-05-16 | **VERDICT: PASS** on re-audit after T058-T061. No unresolved scoped findings; proceed to T052. |
| T052 | Release manager: changelog + v0.1.0 artifacts | release-manager | 2026-05-16 | Produced `CHANGELOG.md` (v0.1.0) and `docs/artifacts/release-v0.1.0.md` with scoped release summary, validation evidence, security gate reference, and residual-risk/follow-up notes; advanced T053 to in_progress. |
| T053 | Final checkpoint + budget variance | orchestrator | 2026-05-16 | Published final closure checkpoint `docs/checkpoints/checkpoint-021-phase5-final.md`, closed task tracking for release milestone, and documented budget variance assessment plus pending non-blocking follow-up tasks (T054-T057). |
| T054 | CI gate: merge-engine unit tests required | backend-developer | 2026-05-16 | CI policy updated to require merge-engine unit tests, with sustained green execution in subsequent develop pipelines. |
| T055 | Align merge_inputs schema/runtime contract | backend-developer | 2026-05-17 | Aligned `merge_concurrent_results` schema and runtime requirements for `merge_inputs`; added regression test coverage in orchestrator tools. |
| T056 | ADR-006 node-level conflict detail reconciliation | solution-architect | 2026-05-17 | ADR-006 addendum published to clarify current stable conflict-class/reason contract and defer node-detail payload expansion to versioned follow-up. |
| T057 | E2E policy path for sidecar reason mapping | qa-engineer | 2026-05-17 | Phase 4 policy-path e2e scenario updated to assert sidecar-origin `empty_merge_input` mapping; regression covered in CI. |
