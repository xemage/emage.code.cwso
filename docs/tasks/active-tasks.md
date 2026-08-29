# Active Tasks

| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
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
| C025 | CONDITIONAL: document IPC-only limitation | technical-writer | pending | P0 | C020 (NO-GO) | 2026-08-12 |
| T199 | Wire `ErrorObj.conflict_matrix` into `mergeengine.Client` and surface it to MCP callers | backend-developer | pending | P2 | — | 2026-08-28 |
| T200 | Reconcile `local-docker-desktop-guide.md` with the current `make up` flow | technical-writer | pending | P2 | C052 (merged) | 2026-08-28 |
| T201 | Reconcile root README.md with the new CONTRIBUTING.md; fix broken TECHNICAL-DEBT.md link | technical-writer | pending | P2 | C053 (merged) | 2026-08-28 |
| T202 | Fix dashboard rate-limit/logging gap (F-C061-01) | backend-developer | pending | P1 | — | 2026-08-29 |
| T203 | Wire missing baseline-required CI security tools (F-C061-02) | devops-engineer | pending | P1 | — | 2026-08-29 |
| C062 | Release v1.0.0 | devops-engineer | pending | P0 | C060 (merged), C061 (merged), C063 | 2026-08-12 |
| C063 | Publish docs/LIMITATIONS.md | technical-writer | pending | P0 | C060 (merged) | 2026-08-12 |

> Status values: `pending` · `in_progress` · `blocked` · `in_review` · `done` · `cancelled`
> Priority values: `P0` (critical path) · `P1` (important) · `P2` (nice-to-have)
> Owners are agent names from `knowledge/agents/`.

> The C-series implements `docs/plans/plan-cwso-v1.0-roadmap.md` (**approved**
> 2026-08-13, incl. the three open-question decisions; C019 was added by decision 3).
> C025 activates only on an ADR-012 NO-GO. Gate dependencies (CG0–CG4) are noted inline.
> CG0 (C001–C005, C030) cleared 2026-08-16 — see `docs/tasks/completed-tasks.md`.
> CG2 (C020–C024, "real filesystem") and CG3 (C030–C034, "Protocol") both cleared
> 2026-08-27. Phase 4 "Correctness" (C040–C044, C042) cleared 2026-08-28 — see
> `docs/tasks/completed-tasks.md`. **CG4 ("One document", C050–C054) cleared 2026-08-28**
> — C050 (single user guide), C051 (delete superseded guides), C052 (receive emage.code
> deployment docs, T403 paired handover), C053 (contributor/user doc separation), and
> C054 (clean-machine command verification, zero genuine failures) all merged; see
> `docs/tasks/completed-tasks.md` for full review history. Unblocks C060–C063 (the
> release-gate chain: debt-register closure, security pass closing T010, publish
> `docs/LIMITATIONS.md`, release v1.0.0).

