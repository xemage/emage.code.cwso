# Active Tasks

| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
| T021 | OverlayFS bind-mount layer | backend-developer | cancelled (superseded by sidecar-mediated shadow FS + sandbox tier routing) | P2 | T020 | 2026-05-22 |
| T023 | Tree-sitter integration | backend-developer | done (Rust+TypeScript completed via T029; 4-language support active) | P0 | T020 | 2026-05-17 |
| T024 | query_ast tool + Unified Symbol Protocol | backend-developer | done (query_ast productionized; cross-language behavior aligned through T029/T055/T057) | P0 | T023 | 2026-05-22 |
| T025 | Merkle-hash incremental indexer | backend-developer | deferred (accepted non-blocking optimization; tracked as post-v0.1.x hardening) | P2 | T024 | 2026-05-22 |
| T028a | Go unit tests for shadow client + shadow_tools | backend-developer | done | P0 | T028 | 2026-05-12 |
| T029 | PoC-debt remediation pass | backend-developer | done | P0 | T028, T028a | 2026-05-12 |
| T030 | Streamable HTTP full duplex SSE | backend-developer | done | P0 | T029 | 2026-05-14 |
| T031 | Async job runner pool | backend-developer | done | P0 | T030 | 2026-05-14 |
| T032 | dispatch_concurrent_jobs tool | backend-developer | done | P0 | T031 | 2026-05-14 |
| T033 | Event-sourced memory broker | backend-developer | done | P0 | T031 | 2026-05-14 |
| T034 | Telemetry throttling + JSON-RPC notifications | backend-developer | done | P1 | T030, T033 | 2026-05-15 |
| T035 | Phase 3 integration tests | qa-engineer | done | P0 | T032, T034 | 2026-05-15 |
| T036 | Phase 3 Tech Lead gate | tech-lead | done | P0 | T035 | 2026-05-15 |
| T037 | Phase 3 Security gate | security-engineer | done | P0 | T036 | 2026-05-15 |
| T038 | Phase 3 coverage boost | backend-developer | done | P0 | T037 | 2026-05-15 |
| T040 | KVM/Firecracker host validation | devops-engineer | done | P0 | T037 | 2026-05-15 |
| T041 | Docker baseline runner | devops-engineer | done | P0 | T040 | 2026-05-15 |
| T042 | gVisor runner | devops-engineer | done | P0 | T041 | 2026-05-15 |
| T043 | Firecracker runner + snapshot CoW | devops-engineer | done | P0 | T042 | 2026-05-15 |
| T044 | Sandbox tier router | backend-developer | done | P0 | T043 | 2026-05-15 |
| T045 | cwso-merge-engine Rust crate | backend-developer | done | P0 | T044 | 2026-05-15 |
| T046 | AST diff + semantic merge algorithm | backend-developer | done | P0 | T045 | 2026-05-15 |
| T047 | merge_concurrent_results tool | backend-developer | done | P0 | T046 | 2026-05-16 |
| T048 | Conflict matrix escalation | backend-developer | done | P1 | T047 | 2026-05-16 |
| T049 | Phase 4 swarm e2e suite | qa-engineer | done | P0 | T048 | 2026-05-16 |
| T050 | Phase 4 Tech Lead gate | tech-lead | done (CONDITIONAL_PASS; conditions tracked in T054-T057) | P0 | T049 | 2026-05-16 |
| T051 | OWASP Top-10 security audit | security-engineer | done (PASS after T058-T061 remediation) | P0 | T050, T058, T059, T060, T061 | 2026-05-16 |
| T052 | Release manager: changelog + v0.1.0 artifacts | release-manager | done | P0 | T051 | 2026-05-16 |
| T053 | Final checkpoint + budget variance | orchestrator | done | P0 | T052 | 2026-05-16 |
| T054 | CI gate: merge-engine unit tests required | backend-developer | done | P1 | T050 | 2026-05-16 |
| T055 | Align merge_inputs schema/runtime contract | backend-developer | done | P1 | T050 | 2026-05-17 |
| T056 | ADR-006 node-level conflict detail reconciliation | solution-architect | done | P1 | T050 | 2026-05-17 |
| T057 | E2E policy path for sidecar reason mapping | qa-engineer | done | P1 | T050 | 2026-05-17 |
| T058 | Harden sidecar socket permissions and peer auth | backend-developer | done | P0 | T051 | 2026-05-16 |
| T059 | Add baseline HTTP security headers | backend-developer | done | P0 | T051 | 2026-05-16 |
| T060 | Enforce POST /mcp Content-Type | backend-developer | done | P0 | T051 | 2026-05-16 |
| T061 | Clarify/implement RS256 support path | backend-developer | done | P1 | T051 | 2026-05-16 |
| T062 | Phase 5 hardware-aware requirements and benchmarks | product-owner | done | P0 | — | 2026-05-22 |
| T063 | Hardware dispatch architecture and provider contracts | solution-architect | done | P0 | T062 | 2026-05-22 |
| T064 | Capability discovery and telemetry fabric | backend-developer | done | P0 | T063 | 2026-05-22 |
| T065 | Dispatch policy engine v2 | backend-developer | done | P0 | T063, T064 | 2026-05-22 |
| T066 | Wasm micro-agent runtime integration | backend-developer | done | P1 | T065 | 2026-05-22 |
| T067 | Sparse and quantized assist spike | backend-developer | done | P1 | T065 | 2026-05-23 |
| T068 | SSM sequence-assist spike | backend-developer | done | P1 | T065 | 2026-05-23 |
| T069 | Event-driven monitoring spike (eBPF + fallback) | backend-developer | done | P1 | T064 | 2026-05-22 |
| T070 | Phase 5 integration and reliability QA | qa-engineer | done | P0 | T066, T067, T068, T069 | 2026-05-23 |
| T071 | Phase 5 security gate and hardening | security-engineer | done (CONDITIONAL_PASS; follow-up hardening tracked as T073-T075) | P0 | T070 | 2026-05-23 |
| T072 | Phase 5 documentation and release readiness | technical-writer | pending | P1 | T071 | 2026-05-22 |

> Status values: `pending` · `in_progress` · `blocked` · `in_review` · `done` · `cancelled` · `partial` · `deferred`
> Priority values: `P0` (critical path) · `P1` (important) · `P2` (nice-to-have)

Per-task briefs live alongside this file as `task-T020.md`, etc.
Phase 0–2 done tasks (T001–T011, T020, T022, T026, T028) are in
[completed-tasks.md](completed-tasks.md).
