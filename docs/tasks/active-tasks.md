# Active Tasks

| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
| T010 | SE: Security audit (auth, secret leakage) | security-engineer | in_review | P1 | T008 | 2026-08-06 |
| T082 | Rust `cwso-hal` crate + CPU-baseline adapter | backend-developer | in_review | P0 | T081 | 2026-08-13 |
| T083 | GPU adapter (vLLM/TensorRT-LLM, OpenAI-compatible) | backend-developer | in_review | P0 | T082 | 2026-08-13 |
| T084 | LPU adapter (Groq-style deterministic low-latency) | backend-developer | in_review | P1 | T082 | 2026-08-13 |
| T085 | Profiling Layer: tensor_tag derivation + workload mapping | backend-developer | in_review | P0 | T082 (soft) | 2026-08-13 |
| T086 | `dispatch_hardware_aware_job` MCP tool + schema | backend-developer | in_review | P0 | T083, T085 | 2026-08-13 |
| T087 | Wire policy_engine_v2 to live adapters (remove spike stubs) | backend-developer | in_review | P0 | T086, T082/T083/T084 | 2026-08-13 |
| T088 | Phase 6 integration + reliability QA | qa-engineer | in_review | P0 | T087 | 2026-08-13 |
| T090 | Thread job context into `hal.Client.Infer` | backend-developer | in_review | P1 | T089 | 2026-08-13 |
| T091 | Active HAL health probing → live `health_state`/`queue_depth` | backend-developer | in_review | P1 | T089 | 2026-08-13 |
| T092 | Hardware-aware job result retrieval | backend-developer | in_review | P2 | T089 | 2026-08-13 |
| T093 | Enforce/document TLS for non-loopback HAL accelerator endpoints | devops-engineer | in_review | P1 | T089 | 2026-08-13 |
| T094 | CI dependency audit (`govulncheck` + `cargo audit`) | devops-engineer | in_review | P2 | T089 | 2026-08-13 |
| T114 | Bump Go toolchain to 1.25 (clear `go:audit` stdlib advisories) | devops-engineer | in_review | P2 | T094 | 2026-08-13 |
| T115 | AST write-spike monitor (generalize `anomaly_monitor`) + userspace fallback | backend-developer | in_review | P0 | T089 | 2026-08-13 |
| T116 | Spike filter (semantic classifier) + semantic-conflict pre-warning | backend-developer | in_review | P1 | T115 | 2026-08-13 |
| T117 | `subscribe_ast_spikes` MCP Resources layer (SSE, threshold-gated) | backend-developer | in_review | P1 | T116 | 2026-08-13 |
| T118 | AST write-event feeder (`write_shadow_file` → monitor/filter) | backend-developer | in_review | P1 | T117 | 2026-08-13 |
| T119 | Sparse Wasm micro-agent sandbox tier design + security envelope review | solution-architect | in_review | P0 | T089 | 2026-08-13 |
| T120 | `cwso-sparse` sidecar: deterministic ternary GEMM kernel + UDS protocol | backend-developer | in_review | P0 | T119 | 2026-08-13 |
| T121 | `.cwsl` pruned-slice container + COW mmap loader + SHA-256 pinning | backend-developer | in_review | P1 | T120 | 2026-08-13 |
| T122 | `create_ephemeral_sparse_agent` + wasmtime lifecycle + agent telemetry | backend-developer | in_review | P0 | T120, T121 | 2026-08-13 |
| T145 | Rollout `num_samples` session fan-out | backend-developer | in_review | P1 | T137 | 2026-08-13 |
| T146 | Gateway async staging + partial trace recovery | backend-developer | in_review | P1 | T132, T144 | 2026-08-13 |
| T148 | Evaluator registry + SWE-bench hook | backend-developer / qa-engineer | in_review | P2 | T146, T144 | 2026-08-13 |
| T154 | IDE integration guide (VS Code / Cursor) | technical-writer | in_review | P0 | T142 | 2026-08-13 |
| T155 | Enable-all-features script | devops-engineer | in_review | P1 | T142 | 2026-08-13 |
| T165 | Author v0.5.0 changelog and release artifact | technical-writer | pending | P0 | T164 | 2026-08-13 |
| T166 | Cut release/v0.5.0 and merge to main | release-manager | pending | P0 | T165 | 2026-08-13 |
| T167 | Tag v0.5.0 and publish GitLab release | release-manager | pending | P0 | T166 | 2026-08-13 |
| T168 | Back-merge main into develop and clean up | release-manager | pending | P0 | T167 | 2026-08-13 |
| T180 | Close Resolved Debt Rows In Register | backend-developer | pending | P2 | — | 2026-08-13 |
| T181 | TD-07 Replace Broker Close Guard With sync.Once | backend-developer | pending | P1 | — | 2026-08-13 |
| T182 | TD-02 Introduce HTTPHandlerConfig For RunHTTP/newHTTPHandler | backend-developer | pending | P2 | — | 2026-08-13 |
| T183 | TD-03 Reduce Internal Helper Parameter Counts | backend-developer | pending | P2 | T182 | 2026-08-13 |
| T184 | TD-01 Extract SSE Helpers And Reduce Function Length | backend-developer | pending | P2 | T183 | 2026-08-13 |
| T185 | TD-09 Evict Zero-Count SSE Connection Entries | backend-developer | pending | P2 | T184 | 2026-08-13 |
| T186 | TD-04 Add Dedicated Unit Test For Broker SSE Deferred Telemetry | qa-engineer | pending | P2 | T184 | 2026-08-13 |
| T187 | TD-03 Residual: Reduce handleBrokerSSE to ≤4 Parameters | backend-developer | pending | P2 | — | 2026-08-13 |
| T188 | TD-10 Fix SSE Telemetry Test Stderr-Capture Race | qa-engineer | pending | P2 | — | 2026-08-13 |
| T189 | TD-11 Investigate and Fix TestRetentionEvictionOldestFirst Flakiness | qa-engineer | pending | P2 | — | 2026-08-13 |
| C011 | Add cwso-rollout behind opt-in profile | devops-engineer | pending | P0 | C010 | 2026-08-12 |
| C012 | Bootstrap .env.jwt.dev on first run **[RELEASE-GATING CONDITION — see note ¹]** | devops-engineer | in_progress | P0 | C010 | 2026-08-16 |
| C013 | scripts/cwso-token.sh replaces JWT heredoc | devops-engineer | pending | P0 | C010 | 2026-08-12 |
| C014 | Fold enable-all-features into compose defaults | devops-engineer | pending | P0 | C010 | 2026-08-12 |
| C015 | Mount user repo read-write (CWSO_WORKSPACE_HOST) | devops-engineer | pending | P0 | C010, C019 | 2026-08-13 |
| C016 | make up one-command target | devops-engineer | pending | P0 | C012, C013, C014, C015 | 2026-08-12 |
| C017 | scripts/cwso-doctor.sh diagnostics | devops-engineer | pending | P0 | C010 | 2026-08-12 |
| C018 | E2E smoke test (v1.0 DoD executable) | qa-engineer | pending | P0 | C016, C017 | 2026-08-12 |
| C019 | Sandbox trustworthiness, non-KVM default path | backend-developer | pending | P0 | C010 | 2026-08-13 |
| C020 | ADR-012: filesystem projection decision | solution-architect | pending | P0 | C010–C018 (CG1) | 2026-08-12 |
| C021 | Implement filesystem projection | backend-developer | pending | P0 | C020 (GO) | 2026-08-12 |
| C022 | Write-back into git ODB | backend-developer | pending | P0 | C021 | 2026-08-12 |
| C023 | Projection lifecycle + crash safety | backend-developer | pending | P0 | C021 | 2026-08-12 |
| C024 | Prove projection E2E in CI | qa-engineer | pending | P0 | C022, C023 | 2026-08-12 |
| C025 | CONDITIONAL: document IPC-only limitation | technical-writer | pending | P0 | C020 (NO-GO) | 2026-08-12 |
| C031 | ADR-013: SDK vs conformance suite | solution-architect | pending | P1 | C030 | 2026-08-12 |
| C032 | Execute ADR-013 decision | backend-developer | pending | P1 | C031 | 2026-08-12 |
| C033 | Client compatibility matrix (3×2) | qa-engineer | pending | P1 | C032 | 2026-08-12 |
| C034 | Contract snapshot test in CI | qa-engineer | pending | P1 | C032 | 2026-08-12 |
| C040 | Scope/binding resolution for find_references | backend-developer | pending | P1 | C024, C033, C034 (CG2+CG3) | 2026-08-12 |
| C041 | Parent-commit tracking per workspace | backend-developer | pending | P1 | C024, C033, C034 (CG2+CG3) | 2026-08-12 |
| C042 | Three-way merge + conflict matrix | backend-developer | pending | P1 | C041 | 2026-08-12 |
| C043 | Connection pooling in shadow client | backend-developer | pending | P1 | C024, C033, C034 (CG2+CG3) | 2026-08-12 |
| C044 | UDS perms 0o660 or documented limitation | backend-developer | pending | P1 | C024, C033, C034 (CG2+CG3) | 2026-08-12 |
| C050 | Write the single user guide | technical-writer | pending | P1 | C040–C044 | 2026-08-12 |
| C051 | Delete the five superseded guides | technical-writer | pending | P1 | C050 | 2026-08-12 |
| C052 | Receive emage.code deployment docs (T403) | technical-writer | pending | P1 | C050, T403 | 2026-08-12 |
| C053 | Contributor vs user doc separation | technical-writer | pending | P1 | C050 | 2026-08-12 |
| C054 | Verify guide commands on clean machine | qa-engineer | pending | P1 | C050, C051, C052, C053 | 2026-08-12 |
| C060 | Debt register: zero unclassified rows | technical-writer | pending | P0 | C050–C054 (CG4) | 2026-08-12 |
| C061 | Security pass closing T010 | security-engineer | pending | P0 | C050–C054 (CG4) | 2026-08-12 |
| C062 | Release v1.0.0 | devops-engineer | pending | P0 | C060, C061, C063 | 2026-08-12 |
| C063 | Publish docs/LIMITATIONS.md | technical-writer | pending | P0 | C060 | 2026-08-12 |

