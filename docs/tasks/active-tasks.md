# Active Tasks

| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
| T021 | OverlayFS bind-mount layer | backend-developer | deferred → Phase 4 (POC-DEBT P2-1) | P0 | T020 | 2026-05-11 |
| T023 | Tree-sitter integration | backend-developer | partial — Go+Python only (POC-DEBT P2-3 → T029) | P0 | T020 | 2026-05-11 |
| T024 | query_ast tool + Unified Symbol Protocol | backend-developer | partial — query_ast wired; USP deferred to T029 | P0 | T023 | 2026-05-11 |
| T025 | Merkle-hash incremental indexer | backend-developer | deferred → T029 (POC-DEBT P2-2) | P1 | T024 | 2026-05-11 |
| T028a | Go unit tests for shadow client + shadow_tools | backend-developer | done | P0 | T028 | 2026-05-12 |
| T029 | PoC-debt remediation pass | backend-developer | done | P0 | T028, T028a | 2026-05-12 |
| T030 | Streamable HTTP full duplex SSE | backend-developer | done | P0 | T029 | 2026-05-14 |
| T031 | Async job runner pool | backend-developer | done | P0 | T030 | 2026-05-14 |
| T032 | dispatch_concurrent_jobs tool | backend-developer | done | P0 | T031 | 2026-05-14 |
| T033 | Event-sourced memory broker | backend-developer | done | P0 | T031 | 2026-05-14 |
| T034 | Telemetry throttling + JSON-RPC notifications | backend-developer | done | P1 | T030, T033 | 2026-05-15 |
| T035 | Phase 3 integration tests | qa-engineer | done | P0 | T032, T034 | 2026-05-15 |
| T036 | Phase 3 Tech Lead gate | tech-lead | done | P0 | T035 | 2026-05-15 |
| T037 | Phase 3 Security gate | security-engineer | in_progress | P0 | T036 | 2026-05-15 |
| T040 | KVM/Firecracker host validation | devops-engineer | pending | P0 | T037 | 2026-05-11 |
| T041 | Docker baseline runner | devops-engineer | pending | P0 | T040 | 2026-05-11 |
| T042 | gVisor runner | devops-engineer | pending | P0 | T041 | 2026-05-11 |
| T043 | Firecracker runner + snapshot CoW | devops-engineer | pending | P0 | T042 | 2026-05-11 |
| T044 | Sandbox tier router | backend-developer | pending | P0 | T043 | 2026-05-11 |
| T045 | cwso-merge-engine Rust crate | backend-developer | pending | P0 | T044 | 2026-05-11 |
| T046 | AST diff + semantic merge algorithm | backend-developer | pending | P0 | T045 | 2026-05-11 |
| T047 | merge_concurrent_results tool | backend-developer | pending | P0 | T046 | 2026-05-11 |
| T048 | Conflict matrix escalation | backend-developer | pending | P1 | T047 | 2026-05-11 |
| T049 | Phase 4 swarm e2e suite | qa-engineer | pending | P0 | T048 | 2026-05-11 |
| T050 | Phase 4 Tech Lead gate | tech-lead | pending | P0 | T049 | 2026-05-11 |
| T051 | OWASP Top-10 security audit | security-engineer | pending | P0 | T050 | 2026-05-11 |
| T052 | Release manager: changelog + v0.1.0 artifacts | release-manager | pending | P0 | T051 | 2026-05-11 |
| T053 | Final checkpoint + budget variance | orchestrator | pending | P0 | T052 | 2026-05-11 |

> Status values: `pending` · `in_progress` · `blocked` · `in_review` · `done` · `cancelled` · `partial` · `deferred`
> Priority values: `P0` (critical path) · `P1` (important) · `P2` (nice-to-have)

Per-task briefs live alongside this file as `task-T020.md`, etc.
Phase 0–2 done tasks (T001–T011, T020, T022, T026, T028) are in
[completed-tasks.md](completed-tasks.md).
