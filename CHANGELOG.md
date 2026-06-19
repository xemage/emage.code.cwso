# Changelog

All notable changes to this project are documented in this file.

## v0.4.0 - 2026-06-09

### Polar parity and operator readiness
- Polar **harness adapter registry** and Docker runtime launcher with shell-command reference
  harness (T144, carried from v0.3.0 scope).
- Rollout **`num_samples`** session fan-out (1–32) with per-session callbacks (T145).
- **Gateway async staging** — INIT/READY/RUNNING/POSTRUN worker pools, evaluator prewarm stub,
  and partial trace recovery on session timeout (T146).
- **Evaluator registry** with merge-SM session reward and SWE-bench stub hook (T148).
- **Trajectory builder v2** — `per_request` and `prefix_merge` strategies, EOT interstitial
  masking, partition-key chain splitting; per-task `trajectory_builder_strategy` on submit (T149).
- Comprehensive **installation guide v2** (`docs/user/installation-v2.md`) — architecture, full
  `CWSO_*` flag reference, MCP/rollout/gateway/evaluator workflows (T156).
- **IDE integration guide** for VS Code / Cursor MCP + rollout proxy routing (T154).
- **`scripts/cwso-enable-all-features.sh`** and env example for local PoC demos (T155).
- CI **tag pipeline deploy fix** — `deploy:registry` uses `needs:optional` on `e2e:phase2` (T153).

### Documentation
- Primary adoption doc for v0.4.0: [`docs/user/installation-v2.md`](docs/user/installation-v2.md).
- Release artifact: [`docs/artifacts/release-v0.4.0.md`](docs/artifacts/release-v0.4.0.md).

### Deferred post-GA
- T150 — KV differential prompting.
- T151 — Offline SFT data generation mode.

## v0.3.0 - 2026-06-07

### Post-RC hardening and operator readiness
- Installation and usage guide (`docs/user/installation-v1.md`) for Docker quick start,
  JWT, MCP HTTP, Phase 4/Next-Gen flags, and troubleshooting (T142).
- OpenAI **Responses API** route (`/v1/responses`) with provider-specific synthetic SSE
  and capture pipeline hardening (T147).
- Polar **harness adapter registry** and Docker runtime (start/stop/exec/upload/download)
  with shell-command reference harness and proxy-capture e2e (T144).
- CI e2e hardening: MCP RPC retry on transient connection errors in phase2 integration.

### Operations (carried from RC)
- KV prefix router (T135, default-off), blocking `go:audit` / `rust:audit` (T140).
- Phases 6–9 feature set unchanged from `v0.3.0-rc1`; see RC CHANGELOG for full scope.

### Deferred post-GA
- Polar parity T145–T151 (session fan-out, gateway staging, evaluators, trajectory parity,
  differential prompting, offline SFT).

## v0.3.0-rc1 - 2026-06-06

### Phase 6 — Heterogeneous Hardware Dispatcher (Feature A)
- Rust `cwso-hal` sidecar with `InferenceBackend` trait, CPU baseline, and optional GPU/LPU
  OpenAI-compatible adapters (`T082`–`T084`).
- Workload profiler and `dispatch_hardware_aware_job` MCP tool with live HAL execution,
  capability sync, and deterministic fallback chain (`T085`–`T087`).
- Context propagation, active health probing, TLS endpoint validation, and CI dependency
  audits (`T090`–`T094`, `T114`).
- Phase 6 gate **PASS/PASS** per `gate-phase6-feature-a-2026-06-02.md`.

### Phase 7 — Sparse Micro-Agents & Spiking Monitors (Features B + C)
- `cwso-sparse` sidecar: deterministic 1.58-bit ternary GEMM, `.cwsl` mmap loader,
  wasmtime agent lifecycle (`T119`–`T122`).
- `create_ephemeral_sparse_agent` MCP tool and quality-floor → dense GPU escalation (`T123`).
- AST write-spike monitor, semantic filter, conflict pre-warning, and `subscribe_ast_spikes`
  MCP resources with write-event feeder (`T115`–`T118`).
- Phase 7 gate **PASS/PASS** per `gate-phase7-feature-bc-2026-06-04.md`.

### Phase 8 — Semantic Sparse-Merging (Feature D)
- Sparse AST tensor encoding spec (ADR-009) and AVX2 sparse diff kernel (`T126`–`T127`).
- Sparse pre-filter in `merge_three_way` and sparse↔dense conformance suite (`T128`–`T129`).
- Large-repo merge benchmark and Phase 8 gate **PASS/PASS**
  (`gate-phase8-feature-d-2026-06-04.md`, `T130`).

### Phase 9 — Rollout-as-a-Service (Features E + F + G)
- `cwso-rollout` hyper reverse proxy with zero-copy capture (`T132`).
- Trajectory builder with prefix merging and Parquet/LZ4 trajectory store (`T133`–`T134`).
- Programmatic merge rewards (+1/−1) and Polar REST API for trainer e2e (`T136`–`T137`).
- Phase 9 integration QA and gate **PASS/PASS** (`qa-phase9-report-v1.md`,
  `gate-phase9-feature-efg-2026-06-05.md`, `T138`).