> Status values: `pending` · `in_progress` · `blocked` · `in_review` · `done` · `cancelled`
> Priority values: `P0` (critical path) · `P1` (important) · `P2` (nice-to-have)
> Owners are agent names from `knowledge/agents/`.

> The C-series implements `docs/plans/plan-cwso-v1.0-roadmap.md` (**approved**
> 2026-08-13, incl. the three open-question decisions; C019 was added by decision 3).
> C025 activates only on an ADR-012 NO-GO. Gate dependencies (CG0–CG4) are noted inline.
> CG0 (C001–C005, C030) cleared 2026-08-16 — see `docs/tasks/completed-tasks.md`.

> ¹ **Tracked condition (CONDITIONAL_PASS, Tech Lead review of C010/MR !113,
> 2026-08-16):** C012 must land and be verified against a truly fresh clone before the
> v1.0 GA/release cut — C010 made the documented `docker compose up -d` quick-start the
> default path, but that path now fails at the orchestrator container with a
> JWT-secret config error on any checkout lacking a manually-created `.env.jwt.dev`.
> This is release-gating, not backlog — see `docs/tasks/task-C012.md` and
> `docs/tasks/task-C062.md` ("Release v1.0.0"). C010 itself is unaffected: its own
> diff was reviewed and confirmed correct and complete.

Per-task briefs live alongside this file as `task-T001.md`, `task-T002.md`, …, `task-C001.md`, …
