# Task T128 — Sparse pre-filter integration in merge engine

> **ID note:** roadmap **Feature D / placeholder T102**. Active **T128** (see `active-tasks.md`).

- **Status:** done
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T127 (sparse_diff kernel)
- **Phase:** 8 — Semantic Sparse-Merging (**Feature D**)
- **Based on:** `docs/decisions/ADR-009-sparse-ast-tensor-encoding.md`, `docs/artifacts/sparse-ast-tensor-encoding-v1.md` §3.1

## Objective

Wire the ADR-009 `sparse_diff` mask into `merge_three_way` before `resolve_base_decisions`, skipping
unchanged top-level units early while preserving dense ADR-006 merge semantics for changed and
conflicting rows.

## Deliverables

- `services/cwso-merge-engine/src/merge.rs` — `sparse_prefilter_mask` + mask-aware `build_side_diff`
- Unit tests: mask seeding for `BothUnchanged`, integration parity with semantic fixtures

## Acceptance Criteria

- [x] `sparse_diff` runs after unit extraction, before side diff state build
- [x] `BothUnchanged` keys seed `NodeState::Unchanged` without per-side byte compare
- [x] Changed/conflict rows use existing `lookup_side_state` / `resolve_base_decisions` path
- [x] Insertion anchor logic unchanged
- [x] `cargo test -p cwso-merge-engine` green (24 tests)
- [x] `go test ./...` green in orchestrator
- [x] Sparse↔dense full corpus conformance (T129, MR !38)

## Notes

- Conformance suite (T129) proves sparse pre-filter ⊆ dense outcome across the full merge corpus.
- **Merged:** GitLab MR !37 → `develop` (`7f489b4`, source `aa509fb`).
- T130 gate follows T129 conformance.
