# Task T168 - Back-merge main into develop and clean up

- **Status:** pending
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** T167
- **Based on:** `.github/instructions/git-workflow.instructions.md` (GitFlow)

## Objective

Return the tagged `main` state to `develop` so the two lines stay in sync, then remove the
release branch. This is a GitFlow **back-merge**, not a hotfix.

## Preconditions

```bash
git fetch origin --prune --tags
git tag -l v0.5.0                             # MUST print v0.5.0
git log origin/main -1 --oneline              # the release merge commit
```

If `v0.5.0` does not exist, **stop** — T167 is incomplete.

## Procedure

### 1. Open the back-merge MR

`develop` is protected, so this must go through an MR:

```bash
glab mr create --source-branch main --target-branch develop \
  --title "chore: back-merge v0.5.0 release into develop" \
  --description "GitFlow back-merge after v0.5.0 tag. Refs T168" --yes
```

If GitLab rejects `main` as a source branch, create an intermediate branch instead:

```bash
git checkout -b chore/T168-backmerge-v0.5.0 origin/main
git push origin chore/T168-backmerge-v0.5.0
glab mr create --source-branch chore/T168-backmerge-v0.5.0 --target-branch develop \
  --title "chore: back-merge v0.5.0 release into develop" \
  --description "GitFlow back-merge after v0.5.0 tag. Refs T168" --yes
```

### 2. Merge with a standard merge

Wait for all 11 CI jobs to be `success`, then:

```bash
glab mr merge <MR_NUMBER> --yes
```

Do **not** squash. Squashing breaks `main` ↔ `develop` ancestry.

If the diff is empty (`develop` already contains everything on `main`), close the MR with a
note instead of merging, and record that in the completion report.

### 3. Clean up the release branch

Only after the back-merge has landed:

```bash
git push origin --delete release/v0.5.0
git branch -d release/v0.5.0 2>/dev/null || true
```

If an intermediate branch was used, delete it the same way.

### 4. Final verification

```bash
git fetch origin --prune --tags
git merge-base --is-ancestor origin/main origin/develop && echo "develop contains main: OK"
glab ci list --per-page 3                     # newest develop pipeline must be (success)
git branch -r | grep release/v0.5.0           # must print nothing
```

## Acceptance Criteria

- [ ] Back-merge MR merged to `develop` (or closed as empty-diff with a recorded note)
- [ ] `git merge-base --is-ancestor origin/main origin/develop` succeeds
- [ ] `develop` CI pipeline green after the back-merge
- [ ] `release/v0.5.0` deleted from `origin` and locally
- [ ] Any intermediate `chore/T168-*` branch deleted

## STOP conditions

- Merge conflicts → stop and report the conflicting paths. Do not resolve without review.
- `develop` CI fails after the back-merge → report; do not delete the release branch yet.

## Forbidden actions

- `git push --force` / `git reset --hard` on `main` or `develop`
- Deleting `release/v0.5.0` before the back-merge lands
- Deleting any tag
