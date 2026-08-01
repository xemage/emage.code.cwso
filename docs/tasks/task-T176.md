# Task T176 - Back-merge main into develop and clean up

- **Status:** pending
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** T175
- **Based on:** `.claude/rules/git-workflow.md` (GitFlow), `docs/tasks/task-T168.md` (v0.5.0
  precedent, commit `677f9db`)

## Objective

Return the tagged `main` state to `develop` so the two lines stay in sync, then remove the
release branch. This is a GitFlow **back-merge**, not a hotfix.

## Preconditions

```bash
git fetch origin --prune --tags
git tag -l v0.5.1                             # MUST print v0.5.1
git log origin/main -1 --oneline              # the release merge commit
```

If `v0.5.1` does not exist, **stop** — T175 is incomplete.

## Procedure

1. Open the back-merge MR (`develop` is protected):
   ```bash
   glab mr create --source-branch main --target-branch develop \
     --title "chore: back-merge v0.5.1 release into develop" \
     --description "GitFlow back-merge after v0.5.1 tag. Refs T176" --yes
   ```
   If GitLab rejects `main` as a source branch, create an intermediate branch
   (`chore/T176-backmerge-v0.5.1`) from `origin/main` instead, mirroring T168's fallback.
2. Wait for CI green, then merge with a **standard merge** (not squash) — squashing breaks
   `main` ↔ `develop` ancestry.
3. Delete `release/v0.5.1` from origin and locally (only after the back-merge lands).
4. Final verification:
   ```bash
   git fetch origin --prune --tags
   git merge-base --is-ancestor origin/main origin/develop && echo "develop contains main: OK"
   git branch -r | grep release/v0.5.1           # must print nothing
   ```

## Acceptance criteria

- [ ] Back-merge MR merged to `develop` (or closed as empty-diff with a recorded note)
- [ ] `git merge-base --is-ancestor origin/main origin/develop` succeeds
- [ ] `develop` CI pipeline green after the back-merge
- [ ] `release/v0.5.1` deleted from `origin` and locally

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Forbidden actions

- `git push --force` / `git reset --hard` on `main` or `develop`
- Deleting `release/v0.5.1` before the back-merge lands
- Deleting any tag
