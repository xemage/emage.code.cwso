# Task C041 — Parent-commit tracking per workspace

**ID:** C041
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C020–C025 (gate CG2), C030–C034 (gate CG3)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B7, P2-4); docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md

## Objective

Shadow workspaces currently produce **orphan commits** — no parent tracking — so there
is no history chain and no basis for a three-way merge. Track HEAD per workspace and
pass the parent into `repo.commit`, so each workspace forms a real chain. This unblocks
C042 (three-way merge).

## Inputs

- `services/cwso-git-shadow/src/repo.rs:180` (the P2-4 orphan-commit marker)
- `services/cwso-git-shadow/src/main.rs` (workspace state: where HEAD should live)

## Rails (read before starting)

### You MUST
- Track the current HEAD commit per workspace (in the workspace state)
- Pass the current HEAD as the parent when committing; update HEAD after each commit
- Handle the first commit in a workspace (no parent — a legitimate root commit, not an "orphan" by accident)
- Add tests: two sequential commits in one workspace form a chain (`git log` shows parent linkage)
- Remove the P2-4 marker and update `docs/DEBT-REGISTER.md` (B7 → `fixed`, closing task C041)

### You MUST NOT
- Change the `commit_shadow` tool signature (parent is derived from workspace state, not a new parameter)
- Implement merging — that is C042, which consumes this task's parents
- Touch the merge-engine or orchestrator
- Break existing single-commit behavior (first commit still works)

## File ownership

- **May create/modify:** `services/cwso-git-shadow/**`, `docs/DEBT-REGISTER.md` (B7 row)
- **Must NOT touch:** `orchestrator/*`, other services, `schemas/*`

## Steps (execute in order)

1. Read `repo.rs` commit path and workspace state.
2. Add HEAD tracking; thread parent into commit.
3. Tests: chain of ≥2 commits; first-commit root case.
4. Remove marker; update DEBT-REGISTER.

## Expected outputs

- Parent-tracked commits in `cwso-git-shadow`
- Chain tests
- P2-4 marker removed; DEBT-REGISTER updated

## Acceptance criteria

1. `git log` in a shadow workspace shows a chain, not orphans
2. First commit is a proper root commit
3. `cargo test -p cwso-git-shadow` passes
4. DEBT-REGISTER B7 = `fixed` / C041

## Verification commands

```bash
cargo test -p cwso-git-shadow commit
grep -n "P2-4" services/cwso-git-shadow/src/repo.rs   # = no hits
```

## Git rails

- Branch: `agent/backend-developer/C041` from `develop`
- Commit: `fix(git-shadow): track parent commits per workspace`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

<filled during execution>
