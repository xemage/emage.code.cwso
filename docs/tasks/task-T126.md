# Task T126 — Sparse AST tensor encoding spec (photonic-ready kernel contract)

> **ID note:** roadmap **Feature D / placeholder T100** ("Sparse AST tensor encoding spec").
> Active ID **T126** (see numbering reconciliation in `active-tasks.md`).

- **Status:** in_review
- **Owner:** solution-architect (reviewers: tech-lead, backend-developer)
- **Priority:** P0
- **Depends on:** T125 (Phase 7 gate)
- **Phase:** 8 — Semantic Sparse-Merging (**Feature D**)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md` §Feature D; `docs/archive/decisions/ADR-006-semantic-ast-merge.md`; `services/cwso-merge-engine/src/merge.rs`

## Objective

Define the sparse AST tensor representation and the pure `sparse_diff` kernel contract so Phase 8
implementation (T127–T130) can add a vectorized pre-filter to `cwso-merge-engine` without changing
ADR-006 conflict semantics.

## Deliverables

- **`docs/decisions/ADR-009-sparse-ast-tensor-encoding.md`** — encoding choice, zero semantics,
  photonic-ready kernel boundary, conformance rule (sparse pre-filter ⊆ dense outcome).
- **`docs/artifacts/sparse-ast-tensor-encoding-v1.md`** — wire format, node identity keys, CSR-style
  layout, `sparse_diff` I/O contract, merge-engine integration points, T127–T130 breakdown.

## Acceptance Criteria

- [x] Sparse tensor layout specified (node keys, structural zeros, stable ordering).
- [x] `sparse_diff(base, ours, theirs) -> SparseDiff` contract is pure (no I/O) and photonic-ready.
- [x] Conformance rule documented: sparse pre-filter must not change final conflict matrix vs dense.
- [x] Integration points in `cwso-merge-engine` identified (pre-filter hook before `resolve_base_decisions`).
- [x] Implementation breakdown T127–T130 with owners, priorities, dependencies.
- [x] Board + reconciliation mapping updated.

## Notes

- Docs/architecture only on this task; kernel + wiring land in T127–T128.
- Promotion path from T120 native ternary GEMM converges here as the merge-side sparse kernel.
