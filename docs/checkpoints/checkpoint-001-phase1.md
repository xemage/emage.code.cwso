# Checkpoint 001 — Phase 1 Complete

> Date: 2026-05-10 · Author: orchestrator · Phase: planning + Phase 1 (PoC)

## Completed tasks

| ID | Title | Artifact(s) |
|----|-------|------------|
| T001 | Requirements distillation | [requirements-v1.md](docs/artifacts/requirements-v1.md) |
| T002 | Architecture v1 + ADRs (6) | [architecture-v1.md](docs/artifacts/architecture-v1.md), ADR-001..006 |
| T003 | Monorepo scaffold + Docker dev env | [Makefile](Makefile), [deploy/](deploy/), [README.md](README.md) |
| T004 | Security baseline | [security-baseline-v1.md](docs/artifacts/security-baseline-v1.md), [SECURITY.md](SECURITY.md) |
| T005 | Go MCP server core | [orchestrator/internal/mcp/](orchestrator/internal/mcp/), [internal/server/](orchestrator/internal/server/) |
| T006 | Baseline FS tools (`read_file_sync`, `write_file_sync`, `list_dir`) | [internal/tools/fs_tools.go](orchestrator/internal/tools/fs_tools.go) |
| T007 | Streamable HTTP transport (POST + SSE skeleton) | [internal/transport/http.go](orchestrator/internal/transport/http.go) |
| T008 | JWT (HS256) + Origin allow-list | [internal/transport/http.go](orchestrator/internal/transport/http.go) |
| T009 | Phase 1 test harness (`go test -race` + e2e python smoke) | [internal/server/server_test.go](orchestrator/internal/server/server_test.go), [internal/transport/http_test.go](orchestrator/internal/transport/http_test.go) |
| T010 | Phase 1 review gate | **VERDICT: PASS** (this checkpoint) |
| T011 | Phase 1 debt scorecard | [POC-DEBT-SCORECARD-phase1.md](POC-DEBT-SCORECARD-phase1.md) |

## Key decisions
- ADR-001 → Hybrid Go (kernel) + Rust (sidecars).
- ADR-002 → MCP spec **2025-03-26** (Streamable HTTP).
- ADR-003 → 3-tier sandbox (Docker / gVisor / Firecracker).
- ADR-004 → `libgit2` Rust sidecar with `go-git` fallback.
- ADR-005 → `gotreesitter` + Merkle incremental indexing.
- ADR-006 → Semantic AST merge instead of line-based.

## Validation gates
- **Architecture Gate** (post-T002): PASS.
- **Phase 1 Review Gate** (post-T009): PASS — all acceptance criteria met (see scorecard "Verified end-to-end").

## Token metrics
| Phase | Budget | Spent (est) | Variance |
|-------|--------|-------------|----------|
| Planning + Architecture | 80k | ~25k | +55k headroom |
| Phase 1 implementation | 120k | ~55k | +65k headroom |

## Blockers
None.

## Next steps (Phase 2 — PoC: Shadow Workspaces + AST)
1. **T020** — `cwso-git-shadow` Rust crate: in-memory bare repo on tmpfs; `git2` blob/tree ops; UDS protocol.
2. **T021** — OverlayFS bind mount layer for sandbox virtual working trees.
3. **T022** — `create_shadow_workspace` MCP tool wired to the sidecar.
4. **T023** — `gotreesitter` integration in Go orchestrator with embedded grammars (Go/Rust/Py/TS).
5. **T024** — `query_ast` tool implementing all 5 query types via Unified Symbol Protocol.
6. **T025** — Merkle-hash incremental indexer.
7. **T026** — Phase 2 bench harness (10 parallel workspaces, 1k-file fixture).
8. **T027** — Phase 2 review gate.
9. **T028** — Phase 2 debt scorecard.

Phase 2 hypothesis is documented in [plan-cwso-mega.md](docs/plans/plan-cwso-mega.md#phase-2-hypothesis).

## Resume brief (for next agent dispatch)
> Phase 1 of CWSO is complete and validated. Begin Phase 2 by reading [requirements-v1.md](docs/artifacts/requirements-v1.md) §FR-2 and FR-4, [architecture-v1.md](docs/artifacts/architecture-v1.md) §3-4, ADR-004 and ADR-005, and the Phase 2 hypothesis in [plan-cwso-mega.md](docs/plans/plan-cwso-mega.md). Implement T020 first; the sidecar protocol and tmpfs-backed bare-repo strategy are the highest-leverage primitives.
