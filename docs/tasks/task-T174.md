# Task T174 - Cut release/v0.5.1 and merge to main

- **Status:** done
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** T173 (done), T177 (done)
- **Completed:** 2026-08-02
- **Based on:** `.claude/rules/git-workflow.md` (GitFlow), `docs/tasks/task-T166.md` (v0.5.0
  precedent)

## Objective

Cut a `release/v0.5.1` branch from `develop` (once the T173 documentation MR has landed there),
verify CI is green on the release branch, and merge it to `main`. This is a **PATCH** release —
no feature freeze concerns beyond the standard release checklist.

## Preconditions

```bash
git fetch origin --prune --tags
git log origin/develop -1 --oneline           # must include the merged T173 docs commit
git tag -l v0.5.1                             # MUST NOT exist yet
```

If the T173 docs MR has not yet merged to `develop`, **stop** — this task is blocked on it.

## Procedure

1. `git checkout -b release/v0.5.1 origin/develop`
2. Push and confirm CI green on the release branch (build, lint, test, audit, e2e — all stages,
   mirroring the 11/11 job precedent from v0.5.0's MR !75/!74).
3. Open MR `release/v0.5.1` → `main`. Require green pipeline + ≥1 approval per branch policy.
4. Merge with a standard merge (not squash) to preserve release-branch ancestry, per T168's
   documented rationale for why squashing breaks `main` ↔ `develop` ancestry.

## Explicit constraints

- **Do NOT tag** — tagging is T175's job, after this merge lands.
- **Do NOT touch `develop`** directly in this task — only `release/v0.5.1` and the merge to
  `main`.
- Requires explicit user go-ahead before opening the MR to `main`, per this session's caution
  pattern around protected branches (see `docs/checkpoints/checkpoint-018-v0.5.1-release-ready.md`
  "Next steps").

## Acceptance criteria

- [ ] `release/v0.5.1` branch created from `develop` (post-T173-merge)
- [ ] CI green on `release/v0.5.1` (all stages)
- [ ] MR `release/v0.5.1` → `main` merged (standard merge, not squash)
- [ ] `main` HEAD is now the release merge commit

## Blocker protocol

Report blockers as: type (`technical` | `dependency` | `unclear_requirements` | `external`) +
severity (`critical` | `major` | `minor`) + one proposed mitigation. Max 2 retries.

## Forbidden actions

- `git push --force` / `git reset --hard` on `main` or `develop`
- Creating the `v0.5.1` tag (reserved for T175)
- Proceeding without explicit user authorization to touch `main`

## Execution notes

Completed 2026-08-02, two attempts:

- **First attempt**: `release/v0.5.1` cut from `develop` (MR !86) hit **292 false conflicts**
  against `main` — discovered `main` was never a real git ancestor of `develop` (T168's original
  back-merge was a flat commit). Filed and completed T177 to fix this properly before continuing.
- **Second attempt** (this task, post-T177): deleted the stale branch/MR, re-cut
  `release/v0.5.1` from the corrected `develop`, pushed, CI green (11/11, mirroring the v0.5.0
  precedent). Opened MR !90 → `main` — confirmed **zero conflicts** this time
  (`has_conflicts: false`), proving the T177 fix. Merged via a direct GitLab API call with
  `squash=false` explicitly in the body (this project's `squash_option: default_on` silently
  squash-merged an earlier attempt at the T177 fix despite the `glab` CLI flag — see
  `docs/tasks/task-T177.md` execution notes — so every merge in this release now uses the
  direct-API method instead of relying on the CLI flag).
- `main` HEAD is now `8e1a479` ("Merge branch 'release/v0.5.1' into 'main'"), parents
  `dd6fbb4` (previous `main` tip) and `d55335a` (`release/v0.5.1`/`develop` tip at merge time) —
  a real two-parent merge, `merge_commit_sha` set, `squash_commit_sha: null`.
- Did not tag (T175's job) and did not touch `develop` directly beyond what T177 required.
