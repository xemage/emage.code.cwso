# Artifact: release-v0.5.2

## Metadata
- Producer agent: release-manager
- Created: 2026-08-03
- Based on: docs/artifacts/release-v0.5.1.md, docs/plans/plan-registry-publishing-completion.md, T178, T179
- develop tip: 8b3f510
- Prior GA tag: v0.5.1

## Release intent

v0.5.2 is a patch release focused on CI registry publishing completeness.
It ensures all four CWSO service images are publishable through CI on both
main and tag pipelines.

No API behavior changes and no breaking changes are introduced.

## Scope

| Item | Status |
|------|--------|
| Add build:rollout CI job | Included |
| Expand deploy:registry needs to include merge-engine and rollout | Included |
| Push all four services as latest on main | Included |
| Push all four services as semver tags on tag pipelines | Included |

## Changelog - v0.5.2

Release Date: 2026-08-03
Previous Version: v0.5.1

### CI and Registry Publishing
- Added `build:rollout` using `deploy/Dockerfile.rollout`.
- Expanded `deploy:registry` to include all four services:
  - orchestrator
  - git-shadow
  - merge-engine
  - rollout
- Registry deploy now:
  - pushes `:latest` for all four services on main
  - pushes `:$CI_COMMIT_TAG` for all four services on tag pipelines

### Validation
- Merge request pipeline for MR !95 passed.
- MR !95 merged to develop with merge commit `8b3f510`.
- CI YAML lint passed (`glab ci lint .gitlab-ci.yml`).
- Local rollout image build succeeded.

## Version rationale

Patch bump v0.5.1 -> v0.5.2 because changes are operational/CI publication
fixes with no new product features and no breaking changes.

## Latest release: v0.5.2

## Install

See docs/user/installation-v2.md for full setup.

```bash
docker compose -f deploy/docker-compose.yml up
```

## Highlights

- Rollout image now built and publishable in CI
- Complete 4-image registry publication on main
- Complete 4-image semver-tag publication on release tags
