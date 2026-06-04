# ADR-009: Sparse AST tensor encoding for merge pre-filter

- **Status:** accepted (merged to `develop` via MR !35, 2026-06-04)
- **Date:** 2026-06-04
- **Decision-maker:** solution-architect
- **Tasks:** T126 (design), T127–T130 (implementation + gate)
- **Based on:** ADR-006, `cwso-nextgen-blueprint-v1.md` Feature D, Phase 7 complete (`146f208`)

## Context

`cwso-merge-engine` already performs deterministic AST-node merge (top-level units keyed by
`kind:name` or string-literal identity). Large repos spend merge time walking unchanged subtrees.
Feature D adds a **sparse pre-filter**: represent each side as a compressed tensor of *changed*
nodes only, diff with vectorized ops, then fall back to the existing dense path for resolution.

The kernel must be **photonic-ready** (pure function, no I/O) so the same contract can later run on
an optical/photonic co-processor via the HAL without algorithm changes.

## Decision

1. **Encoding:** CSR-style sparse row storage per merge file — each row is one **top-level AST unit**
   (same granularity as today's `AstUnit.key` in `merge.rs`). Unchanged units relative to `base` are
   **structural zeros** (omitted from the sparse tensor). Row payload is the unit's source bytes
   (opaque to the kernel; semantic interpretation stays in the dense merge).
2. **Node identity:** Reuse the existing unified symbol key (`kind:name` for declarations,
   `literal:<content>` for string literals). Keys are stable across sides for a given parse.
3. **Kernel contract:** Pure function
   `sparse_diff(base_sparse, ours_sparse, theirs_sparse) -> SparseDiffMask` returning per-key
   `{unchanged | modified_ours | modified_theirs | modified_both | inserted_ours | inserted_theirs |
   inserted_both | deleted_ours | deleted_theirs | deleted_both}` — sufficient to skip
   `build_side_diff` work for keys marked `unchanged` on both sides.
4. **Conformance:** Sparse output is advisory only. **ADR-006 dense merge remains authoritative**
   for conflict classification and serialized output. CI must prove sparse+dense produce identical
   conflict matrices (T129).
5. **Vectorization:** T127 implements the kernel with Rust `std::simd` (AVX-512 when available);
   scalar fallback required for portability. No I/O, no allocation in the hot loop beyond outputs.

## Alternatives considered

| Alt | Rejected because |
|-----|------------------|
| A — Line-based sparse diff | Violates blueprint; blind to AST structure |
| B — Full-tree sparse tensor (all nodes) | Higher encoding cost; top-level units already match merge granularity |
| C — Replace dense merge entirely | Risk to determinism; ADR-006 is production contract |

## Consequences

- (+) Large-repo merges skip unchanged top-level units before expensive diff/resolve.
- (+) Kernel boundary is HAL-offloadable later with zero contract change.
- (−) Two paths must stay in sync — conformance suite is mandatory (T129).
- (−) Insertion anchor logic still needs dense fallback when sparse mask is ambiguous.

## Implementation mapping (active IDs)

| Roadmap | Active | Title |
|---------|--------|-------|
| T100 | T126 | Sparse AST tensor encoding spec (this ADR) |
| T101 | T127 | AVX-512 / `std::simd` sparse diff kernel |
| T102 | T128 | Sparse pre-filter integration in merge engine |
| T103 | T129 | Sparse↔dense conflict-matrix conformance |
| T104 | T130 | Phase 8 Tech-Lead gate + large-repo benchmark |
