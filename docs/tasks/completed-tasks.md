# Completed Tasks

Append-only log. Entries move here after the orchestrator marks a task `done`.

| ID | Title | Owner | Done on | Outcome / artifact |
|----|-------|-------|---------|--------------------|
| T080 | Phase 6 requirements + hardware benchmark targets | product-owner | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T081 | HAL design: `InferenceBackend` trait + plugin loading + `dispatch.provider/v2` | solution-architect | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T082 | Rust `cwso-hal` crate + CPU-baseline adapter | backend-developer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T083 | GPU adapter (vLLM/TensorRT-LLM, OpenAI-compatible) | backend-developer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T084 | LPU adapter (Groq-style deterministic low-latency) | backend-developer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T085 | Profiling layer: tensor_tag derivation + workload mapping | backend-developer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T086 | `dispatch_hardware_aware_job` MCP tool + schema | backend-developer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T087 | Wire policy_engine_v2 to live adapters (remove spike stubs) | backend-developer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T088 | Phase 6 integration + reliability QA (fallback ≤ 2.0s, overhead ≤ 10ms) | qa-engineer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T089 | Phase 6 Tech-Lead + Security gate | tech-lead / security-engineer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T090 | Thread job context into `hal.Client.Infer` (cancellation propagation) | backend-developer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T091 | Active HAL health probing → live `health_state`/`queue_depth` | backend-developer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T092 | Hardware-aware job result retrieval (poll/stream completion) | backend-developer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T093 | Enforce/document TLS for non-loopback HAL accelerator endpoints | devops-engineer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T094 | CI dependency audit (`govulncheck` + `cargo audit`) | devops-engineer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T114 | Bump Go toolchain to 1.25 (clear `go:audit` stdlib advisories) | devops-engineer | 2026-06-02 | Migrated from active-tasks.md board cleanup |
| T115 | AST write-spike monitor (generalize `anomaly_monitor`) + userspace fallback | backend-developer | 2026-06-03 | Migrated from active-tasks.md board cleanup |
| T116 | Spike filter (semantic classifier) + semantic-conflict pre-warning | backend-developer | 2026-06-03 | Migrated from active-tasks.md board cleanup |
| T117 | `subscribe_ast_spikes` MCP Resources layer (SSE, threshold-gated) | backend-developer | 2026-06-03 | Migrated from active-tasks.md board cleanup |
| T118 | AST write-event feeder wiring (`write_shadow_file` → monitor/filter) | backend-developer | 2026-06-03 | Migrated from active-tasks.md board cleanup |
| T119 | Sparse Wasm micro-agent sandbox tier design + security envelope review | solution-architect | 2026-06-03 | Migrated from active-tasks.md board cleanup |
| T120 | Rust `cwso-sparse` sidecar: deterministic ternary GEMM kernel + UDS protocol | backend-developer | 2026-06-03 | Migrated from active-tasks.md board cleanup |
| T121 | `.cwsl` pruned-slice container + COW mmap loader + SHA-256 pinning | backend-developer | 2026-06-03 | Migrated from active-tasks.md board cleanup |
| T122 | `create_ephemeral_sparse_agent` MCP tool + wasmtime lifecycle + agent telemetry resource | backend-developer | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T123 | Quality-floor guardrail → dense GPU escalation (reuse `quality_guardrail_autodisable`) | backend-developer | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T124 | Phase 7 integration QA (cold start < 10 ms, 0% idle CPU, escalation) | qa-engineer | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T125 | Phase 7 Tech-Lead + Security gate (Feature B + C) | tech-lead / security-engineer | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T126 | Sparse AST tensor encoding spec (photonic-ready kernel contract) | solution-architect | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T127 | AVX-512 / `std::simd` sparse diff kernel in cwso-merge-engine | backend-developer | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T128 | Sparse pre-filter integration (skip shared base subtrees) | backend-developer | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T129 | Sparse↔dense conflict-matrix conformance suite | qa-engineer | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T130 | Phase 8 Tech-Lead gate + large-repo merge benchmark | tech-lead | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T131 | Rollout architecture: proxy boundary + Polar REST API | solution-architect | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T132 | Rust `hyper` reverse proxy + zero-copy capture | backend-developer | 2026-06-04 | Migrated from active-tasks.md board cleanup |
| T133 | Trajectory builder + prefix merging | backend-developer | 2026-06-05 | Migrated from active-tasks.md board cleanup |
| T134 | Trajectory store (Arrow + LZ4 + Parquet) | backend-developer | 2026-06-05 | Migrated from active-tasks.md board cleanup |
| T136 | Programmatic reward emission (merge SM hook) | backend-developer | 2026-06-05 | Migrated from active-tasks.md board cleanup |
| T137 | Polar REST API + trainer e2e | backend-developer | 2026-06-05 | Migrated from active-tasks.md board cleanup |
| T138 | Phase 9 integration QA + security gate | qa / security | 2026-06-06 | Migrated from active-tasks.md board cleanup |
| T139 | v0.3.0 release readiness (Phases 6–9) | release-manager | 2026-06-06 | Migrated from active-tasks.md board cleanup |
| T135 | KV-cache prefix router | backend-developer | 2026-06-06 | Migrated from active-tasks.md board cleanup |
| T140 | CI audit hardening (promote go:audit/rust:audit to blocking) | devops-engineer | 2026-06-06 | Migrated from active-tasks.md board cleanup |
| T141 | Publish GitLab release v0.3.0-rc1 + GA prep checkpoint | release-manager | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T143 | Root hygiene & PoC debt archive | tech-lead | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T142 | Installation & usage documentation | technical-writer | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T147 | OpenAI Responses API + proxy hardening | backend-developer | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T152 | v0.3.0 GA release readiness | release-manager | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T153 | Tag pipeline deploy fix (`needs:optional` e2e) | devops-engineer | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T144 | Polar harness adapters + runtime launcher | backend-developer | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T145 | Rollout `num_samples` session fan-out | backend-developer | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T154 | IDE integration guide (VS Code / Cursor) | technical-writer | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T155 | Enable-all-features script | devops-engineer | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T146 | Gateway async staging + partial traces | backend-developer | 2026-06-07 | Migrated from active-tasks.md board cleanup |
| T149 | Trajectory builder Polar parity | backend-developer | 2026-06-09 | Migrated from active-tasks.md board cleanup |
| T148 | Evaluator registry + SWE-bench hook | backend-developer | 2026-06-09 | Migrated from active-tasks.md board cleanup |
| T156 | Comprehensive installation guide v2 (v0.4.0) | technical-writer | 2026-06-09 | Migrated from active-tasks.md board cleanup |
| T157 | v0.4.0 release readiness | release-manager | 2026-06-09 | Migrated from active-tasks.md board cleanup |
| T169 | Root-cause investigation: rollout healthcheck 405 and trajectory store path mismatch | backend-developer | 2026-08-01 | `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md` — Issue 1 (405) verdict NEEDS-REFINEMENT (no `/v1/models` route exists under any method; global POST-only gate at `proxy.rs:46-51`; no callers/tests depend on current behavior); Issue 2 (trajectory store path) verdict CONFIRMED (`store.rs:46` reads `CWSO_ROLLOUT_STORE_PATH`, never `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH`, which `deploy/Dockerfile.rollout:27` sets) |
| T170 | Implement and verify fix for confirmed rollout defect(s) | backend-developer | 2026-08-01 | `docs/artifacts/fix-verification-cwso-rollout-v1.md` — added `GET /healthz` liveness route (`services/cwso-rollout/src/proxy.rs`) + `HEALTHCHECK` in `deploy/Dockerfile.rollout`; `StoreConfig::from_env` (`services/cwso-rollout/src/store.rs`) now checks `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` before `CWSO_ROLLOUT_STORE_PATH`; verified via real `cargo build`/`cargo test` (35/35 pass) and real `docker build`/`docker run`: sustained `(healthy)`, `FailingStreak:0` across 5/5 probes (both explicit health-cmd and Dockerfile-native), trajectory store dir created cleanly with no startup error, `/v1/models` behavior unchanged (405) |
| T171 | Bump/patch git2 and memmap2 to clear rust:audit RUSTSEC findings | backend-developer | 2026-08-01 | `docs/tasks/task-T171.md` execution notes — memmap2 0.9.10→0.9.11 (RUSTSEC-2026-0186, fixed); git2 fix blocked by Rust 1.87 MSRV requirement, reverted, scoped `cargo audit --ignore RUSTSEC-2026-0183 RUSTSEC-2026-0184` added to `.gitlab-ci.yml` pending T172; bonus fixes for anyhow (RUSTSEC-2026-0190) and wasmtime (RUSTSEC-2026-0222), both discovered live during this task. Final `cargo audit` exit 0. |
| T172 | Bump Rust toolchain to 1.87+ to unblock git2 RUSTSEC fix | devops-engineer | 2026-08-01 | `docs/artifacts/rust-toolchain-1.87-bump-verification-v1.md` — all 3 Rust Dockerfiles + all 3 Rust CI jobs bumped rust:1.86→1.87; git2 0.20.4→0.21.0 (resolves RUSTSEC-2026-0183/0184); `cargo audit --ignore ...` flags from T171 removed, `cargo audit` now exits 0 with zero ignores. Verified: full workspace build+test (all 5 crates, 0 failures), `cargo fmt --check` clean, real `docker build` for all 3 affected images. |
