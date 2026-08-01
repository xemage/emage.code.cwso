# Checkpoint 018 — v0.5.1 Release Ready

**Phase:** Release → documentation complete, cut/tag/publish pending
**Date:** 2026-08-01
**Branch:** `docs/T173-release-v0.5.1-notes` (branched from `develop@493d4de`)
**Previous checkpoint:** `checkpoint-017-rollout-healthcheck-and-store-fix-complete.md`
**Status:** RELEASE_DOCS_READY (release notes and changelog complete; branch cut, tag, GitLab
release, and back-merge are explicitly NOT done — reserved for user-gated follow-up tasks)

---

## Executive Summary

v0.5.1 release documentation is complete: the release artifact
`docs/artifacts/release-v0.5.1.md` and the `## v0.5.1 - 2026-08-01` section of `CHANGELOG.md`
both fully describe the patch's scope (T170 bug fix, T171/T172 security and toolchain
maintenance, T169 as the investigation that grounded T170). No new features and no breaking
changes are introduced versus `v0.5.0`, so this is a **PATCH** release per this repo's
version-decision convention. Nothing has been tagged, merged to `main`, or published as a
GitLab release in this session — those steps are intentionally deferred to follow-up tasks
T174–T176, which require explicit user go-ahead before any `main`/tag-touching operation, per
this session's established caution pattern (see checkpoint-017's precedent of leaving branch/MR
creation for explicit maintainer trigger).

---

## Completed tasks (this release's scope)

| Task | Outcome | Artifact |
|------|---------|----------|
| T169 | Root-cause investigation: rollout healthcheck 405 + trajectory store path mismatch (no direct fix — investigation only, folded into T170) | `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md` |
| T170 | `GET /healthz` liveness route + `Dockerfile.rollout` `HEALTHCHECK` + trajectory store env-var precedence fix, with regression tests and real build/docker verification | `docs/artifacts/fix-verification-cwso-rollout-v1.md`; Tech Lead review `docs/artifacts/tech-lead-review-cwso-rollout-fix-v1.md` — **VERDICT: PASS** |
| T171 | Security dependency bumps (`memmap2`, `anyhow`, `wasmtime`); `git2` fix scoped-ignored (`RUSTSEC-2026-0183`/`0184`) pending toolchain bump | Tech Lead review `docs/artifacts/tech-lead-review-t171-audit-fix-v1.md` — **VERDICT: PASS** (two optional, non-blocking recommendations noted; neither required before merge) |
| T172 | Rust toolchain 1.86 → 1.87 bump across all 3 Rust Dockerfiles/CI jobs; `git2` 0.20.4 → 0.21.0, fully resolving `RUSTSEC-2026-0183`/`0184`; `cargo audit` now exits 0 with zero ignore flags | `docs/artifacts/rust-toolchain-1.87-bump-verification-v1.md` — blocker status: none |

All four tasks are already present as `done` rows in `docs/tasks/completed-tasks.md` (each dated
2026-08-01) and were merged to `develop` prior to this checkpoint:

| Task | Merge commit | Feature commit |
|------|--------------|-----------------|
| T170 | `ef603ba` | `f7400f3` |
| T171 | `a59780e` | `349a891` |
| T172 | `1d29953` | `4293149` |

### Release documentation produced this session

| Artifact | Status |
|----------|--------|
| `docs/artifacts/release-v0.5.1.md` | Complete — scope table, full changelog body, feature-flag matrix (unchanged from v0.5.0), CI/validation evidence, version rationale, migration guide (no breaking changes) |
| `CHANGELOG.md` — `## v0.5.1 - 2026-08-01` section | Complete — Bug Fixes (T170), Security (T171/T172), Operations, Documentation subsections, prepended above the existing `## v0.5.0 - 2026-07-27` entry |

Both artifacts were authored by the technical-writer/release-manager pairing under task **T173**,
which is still `in_progress` as of this checkpoint — it concludes with this checkpoint being
written, and the orchestrator will move it to `done` in `docs/tasks/completed-tasks.md`
immediately after. This checkpoint does not perform that task-board transition itself.

---

## Version rationale

**v0.5.0 → v0.5.1 is a PATCH bump.** Scope is limited to:
- A bug fix (T170: healthcheck route + store-path env-var resolution, both backward compatible)
- Security dependency maintenance (T171: `memmap2`/`anyhow`/`wasmtime`; T172: toolchain +
  `git2`)

No new features and no breaking changes are introduced. This matches the repo's convention
(breaking → MAJOR, new feature → MINOR, else → PATCH) as stated in `release-v0.5.1.md`'s own
"Version rationale" section.

---

## Prior release lineage (context, not re-verified here — orchestrator-confirmed)

- Prior GA tag: **`v0.5.0`** @ commit `dd6fbb4` on `main`, tagged 2026-07-27, back-merged to
  `develop` at `677f9db`.
