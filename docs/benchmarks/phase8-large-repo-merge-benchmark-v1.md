# Phase 8 Large-Repo Merge Benchmark v1 — Feature D

**Based on:** ADR-009, `sparse-ast-tensor-encoding-v1.md`, T128–T129 merge paths  
**Date:** 2026-06-04  
**Harness:** `services/cwso-merge-engine/src/merge.rs` (`large_repo_*` tests)

## Methodology

Synthetic Go three-way merge with **N** top-level `func unit_i()` declarations (structural zeros on
~N−2 keys). Two disjoint edits (`unit_0` on ours, `unit_1` on theirs). Compare:

- **Dense:** `merge_three_way_dense` (no sparse mask; byte-compare every base key)
- **Sparse:** `merge_three_way` (encode sparse tensors → `sparse_diff` → skip `BothUnchanged` diffs)

Run **15** iterations per path; report **median** wall time (`std::time::Instant`, release build).

Manual command:

```bash
cd services
cargo test -p cwso-merge-engine large_repo_merge_prefilter_benchmark --release -- --ignored --nocapture
```

CI runs `large_repo_sparse_dense_equivalence` only (500 units, correctness, no timing assert).

## Results (local, release, 2026-06-04)

| Units | Dense median | Sparse median | Ratio (dense/sparse) | Notes |
|-------|--------------|---------------|----------------------|-------|
| 2000 | 43.0 ms | 45.1 ms | 0.95× | Sparse encoding + mask build dominates skipped compares |
| 5000 | ~107 ms | ~112 ms | ~0.96× | Same trend (manual run) |
| 10000 | ~215 ms | ~225 ms | ~0.96× | Same trend (manual run) |

**Interpretation:** On CPU-only v1, the pre-filter is a **correctness guard** (T129) and integration
hook for future HAL/photonic offload, not a latency win at current unit counts. Skipping
`build_side_diff` byte work is offset by `encode_sparse_side` + `sparse_diff` for every merge.

## Success criteria (PoC)

| Criterion | Result |
|-----------|--------|
| Sparse ≡ dense output on 500+ unit fixture | **Pass** (`large_repo_sparse_dense_equivalence`) |
| Full 48-case conformance corpus | **Pass** (T129) |
| Median sparse faster than dense at 2k units | **Not met** (documented; non-blocking for gate) |

## Follow-ups

1. Profile `encode_sparse_side` / tensor allocation — cache masks per file revision in git-shadow.
2. HAL offload of `sparse_diff` hot loop (ADR-009 §5) before expecting merge latency wins.
3. Re-benchmark after mask caching lands; target ≥1.2× speedup at 5k+ units on unchanged-heavy repos.
