# Task T177 - Fix develop/main ancestry break (redo v0.5.0 back-merge as a real merge)

- **Status:** in_progress
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** —
- **Based on:** `.claude/rules/git-workflow.md` (GitFlow), discovered while executing T174

## Context

While cutting `release/v0.5.1` from `develop` and opening the MR into `main` (T174), the MR
(`!86`) came back `cannot_be_merged` / `conflict` with **292 conflicting files**.

Root cause: `docs/tasks/task-T168.md`'s "back-merge main into develop" (commit `677f9db`,
2026-07-27) has **only one parent** — it is a flat/squashed commit that copied `main`'s content
into `develop`, not a real two-parent merge. `git merge-base --is-ancestor origin/main
origin/develop` fails right now, independent of any T169-T176 work. `main`'s tip (`dd6fbb4`, the
v0.5.0 merge) was therefore never recorded as a git ancestor of `develop`.

Verified via `git merge-tree` in a disposable scratch clone (no branches touched): every one of
the 292 conflicting files was checked file-by-file (content hash + line-level diff) — `main`
contains **zero unique content** anywhere; every conflict is `main`'s strictly older/superseded
version (pre-T164/T170/T171/T172/T173/harness-sync) against `develop`'s strictly newer version.
Confirmed for source files (`services/cwso-rollout/src/proxy.rs`, `store.rs` — main is missing the
T170 fix entirely), dependency files (`Cargo.lock`, `cwso-git-shadow/Cargo.toml` — main has
pre-T171/T172 versions), CI/Docker config (`.gitlab-ci.yml`, three Dockerfiles — main still has
`rust:1.86`), task ledger files (`active-tasks.md`, `completed-tasks.md`, `task-T150/151/158-164.md`
— main has stale `pending`/un-archived rows), and generated harness projections (`.cursor/`,
`.gemini/`, `.opencode/`, `.pi/`, `.github/`, `AGENTS.md` — main has pre-harness-sync wording).

User explicitly chose the "fix develop first" remediation over resolving on the release MR alone,
so the fix is durable rather than a one-off patch.

## Objective

Create a real, two-parent merge commit that folds `main` into `develop`, using the `ort`
merge strategy with `-s ours` (not `-X ours`) so the resulting tree is byte-for-byte identical to
`develop`'s current tree (verified: `git rev-parse HEAD^{tree}` equals
`origin/release/v0.5.1^{tree}` in a disposable test) — this avoids the trap of `-X ours`, which
lets git's line-based auto-merge silently blend `Cargo.lock` (a generated file) and produce
233+ incorrect extra lines. `-s ours` guarantees zero content drift while making
`git merge-base --is-ancestor origin/main origin/develop` succeed from this point forward.

## Procedure

1. Branch `chore/T177-fix-develop-main-ancestry` from `origin/develop`.
2. `git merge -s ours --no-ff origin/main -m "chore: fix develop/main ancestry (redo v0.5.0 back-merge as a real merge, T177)"`.
3. Verify: resulting tree hash == `origin/develop`'s tree hash; `git merge-base --is-ancestor
   origin/main HEAD` succeeds.
4. Push, open MR into `develop` (protected branch), wait for CI green, merge with a **standard
   merge** (not squash) — squashing would collapse the two-parent structure and reintroduce the
   same break.
5. After this lands, delete and re-cut `release/v0.5.1` from the now-fixed `develop` (T174 must
   restart its branch-cut step from the corrected `develop`).

## Acceptance criteria

- [ ] New merge commit on `develop` has two parents: previous `develop` tip and `main`'s tip.
- [ ] Resulting tree is byte-identical to pre-merge `develop` (no content drift).
- [ ] `git merge-base --is-ancestor origin/main origin/develop` succeeds.
- [ ] `release/v0.5.1` re-cut from the fixed `develop` merges into `main` with zero conflicts.

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Forbidden actions

- `git push --force` / `git reset --hard` on `main` or `develop`
- Using `-X ours` (or any strategy that lets git auto-merge machine-generated files like
  `Cargo.lock` line-by-line) instead of `-s ours`
- Squash-merging the fix MR
