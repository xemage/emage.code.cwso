# Task T176 - Back-merge main into develop and clean up

- **Status:** done
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** T175 (done)
- **Completed:** 2026-08-02
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

## Execution notes

Completed 2026-08-02, no blockers. GitLab accepted `main` directly as a source branch (no
fallback intermediate branch needed).

- MR !93 (`main` → `develop`) opened, zero conflicts (develop already contained everything in
  `main`'s content via T177 + the release branch lineage — only the ancestry link was missing).
  As with every merge in this release chain, this project's `squash_option: default_on` would
  have silently squashed it; `squash` was explicitly patched to `false` via direct API before
  merging, and the merge itself was executed via a direct API call with `squash=false` in the
  body. `merge_commit_sha` set, `squash_commit_sha: null`.
- Verified: `git merge-base --is-ancestor origin/main origin/develop` succeeds; `develop`'s
  post-merge pipeline (triggered by the merge commit `e021262`) green 11/11.
- `release/v0.5.1` was already gone from `origin` (GitLab's `force_remove_source_branch` auto-
  deleted it when MR !90 merged into `main`); deleted the stale local branch ref.
- This closes the full T169-T176 chain for the v0.5.1 patch release.
