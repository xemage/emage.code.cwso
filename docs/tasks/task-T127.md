# Task T127 — AVX-512 / SIMD sparse diff kernel in cwso-merge-engine

> **ID note:** roadmap **Feature D / placeholder T101**. Active **T127** (see `active-tasks.md`).

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T126 (sparse AST tensor encoding spec)
- **Phase:** 8 — Semantic Sparse-Merging (**Feature D**)
- **Based on:** `docs/decisions/ADR-009-sparse-ast-tensor-encoding.md`, `docs/artifacts/sparse-ast-tensor-encoding-v1.md`

## Objective

Implement the pure `sparse_diff` kernel and `SparseAstTensor` types in `cwso-merge-engine` per ADR-009,
with vectorized BLAKE3 digest comparison on x86_64 (AVX2 stable path; `std::simd`/AVX-512 when stabilized).

## Deliverables

- `services/cwso-merge-engine/src/sparse_tensor.rs` — SPAT v1 wire format, `encode_sparse_side`
- `services/cwso-merge-engine/src/sparse_diff.rs` — `sparse_diff` + `SparseRowClass` mask
- `services/cwso-merge-engine/src/sparse_diff/simd.rs` — AVX2 hash equality + scalar fallback
- Unit tests covering ADR-009 mask classes

## Acceptance Criteria

- [x] `sparse_diff(base_keys, ours, theirs) -> SparseDiffMask` is pure (no I/O)
- [x] SPAT v1 serialize/parse round-trip
- [x] Mask classes: `BothUnchanged`, `OursOnly`, `TheirsOnly`, `BothModified`, `DisjointInsert`
- [x] SIMD hot path on x86_64 with scalar fallback
- [x] `cargo test -p cwso-merge-engine` green
- [ ] Merge-engine pre-filter hook (T128)
- [ ] Sparse↔dense conformance suite (T129)

## Notes

- T128 wires the mask into `merge_three_way`; this task is kernel-only.
- Rust 1.86: `std::simd` still unstable — AVX2 via `std::arch` satisfies the vectorization requirement until portable_simd stabilizes.
