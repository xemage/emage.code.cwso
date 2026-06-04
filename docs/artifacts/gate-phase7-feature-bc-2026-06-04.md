# Phase 7 Validation Gate — Features B + C (Sparse Wasm Micro-Agents + Spiking AST Monitors)

**Target:** Phase 7 Feature B (T119–T124) + Feature C (T115–T118) integration on `develop`
**Based on:** `docs/artifacts/qa-phase7-report-v1.md`, `wasm-sparse-agent-design-v1.md`,
`docs/decisions/ADR-008-wasm-sparse-agent-tier.md`, `task-T118`…`task-T124`
**Date:** 2026-06-04
**Gate MR:** !34 merged 2026-06-04 → `develop` @ `146f208` (branch tip `70019c3`, pipeline #2575994520)

Scope reviewed (merged to `develop` via MRs !28–!34; T124 merge `eb4aa45`, T125 merge `146f208`):

- **Feature C (Go):** `ast_spike_monitor.go`, `ast_spike_filter.go`, `spike_subscriptions.go`,
  `write_event_sink.go`, `write_shadow_file` feeder, `ast_spike_tools.go`, MCP Resources +
  scoped SSE (`transport/http.go`), config-gated `CWSO_AST_SPIKE_*` flags.
- **Feature B (Rust `services/cwso-sparse`):** `gemm.rs`, `slice.rs`, `agent.rs`, `ipc.rs`,
  `proto.rs`; wasmtime sandbox + `{ternary_gemm}` host-call allowlist; UDS peer-auth.
- **Feature B (Go):** `internal/sparse` client, `sparse_agents.go`, `sparse_escalation.go`,
  `create_ephemeral_sparse_agent` tool + schema, server wiring, quality-floor guardrail (T123).
- **T124 QA guards:** cold-start p95, control-plane overhead, idle spike pipeline, server-level
  guardrail → HAL escalation.

Evidence base: `go test -race ./...` green on `develop` post-merge; `cargo test --release -p cwso-sparse`
27 tests green; T124 CI pipeline #2575895437 success at `e964e1b` (MR !33).

---

## Gate Verdict: Implementation Review

**Gate:** implementation
**Executor:** tech-lead
**Date:** 2026-06-04
**Target:** Phase 7 Features B + C (T118–T124)

### Verdict: PASS

### Findings

| # | Severity | Category | Description | Recommendation |
|---|----------|----------|-------------|----------------|
| 1 | medium | fidelity | T118 `write_shadow_file` feeder approximates symbols by file path + content SHA-256, not AST-symbol extraction via `query_ast`. Volume/semantic/conflict detection works but precision is file-granular. | Add AST-level symbol/signature extraction when `query_ast` integration is scheduled; document as known PoC limitation. |
| 2 | medium | feature gap | Default `HeuristicSemanticScorer` is deterministic/heuristic; the design's sparse Wasm mini-model scorer is not wired (seam exists on `ASTSpikeFilter.Scorer`). | Plug Wasm scorer when Feature B scorer slice is ready; until then heuristic is acceptable for PoC. |
| 3 | low | observability | `tokens_per_sec` in agent telemetry is placeholder (0) until an inference loop drives the wasm orchestration module. | Populate when token-generation path lands; non-blocking for gate. |
| 4 | low | performance | `sparse.Client` dials a fresh UDS connection per request (same pattern as early HAL client). | Acceptable at current volume; revisit if agent churn QPS grows. |
| 5 | low | complexity | Two Wasm runtimes (wazero control-side, wasmtime data-side) increase dependency/review surface (ADR-008 acknowledged). | Keep promotion path to in-Wasm GEMM documented; no action for PoC gate. |

### Summary

Phase 7 implementation is convention-compliant, feature-flagged off by default, and backed by
T124 integration guards for the advertised budgets (cold start p95, 0% idle spike emissions,
quality-floor escalation). Feature C is wired end-to-end (write → spike topics → `cwso://spikes`
resources). Feature B follows ADR-008: sidecar-hosted wasmtime, SHA-256-pinned slices, bounded
host-call surface, orchestrator-only MCP tool. No critical/high findings.

---

## Gate Verdict: Security Audit

**Gate:** security
**Executor:** security-engineer
**Date:** 2026-06-04
**Target:** Phase 7 Features B + C (T118–T124)

### Verdict: PASS

### Findings

| # | Severity | Category | Description | Recommendation |
|---|----------|----------|-------------|----------------|
| 1 | medium | supply chain | CI `go:audit` / `rust:audit` remain `allow_failure: true` (inherited from T094). Fresh advisories may not block merge. | Tighten to blocking before production hardening; track outside Phase 7 gate. |
| 2 | low | privacy | AST spike hot paths and symbol fields can leak filesystem structure unless `anomaly_notes_mode=drop` redaction is enabled (tests verify redaction). | Enable redaction in sensitive deployments; document in operator runbook. |
| 3 | low | deployment | Skill-slice manifest paths are operator-controlled JSON relative to the manifest directory; integrity is SHA-256-pinned but manifest content is trusted configuration. | Restrict manifest directory permissions; only ship manifests from trusted build pipeline. |

### Security controls verified (no findings)

- **ADR-008 envelope:** wasmtime sidecar has no WASI FS/network; `StoreLimits` memory cap;
  optional Wasm module requires `CWSO_SPARSE_WASM_MODULE_SHA256` when a module path is set;
  weight slices verified via SHA-256 before mmap; host-call surface is bounds-checked
  `ternary_gemm` only (IPC + linker tests).
- **IPC authz:** `cwso-sparse` UDS uses `SO_PEERCRED` UID/GID allowlist + `0o660` socket
  (mirrors `cwso-hal`); unauthorized peers rejected (`authorize_stream_rejects_unauthorized_peer`).
- **AuthZ:** `create_ephemeral_sparse_agent` is `RoleOrchestrator` only; `subscribe_ast_spikes`
  and AST monitors are config-gated (`CWSO_AST_SPIKE_*`, `CWSO_AST_SPIKE_RESOURCES_ENABLED`).
- **Input validation:** `skill_domain` required; `quantization` allowlist (1.58-bit implemented);
  `max_ram_mb` bounded by host cap; `quality_floor ∈ [0,1]`; spike subscription path/glob/workspace
  validated at tool registration.
- **Escalation safety:** quality-floor breach returns `quality_guardrail_autodisable` and routes
  to dense HAL without instantiating a sparse agent (T124 server integration test).
- **Secrets:** no API keys or tokens in sparse/AST code paths; env-var configuration only.
- **Feature flags:** `CWSO_SPARSE_AGENTS_ENABLED`, `CWSO_SPARSE_QUALITY_GUARDRAIL_ENABLED`, and
  AST spike flags default **false**.

Detailed OWASP-oriented checklist: `docs/artifacts/security-phase7-checklist-v1.md`.

### Summary

No critical or high security findings. The T0 sparse tier reuses established Wasm/UDS controls
without expanding host-call capabilities beyond the audited `ternary_gemm` import. Feature C
spike pipeline is event-driven with optional redaction and orchestrator-gated MCP exposure.

---

## Combined Gate Outcome

| Gate | Verdict |
|------|---------|
| Implementation (Tech-Lead) | **PASS** |
| Security | **PASS** |

**Phase 7 Features B + C are cleared to proceed** (PoC integration complete). Medium/low
findings are non-blocking enhancements (AST symbol fidelity, Wasm semantic scorer, audit job
hardening). No fix tasks required before closing T125.

**Closure:** T125 closed with MR !34 merge to `develop` (`146f208`, 2026-06-04). Phase 8 (Feature D)
kickoff: active **T126** (roadmap placeholder T100).
