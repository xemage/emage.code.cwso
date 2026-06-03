# Task T120 — `cwso-sparse` sidecar: deterministic ternary GEMM kernel + UDS protocol

> **ID note:** roadmap **Feature B / placeholder T091** ("Wasm inference host + 1.58-bit ternary
> GEMM kernel"). Active ID **T120** (see numbering reconciliation in `active-tasks.md`).

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T119 (done — sparse Wasm micro-agent tier design + ADR-008)
- **Phase:** 7 — Sparse Micro-Agents & Spiking Monitors (**Feature B** — Ephemeral Wasm Micro-Agents)
- **Based on:** `docs/decisions/ADR-008-wasm-sparse-agent-tier.md`, `docs/artifacts/wasm-sparse-agent-design-v1.md`, `services/cwso-hal/src/ipc.rs` (protocol/peer-auth pattern)

## Objective
Stand up the data-side sidecar for the sparse micro-agent tier with its deterministic compute core
and wire protocol, so subsequent tasks (slice packaging T121, agent lifecycle/tool T122) have a
working `ternary_gemm` host-call to build on.

## Changes
- **New crate `services/cwso-sparse`** (added to `services/Cargo.toml` workspace members and to the
  `rust:test` CI job). Deps: `serde`, `serde_json`, `base64`, `libc`, `anyhow`, `thiserror`,
  `tracing(-subscriber)` — intentionally no `wasmtime` yet (see delivery note).
- **`src/gemm.rs`** — deterministic 1.58-bit ternary GEMM kernel. Weights ∈ {-1, 0, +1} packed as
  2-bit codes (4/byte, LSB-first: `00`→0, `01`→+1, `10`→−1, `11` invalid) + per-output-row `f32`
  scale. `Y[m,n] = scale ∘ (A[m,k] · Wᵀ)` via add/subtract/skip in fixed k-ascending order
  (byte-identical across runs). `TernaryWeights::{new, from_dense, gemm}`, `pack_ternary`,
  `packed_row_bytes`, typed `GemmError`.
- **`src/proto.rs`** — `Envelope<T>` + `Request` (`stat`, `ternary_gemm`) + untagged `Response`
  (`ok`/`err` with `reason_code`), mirroring the cwso-hal wire contract.
- **`src/ipc.rs`** — UDS server: 4-byte BE length-prefixed JSON frames, `SO_PEERCRED` peer-auth via
  `CWSO_IPC_ALLOWED_UIDS/GIDS`, per-connection thread, `dispatch()` over `Request`. The
  `ternary_gemm` op base64-decodes the packed matrix, builds `TernaryWeights`, and runs the kernel —
  the **only** compute capability exposed (no FS/network/process surface).
- **`src/main.rs`** — tracing init + socket bind (`CWSO_SPARSE_SOCKET`, default `/run/cwso/sparse.sock`).

## Acceptance Criteria
- [x] Deterministic ternary GEMM: output byte-identical across 1000 runs for fixed inputs.
- [x] Kernel matches an independent dense i8 reference (incl. `k` not a multiple of 4 / tail byte).
- [x] 2-bit packing verified (4/byte, LSB-first); non-ternary input and invalid `11` code rejected.
- [x] Shape-mismatch (scales/packed/activations) rejected with typed errors.
- [x] IPC `stat` + `ternary_gemm` ops; bad base64 / bad shape → `invalid_input` with `reason_code`.
- [x] `SO_PEERCRED` peer-auth rejects non-allowlisted peers (Linux).
- [x] `cargo test --release -p cwso-sparse` (15 tests) + `cargo fmt --check` + workspace build green.

## Notes / Follow-ups
- **Delivery split (ADR-008):** the wasmtime module-instantiation envelope is deferred to **T122**
  (agent lifecycle), where it is first exercised, to keep the heavy `wasmtime` dependency in one
  separately-reviewable change. T120 ships the sidecar, protocol, kernel, and host-call contract.
  The security envelope is unchanged — host-call surface is exactly `{ternary_gemm}`.
- No Dockerfile/deploy wiring yet (the sidecar isn't part of the running compose topology until the
  tool lands in T122); CI exercises it via `cargo test`.
- **T121** (next): `.cwsl` pruned-slice container format + COW mmap loader + SHA-256 pinning,
  feeding `TernaryWeights`. **T122**: `create_ephemeral_sparse_agent` tool + wasmtime instantiation
  + `cwso://agents/{id}/telemetry`. **T123**: quality-floor → dense escalation.
