# Task T166 - Cut release/v0.5.0 and merge to main

- **Status:** pending
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** T165
- **Based on:** `.github/instructions/git-workflow.instructions.md`, `docs/plans/plan-v0.5.0-release.md`

## Objective

Create the `release/v0.5.0` branch from `develop` and merge it into `main` via merge request.
**No file edits in this task** — it is pure branch and MR mechanics.

## Preconditions — verify ALL before starting

```bash
git fetch origin --prune
git log origin/develop -1 --oneline          # note the SHA
glab ci list --per-page 3                    # newest develop pipeline MUST be (success)
test -f docs/artifacts/release-v0.5.0.md && echo "artifact OK"
git tag -l v0.5.0                            # MUST print nothing
```

If the newest `develop` pipeline is not `success`, **stop** — T164 or T165 is incomplete.

## Procedure

### 1. Cut the release branch

```bash
git checkout develop && git pull origin develop
git checkout -b release/v0.5.0
git push origin release/v0.5.0
```

Do not commit anything to this branch. It is a snapshot of `develop`.

### 2. Open the merge request to main

```bash
glab mr create --source-branch release/v0.5.0 --target-branch main \
  --title "release(v0.5.0): Phase 3.1 task assignment, transport hardening, security updates" \
  --description "Release v0.5.0.

Release notes: docs/artifacts/release-v0.5.0.md

Includes GO-2026-5856 (Go 1.25.12) and RUSTSEC-2026-0204 (crossbeam-epoch 0.9.20) remediations.

Refs T166" --yes
```

### 3. Wait for CI

Poll until the MR pipeline reaches a terminal state:

```bash
glab ci list --per-page 5
```

All 11 jobs must be `success`. Do not merge on a `failed` or `running` pipeline.

### 4. Merge — STANDARD MERGE, NOT SQUASH

```bash
glab mr merge <MR_NUMBER> --yes
```

**Critical:** this MR must produce a real merge commit on `main`, matching the precedent
`Merge branch 'release/v0.4.1' into 'main'` (commit `03bbf25`, which carries tag `v0.4.1`).

- Do **not** pass `--squash`.
- Do **not** pass `--remove-source-branch`. The release branch stays until T168 completes.

If the GitLab project default is squash-on-merge, override it in the MR UI or with
`glab mr update <MR_NUMBER> --squash-before-merge=false` before merging.

## Acceptance Criteria

- [ ] `release/v0.5.0` exists on `origin`, identical to `develop` at cut time
- [ ] MR `release/v0.5.0 → main` created, referencing the release artifact
- [ ] All 11 CI jobs green on the MR pipeline
- [ ] MR merged with a **merge commit** (verify: `git log origin/main -1 --oneline` starts with `Merge branch 'release/v0.5.0'`)
- [ ] `release/v0.5.0` branch still present on origin
- [ ] Record the resulting `main` merge-commit SHA — T167 needs it

## STOP conditions

- Any CI job fails → report the job name and trace excerpt; do not retry blindly more than once.
- Merge conflicts against `main` → stop and report. Do not resolve conflicts here; that means
  `main` has commits `develop` lacks and needs a separate reconciliation task.
- Tag `v0.5.0` already exists → stop, the release was already partially cut.
