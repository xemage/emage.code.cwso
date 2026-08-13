# Task C042 — Three-way merge + conflict matrix

**ID:** C042
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C041
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B7); docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md; input/CWSO_ Agentic AI Orchestration Blueprint.md §5.4

## Objective

Now that C041 supplies parents, enable a genuine three-way merge in
`merge_concurrent_results`. Where a merge is unresolvable, return the **conflict
matrix** the Blueprint §5.4 promises — never a corrupted file.

## Inputs

- C041 (parent commits)
- `services/cwso-merge-engine/` (the merge sidecar)
- Blueprint §5.4 (the conflict-matrix contract — read it before designing the response)
- `schemas/merge_concurrent_results.json`

## Rails (read before starting)

### You MUST
- Implement three-way merge using the common ancestor (from the now-chained history)
- On unresolvable conflict, return a structured conflict matrix per Blueprint §5.4 (deterministic conflict classes and reason codes) — as **data**, per the existing schema
- Add tests: (a) a genuine three-way merge succeeds; (b) an unresolvable merge returns a conflict matrix and **no file is corrupted** (the pre-merge state is preserved)
- Keep the merge deterministic — same inputs → same outputs, always
- Update `docs/DEBT-REGISTER.md` if any shortcut is introduced

### You MUST NOT
- Ever write a partially-merged/corrupted file — conflict matrix or clean merge, nothing in between
- Change the `merge_concurrent_results` schema shape (extend within it if the Blueprint requires; flag any schema change to the orchestrator before making it)
- Implement merge *strategies* beyond the Blueprint §5.4 contract
- Touch git-shadow's projection code (C021–C023) or the orchestrator

## File ownership

- **May create/modify:** `services/cwso-merge-engine/**`, `services/cwso-git-shadow/**` (only if ancestor lookup requires it — justify in MR), `docs/DEBT-REGISTER.md` (only if new debt)
- **Must NOT touch:** `orchestrator/*`, `schemas/*` (without orchestrator sign-off), other services

## Steps (execute in order)

1. Read Blueprint §5.4 and the current merge-engine.
2. Implement ancestor-based three-way merge.
3. Implement the conflict-matrix return for unresolvable cases.
4. Tests: clean merge + conflict matrix + no-corruption.
5. Verification.

## Expected outputs

- Three-way merge in `cwso-merge-engine`
- Conflict-matrix return per §5.4
- Tests for both outcomes

## Acceptance criteria

1. A genuine three-way merge succeeds
2. An unresolvable merge returns a conflict matrix; pre-merge state intact
3. Merge is deterministic across repeated runs
4. `cargo test -p cwso-merge-engine` passes

## Verification commands

```bash
cargo test -p cwso-merge-engine
cargo test -p cwso-git-shadow
```

## Git rails

- Branch: `agent/backend-developer/C042` from `develop` (rebased on merged C041)
- Commit: `feat(merge-engine): three-way merge with conflict matrix`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the Blueprint §5.4 contract is ambiguous, cite the ambiguity and report
`unclear_requirements` / `major` — do not invent a conflict format.

## Execution notes

<filled during execution>
