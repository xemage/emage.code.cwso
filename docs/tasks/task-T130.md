# Task T130 — Phase 8 Tech-Lead gate + large-repo merge benchmark

> **ID note:** roadmap **Feature D / placeholder T104**. Active **T130** (see `active-tasks.md`).

- **Status:** done
- **Owner:** tech-lead / security-engineer
- **Priority:** P0
- **Depends on:** T129 (sparse↔dense conformance)
- **Phase:** 8 — Semantic Sparse-Merging (**Feature D**)
- **Based on:** ADR-009, `sparse-ast-tensor-encoding-v1.md`, tasks T126–T129, `gate-phase7-feature-bc-2026-06-04.md`

## Objective

Structured validation gate for Phase 8 Feature D (sparse AST merge pre-filter). Produce immutable
gate artifacts with `PASS | CONDITIONAL_PASS | FAIL` verdicts. Document large-repo merge benchmark
evidence for sparse pre-filter performance on mostly-unchanged corpora.

## Inputs

- Design: `docs/decisions/ADR-009-sparse-ast-tensor-encoding.md`, `docs/artifacts/sparse-ast-tensor-encoding-v1.md`
- Implementation: T127–T129 (MRs !35–!38 on `develop`)
- Conformance: `sparse_dense_conformance_full_corpus` (48 cases)
- Local evidence: `cargo test -p cwso-merge-engine`, `go test ./...`

## Expected Outputs

- `docs/artifacts/gate-phase8-feature-d-2026-06-04.md` — Tech-Lead + Security verdicts
- `docs/artifacts/security-phase8-checklist-v1.md` — OWASP-oriented control checklist
- `docs/benchmarks/phase8-large-repo-merge-benchmark-v1.md` — methodology + results
- `services/cwso-merge-engine/src/merge.rs` — large-repo equivalence + `#[ignore]` benchmark harness
- `docs/checkpoints/checkpoint-010-phase8-complete.md` (phase boundary)
- Task board: T130 → `in_review` with MR link; **do not merge MR until user requests**

## Acceptance Criteria

- [x] Implementation gate verdict with evidence-based findings table
- [x] Security gate verdict; ADR-009 determinism / no-I/O kernel verified
- [x] Large-repo benchmark documents sparse vs dense median timing
- [x] Combined outcome states whether Phase 8 Feature D may proceed
- [x] CI green on T130 MR (pipeline #2577485639 at `d77d08c`)
- [x] User approval before merge (gate protocol)

## Notes

- Review-only gate; benchmark harness is test-only (`#[cfg(test)]` / `#[ignore]` manual bench).
- Follows `validation-gates` skill (mirrors Phase 7 `gate-phase7-feature-bc-2026-06-04.md`).
- **Merged:** GitLab MR !39 → `develop` (merge `7dc4e7a`, source `d77d08c`).
