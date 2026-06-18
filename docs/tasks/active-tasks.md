# Active Tasks

| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
| T080 | Phase 6 requirements + hardware benchmark targets | product-owner | done | P0 | — | 2026-06-02 |
| T081 | HAL design: `InferenceBackend` trait + plugin loading + `dispatch.provider/v2` | solution-architect | done | P0 | T080 | 2026-06-02 |
| T082 | Rust `cwso-hal` crate + CPU-baseline adapter | backend-developer | done | P0 | T081 | 2026-06-02 |
| T083 | GPU adapter (vLLM/TensorRT-LLM, OpenAI-compatible) | backend-developer | done | P0 | T082 | 2026-06-02 |
| T084 | LPU adapter (Groq-style deterministic low-latency) | backend-developer | done | P1 | T082 | 2026-06-02 |
| T085 | Profiling layer: tensor_tag derivation + workload mapping | backend-developer | done | P0 | T082 | 2026-06-02 |
| T086 | `dispatch_hardware_aware_job` MCP tool + schema | backend-developer | done | P0 | T083, T085 | 2026-06-02 |
| T087 | Wire policy_engine_v2 to live adapters (remove spike stubs) | backend-developer | done | P0 | T086 | 2026-06-02 |
| T088 | Phase 6 integration + reliability QA (fallback ≤ 2.0s, overhead ≤ 10ms) | qa-engineer | done | P0 | T087 | 2026-06-02 |
| T089 | Phase 6 Tech-Lead + Security gate | tech-lead / security-engineer | done | P0 | T088 | 2026-06-02 |
| T090 | Thread job context into `hal.Client.Infer` (cancellation propagation) | backend-developer | done | P1 | T089 | 2026-06-02 |
| T091 | Active HAL health probing → live `health_state`/`queue_depth` | backend-developer | done | P1 | T089 | 2026-06-02 |
| T092 | Hardware-aware job result retrieval (poll/stream completion) | backend-developer | done | P2 | T089 | 2026-06-02 |
| T093 | Enforce/document TLS for non-loopback HAL accelerator endpoints | devops-engineer | done | P1 | T089 | 2026-06-02 |
| T094 | CI dependency audit (`govulncheck` + `cargo audit`) | devops-engineer | done | P2 | T089 | 2026-06-02 |
| T114 | Bump Go toolchain to 1.25 (clear `go:audit` stdlib advisories) | devops-engineer | done | P2 | T094 | 2026-06-02 |
| T115 | AST write-spike monitor (generalize `anomaly_monitor`) + userspace fallback | backend-developer | done | P0 | T089 | 2026-06-03 |
| T116 | Spike filter (semantic classifier) + semantic-conflict pre-warning | backend-developer | done | P1 | T115 | 2026-06-03 |
| T117 | `subscribe_ast_spikes` MCP Resources layer (SSE, threshold-gated) | backend-developer | done | P1 | T116 | 2026-06-03 |
| T118 | AST write-event feeder wiring (`write_shadow_file` → monitor/filter) | backend-developer | done | P1 | T117 | 2026-06-03 |
| T119 | Sparse Wasm micro-agent sandbox tier design + security envelope review | solution-architect | done | P0 | T089 | 2026-06-03 |
| T120 | Rust `cwso-sparse` sidecar: deterministic ternary GEMM kernel + UDS protocol | backend-developer | done | P0 | T119 | 2026-06-03 |
| T121 | `.cwsl` pruned-slice container + COW mmap loader + SHA-256 pinning | backend-developer | done | P1 | T120 | 2026-06-03 |
| T122 | `create_ephemeral_sparse_agent` MCP tool + wasmtime lifecycle + agent telemetry resource | backend-developer | done | P0 | T120, T121 | 2026-06-04 |
| T123 | Quality-floor guardrail → dense GPU escalation (reuse `quality_guardrail_autodisable`) | backend-developer | done | P0 | T122 | 2026-06-04 |
| T124 | Phase 7 integration QA (cold start < 10 ms, 0% idle CPU, escalation) | qa-engineer | done | P0 | T118, T123 | 2026-06-04 |
| T125 | Phase 7 Tech-Lead + Security gate (Feature B + C) | tech-lead / security-engineer | done | P0 | T124 | 2026-06-04 |
| T126 | Sparse AST tensor encoding spec (photonic-ready kernel contract) | solution-architect | done | P0 | T125 | 2026-06-04 |
| T127 | AVX-512 / `std::simd` sparse diff kernel in cwso-merge-engine | backend-developer | done | P0 | T126 | 2026-06-04 |
| T128 | Sparse pre-filter integration (skip shared base subtrees) | backend-developer | done | P0 | T127 | 2026-06-04 |
| T129 | Sparse↔dense conflict-matrix conformance suite | qa-engineer | done | P0 | T128 | 2026-06-04 |
| T130 | Phase 8 Tech-Lead gate + large-repo merge benchmark | tech-lead | done | P0 | T129 | 2026-06-04 |
| T131 | Rollout architecture: proxy boundary + Polar REST API | solution-architect | done | P0 | T130 | 2026-06-04 |
| T132 | Rust `hyper` reverse proxy + zero-copy capture | backend-developer | done | P0 | T131 | 2026-06-04 |
| T133 | Trajectory builder + prefix merging | backend-developer | done | P0 | T132 | 2026-06-05 |
| T134 | Trajectory store (Arrow + LZ4 + Parquet) | backend-developer | done | P1 | T133 | 2026-06-05 |
| T136 | Programmatic reward emission (merge SM hook) | backend-developer | done | P0 | T133 | 2026-06-05 |
| T137 | Polar REST API + trainer e2e | backend-developer | done | P0 | T134, T136 | 2026-06-05 |
| T138 | Phase 9 integration QA + security gate | qa / security | done | P0 | T137 | 2026-06-06 |
| T139 | v0.3.0 release readiness (Phases 6–9) | release-manager | done | P0 | T138 | 2026-06-06 |
| T135 | KV-cache prefix router | backend-developer | done | P1 | T132 | 2026-06-06 |
| T140 | CI audit hardening (promote go:audit/rust:audit to blocking) | devops-engineer | done | P1 | T094, T139 | 2026-06-06 |
| T141 | Publish GitLab release v0.3.0-rc1 + GA prep checkpoint | release-manager | done | P0 | T139, T140 | 2026-06-07 |
| T143 | Root hygiene & PoC debt archive | tech-lead | done | P2 | T141 | 2026-06-07 |
| T142 | Installation & usage documentation | technical-writer | done | P0 | T141 | 2026-06-07 |
| T147 | OpenAI Responses API + proxy hardening | backend-developer | done | P1 | T132 | 2026-06-07 |
| T152 | v0.3.0 GA release readiness | release-manager | done | P0 | T142, T147 | 2026-06-07 |
| T153 | Tag pipeline deploy fix (`needs:optional` e2e) | devops-engineer | done | P1 | T152 | 2026-06-07 |
| T144 | Polar harness adapters + runtime launcher | backend-developer | done | P1 | T137, T142 | 2026-06-07 |
| T145 | Rollout `num_samples` session fan-out | backend-developer | done | P1 | T137 | 2026-06-07 |
| T154 | IDE integration guide (VS Code / Cursor) | technical-writer | done | P0 | T142 | 2026-06-07 |
| T155 | Enable-all-features script | devops-engineer | done | P1 | T142 | 2026-06-07 |
| T146 | Gateway async staging + partial traces | backend-developer | done | P1 | T132 | 2026-06-07 |
| T149 | Trajectory builder Polar parity | backend-developer | done | P2 | T133 | 2026-06-09 |
| T148 | Evaluator registry + SWE-bench hook | backend-developer | done | P2 | T146 | 2026-06-09 |
| T156 | Comprehensive installation guide v2 (v0.4.0) | technical-writer | done | P0 | T148, T154 | 2026-06-09 |
| T157 | v0.4.0 release readiness | release-manager | done | P0 | T156, T149 | 2026-06-09 |
| T150 | KV differential prompting | backend-developer | pending | P2 | T135 | 2026-06-07 |
| T151 | Offline SFT data generation mode | backend-developer | pending | P2 | T134 | 2026-06-07 |

> Status values: `pending` · `in_progress` · `blocked` · `in_review` · `done` · `cancelled`
> Priority values: `P0` (critical path) · `P1` (important) · `P2` (nice-to-have)
> Owners are agent names from `knowledge/agents/`.

Per-task briefs live alongside this file as `task-T001.md`, `task-T002.md`, …
