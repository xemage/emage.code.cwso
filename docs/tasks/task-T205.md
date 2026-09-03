# Task T205 — Fix stale row-count and C063-status claims in DEBT-REGISTER.md footer

**ID:** T205
**Owner:** technical-writer
**Status:** pending
**Priority:** P2
**Depends on:** —
**Created:** 2026-09-03
**Completed:** —
**Based on:** Discovered during the C062 (release v1.0.0) merge-to-main sanity check, while
verifying MR !213's diff against `docs/DEBT-REGISTER.md`. Logged separately per the
established T199/T200/T201 cross-boundary-gap pattern — cosmetic, non-blocking, does not
gate the release.

## Objective

`docs/DEBT-REGISTER.md`'s closing footer paragraph (around line 419-424, distinct from the
"C060 release classification" section already corrected in MR !211) has two stale claims
in one sentence, both now factually wrong:

1. It says **"26/26 rows classified, zero unclassified"** — this was accurate when C060 ran
   its initial classification pass (2026-08-29), but C061's own audit subsequently added two
   more classified rows (R-11, R-12, both `v1.1`-classified LOW findings). The current live
   count is **28/28**, not 26/26. (Confirmed against `docs/artifacts/release-v1.0.0.md`'s
   Evidence section, which already cites "28/28 rows classified, zero unclassified (C060)"
   as of the v1.0.0 release — the footer just never caught up.)
2. It says the B1/R-1/R-6 `documented-limitation` rows "still need matching
   `docs/LIMITATIONS.md` entries from **C063**, which had not yet run at the time of this
   classification pass." C063 has since run and published `docs/LIMITATIONS.md` (covering
   B1, R-1, R-6 plus R-11/R-12 and the deferred-feature set, per
   `docs/artifacts/release-v1.0.0.md`'s Evidence section). This sentence is now inaccurate
   and should either be updated to reflect that C063 closed the gap, or removed if it's no
   longer forward-looking information worth keeping.

Fix the footer paragraph to be accurate as of the current state of the register. Do not
touch the historical scorecard tables above it (Phase 1 / Phase 2 archived tables) — those
are explicitly frozen as point-in-time records per their own preceding note ("intentionally
left unchanged by C060's ... re-classification pass").

## Inputs

- `docs/DEBT-REGISTER.md` (the file to fix — footer paragraph, currently around line
  419-424)
- `docs/artifacts/release-v1.0.0.md` (canonical current numbers: "28/28 rows classified,
  zero unclassified (C060)"; confirms `docs/LIMITATIONS.md` is published and covers B1,
  R-1, R-6)
- `docs/LIMITATIONS.md` (C063's output — confirm it actually covers B1/R-1/R-6 before
  asserting the gap is closed)

## Acceptance criteria

1. The footer paragraph's row count matches the register's actual current live count
   (28/28 as of this writing — re-verify by counting, don't just trust this brief, in case
   more rows land between now and execution)
2. The footer no longer claims C063 "had not yet run" — replace with accurate,
   present-tense language (or remove the sentence if it's no longer useful once accurate)
3. No other content in the file is altered (especially the frozen historical scorecard
   tables)
4. The fix is purely descriptive/cosmetic — it must not change any row's actual
   classification (`fixed` / `documented-limitation` / `v1.1`)

## Verification commands

```bash
grep -c '| [A-Z][0-9]*-\?[0-9]* \|' docs/DEBT-REGISTER.md  # sanity count against footer claim
grep -n "26/26\|had not yet run" docs/DEBT-REGISTER.md      # should be zero hits after fix
```

## Git rails

- Branch: `agent/technical-writer/T205` from `develop`
- Commit: `docs(debt-register): fix stale footer row-count and C063-status claims`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

<filled during execution>
