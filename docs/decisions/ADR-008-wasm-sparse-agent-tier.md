# ADR-008 — wasm-sparse-agent-tier

- **Status**: accepted
- **Date**: 2026-06-03
- **Decider(s)**: solution-architect, tech-lead, security-engineer
- **Tasks**: T119 (design), T120–T123 (implementation), T124–T125 (QA + gate)
- **Requirements**: `docs/artifacts/cwso-nextgen-blueprint-v1.md` §3.5, §Feature B; roadmap `plan-cwso-nextgen-phase6plus.md` Phase 7 (Feature B)

## Context

Phase 7 Feature B introduces **ephemeral sparse Wasm micro-agents**: a "T0" sandbox tier below
the existing gVisor/Firecracker container tiers (ADR-003). The goal is to replace heavy container
sub-agents (2–3 s cold start) with in-process, RAM-resident micro-agents that cold-start in
**< 10 ms** for small, deterministic edits (rename symbol, add type annotation, batch lint-fix),
running a **1.58-bit ternary (BitNet b1.58, weights ∈ {-1, 0, +1})** inference kernel.

Constraints:

- **Security must not regress.** The orchestrator already ships a hardened Wasm host
  (`dispatch/wasm_scoring_plugin.go`, wazero): SHA-256 module pinning, memory-page cap, wall-clock
  call timeout, explicit host-call allowlist, no WASI FS/network. ADR-003 established the tiered
  sandbox strategy; this tier must slot beneath it without weakening any envelope.
- **Determinism.** Hardware-aware dispatch (Phase 6) and the merge state machine assume reproducible
  sub-agent behavior. Ternary inference output must be deterministic for a given (weights, input).
- **Go/Rust split (ADR-001).** Control plane is Go; performance-critical data plane is Rust.
- **Maturity gap.** Compiling a full BitNet inference graph to Wasm is immature; a pragmatic PoC
  path is required with an explicit promotion path to a fully in-Wasm kernel.

## Decision

We adopt a **two-runtime, sidecar-hosted** design for the sparse micro-agent tier:

1. **Control-side scorers run in the existing wazero host (Go).** Tiny classifier/scorer micro-agents
   (e.g. the Feature C spike semantic-scorer seam) reuse `wasm_scoring_plugin.go`'s security envelope
   **verbatim**. No new runtime is introduced for these.

2. **Data-side inference runs in a new Rust sidecar `cwso-sparse` using `wasmtime`.** It follows the
   established `cwso-hal` pattern: a framed-JSON protocol over a Unix Domain Socket, peer-auth'd,
   owned and supervised by the Go orchestrator. The orchestrator never executes untrusted inference
   in-process.

3. **The ternary GEMM is a deterministic native Rust kernel invoked via a tight host-call allowlist**
   (not compiled into Wasm for the PoC). A thin Wasm orchestration module drives tokenization/layer
   loops inside the sandbox and calls the host `ternary_gemm` import for the hot matmul. This keeps
   the sandbox boundary while sidestepping the immature "full BitNet → Wasm" path. **Promotion path:**
   move the GEMM into Wasm + `std::simd`/relaxed-SIMD once a conformance suite exists (shares the
   Feature D sparse-kernel contract, ADR for Phase 8).

4. **Weights are content-addressed, SHA-256-pinned, pruned skill-slices, mmap'd copy-on-write.**
   A "skill domain" (e.g. `react-hooks`) is an unstructured-pruned ternary slice of a base model,
   packed `{-1,0,+1}` at 2 bits/weight + a scale vector. The host `mmap`s one read-only resident copy;
   N agents over the same slice share physical pages (COW), so per-agent marginal RAM is bounded by
   activations, not weights. This is what makes < 10 ms cold start and low `resident_ram_mb` feasible.

5. **Guardrail: micro-agents are restricted to deterministic small tasks; a quality-floor breach
   auto-escalates to a dense GPU model** by reusing the existing `quality_guardrail_autodisable`
   reason path in `policy_engine_v2.go` and routing through the Phase 6 HAL. Micro-agents are never
   the only path; the dense fallback is always reachable.

