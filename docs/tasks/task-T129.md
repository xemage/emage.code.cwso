# Task T129 — Sparse↔dense conflict-matrix conformance suite

> **ID note:** roadmap **Feature D / placeholder T103**. Active **T129** (see `active-tasks.md`).

- **Status:** in_review
- **Owner:** qa-engineer
- **Priority:** P0
- **Depends on:** T128 (sparse pre-filter integration)
- **Phase:** 8 — Semantic Sparse-Merging (**Feature D**)
- **Based on:** `docs/decisions/ADR-009-sparse-ast-tensor-encoding.md`, `docs/artifacts/sparse-ast-tensor-encoding-v1.md` §4

## Objective

Prove the sparse pre-filter path is advisory only: for every case in the merge test corpus,
`merge_three_way` (sparse) must produce identical output bytes and per-key conflict matrices as
the ADR-006 dense path (no sparse mask).

## Deliverables

- `services/cwso-merge-engine/src/merge.rs` — `merge_three_way_dense` + `merge_conflict_matrix` (test-only)
- `sparse_dense_conformance_full_corpus` test enumerating trivial, semantic, and insertion cases

## Acceptance Criteria

- [x] Dense reference path (`use_sparse_prefilter = false`) mirrors pre-T128 side-diff logic
- [x] Full corpus: trivial × 6 variants, semantic × 5 variants, insertion × 4 languages
- [x] Each case: `dense == sparse` for `Result<Vec<u8>, MergeError>`
- [x] Successful merges: per-key conflict matrix (`SideTag` × `DecisionTag`) matches
- [x] `cargo test -p cwso-merge-engine` green (25 tests)
- [x] `go test ./...` green in orchestrator
- [ ] CI green on MR !38
- [ ] MR merged (awaiting user approval per gate protocol)

## Notes

- **MR:** !38 — https://gitlab.com/em-age/emage.code.cwso/-/merge_requests/38
- **Branch:** `feature/T129-sparse-conformance` @ `7c2732a`
- **Pipeline:** #2577175850 (pending at last check)
