# Task T119 — Sparse Wasm micro-agent sandbox tier design + security envelope review

> **ID note:** roadmap **Feature B / placeholder T090** ("T0 Wasm sandbox tier design + security
> envelope review"). Active ID **T119** (see numbering reconciliation in `active-tasks.md`).

- **Status:** in_review
- **Owner:** solution-architect (reviewers: tech-lead, security-engineer)
- **Priority:** P0
- **Depends on:** T089 (Phase 6 gate) — and builds on the shipped Wasm host + sandbox tiers
- **Phase:** 7 — Sparse Micro-Agents & Spiking Monitors (**Feature B** — Ephemeral Wasm Micro-Agents)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md` §3.5 + §Feature B; `docs/decisions/ADR-003-tiered-sandbox-strategy.md`; `orchestrator/internal/dispatch/wasm_scoring_plugin.go`

## Objective
Produce the gating design for the T0 sparse Wasm micro-agent tier so the Feature B implementation
tasks (T120–T123) can proceed without further architectural decisions, and confirm the existing Wasm
security envelope is reused without regression.

## Context
Feature C (spiking AST monitors) is wired end-to-end (T115–T118). Feature B — the other half of
Phase 7 — has not started; its roadmap IDs (T090–T094) were consumed by Phase 6 follow-ups, so this
work takes fresh active IDs from T119. The blueprint specifies ephemeral 1.58-bit ternary Wasm
micro-agents with < 10 ms cold start, restricted to deterministic small edits, escalating to dense
GPU on a quality-floor breach. A hardened Wasm host already exists (`wasm_scoring_plugin.go`); this
task decides how to extend it into a data-side inference tier.

## Deliverables
- **`docs/decisions/ADR-008-wasm-sparse-agent-tier.md`** — runtime split (wazero control-side /
  wasmtime data-side sidecar), native ternary GEMM via host-call allowlist (vs full-Wasm kernel,
  deferred), COW mmap SHA-256-pinned weight slices, quality-floor escalation, security envelope reuse.
  Alternatives A–D tabulated; chosen = D.
- **`docs/artifacts/wasm-sparse-agent-design-v1.md`** — T0 tier placement vs ADR-003 tiers, ternary
  inference contract + `ternary_gemm` host-call, `.cwsl` weight packaging + COW mmap, the
  `create_ephemeral_sparse_agent` schema + lifecycle, `cwso://agents/{id}/telemetry` resource,
  security-envelope mapping table, and the T120–T125 implementation breakdown.

## Acceptance Criteria
- [x] T0 tier positioned relative to existing gVisor/Firecracker tiers (cold-start/isolation/use).
- [x] Runtime split decided + recorded (wazero control / wasmtime data sidecar over UDS).
- [x] Deterministic ternary inference contract + bounds-checked host-call allowlist defined.
- [x] Weight packaging + COW mmap + SHA-256 pinning specified (per-agent marginal RAM = activations).
- [x] `create_ephemeral_sparse_agent` schema + lifecycle + telemetry resource specified.
- [x] Quality-floor guardrail mapped to the existing `quality_guardrail_autodisable` path.
- [x] Security envelope reused verbatim — no new capabilities beyond `ternary_gemm` (+ telemetry).
- [x] Implementation task breakdown (T120–T125) with owners/priorities/dependencies.
- [x] Board + reconciliation mapping + roadmap numbering note updated.

## Notes / Follow-ups
- This is a docs/architecture task — no code changes. Implementation is sequenced into:
  **T120** (sidecar + GEMM + UDS), **T121** (slice packaging + COW mmap), **T122** (tool + schema +
  telemetry), **T123** (quality-floor escalation), **T124** (Phase 7 QA), **T125** (Phase 7 gate).
- The fully in-Wasm SIMD ternary GEMM is the **promotion path** and converges with Phase 8 Feature D's
  sparse-kernel contract; intentionally deferred out of the PoC.
