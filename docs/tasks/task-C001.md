# Task C001 — README version truth + CI drift guard

**ID:** C001
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** —
**Created:** 2026-08-12
**Completed:** 2026-08-15
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B9); docs/plans/plan-cwso-v1.0-phase0-honest-baseline-v1.md

## Objective

The README status table (`README.md:23`) claims "Current state v0.4.1 GA" while the
newest release artifact is `docs/artifacts/release-v0.6.1.md` and the CHANGELOG's top
entry is v0.5.2 — three documents, three versions. Make all three agree on **v0.6.1**,
then add a CI check that fails the pipeline when the README's stated version ever lags
the newest CHANGELOG entry again.

## Inputs

- `README.md` (status table, lines 17–26)
- `CHANGELOG.md` (top entry currently v0.5.2)
- `docs/artifacts/release-v0.6.0.md` and `docs/artifacts/release-v0.6.1.md` (source of truth for the missing CHANGELOG entries)
- `.gitlab-ci.yml`

## Rails (read before starting)

### You MUST
- Backfill CHANGELOG sections for v0.6.0 and v0.6.1 by summarizing the two release artifacts (they are the source of truth — do not invent content)
- Update the README status table row 3 to state v0.6.1 as current
- Create `scripts/check-version-drift.sh`: extract the newest CHANGELOG version (first `## vX.Y.Z` heading) and the README "Current state" version; exit 1 with a clear message if they differ, exit 0 otherwise
- Add one CI job (suggested name `check:version-drift`) that runs the script on every pipeline
- Prove the check works: temporarily break the README version, run the script, confirm exit 1, then restore

### You MUST NOT
- Touch any README section other than the status table (lines 17–26)
- Modify the release artifacts — they are immutable records
- Change any application code, compose file, or Dockerfile
- Make the check depend on network access (it must run offline in CI)
- Use a fuzzy/regex-heuristic comparison: compare exact `vX.Y.Z` strings, with the extraction rule documented in the script header

## File ownership

- **May create/modify:** `README.md` (status table only), `CHANGELOG.md` (prepend two sections), `scripts/check-version-drift.sh` (new), `.gitlab-ci.yml` (add one job)
- **Must NOT touch:** everything else, including `docs/artifacts/*`, `deploy/*`, `orchestrator/*`, `services/*`

## Steps (execute in order)

1. Read `docs/artifacts/release-v0.6.0.md` and `docs/artifacts/release-v0.6.1.md`.
2. Prepend CHANGELOG sections for v0.6.0 and v0.6.1 summarizing those artifacts, following the existing CHANGELOG format (`## vX.Y.Z - YYYY-MM-DD`).
3. Update the README status table (line 23) so "Current state" reads v0.6.1.
4. Write `scripts/check-version-drift.sh` per the rails above; `chmod +x`.
5. Add the CI job to `.gitlab-ci.yml` in an appropriate early stage.
6. Run the verification commands below, including the deliberate-break test.

## Expected outputs

- Updated `README.md` status table (v0.6.1)
- `CHANGELOG.md` with v0.6.0 and v0.6.1 sections
- `scripts/check-version-drift.sh` (executable)
- `.gitlab-ci.yml` with the drift-check job

## Acceptance criteria

1. README "Current state" = v0.6.1; CHANGELOG top entry = v0.6.1
2. `bash scripts/check-version-drift.sh` exits 0 on the merged state
3. Deliberately reverting the README version makes the script exit 1 (evidence in MR description)
4. The CI job appears in `.gitlab-ci.yml` and runs the script

## Verification commands

```bash
bash scripts/check-version-drift.sh && echo "PASS: versions agree"
sed -i 's/v0\.6\.1/v0.0.1/' README.md
bash scripts/check-version-drift.sh; test $? -eq 1 && echo "PASS: drift detected"
git checkout -- README.md
bash scripts/check-version-drift.sh && echo "PASS: restored"
```

## Git rails

- Branch: `agent/devops-engineer/C001` from `develop`
- Commits: `docs(changelog): backfill v0.6.0 and v0.6.1 entries`, `fix(readme): correct current-state version to v0.6.1`, `ci: add version-drift check`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type (`technical` | `dependency` | `unclear_requirements` | `external`)
+ severity (`critical` | `major` | `minor`) + one proposed mitigation. Max 2 retries.
Never guess a version number — if the true current release is ambiguous, stop and escalate.

## Execution notes

<filled during execution>