- Current `develop` tip: **`493d4de`** (2026-08-01) — confirmed identical to this branch's
  merge-base with `develop` via `git merge-base HEAD develop`, i.e. `docs/T173-release-v0.5.1-notes`
  branched cleanly from `develop` tip with no divergence.

---

## Key decisions

- Release notes live at `docs/artifacts/release-v0.5.1.md`, following repository precedent
  (`release-v0.3.0.md`, `release-v0.4.0.md`, `release-v0.4.1.md`, `release-v0.5.0.md`) rather
  than a `docs/releases/vX.Y.Z.md` path, which does not exist in this repository.
- T169 is credited as "investigation only" in both the release artifact and this checkpoint —
  it produced no direct code change and is folded into T170's changelog entry rather than given
  its own changelog line, since the root-cause analysis is what grounded T170's fix scope, not a
  user-facing change in itself.
- No CI pipeline links or staging-deployment evidence are included in this checkpoint's gate
  table below (unlike checkpoint-016's v0.4.1 precedent), because no release branch, tag, or MR
  has been created yet in this cycle — that evidence will be gathered as part of T174/T175, not
  before.
- Per the task instructions and this session's caution pattern, no `git` state-mutating command
  (branch, tag, commit, push) was run in producing this checkpoint. Only read-only `git log`/
  `git merge-base`/`git branch --show-current` were used to verify the facts above.

---

## Release Gate status (informational — full RELEASE VERDICT deferred to T175)

| Gate | Status | Artifact |
|------|--------|----------|
| Tech Lead review — T170 (rollout fix) | PASS | `docs/artifacts/tech-lead-review-cwso-rollout-fix-v1.md` |
| Tech Lead review — T171 (audit/dependency bump) | PASS | `docs/artifacts/tech-lead-review-t171-audit-fix-v1.md` |
| T172 acceptance criteria | PASS (all 4 met; CI pipeline confirmation on the real MR pipeline still pending, per the artifact's own §"Acceptance criteria" item 4) | `docs/artifacts/rust-toolchain-1.87-bump-verification-v1.md` |
| CI/CD pipeline (release branch) | Not yet applicable — no release branch cut | n/a (T174) |
| Staging deployment | Not yet applicable | n/a (T174/T175) |

No QA sign-off or security-audit gate artifact separate from the two Tech Lead reviews above was
produced for this release cycle; the Tech Lead reviews serve as the merge gate for T170/T171, and
T172's own verification artifact serves as its gate (blocker status: none, per that artifact).
A dedicated end-to-end RELEASE VERDICT (PASS/CONDITIONAL_PASS/FAIL) against the full gate table
in this repo's Release Manager format will be produced at T175 once staging/CI evidence for the
actual `release/v0.5.1` branch exists.

---

## Blockers

**None open.** `docs/artifacts/tech-lead-review-t171-audit-fix-v1.md`'s verdict is **PASS** (not
`CONDITIONAL_PASS` or `FAIL`), so no risk-acceptance condition or escalation is required before
proceeding to T174. Its two "optional, non-blocking" recommendations (§8: independent audit of
`cwso-git-shadow`'s `git2` call sites against the RUSTSEC preconditions; cosmetic
`task-T171.md` header-status fix) do not gate this release and are not tracked as blockers here.

---

## Active tasks

- **T173** — "Author v0.5.1 release notes/changelog/checkpoint" — `in_progress`, concluding with
  this checkpoint. To be transitioned to `done` by the orchestrator (not by this checkpoint).

---

## Next steps (ordered — all require explicit user go-ahead before touching `main` or tags)

1. **[PENDING — T174, owner: release-manager]** Cut `release/v0.5.1` branch from `develop@493d4de`,
   verify CI green on the release branch, then merge to `main`. **Not to be started without
   explicit user authorization**, since it is the first step that touches the protected `main`
   line.
2. **[PENDING — T175, owner: release-manager]** Tag `v0.5.1` on `main` and publish the GitLab
   release referencing `docs/artifacts/release-v0.5.1.md` and the `CHANGELOG.md` v0.5.1 section.
   Also the point at which a full RELEASE VERDICT (per this repo's Release Gate format) should be
   produced. **Gated on T174 completing and on user go-ahead** — tag creation is an irreversible,
   state-mutating operation this session explicitly avoids without approval.
3. **[PENDING — T176, owner: release-manager]** Back-merge `main` into `develop` post-tag and
   clean up the `release/v0.5.1` branch (delete after merge), mirroring the v0.5.0 cycle's
   `2eab283`/`dd6fbb4` back-merge precedent.

T174, T175, and T176 are being filed by the orchestrator as `pending` rows in
`docs/tasks/active-tasks.md`, owned by release-manager, gated on explicit user approval before any
of them touches `main` or creates a tag. This checkpoint does not create those task files — that
remains the orchestrator's responsibility per this session's task-protocol division of labor.

---

## Token budget

| Phase | Budget | Used (est.) |
|-------|--------|--------------|
| QA / Security / Release (this checkpoint + release-doc authoring) | 60k | ~10k |

On-budget.