6. **Telemetry streams over the existing broker/SSE resource layer** (T117): `resident_ram_mb` +
   `tokens_per_sec` published to `cwso://agents/{wasm_agent_id}/telemetry`. No new transport.

The tier is **feature-flagged off by default** (`CWSO_SPARSE_AGENTS_ENABLED=false`), mirroring every
prior accelerator/Wasm feature.

## Alternatives considered

| Option | Pros | Cons | Why not chosen |
|--------|------|------|----------------|
| **A. Single runtime — run inference in the Go wazero host** | One runtime; reuse envelope as-is | wazero is interpreter/compiler-in-Go; ternary GEMM hot loop too slow for token SLO; pulls heavy compute into the control plane (violates ADR-001) | Rejected — perf + plane-separation |
| **B. Native Rust inference, no Wasm sandbox at all** | Simplest, fastest | Drops the Wasm security boundary for model-driven code paths; loses the "untrusted slice" isolation story | Rejected — security regression vs ADR-003 intent |
| **C. Full BitNet kernel compiled to Wasm now** | Purest "inference-in-Wasm"; no host-call for matmul | Immature toolchains; large effort before any working demo; hard to hit cold-start SLO early | Deferred — this is the promotion path, not the PoC |
| **D (chosen). wasmtime sidecar + native ternary GEMM via host-call allowlist + COW mmap weights** | Keeps sandbox boundary; reuses cwso-hal/UDS + wazero envelope; fast hot loop; clear promotion path | Two Wasm runtimes in the tree; host-call surface must be audited | **Chosen** — best balance of safety, speed, and incrementality |

## Consequences

- **Positive**: New T0 tier with < 10 ms target cold start; security envelope reused verbatim
  (SHA-256 pin, mem cap, timeout, host-call allowlist, no WASI FS/net, UDS peer-auth); deterministic;
  cleanly feature-flagged; telemetry/escalation reuse existing layers; explicit promotion path to
  fully in-Wasm SIMD GEMM (converges with Phase 8 Feature D).
- **Negative**: Two Wasm runtimes (wazero control-side, wasmtime data-side) increase the dependency
  and review surface; a new sidecar to build and supervise.
- **Risks introduced**: (1) **1.58-bit quality regression** → mitigated by deterministic-task
  restriction + quality-floor escalation (blueprint risk #4). (2) **host-call `ternary_gemm` is an
  escape surface** → mitigated by a single, bounds-checked, allowlisted import operating only on
  caller-provided buffers within the memory cap; no pointers to host state. (3) **weight-slice
  provenance** → mitigated by SHA-256 content addressing + pinning, identical to scoring modules.
- **Follow-ups**: tasks T120 (sidecar + GEMM host-call + UDS), T121 (skill-slice packaging + COW mmap
  loader), T122 (`create_ephemeral_sparse_agent` tool + schema + telemetry resource), T123
  (quality-floor → dense escalation), T124 (Phase 7 integration QA: cold start, 0% idle CPU,
  escalation), T125 (Phase 7 Tech-Lead + Security gate).

## Validation

- **Cold start**: `create_ephemeral_sparse_agent` reports `cold_start_ms < 10` (p95) for a warm,
  already-mmap'd skill slice, measured in T122/T124.
- **Memory sharing**: N concurrent agents over one slice add < (activation footprint) each;
  resident weight pages counted once (verified via `resident_ram_mb` telemetry + COW page accounting).
- **Determinism**: identical (slice, input) → byte-identical output across 1000 runs (T120 kernel test).
- **Security**: security-engineer confirms (T125) zero new host-call capabilities beyond the audited
  allowlist; no WASI FS/network; SHA-256 pin enforced; sidecar UDS peer-auth on.
- **Guardrail**: an injected low-quality result triggers `quality_guardrail_autodisable` and routes to
  the dense HAL backend (T123/T124).
