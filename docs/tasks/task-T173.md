# Task T173 - Author v0.5.1 changelog, release artifact, and release-ready checkpoint

- **Status:** done
- **Owner:** technical-writer / release-manager
- **Priority:** P0
- **Depends on:** T170 (done), T171 (done), T172 (done)
- **Based on:** `docs/artifacts/release-v0.5.0.md`, `CHANGELOG.md`, `docs/checkpoints/checkpoint-017-rollout-healthcheck-and-store-fix-complete.md`

## Objective

Produce the release documentation required before a v0.5.1 tag can be cut. This is a
**documentation-only** task — no code changes. Covers the scope that accumulated on `develop`
since `v0.5.0` (2026-07-27): T169 (investigation), T170 (bug fix), T171 (security), T172
(internal/toolchain). No new features, no breaking changes → **PATCH** release.

## Scope — files created/edited

- `docs/artifacts/release-v0.5.1.md` (created)
- `CHANGELOG.md` (edited: prepended `## v0.5.1 - 2026-08-01` section above `## v0.5.0`)
- `docs/checkpoints/checkpoint-018-v0.5.1-release-ready.md` (created)

## Files NOT touched

- Anything under `orchestrator/`, `services/`, `scripts/`, `deploy/`, `.gitlab-ci.yml`
- Existing `docs/artifacts/release-v0.*.md` (immutable artifacts)

## Execution notes

- Delegated to Technical Writer (changelog + release artifact) and Release Manager
  (release-ready checkpoint), both producing content strictly from source-verified facts
  (commit SHAs `f7400f3`/`ef603ba` (T170), `349a891`/`a59780e` (T171), `4293149`/`1d29953`
  (T172); prior tag `v0.5.0` @ `dd6fbb4`; develop tip `493d4de`) gathered directly from `git
  log`/`git show`/source inspection by the orchestrator before delegation.
- Version rationale: patch bump per this repo's convention (breaking → MAJOR, new feature →
  MINOR, else → PATCH) — scope is one bug fix + security/toolchain maintenance, no features.
- Gate evidence cited, not re-verified: `docs/artifacts/tech-lead-review-cwso-rollout-fix-v1.md`
  (T170, VERDICT PASS), `docs/artifacts/tech-lead-review-t171-audit-fix-v1.md` (T171, VERDICT
  PASS), `docs/artifacts/rust-toolchain-1.87-bump-verification-v1.md` (T172, no blockers).
- All work done on branch `docs/T173-release-v0.5.1-notes` (branched from `develop@493d4de`).
  **Not committed, not pushed, no MR opened** — left for maintainer review per instruction.

## Acceptance criteria

- [x] `docs/artifacts/release-v0.5.1.md` created, heading order matches `release-v0.5.0.md`
- [x] `## v0.5.1 - 2026-08-01` section prepended to `CHANGELOG.md` above `## v0.5.0`
- [x] Security section cites all 5 RUSTSEC IDs (0186, 0190, 0222, 0183, 0184)
- [x] `docs/checkpoints/checkpoint-018-v0.5.1-release-ready.md` created
- [x] No files outside the three listed above modified
- [ ] MR merged to `develop` — **not done**, intentionally deferred; docs-only changes still
      require MR since `develop` is protected. Maintainer to open and merge after review.

## Next steps

This branch (`docs/T173-release-v0.5.1-notes`) still needs a merge request into `develop` before
T174 (cutting `release/v0.5.1`) can proceed, since T174 branches from `develop`. That MR is left
for the maintainer to open per this session's instruction not to push or open MRs.
