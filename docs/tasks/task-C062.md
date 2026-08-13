# Task C062 — Release v1.0.0

**ID:** C062
**Owner:** devops-engineer
**Status:** pending
**Priority:** P0
**Depends on:** C060, C061, C063
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C062 row); docs/plans/plan-cwso-v1.0-phase6-release-v1.md

## Objective

Ship v1.0.0: version bump, CHANGELOG, annotated tag, all four service images, and the
release artifact — following the established release pattern (v0.3.0–v0.6.1 precedent).
The release is defined by evidence: C018 smoke test green on a clean host at the
release commit.

## Inputs

- `docs/artifacts/release-v0.6.1.md` (the pattern to follow)
- `CHANGELOG.md` (Unreleased section accumulated through Phases 0–5)
- `scripts/release-assets.sh`, `.gitlab-ci.yml` (existing release mechanics)
- C018 smoke test (must be green at the release commit)
- C060 (zero unclassified debt), C061 (security verdict), C063 (LIMITATIONS.md)

## Rails (read before starting)

### You MUST
- Confirm the entry criteria first: C018 green at the release commit, C060/C061/C063 merged — if any is missing, stop
- Rename the CHANGELOG `## Unreleased` section to `## v1.0.0 - YYYY-MM-DD` and finalize it
- Produce `docs/artifacts/release-v1.0.0.md` following the established release-artifact pattern (scope vs prior release, evidence, known limitations link)
- Create annotated tag `v1.0.0`; confirm CI publishes all four service images with the tag
- Verify the version-drift check (C001) passes with the new CHANGELOG top entry

### You MUST NOT
- Tag before the entry criteria are met — the tag is the statement that they are
- Hand-edit published images or bypass CI publishing
- Skip the release artifact (every prior release has one)
- Modify code (release mechanics only; a release-blocking bug is a new task, not a release-MR drive-by)

## File ownership

- **May create/modify:** `CHANGELOG.md`, `docs/artifacts/release-v1.0.0.md` (new), version files if the repo has them (check `orchestrator/` for a version constant), git tag
- **Must NOT touch:** application code, `docs/LIMITATIONS.md` (C063 owns it)

## Steps (execute in order)

1. Verify entry criteria (C018 green, C060/C061/C063 merged).
2. Finalize CHANGELOG v1.0.0.
3. Write the release artifact.
4. Tag `v1.0.0`; push; confirm CI publishes four images.
5. Confirm the drift check passes.

## Expected outputs

- CHANGELOG v1.0.0 section
- `docs/artifacts/release-v1.0.0.md`
- Annotated tag `v1.0.0` + four published images

## Acceptance criteria

1. C018 smoke test green at the release commit
2. Tag `v1.0.0` pushed; all four images published with the tag
3. Release artifact follows the established pattern
4. Version-drift check passes

## Verification commands

```bash
bash scripts/cwso-smoke-test.sh   # at the release commit
bash scripts/check-version-drift.sh
git tag -l v1.0.0
# confirm registry tags for orchestrator, git-shadow, merge-engine, rollout
```

## Git rails

- Branch: `agent/devops-engineer/C062` from `develop`
- Commit: `chore(release): v1.0.0`
- MR target: `develop`, then tag per the repo's release flow (GitFlow: release → main)
- Follow `docs/branching.md` for the main-vs-develop tagging convention

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If an entry criterion is unmet, the release slips — report `dependency` / `critical`.
Do not tag around a missing criterion.

## Execution notes

<filled during execution>
