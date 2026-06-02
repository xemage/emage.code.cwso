# Artifact: release-v0.2.0-draft

## Metadata
- Producer agent: release-manager
- Created: 2026-05-24
- Status: DRAFT (do not publish until T079 blockers are cleared)
- Based on: docs/artifacts/release-v0.2.0-rc1.md, docs/artifacts/operator-validation-v0.2.0-rc1.md

## Release intent
Promote the validated v0.2.0-rc1 line to stable v0.2.0 after final stakeholder acceptance and operational sign-off.

## Scope summary
- Includes all phase-5 hardware-aware baseline and hardening closure already released in rc1.
- No additional implementation deltas are intended between rc1 and GA unless blocker feedback requires patch updates.

## Included workstreams
- T062-T072 baseline delivery.
- T073-T075 hardening follow-ups.
- T076-T078 release-candidate readiness/publication/operator validation.

## Expected GA asset set
- `cwso-orchestrator-linux-amd64`
- `cwso-git-shadow-linux-amd64`
- `cwso-merge-engine-linux-amd64`
- `cwso-orchestrator-image-v0.2.0.tar.gz`
- `cwso-git-shadow-image-v0.2.0.tar.gz`
- `cwso-merge-engine-image-v0.2.0.tar.gz`

## Gate conditions before publish
- Stakeholder acceptance walkthrough outcome recorded.
- Soak/rollback evidence or signed waiver recorded.
- T079 marked PASS by release-manager.

## Draft GA verdict
PENDING_EXTERNAL_SIGNOFF
