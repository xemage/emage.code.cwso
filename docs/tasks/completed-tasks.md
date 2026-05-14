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
