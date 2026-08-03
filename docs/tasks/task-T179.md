# Task T179 — Expand `deploy:registry` to push all 4 images with semver tags

**ID:** T179
**Owner:** devops-engineer
**Status:** done
**Priority:** P1
**Depends on:** T178
**Created:** 2026-08-03
**Completed:** 2026-08-03
**Based on:** docs/plans/plan-registry-publishing-completion.md, docs/artifacts/emagecode-integration-registry-gap-v1.md

## Objective

`deploy:registry` currently only pushes `orchestrator` and `git-shadow` as `:latest` and has
`merge-engine` and `rollout` absent from its `needs:` and push script. Expand it to:
1. Add `build:merge-engine` and `build:rollout` to `needs:`
2. Push all 4 images (orchestrator, git-shadow, merge-engine, rollout)
3. Push both `:latest` and `:$CI_COMMIT_TAG` when `$CI_COMMIT_TAG` is set (semver tagging)

This unblocks emage.code from switching `deploy/docker-compose-t226.yml` to registry-pull mode.

## Inputs

- `.gitlab-ci.yml` (current — with T178's `build:rollout` job already merged)
- `docs/plans/plan-registry-publishing-completion.md` § "Updated `deploy:registry` job" — full YAML snippet

## Expected outputs

- `.gitlab-ci.yml` with expanded `deploy:registry` job

## Acceptance criteria

1. `deploy:registry` `needs:` includes `build:merge-engine` and `build:rollout`
2. All 4 images are pushed as `:latest` on `main` pipeline
3. All 4 images are pushed as `:$CI_COMMIT_TAG` when a tag pipeline runs
4. CI pipeline green on MR
5. After merge and pipeline completes on `main`:
   - `glab api "projects/em-age%2Femage.code.cwso/registry/repositories"` lists all 4 image repositories
   - Each has at least a `:latest` tag

## Blocker protocol
Report blockers as: type (`technical` | `dependency` | `unclear_requirements` | `external`)
+ severity (`critical` | `major` | `minor`) + one proposed mitigation. Max 2 retries.

## Execution notes
- Expanded `deploy:registry` in `.gitlab-ci.yml` to include `build:merge-engine` and `build:rollout` in `needs:` while keeping `e2e:phase2` optional.
- Added `ensure_local_image` calls for `merge-engine` and `rollout` in addition to existing services.
- Replaced service-specific tag/push commands with a loop over `orchestrator`, `git-shadow`, `merge-engine`, `rollout`:
   - Always tags/pushes `:latest`
   - Conditionally tags/pushes `:$CI_COMMIT_TAG` when set.
- Validation evidence:
   - `rg` confirms deploy block and four-service loop.
   - `glab ci lint .gitlab-ci.yml` returned `CI/CD YAML is valid`.
   - `docker build -f deploy/Dockerfile.rollout . -t cwso-rollout:local-verify` succeeded.
