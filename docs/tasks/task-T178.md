# Task T178 — Add `build:rollout` CI job to `.gitlab-ci.yml`

**ID:** T178
**Owner:** devops-engineer
**Status:** done
**Priority:** P1
**Depends on:** —
**Created:** 2026-08-03
**Completed:** 2026-08-03
**Based on:** docs/plans/plan-registry-publishing-completion.md, docs/artifacts/emagecode-integration-registry-gap-v1.md

## Objective

`deploy/Dockerfile.rollout` exists but there is no `build:rollout` CI job. This means the rollout
image has never been built or published to the container registry, which breaks the downstream
emage.code `deploy/docker-compose-t226.yml` portability goal. Add a `build:rollout` job that
mirrors the existing `build:merge-engine` job exactly.

## Inputs

- `.gitlab-ci.yml` (current — the job is absent, confirmed by `grep "build:rollout" .gitlab-ci.yml` returning empty)
- `deploy/Dockerfile.rollout` (existing, never CI-built)
- `docs/plans/plan-registry-publishing-completion.md` § "Suggested CI YAML" — `build:rollout` snippet

## Expected outputs

- `.gitlab-ci.yml` with a new `build:rollout` job added after `build:merge-engine`

## Acceptance criteria

1. `grep -c "build:rollout" .gitlab-ci.yml` → 1 or more
2. Job uses the same `extends: .docker-base` as `build:merge-engine`
3. Script: `docker build -t $CI_REGISTRY_IMAGE/rollout:$CI_COMMIT_SHORT_SHA -f deploy/Dockerfile.rollout .`
4. Rules: same 4 conditions as `build:merge-engine` (MR event, develop, main, tag)
5. CI pipeline green on MR (required before merge per project policy)

## Blocker protocol
Report blockers as: type (`technical` | `dependency` | `unclear_requirements` | `external`)
+ severity (`critical` | `major` | `minor`) + one proposed mitigation. Max 2 retries.

## Execution notes
- Implemented `build:rollout` in `.gitlab-ci.yml` directly after `build:merge-engine`.
- Job matches required shape: `extends: .docker-base`, rollout build script, and MR/develop/main/tag rules.
- Validation evidence:
	- `rg -n "^build:rollout:" .gitlab-ci.yml` hit.
	- `glab ci lint .gitlab-ci.yml` returned `CI/CD YAML is valid`.
