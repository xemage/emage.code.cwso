# Task T077 — Publish v0.2.0-rc1 release assets + smoke verification

- Phase: **5 (Release Candidate Publication)** · Owner: **release-manager** · Priority: **P0**
- Depends on: T076 · Blocks: —
- Status: done (2026-05-24)

## Objective
Publish the `v0.2.0-rc1` tag as a GitLab release with install assets and capture executable smoke evidence for operator confidence before GA promotion.

## Inputs
- [docs/tasks/task-T076.md](task-T076.md)
- [docs/artifacts/release-v0.2.0-rc1.md](../artifacts/release-v0.2.0-rc1.md)
- [docs/checkpoints/checkpoint-022-phase5-rc1-ready.md](../checkpoints/checkpoint-022-phase5-rc1-ready.md)

## Constraints
- Use HTTPS only for push actions.
- Ignore known transient/benign "Invalid token" warning behavior from GitLab auth path.
- Keep release payload aligned with `make release-assets TAG=v0.2.0-rc1` outputs.

## Expected outputs
- Published Git tag and GitLab release for `v0.2.0-rc1`.
- Uploaded binary and image tarball assets.
- Smoke verification evidence added to release artifact.
- Checkpoint documenting rc1 publication handoff.

## Acceptance criteria
1. `v0.2.0-rc1` tag exists on origin.
2. GitLab release exists and contains six expected assets.
3. Smoke checks demonstrate startup viability for orchestrator and sidecars with local runtime paths.
4. Task and checkpoint artifacts are updated with publication evidence.

## Completion notes (2026-05-24)
- Created and pushed annotated tag `v0.2.0-rc1` to origin over HTTPS.
- Created GitLab release from RC notes and uploaded assets via `make release-assets TAG=v0.2.0-rc1`.
- Validated release asset links via GitLab API.
- Ran smoke checks:
  - orchestrator `--help` output OK
  - git-shadow startup OK with temp storage/socket env overrides
  - merge-engine startup OK with temp socket env override
  - `docker compose -f deploy/docker-compose.yml config` succeeded

### Evidence
- GitLab release: https://gitlab.com/em-age/emage.code.cwso/-/releases/v0.2.0-rc1
- Updated artifact: [docs/artifacts/release-v0.2.0-rc1.md](../artifacts/release-v0.2.0-rc1.md)
- Checkpoint: [docs/checkpoints/checkpoint-023-phase5-rc1-published.md](../checkpoints/checkpoint-023-phase5-rc1-published.md)
