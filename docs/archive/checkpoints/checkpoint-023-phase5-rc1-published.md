# Checkpoint 023 — phase5 rc1 published

## Phase summary
Release candidate `v0.2.0-rc1` has been published from `develop` with binary and container-image artifacts uploaded to GitLab Release, plus smoke-verification evidence captured.

## Completed tasks (this phase)
| ID | Title | Owner | Outcome |
|----|-------|-------|---------|
| T077 | Publish v0.2.0-rc1 release assets + smoke verification | release-manager | done |

## Open / carried over
| ID | Title | Owner | Status | Notes |
|----|-------|-------|--------|-------|
| T025 | Merkle-hash incremental indexer | backend-developer | deferred | Post-v0.1.x optimization; not release-blocking for v0.2.0 line |

## Key decisions
- Keep RC as validation target before GA promotion (`v0.2.0`).
- Require stakeholder/operator validation cycle against published artifacts before GA tag.

## Artifacts produced
- `docs/tasks/task-T077.md`
- `docs/artifacts/release-v0.2.0-rc1.md` (publication + smoke evidence update)
- `docs/checkpoints/checkpoint-023-phase5-rc1-published.md`

## Publication evidence
- Release URL: https://gitlab.com/em-age/emage.code.cwso/-/releases/v0.2.0-rc1
- Published assets:
  - `cwso-orchestrator-linux-amd64`
  - `cwso-git-shadow-linux-amd64`
  - `cwso-merge-engine-linux-amd64`
  - `cwso-orchestrator-image-v0.2.0-rc1.tar.gz`
  - `cwso-git-shadow-image-v0.2.0-rc1.tar.gz`
  - `cwso-merge-engine-image-v0.2.0-rc1.tar.gz`

## Blockers (active)
| ID | Type | Severity | Owner | Reported | Status |
|----|------|----------|-------|----------|--------|
| none | none | none | none | n/a | closed |

## Token usage
| Phase | Budget | Spent | % |
|-------|--------|-------|---|
| QA / Security / Release | 60k | n/a | n/a |

## Next steps
- Phase: RC validation and GA decision.
- Actions:
  - execute stakeholder validation against rc1 assets
  - capture defects/feedback as follow-up tasks if found
  - promote to `v0.2.0` after release-manager sign-off

## Compression note
This checkpoint is the canonical handoff after RC publication and smoke verification.
