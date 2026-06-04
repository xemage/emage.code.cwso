# Sparse AST Tensor Encoding v1 — Feature D Design

**Based on:** `docs/decisions/ADR-009-sparse-ast-tensor-encoding.md`, ADR-006,
`cwso-nextgen-blueprint-v1.md` §Feature D, `services/cwso-merge-engine/src/merge.rs`
**Status:** accepted (spec merged `develop` via MR !35) — gates T127–T130
**Phase:** 8 — Semantic Sparse-Merging

## 1. Problem

The merge engine diffs every top-level AST unit on each three-way merge. When `base`, `ours`, and
`theirs` agree on most units, work is wasted re-hashing and re-comparing unchanged regions. Feature D
introduces a **sparse representation** where unchanged units are structural zeros, plus a vectorized
`sparse_diff` kernel to classify changed keys before the dense ADR-006 path runs.

## 2. Sparse tensor layout

### 2.1 Granularity

One sparse **row** = one top-level `AstUnit` (same as `extract_top_level_units` today):

- Declarations: key `"{kind}:{name}"` (e.g. `function:main`, `class:Foo`).
- String literals at file top level: key `literal:{content}`.

### 2.2 CSR-style encoding (`SparseAstTensor`)

```text
magic: "SPAT" (4 bytes)
version: u16 = 1
row_count: u32          # number of non-zero (changed vs base) rows
keys: [row_count] UTF-8 key strings (length-prefixed)
payload_lens: [row_count] u32
payloads: concatenated source bytes per row (opaque blob)
base_order_hint: optional u32[] — indices into base key order for assembly
```

- **Structural zero:** a base key absent from `ours` sparse tensor ⇒ unchanged on ours side (or
  deleted — dense path still resolves deletions).
- **Non-zero row:** unit text differs from base (or is an insertion not in base).

Encoding is built in `cwso-merge-engine` after `extract_top_level_units`; no change to tree-sitter
parse rules.

### 2.3 Building sparse tensors

```rust
fn encode_sparse_side(base: &[AstUnit], side: &[AstUnit]) -> SparseAstTensor
```

- Walk `side` units; include row iff `side_text != base_text` for that key, or key not in base
  (insertion).
- Sort rows by `base_order_hint` when present else lexicographic key order (deterministic).

## 3. `sparse_diff` kernel contract (photonic-ready)

```rust
/// Pure, no I/O. Inputs are three sparse tensors for one file.
pub fn sparse_diff(
    base_keys: &[String],           // ordered base key list (dense reference)
    ours: &SparseAstTensor,
    theirs: &SparseAstTensor,
) -> SparseDiffMask;
```

`SparseDiffMask` is a `BTreeMap<String, SparseRowClass>` where `SparseRowClass` is:

| Class | Meaning | Dense merge may skip |
|-------|---------|----------------------|
| `BothUnchanged` | Not present in either sparse tensor | Yes — copy base unit |
| `OursOnly` | Only ours sparse has row | No — need ours diff |
| `TheirsOnly` | Only theirs sparse has row | No |
| `BothModified` | Both sparse tensors have row | No — conflict check |
| `DisjointInsert` | Insertions on disjoint anchors | No (anchor merge) |

The kernel compares **key sets and payload hashes** (BLAKE3 of payload bytes) using SIMD where
available; it does not parse AST.

### 3.1 Integration hook (T128)

In `merge_three_way`:

1. Parse → `base_units`, `ours_units`, `theirs_units` (unchanged).
2. `mask = sparse_diff(...)`.
3. For keys with `BothUnchanged`, seed `NodeState::Unchanged` without `build_side_diff` byte compare.
4. Run existing `resolve_base_decisions` / `merge_insertions` / `assemble_output` for all non-skipped keys.

Incorrect sparse classification must not change output — T129 conformance proves equivalence.

## 4. Conformance rule

For every fixture in the merge test corpus:

```text
dense_merge(base, ours, theirs) == sparse_prefilter_merge(base, ours, theirs)
```

Conflict presence and `MergeError::SemanticConflict` must match. Output bytes must be identical.

## 5. Photonic / HAL offload (future)

The kernel is a pure `fn(sparse_a, sparse_b) -> mask` with fixed-size numeric payloads (key hashes).
Feature A HAL can register a `merge-sparse-diff` provider later; T127 ships CPU SIMD first.

## 6. Security & determinism

- No network/filesystem in kernel.
- Key ordering fixed (BTreeMap / sorted keys).
- Payload comparison uses constant-time hash equality for equal-length payloads in hot path optional;
  correctness path uses byte equality fallback.

## 7. Implementation breakdown

| Active ID | Roadmap | Title | Owner | Pri | Depends |
|-----------|---------|-------|-------|-----|---------|
| T126 | T100 | Sparse AST tensor encoding spec | solution-architect | P0 | T125 |
| T127 | T101 | `std::simd` / AVX-512 `sparse_diff` kernel | backend-developer | P0 | T126 |
| T128 | T102 | Merge-engine pre-filter hook | backend-developer | P0 | T127 |
| T129 | T103 | Sparse↔dense conformance suite | qa-engineer | P0 | T128 |
| T130 | T104 | Phase 8 Tech-Lead gate + large-repo benchmark | tech-lead | P0 | T129 |

## 8. Out of scope (v1)

- Sub-top-level node sparse tensors (future v2).
- Changing `merge_concurrent_results` MCP payload (ADR-006 addendum unchanged).
- Photonic hardware dispatch (HAL registration only sketched).