### Operations and Documentation
- Release readiness artifact: `docs/artifacts/release-v0.3.0-rc1.md`.
- Checkpoints 007–011 cover Phases 6–9 completion on `develop` @ `5d2cfca`.

### CI / Gates
- T138 merged via MR !47 (squash `011d8c8`); pipeline green on feature branch pre-merge.
- All new capabilities ship default-off behind `CWSO_*` flags.

### Known residual risk (RC)
- KV prefix router (T135) and trainer fleet proxy benchmark deferred.
- CI `govulncheck` / `cargo audit` remain `allow_failure: true` (T094 PoC posture).
- Orchestrator `/v1/chat/completions` is a 501 stub; transparent proxy on `cwso-rollout`.

## v0.2.0-rc1 - 2026-05-24

### Phase 5 Hardening Closure
- Closed all security hardening follow-ups from Phase 5 conditional pass:
  - T073: Wasm module integrity verification (SHA-256 pin + trusted path)
  - T074: Telemetry minimization/redaction policy (request ID and anomaly notes)
  - T075: eBPF latency semantics hardening (explicit advisory signaling)
- Updated dispatch telemetry and anomaly contracts to reduce false precision and
  sensitive-field exposure while preserving deterministic fallback behavior.

### Operations and Documentation
- Expanded hardware-aware operator guidance in README with:
  - mandatory Wasm integrity controls (`CWSO_HHD_WASM_SCORING_MODULE_SHA256`,
    `CWSO_HHD_WASM_SCORING_TRUSTED_DIR`)
  - telemetry redaction controls (`CWSO_HHD_TELEMETRY_*`)
  - explicit advisory interpretation for `ebpf-hook` latency fields.
- Added release-candidate readiness artifact for v0.2.0-rc1.

### CI / Gates
- Release-candidate validation reached green pipeline on `develop` after the
  final hardening changes (`2548879153`).
- No open active tasks remain in Phase 5 scope.

## v0.1.1 - 2026-05-22

### Release Blockers Closed
- Closed all tracked post-v0.1.0 release blockers from the Phase 4 conditional pass:
  - T054: merge-engine unit-test CI gate requirement
  - T055: `merge_inputs` schema/runtime alignment
  - T056: ADR-006 reconciliation for node-level conflict-detail scope
  - T057: e2e policy-path validation for sidecar reason mapping
- Reconciled task board state to reflect blocker completion and current non-blocking deferrals.

### Documentation
- Updated [README.md](README.md) with a clearer "What CWSO is" overview and a
  practical "How to use CWSO" section covering startup, auth, MCP invocation,
  and validation commands.
- Added [release-v0.1.1 artifact](docs/artifacts/release-v0.1.1.md) with scope,
  validation, and release readiness summary.

### CI / Gates
- Release-ready baseline confirmed on `develop` with green lint/build/test/e2e
  pipeline status prior to release packaging.

## v0.1.0 - 2026-05-16

### Added
- Phase 1 foundation (T001-T011): requirements and architecture baselines, security baseline, Go orchestrator MCP server core, baseline filesystem tools, Streamable HTTP transport skeleton, and HS256 + Origin controls.
- Phase 2 shadow workspace + AST (T020, T022, T026, T028, T029): Rust `cwso-git-shadow` sidecar, UDS shadow client/tools, end-to-end integration harness, and PoC debt remediation pass.
- Phase 3 transport + concurrency (T030-T038): full-duplex SSE transport, async job runner pool, concurrent dispatch tool, event-sourced memory broker, telemetry throttling, and completed tech-lead/security gates.
- Phase 4 sandbox + merge pipeline (T040-T050): Docker/gVisor/Firecracker runner path, sandbox tier router, Rust merge engine, AST semantic merge flow, conflict-matrix escalation, and matrix-aware swarm e2e suite.

### Security
- Security gate T051 re-audit passed after remediation completion (see checkpoint-020).
- T058 hardened sidecar IPC socket permissions and Linux peer authorization.
- T059 added baseline HTTP security headers in transport middleware.
- T060 enforced `application/json` Content-Type for `POST /mcp`.
- T061 removed RS256 ambiguity by constraining current build/runtime to HS256.

### Testing and Validation
- Phase 1 review gate: PASS (checkpoint-001).
- Phase 2 integration validation: PASS for sidecar + shadow workspace + AST flows (checkpoint-002).
- Phase 3 tech-lead and security gates: PASS (checkpoint-008).
- Phase 4 quality gate: CONDITIONAL_PASS with tracked follow-up items (checkpoint-018).
- Security re-audit gate: PASS (checkpoint-020), with evidence:
  - `cargo test -p cwso-git-shadow -p cwso-merge-engine` (Rust sidecars): PASS.
  - `go test ./internal/config ./internal/transport` (orchestrator): PASS.

### Notes / Known Residual Risk
- Non-Linux peer-credential fallback remains permissive; acceptable for current Linux deployment scope, but must be revisited if portability scope expands.
- HSTS effectiveness depends on HTTPS termination configuration in deployment.
- T050 follow-up conditions remain tracked as open work for post-v0.1.0 hardening/alignment: T054, T055, T056, T057.