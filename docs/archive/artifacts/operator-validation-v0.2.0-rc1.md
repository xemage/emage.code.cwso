# Artifact: operator-validation-v0.2.0-rc1

## Metadata
- Producer agent: release-manager
- Created: 2026-05-24
- Based on: docs/artifacts/release-v0.2.0-rc1.md, docs/tasks/task-T077.md, docs/checkpoints/checkpoint-023-phase5-rc1-published.md

## Purpose
Capture operator-facing validation evidence for v0.2.0-rc1 and define the remaining acceptance checks required before GA promotion to v0.2.0.

## Validation scope executed

### Release publication checks
- Git tag exists on origin: `v0.2.0-rc1`.
- GitLab release exists: https://gitlab.com/em-age/emage.code.cwso/-/releases/v0.2.0-rc1.
- Expected six assets present:
  - `cwso-orchestrator-linux-amd64`
  - `cwso-git-shadow-linux-amd64`
  - `cwso-merge-engine-linux-amd64`
  - `cwso-orchestrator-image-v0.2.0-rc1.tar.gz`
  - `cwso-git-shadow-image-v0.2.0-rc1.tar.gz`
  - `cwso-merge-engine-image-v0.2.0-rc1.tar.gz`

### Runtime smoke checks
- Orchestrator binary starts and returns CLI usage (`--help`).
- Git-shadow sidecar starts with writable local overrides and logs ready state.
- Merge-engine sidecar starts with writable local override and logs ready state.
- Docker compose manifest validation passes (`deploy/docker-compose.yml` parse success).

### CI checks
- Publication-tracking commit pipeline: https://gitlab.com/em-age/emage.code.cwso/-/pipelines/2549079715
- Verdict: success

## Validation matrix

| Category | Check | Result | Evidence |
|---|---|---|---|
| Release | Tag published | PASS | git tag push for `v0.2.0-rc1` |
| Release | GitLab release exists | PASS | release URL above |
| Release | Asset set completeness | PASS | six assets listed |
| Binary | orchestrator startup | PASS | `--help` output |
| Binary | git-shadow startup | PASS | ready log with temp runtime paths |
| Binary | merge-engine startup | PASS | ready log with temp runtime path |
| Deploy | compose config validity | PASS | compose config parse success |
| CI | pipeline gates | PASS | pipeline 2549079715 success |

## Residual validation not yet executed
- Stakeholder acceptance walkthrough for hardware-aware assist behavior in target environment.
- Extended soak test window on rc1 artifact set.
- Operator rollback drill from rc1 binaries/images to prior stable release.

## Verdict
CONDITIONAL_PASS (RC_OPERATIONS_READY)

Rationale:
- All release publication, artifact integrity, smoke runtime, and CI checks passed.
- Remaining items are operational/governance acceptance activities, not implementation blockers.

## Promotion recommendation
Proceed to GA promotion gate task (T079). Promote to `v0.2.0` after stakeholder validation sign-off and release-manager approval.
