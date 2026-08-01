# Task T174 - Cut release/v0.5.1 and merge to main

- **Status:** pending
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** T173 (done — docs merged to `develop` first; see T173's own "Next steps")
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
