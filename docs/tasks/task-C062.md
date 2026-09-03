# Task C062 — Release v1.0.0

**ID:** C062
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** C060, C061, C063
**Created:** 2026-08-12
**Completed:** 2026-09-03
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
- Confirm C016 ("`make up`: one command to a working stack") is `done` and its
  release-gating condition independently verified against a truly fresh clone (not
  just re-use of a dev machine that already has `.env.jwt.dev`) — specifically that
  the *documented* quick-start in `README.md` / `docs/user/installation-v3.md`
  succeeds with zero manual file creation. This condition originated on C010's
  CONDITIONAL_PASS review (2026-08-16, MR !113), moved to C012 (MR !115, C012's script
  was correct but nothing called it yet), and finally to **C016** (2026-08-16, the task
  that actually wires the bootstrap script into `make up` and updates the quick-start
  docs) — see `docs/tasks/task-C016.md` § "Release-gating condition" for the full
  chain. C016 is already transitively required by C018 (C018 ← C016/C017), so this
  should already be satisfied by the time entry criteria are checked; this line makes
  that explicit rather than relying solely on the transitive dependency chain
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

Coordinator gave an explicit GO after a deliberate go/no-go review (per this task's own
standing instruction not to auto-dispatch). Release MR !212 merged to `develop` (merge
commit `5eb107d`) with CHANGELOG finalized, `docs/artifacts/release-v1.0.0.md`
published, Tech Lead PASS. Merge-to-main MR !213 merged non-squash (`main` HEAD
`7b75123`, genuine two-parent merge, tree byte-identical to `develop`). Tag `v1.0.0`
pushed at `7b75123`. All four service images independently confirmed published with
the `:v1.0.0` tag against the live registry API (digests + timestamps checked, not
just CI job status). `scripts/check-version-drift.sh` independently re-run against the
real tagged commit tree, exit 0. Full outcome record with all verification detail: see
`docs/tasks/completed-tasks.md`'s C062 row. Closes the entire CWSO v1.0 roadmap.
