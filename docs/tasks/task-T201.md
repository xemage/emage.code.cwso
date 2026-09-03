# Task T201 — Reconcile root README.md with the new CONTRIBUTING.md; fix broken TECHNICAL-DEBT.md link

**ID:** T201
**Owner:** technical-writer
**Status:** pending
**Priority:** P2
**Depends on:** C053 (merged)
**Created:** 2026-08-28
**Completed:** —
**Based on:** Discovered during C053's Tech Lead review (both conditions of a
CONDITIONAL_PASS verdict, resolved as non-blocking follow-ups rather than reworking C053
itself, since root `README.md` is outside C053's file-ownership boundary). Logged
separately per the established T197/T198/T199/T200 cross-boundary-gap pattern.

## Objective

Two independent, pre-existing issues in root `README.md`, both confirmed by independent
Tech Lead review during C053:

1. **Double-source-of-truth risk.** `README.md`'s own `## Contributing` section
   (currently ~lines 119-127) is not broken — its links (`docs/branching.md`,
   `.github/instructions/git-workflow.instructions.md`, `.gitlab/issue_templates`) all
   resolve — but it now duplicates process content that C053's new root `CONTRIBUTING.md`
   is meant to be the single canonical source for, with no cross-link between the two.
   A reader landing on `README.md` first has no signal that a more complete contributor
   reference exists.
2. **Broken link.** `README.md`'s docs-index list (currently ~line 229) contains
   `[Technical debt register](TECHNICAL-DEBT.md)`, which 404s — confirmed via direct file
   check that `TECHNICAL-DEBT.md` does not exist in this repo. Per
   `docs/artifacts/release-v0.6.1.md`, it was deliberately removed (all 11 items
   resolved) and its content now lives in `docs/DEBT-REGISTER.md`, which is not what this
   link points to.

## Inputs

- `README.md` (repo root) — both issues are here
- `CONTRIBUTING.md` (C053) — the new canonical contributor-process reference
- `docs/DEBT-REGISTER.md` — the actual, current debt register (what the broken link
  should point to instead)

## Rails (read before starting)

### You MUST
- Fix the broken link: `[Technical debt register](TECHNICAL-DEBT.md)` → point at
  `docs/DEBT-REGISTER.md` (or remove the bullet if it's genuinely redundant with another
  nearby docs-index entry — check first)
- Resolve the double-source-of-truth risk: either (a) replace `README.md`'s
  `## Contributing` section with a short pointer to `CONTRIBUTING.md` (mirroring how
  `docs/user/README.md` links to it, per C053's cross-link convention), or (b) keep
  `README.md`'s section as a brief summary but add an explicit link to `CONTRIBUTING.md`
  for the full reference — your judgment, but do not leave two un-cross-linked
  descriptions of the same process
- Add a CHANGELOG entry

### You MUST NOT
- Duplicate `CONTRIBUTING.md`'s content into `README.md` — link, don't copy (same
  discipline C053 already established)
- Touch `CONTRIBUTING.md` or `docs/user/README.md` themselves — this task's scope is
  `README.md` only
- Introduce a second `docs/DEBT-REGISTER.md`-equivalent link if the docs-index already
  has one elsewhere in the file — check for duplicates before adding

## File ownership

- **May create/modify:** `README.md` (repo root), `CHANGELOG.md`
- **Must NOT touch:** `CONTRIBUTING.md`, `docs/user/**`, code

## Steps (execute in order)

1. Read `README.md` in full; locate both issues precisely (line numbers may have shifted
   since the review).
2. Fix the broken `TECHNICAL-DEBT.md` link.
3. Resolve the Contributing-section duplication per the rails above.
4. CHANGELOG entry.

## Expected outputs

- `README.md` with the broken link fixed and the Contributing section cross-linked to
  `CONTRIBUTING.md` (not duplicated)
- CHANGELOG entry

## Acceptance criteria

1. `grep -n "TECHNICAL-DEBT.md" README.md` returns zero hits (or the reference now points
   at an existing file)
2. `README.md`'s `## Contributing` section (or equivalent) links to `CONTRIBUTING.md`
3. No content duplication between `README.md` and `CONTRIBUTING.md` beyond a brief,
   clearly-linked summary

## Verification commands

```bash
grep -n "TECHNICAL-DEBT.md" README.md   # expect 0, or a corrected valid path
grep -n "CONTRIBUTING.md" README.md     # expect >= 1
test -f TECHNICAL-DEBT.md && echo "still exists??" || echo "confirmed absent, as expected"
```

## Git rails

- Branch: `agent/technical-writer/T201` from `develop`
- Commit: `docs: cross-link root README.md to CONTRIBUTING.md, fix broken debt-register link`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

<filled during execution>
