# Artifact: release-v0.3.0-rc1

## Metadata
- Producer agent: release-manager
- Created: 2026-06-06
- Based on: `docs/tasks/task-T139.md`, `plan-cwso-nextgen-phase6plus.md`, checkpoints 007–011,
  `gate-phase6-feature-a-2026-06-02.md`, `gate-phase7-feature-bc-2026-06-04.md`,
  `gate-phase8-feature-d-2026-06-04.md`, `gate-phase9-feature-efg-2026-06-05.md`, CHANGELOG.md
- **develop tip:** `5d2cfca` (T138 merged via MR !47, squash `011d8c8`)

## Release intent

**v0.3.0-rc1** is the release-candidate consolidation for the Next-Gen line (Phases 6–9): heterogeneous
hardware dispatch, sparse Wasm micro-agents, spiking AST monitors, semantic sparse-merging, and the
Polar-style rollout substrate (proxy capture, trajectory store, merge rewards, trainer REST API).

## Scope included

### Phase 6 — Feature A: Heterogeneous Hardware Dispatcher (T080–T094, T114)
- Rust `cwso-hal` crate with `InferenceBackend` trait and CPU/GPU/LPU adapters.
- `dispatch_hardware_aware_job` MCP tool + live HAL wiring + capability sync.
- Reliability budgets verified (dispatch overhead ≤ 10 ms, fallback ≤ 2.0 s).
- Gate: **PASS / PASS** — `gate-phase6-feature-a-2026-06-02.md`.

### Phase 7 — Features B + C: Sparse Micro-Agents & Spiking Monitors (T115–T125)
- `cwso-sparse` sidecar: deterministic ternary GEMM, `.cwsl` mmap loader, wasmtime lifecycle.
- `create_ephemeral_sparse_agent` MCP tool + quality-floor → dense GPU escalation.
- AST write-spike monitor, semantic filter, `subscribe_ast_spikes` MCP resources + feeder.
- Budgets: cold start < 10 ms (p95), 0% idle CPU on spike pipeline.
- Gate: **PASS / PASS** — `gate-phase7-feature-bc-2026-06-04.md`.

### Phase 8 — Feature D: Semantic Sparse-Merging (T126–T130)
- Sparse AST tensor encoding (ADR-009), AVX2 sparse diff kernel, pre-filter in `merge_three_way`.
- Sparse↔dense conflict-matrix conformance suite + large-repo benchmark.
- Gate: **PASS / PASS** — `gate-phase8-feature-d-2026-06-04.md`.

### Phase 9 — Features E + F + G: Rollout-as-a-Service (T131–T138)
- `cwso-rollout` hyper proxy + zero-copy capture; trajectory builder + Parquet/LZ4 store.
- Programmatic merge rewards (+1/−1); Polar REST API (`/rollout/*`, `/callbacks/*`, `/nodes/*`).
- Trainer e2e integration tests (submit → reward → poll → callback).
- Gate: **PASS / PASS** — `gate-phase9-feature-efg-2026-06-05.md` (merged MR !47).

## Gate summary (Phases 6–9)

| Phase | Feature(s) | Implementation | Security | Artifact |
|-------|------------|----------------|----------|----------|
| 6 | A — HAL | PASS | PASS | `gate-phase6-feature-a-2026-06-02.md` |
| 7 | B + C — Sparse + Spikes | PASS | PASS | `gate-phase7-feature-bc-2026-06-04.md` |
| 8 | D — Sparse merge | PASS | PASS | `gate-phase8-feature-d-2026-06-04.md` |
| 9 | E + F + G — Rollout | PASS | PASS | `gate-phase9-feature-efg-2026-06-05.md` |

## Deferred / non-blocking items

| Item | Task | Impact on RC |
|------|------|--------------|
| KV prefix router | T135 (pending) | Synthetic `prefix_key` on submit; PoC-acceptable |
| Orchestrator `/v1/chat/completions` stub | — | Trainers point at `cwso-rollout` proxy |
| CI dependency audits `allow_failure` | T094 | Advisory only; harden before GA |
| Trainer fleet proxy p95 benchmark | ops | Unit-tested; fleet validation deferred |
| Trajectory chain columns | store v2 | Raw completion records sufficient for RC |

## Validation and CI evidence

- **T138 pre-merge:** pipeline green on `feature/T138-phase9-gate` branch.
- **develop tip:** `5d2cfca` — T138 squash merge `011d8c8` via MR !47.
- **Local suites:** `go test ./... -race`, `cargo test -p cwso-hal -p cwso-sparse -p cwso-merge-engine -p cwso-rollout` expected green on develop.
- **T139 MR:** !48 — CI pending at publish time.

## Operator flags (new since v0.2.0-rc1)

### Hardware dispatch
- `CWSO_HAL_SOCKET`, `CWSO_HAL_{GPU,LPU}_BASE_URL`, `CWSO_HAL_CAPABILITY_SYNC_SECONDS`
- `CWSO_HAL_HEALTH_PROBE_SECONDS`, `CWSO_HAL_ALLOW_INSECURE_ENDPOINTS`

### Sparse agents
- `CWSO_SPARSE_AGENTS_ENABLED`, `CWSO_SPARSE_SOCKET`, `CWSO_SPARSE_SLICE_MANIFEST`
- `CWSO_SPARSE_QUALITY_GUARDRAIL_ENABLED`, `CWSO_HHD_SPARSE_QUANTIZED_QUALITY_GUARDRAIL_MIN_SCORE`

### AST spike pipeline
- `CWSO_AST_SPIKE_MONITOR_ENABLED`, `CWSO_AST_SPIKE_RESOURCES_ENABLED`
- `CWSO_AST_SPIKE_{WINDOW_MS,THRESHOLD,SEMANTIC_THRESHOLD,...}`

### Rollout / Polar
- `CWSO_ROLLOUT_PROXY_ENABLED`, `CWSO_ROLLOUT_STORE_ENABLED`, `CWSO_ROLLOUT_REWARD_ENABLED`
- `CWSO_ROLLOUT_API_ENABLED`

All new capabilities default **off** with deterministic CPU/shadow fallback per plan guardrails.

## Release candidate verdict

**CONDITIONAL_PASS (RC_READY)**

Rationale:
- All four phase gates report Implementation **PASS** and Security **PASS**.
- T138 integration QA and trainer e2e tests merged to develop.
- Deferred items are documented, non-critical for PoC RC, and tracked (T135, audit hardening).

### Conditions for GA (`v0.3.0`)
- Stakeholder RC validation on published artifacts.
- Promote CI `govulncheck` / `cargo audit` from `allow_failure` (T094 follow-up).
- Risk acceptance or completion of T135 before production trainer fleet.

## Next release actions

1. Merge T139 MR !48 after CI green.
2. Tag `v0.3.0-rc1` on `develop` (or release branch per GitFlow).
3. Publish GitLab release with CHANGELOG excerpt and binary assets.
4. Capture RC feedback; open GA hardening tasks before `v0.3.0` promotion.
