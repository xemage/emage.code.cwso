# Task T167 - Tag v0.5.0 and publish GitLab release

- **Status:** pending
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** T166
- **Based on:** `docs/artifacts/release-v0.5.0.md`, prior tag `v0.4.1` @ `03bbf25`

## Objective

Create the annotated `v0.5.0` tag on the `main` merge commit and publish the GitLab release
entry from the release artifact file.

## Preconditions

```bash
git fetch origin --prune --tags
git checkout main && git pull origin main
git log -1 --oneline                          # MUST be the release/v0.5.0 merge commit
git tag -l v0.5.0                             # MUST print nothing
test -f docs/artifacts/release-v0.5.0.md && echo "artifact OK"
```

If `main` HEAD is not the `release/v0.5.0` merge commit, **stop** — T166 is incomplete.

## Procedure

### 1. Create and push the annotated tag

```bash
git tag -a v0.5.0 -m "release: v0.5.0"
git push origin v0.5.0
```

The tag must sit on the `main` merge commit, mirroring how `v0.4.1` sits on `03bbf25`.
Tag creation happens **after** the merge — never tag ahead of a commit.

### 2. Wait for the tag pipeline

```bash
glab ci list --per-page 3
```

The tag pipeline (`ref: v0.5.0`) must reach `success` before publishing the release entry.

### 3. Publish the GitLab release

```bash
glab release create v0.5.0 \
  --ref v0.5.0 \
  --name v0.5.0 \
  -F docs/artifacts/release-v0.5.0.md
```

Use `-F` with the artifact file only. Do **not** pass inline `--notes` — ad-hoc notes drift
from the committed artifact.

### 4. Verify publication

```bash
glab release view v0.5.0
git describe --tags --exact-match main        # must print v0.5.0
```

## Acceptance Criteria

- [ ] Annotated tag `v0.5.0` exists on `origin`, pointing at the `main` merge commit
- [ ] Tag pipeline green
- [ ] GitLab release `v0.5.0` published, body sourced from `docs/artifacts/release-v0.5.0.md`
- [ ] `git describe --tags --exact-match main` prints `v0.5.0`

## STOP conditions

- Tag pipeline fails → **do not** publish the release entry. Report the failing job.
- `glab release create` errors with "tag not found" → the tag push did not land; re-run step 1.
- A release entry for `v0.5.0` already exists → stop and report; do not overwrite.

## Forbidden actions

- `git tag -f` / `git push --force` / `git push -f` on any ref
- Deleting or moving an existing tag
- Publishing the release before the tag pipeline is green