> ¹ **RESOLVED 2026-08-20.** Tracked condition (originated CONDITIONAL_PASS, Tech Lead
> review of C010/MR !113, 2026-08-16; refined CONDITIONAL_PASS, Tech Lead review of
> C012/MR !115, 2026-08-16): C010 made the documented `docker compose up -d`
> quick-start the default path, but that path failed at the orchestrator container
> with a JWT-secret config error on any checkout lacking a manually-created
> `.env.jwt.dev`. C012 (merged, CONDITIONAL_PASS) built a correct, verified bootstrap
> script (`scripts/cwso-bootstrap-secrets.sh`) but its own review found **nothing
> called it yet**. The condition moved to **C016**, the task that wires the bootstrap
> script into a one-command path, with three closing requirements: (a) `make up`'s
> first step invokes the bootstrap script; (b) fresh-clone re-verification with zero
> manual file creation; (c) `README.md`/`docs/user/installation-v3.md`'s quick-starts
> reference `make up`. A second addendum (2026-08-16, discovered during C019/MR !123)
> found a related, independent gap — `.env.jwt.dev` born `chmod 600`, unreadable by
> the orchestrator container's non-root user — tracked and fixed as **T191** (MR !132,
> merged 2026-08-19).
>
> **C016 (MR !135, merged 2026-08-20, PASS no conditions) satisfies all three closing
> requirements**, independently reproduced live by Tech Lead review: `make up` calls
> `scripts/cwso-bootstrap-secrets.sh` first; a genuinely clean-state cycle (no
> `.env.jwt.dev`, `make down && make up`) reaches a healthy, token-authenticated stack
> in ~7.7s with zero manual steps (exercising T191's fix with no regression); and
> `README.md`/`docs/user/installation-v3.md`'s quick-start blocks now call `make up`
> and remain byte-identical to each other. **The full C010 → C012 → C016 (+ T191)
> release-gating chain, tracked since 2026-08-16, is closed.** See
> `docs/tasks/task-C016.md` § "Release-gating condition" and `docs/tasks/completed-tasks.md`
> for the complete history; `docs/tasks/task-C062.md` ("Release v1.0.0") can rely on
> this being satisfied without re-deriving it.

> ² **T403 (emage.code repo) confirmed `done` 2026-08-28** — verified directly against
> `emage.code`'s own `docs/tasks/completed-tasks.md` and the staged handoff directory
> (`docs/archiv/cwso-deployment-guides-pending-t473-handoff/` in that repo). The 6
> deployment guides were relocated out of `docs/deployment/` there and staged verbatim,
> ready for C052 to receive. Note: `emage.code`'s own `plan-035` internally calls the
> CWSO-side receiving task **"T473"** (a placeholder id guessed before this roadmap
> assigned it), which is this repo's **C052** — same task, cross-repo naming mismatch
> only, no duplicate work exists under either name in either repo (confirmed via grep in
> both repos' active/completed task ledgers). C052 is unblocked on the T403 side; only
> C050 (this repo) remains as its blocker.

> ³ **T010 investigated 2026-08-28 (orchestrator, at the coordinator's request) before
> dispatching C061.** Findings: T010 was never actually started — zero evidence of any
> audit work (no branch, no MR, no artifact matching its scope anywhere in the repo).
> It was created 2026-08-06 as the security gate for the operator-dashboard feature
> chain (T001–T010, scoped per `docs/plans/feature-operator-dashboard.md`: "auth on
> dashboard, no secret leakage in JSON"), but the feature shipped in release v0.6.0 the
> same day without the audit ever running (`T001-T009` were archived; T010 was
> conspicuously left `in_review` and never touched again).
> `docs/archive/artifacts/security-phase5-audit-v1.md` was investigated as a possible
> match (its name is suggestive) and confirmed **unrelated** — it is task T071's output
> from 2026-05-23, over two months before T010 existed, auditing an entirely different
> code surface (dispatch/Wasm-scoring-plugin/telemetry from an earlier, unrelated
> "Phase 5" numbering scheme). This is a deliberate, documented carry-forward decision,
> not neglect: task C004 (Phase 0) discovered T010 as the sole `in_review` ledger
> anomaly; the human-approved v1.0 roadmap (2026-08-13) explicitly chose to defer its
> resolution to C061 rather than reopen it mid-roadmap — see
> `docs/checkpoints/checkpoint-020-v1.0-planning-complete.md` and
> `docs/checkpoints/checkpoint-021-v1.0-approved.md`'s "Open / carried over" tables,
> both annotated "Closed by C061 in Phase 6." **Decision: C061 proceeds as a fresh,
> full-v1.0-surface OWASP audit** (already how its brief was scoped — JWT auth,
> secret handling, the C015 workspace mount, C044's socket-permission outcome,
> container hardening, MCP-boundary input validation — none of which existed in their
> current form on 2026-08-06), not a resumption of T010's narrow original scope.
> `docs/tasks/task-C061.md`'s stale reference to a nonexistent `task-T010.md` brief was
> corrected in the same edit that dispatched C061.

Per-task briefs live alongside this file as `task-T001.md`, `task-T002.md`, …, `task-C001.md`, …
