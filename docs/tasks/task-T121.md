# Task T121 — `.cwsl` pruned-slice container + COW mmap loader + SHA-256 pinning

> **ID note:** roadmap **Feature B / placeholder T092** ("Pruned skill-slice packaging + COW weight
> mmap"). Active ID **T121** (see numbering reconciliation in `active-tasks.md`).

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T120 (done — `cwso-sparse` sidecar + deterministic ternary GEMM kernel)
- **Phase:** 7 — Sparse Micro-Agents & Spiking Monitors (**Feature B** — Ephemeral Wasm Micro-Agents)
- **Based on:** `docs/decisions/ADR-008-wasm-sparse-agent-tier.md` §Decision(4), `docs/artifacts/wasm-sparse-agent-design-v1.md` §5

## Objective
Give the ternary GEMM kernel its weight-supply path: a content-addressed, SHA-256-pinned `.cwsl`
slice container and a read-only mmap loader so N ephemeral agents share one resident copy of a
pruned skill slice (the copy-on-write story), with integrity verified before use.

## Changes
- **`gemm.rs` (refactor):** extracted a borrowed `TernaryView<'a>` holding `scales: &[f32]` +
  `packed: &[u8]`; moved the GEMM loop + dimension validation there (shared `validate_dims`). Owning
  `TernaryWeights` now exposes `as_view()` and delegates `gemm`. Enables zero-copy inference over an
  mmap. All T120 kernel tests preserved.
- **`slice.rs` (new):**
  - **`.cwsl` container** — 28-byte LE header (`CWSL` magic, `format_version`, `quantization`,
    `n`, `k`, `scale_count`, `packed_len`) + `f32` scales + 2-bit-packed ternary weights.
    `serialize(n,k,scales,packed)`, `SliceHeader::parse`, `content_hash` (SHA-256 hex).
  - **`MappedSlice::open(path, expected_sha256)`** — `memmap2` read-only map (shared resident
    weight pages), length check, **SHA-256 integrity pin** (hard error on mismatch / bad pin),
    kernel-contract dimension validation; materialises only the small scale vector and exposes a
    zero-copy `view()` borrowing `packed` from the mmap.
  - **`SliceManifest`** — JSON `{ "slices": [{ skill_domain, path, sha256 }] }`, relative-path
    resolution, `domains()`, `get()`, `load_slice(domain)` → verified `MappedSlice`.
- **Deps:** add `memmap2`, `hex` (`sha2` already in the workspace). `Cargo.lock` updated.

## Acceptance Criteria
- [x] `.cwsl` serialize/parse round-trips; `total_len()` matches encoded size.
- [x] `content_hash` is the SHA-256 hex content address; `open` recomputes and pins against it.
- [x] `open` runs GEMM directly from the mmap (`view().gemm`) — weights not copied per agent.
- [x] Integrity mismatch, non-hex pin, truncation, bad magic, unsupported version, and length
      mismatch are all rejected with typed errors.
- [x] `SliceManifest` loads JSON, resolves relative paths, returns a verified slice per domain, and
      errors on unknown domains.
- [x] Borrowed `TernaryView` parity with owning `TernaryWeights` (shared kernel path).
- [x] `cargo test --release -p cwso-sparse` (24 tests) + `cargo fmt --check` + workspace build green.

## Notes / Follow-ups
- **Read-only mapping = sharing.** A read-only `Mmap` of one file is inherently shared by the OS
  across mappings, which is exactly the per-agent RAM saving the design wants; only the tiny scale
  vector is materialised per slice.
- **No slice-producer/training here.** Generating pruned `.cwsl` slices from a base model is out of
  scope (Phase 9 / Feature E); `serialize` is the canonical writer used by tests and any future
  packer.
- **T122** (next) consumes this loader: `create_ephemeral_sparse_agent` resolves a `skill_domain`
  via the manifest, opens the pinned slice, instantiates the wasmtime orchestration module, and
  streams `cwso://agents/{id}/telemetry`.
