# Artifact: ga-promotion-checklist-v0.2.0

## Metadata
- Producer agent: release-manager
- Created: 2026-05-24
- Based on: docs/artifacts/operator-validation-v0.2.0-rc1.md, docs/artifacts/release-v0.2.0-rc1.md, docs/tasks/task-T079.md

## Objective
Track GA promotion readiness from v0.2.0-rc1 to v0.2.0 with explicit completion state for each release gate item.

## Checklist status

| Item | Status | Evidence |
|---|---|---|
| Confirm rc1 publication baseline and asset completeness | DONE | https://gitlab.com/em-age/emage.code.cwso/-/releases/v0.2.0-rc1 |
| Confirm no existing v0.2.0 tag/release to avoid duplicate promotion | DONE | local tag/release preflight (no GA tag/release found) |
| Prepare GA release notes/changelog delta draft | DONE | docs/artifacts/release-v0.2.0-draft.md |
| Capture stakeholder acceptance walkthrough outcome | PENDING | external sign-off input required |
| Capture soak/rollback evidence or signed waiver | PENDING | external operations input required |
| Create and push v0.2.0 tag (HTTPS) | PENDING | blocked by pending acceptance items |
| Publish v0.2.0 release assets | PENDING | blocked by pending acceptance items |
| Record final GA checkpoint and close T079 | PENDING | blocked by promotion completion |

## Current gate status
IN_PROGRESS

## Blocking inputs
- Stakeholder acceptance decision on rc1 behavior in target environment.
- Soak/rollback evidence package (or explicit waiver) approved by release-manager.

## Promotion command plan (ready once blockers clear)
1. `git tag -a v0.2.0 -m "v0.2.0"`
2. `git push origin v0.2.0`
3. `glab release create v0.2.0 --ref v0.2.0 --notes-file docs/artifacts/release-v0.2.0-draft.md`
4. `make release-assets TAG=v0.2.0`

## Notes
- Push and tag workflow remains HTTPS-only.
- Known transient "Invalid token" warnings in GitLab auth path are non-blocking if command exit status is successful.
