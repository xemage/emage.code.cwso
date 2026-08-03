# Checkpoint 019 - Registry Publishing Completion

Date: 2026-08-03
Phase: implementation -> ready for MR/CI gate
Based on: docs/plans/plan-registry-publishing-completion.md

## Completed tasks

- T178 - Add `build:rollout` CI job to `.gitlab-ci.yml`
- T179 - Expand `deploy:registry` to all 4 services with semver tag push on tag pipelines

## Artifacts and file changes

- `.gitlab-ci.yml`
  - Added `build:rollout` build job using `deploy/Dockerfile.rollout`
  - Expanded `deploy:registry.needs` with `build:merge-engine` and `build:rollout`
  - Expanded local image ensure/build fallback for all four services
  - Added loop that pushes `:latest` for all four images and `:$CI_COMMIT_TAG` when present
- `docs/tasks/task-T178.md` (status + execution notes)
- `docs/tasks/task-T179.md` (status + execution notes)
- `docs/tasks/active-tasks.md` (removed T178/T179 rows per active-board invariant)
- `docs/tasks/completed-tasks.md` (archived T178/T179)

## Verification evidence

- `rg -n "^build:rollout:|deploy:registry:|for svc in orchestrator git-shadow merge-engine rollout" .gitlab-ci.yml`
- `glab ci lint .gitlab-ci.yml` -> `CI/CD YAML is valid`
- `docker build -f deploy/Dockerfile.rollout . -t cwso-rollout:local-verify` -> success

## Key decisions

- Followed plan-specified approach using existing `ensure_local_image` fallback pattern (T153 precedent) to tolerate build/deploy runner locality differences.
- Kept deploy rules as `main` and `tag` only, preserving existing release behavior.

## Blockers

- None.

## Token metrics (approx)

- Planning/coordination: low
- Implementation/delegation: low
- Within phase budget.

## Next steps

1. Commit changes on `bugfix/T178-registry-publishing-completion`.
2. Open MR to `develop` referencing T178 and T179.
3. Wait for green MR pipeline.
4. Merge to `develop`, then merge forward per release flow.
5. On next `main` pipeline, confirm all 4 `:latest` images exist in registry.
6. On next tag pipeline, confirm all 4 `:$CI_COMMIT_TAG` images exist.
