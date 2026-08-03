# Task T175 - Tag v0.5.1 and publish GitLab release

- **Status:** done
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** T174 (done)
- **Completed:** 2026-08-02
- **Based on:** `.claude/rules/git-workflow.md`, orchestrator Release Workflow Preflights,
  `docs/tasks/task-T167.md` (v0.5.0 precedent)

## Objective

Tag `v0.5.1` on `main` (post T174 merge) and publish the GitLab release referencing the release
documentation authored in T173.

## Preconditions

```bash
git fetch origin --prune --tags
git log origin/main -1 --oneline              # must be the T174 release-merge commit
test -f docs/artifacts/release-v0.5.1.md && echo OK
grep -q "^## v0.5.1" CHANGELOG.md && echo OK
```

Both `OK` lines must print. If `docs/artifacts/release-v0.5.1.md` is missing or not yet merged
to `main`'s history via `develop`, **stop** — T173/T174 are incomplete.

## Procedure

1. Tag the `main` HEAD (the T174 release-merge commit) as `v0.5.1`.
2. Publish the release notes **only** from `docs/artifacts/release-v0.5.1.md` — do not pass
   ad-hoc inline `--notes` (would drift from the committed artifact):
   ```bash
   glab release create v0.5.1 --ref v0.5.1 --name v0.5.1 -F docs/artifacts/release-v0.5.1.md
   ```
3. Produce the full **RELEASE VERDICT** block (PASS/CONDITIONAL_PASS/FAIL) per this repo's
   Release Gate format, using the gate evidence already documented in
   `docs/checkpoints/checkpoint-018-v0.5.1-release-ready.md` (Tech Lead PASS on T170 and T171,
   T172 verification with no blockers) plus this task's own CI evidence for the
   `release/v0.5.1` → `main` pipeline.

## Explicit constraints

- **Do NOT** cut the tag or publish the release without explicit user go-ahead — this is an
  irreversible, consequential action this session has consistently paused on.
- Tag commit must already be on `origin/main` (i.e., T174 merged) before tagging — never tag
  ahead of the commit landing.

## Acceptance criteria

- [ ] `v0.5.1` tag exists, points at the T174 release-merge commit on `main`
- [ ] GitLab release published from `docs/artifacts/release-v0.5.1.md` only (no inline notes)
- [ ] RELEASE VERDICT block produced and reported to the user

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Forbidden actions

- Tagging before T174's merge commit is confirmed on `origin/main`
- Passing ad-hoc `--notes` instead of `-F docs/artifacts/release-v0.5.1.md`
- Proceeding without explicit user authorization

## Execution notes

Completed 2026-08-02, no blockers. Preconditions verified (`main` HEAD = `8e1a479`, the T174
release-merge commit; `docs/artifacts/release-v0.5.1.md` and `CHANGELOG.md`'s `## v0.5.1` section
both present). User gave explicit go-ahead before tagging/publishing.

- Tagged `v0.5.1` (annotated) on `main` HEAD (`8e1a479`), pushed to `origin`.
- Published GitLab release from `docs/artifacts/release-v0.5.1.md` only (`-F`, no inline
  `--notes`): https://gitlab.com/em-age/emage.code.cwso/-/releases/v0.5.1
- RELEASE VERDICT: **PASS** — reported to user in full (CI 11/11 on `release/v0.5.1` → `main`
  MR !90, Tech Lead PASS on T170/T171 per checkpoint-018, T172 clean audit/build, T177 ancestry
  fix verified, zero open blockers).
