# Task C063 — Publish docs/LIMITATIONS.md

**ID:** C063
**Owner:** technical-writer
**Status:** pending
**Priority:** P0
**Depends on:** C060 (register reclassified — its `documented-limitation` rows are this file's content)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C063 row); docs/plans/plan-cwso-v1.0-phase6-release-v1.md

## Objective

Publish `docs/LIMITATIONS.md`: what CWSO v1.0 does **not** do. This is a feature — it
prevents the next version-drift cycle by making the gap between claim and reality a
maintained document instead of a discovery.

## Inputs

- `docs/DEBT-REGISTER.md` (post-C060: every `documented-limitation` row)
- `docs/SCOPE-v1.0.md` (C005 — the non-goals)
- C025's seed (if the IPC-only fallback activated), C044's UDS outcome, the file-based-secret note (compose `secrets:` comment)
- Roadmap §2.4 (deferred features — as "not in v1.0", not as limitations of what exists)

## Rails (read before starting)

### You MUST
- List every `documented-limitation` row from the register as a full entry: what it is, why it exists, blast radius, and the v1.1 remediation pointer
- Include the standing items: file-based JWT secret (dev-grade secret management; Vault/SOPS is v1.1), Firecracker tier ships as documented fallback (not promoted), UDS perms (if C044 documented rather than fixed), IPC-only workspaces (only if C025 activated)
- Include a "Not in v1.0" section summarizing §2.4 deferred features (HAL, sparse, rollout beyond opt-in, SWE-bench/Terminal-Bench evaluators, K8s operator, Merkle indexer)
- Keep the tone factual: a limitation is a fact, not an apology
- Cross-link from the README and the user guide (one link each)

### You MUST NOT
- List anything as a limitation that the register marks `fixed` (contradiction = CG0 regression)
- Use hedging language ("might not work perfectly") — state what does and does not work
- Turn the file into a roadmap — one pointer to v1.1 per entry, no plans
- Touch code

## File ownership

- **May create/modify:** `docs/LIMITATIONS.md` (new), `README.md` (one link), `docs/user/README.md` (one link)
- **Must NOT touch:** code, `docs/DEBT-REGISTER.md` (C060 owns it)

## Steps (execute in order)

1. Collect the `documented-limitation` rows from the post-C060 register.
2. Write one entry per row + the standing items + the "Not in v1.0" section.
3. Cross-link from README and the user guide.
4. Verify no contradiction with the register.

## Expected outputs

- `docs/LIMITATIONS.md`
- One link each from README and the user guide

## Acceptance criteria

1. Every `documented-limitation` register row has a LIMITATIONS.md entry (C060 cross-check passes)
2. "Not in v1.0" section covers §2.4
3. No contradiction between the register and LIMITATIONS.md
4. Published alongside the v1.0.0 release (C062 consumes it)

## Verification commands

```bash
grep "documented-limitation" docs/DEBT-REGISTER.md | wc -l
grep -c "^## " docs/LIMITATIONS.md
grep -c "LIMITATIONS" README.md docs/user/README.md   # = 1 each
```

## Git rails

- Branch: `agent/technical-writer/C063` from `develop` (rebased on merged C060)
- Commit: `docs: publish v1.0 limitations`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

**Sources read in full before writing:** `docs/DEBT-REGISTER.md` (post-C060, all 28
rows and the "C060 release classification" section), `docs/artifacts/mcp-gap-analysis-v1.md`,
`docs/decisions/ADR-013-mcp-protocol-path.md`, `docs/SCOPE-v1.0.md`, `docs/user/README.md`,
`README.md`, `docs/tasks/active-tasks.md` (for C025's status).

**R-11/R-12 presentation choice:** These two are classified `v1.1` in the register — real,
deferred debt, not a `documented-limitation` — a materially different disposition than
B1/R-1/R-6, whose `documented-limitation` classification is what makes them a *required*
part of C060's release-gate cross-check. I chose option (b) from the brief: a separate,
clearly-labeled "§2 Additional known gaps (tracked as v1.1 debt, disclosed for
transparency)" section, distinct from "§1 Documented limitations (required disclosures,
v1.0 release-gate)". I did not fold them into §1 because doing so would silently blur two
different register semantics (a required-for-v1.0 disclosure vs. deferred-and-not-required
debt) into one undifferentiated list — a reader (or a future automated cross-check against
the register) should be able to tell, from the file's own structure, which entries C060's
release gate actually depends on. Each of §2's two entries states its `v1.1` classification
and register row number explicitly in its own heading, so the distinction is visible at a
glance, not just in prose.

**Contradiction check against the register's current (post-C060) disposition — done
per-entry, not assumed:**
- B1: register says `documented-limitation` (re-verified 2026-08-29, downgraded from a
  prior `fixed` label specifically because the row's own description — a partial method
  set — is still literally true by design). §1.1 states the same: a disclosed, ADR-013-kept
  subset, not fixed. No contradiction.
- R-1: register says `documented-limitation` / `open` / `v1.0-blocker (document)`. §1.2
  matches: file-based secret, acceptable for v1.0's local-only model, production half (R-2)
  still `v1.1`/not started. No contradiction.
- R-6: register says `documented-limitation` (relabeled from `wontfix`, no factual change).
  §1.3 matches: permanent, reviewed, no v1.1 remediation claimed (I explicitly wrote "None"
  rather than inventing a pointer, since the register itself records the Tech Lead review
  recommended never revisiting this). No contradiction.
- R-11, R-12: register says `v1.1` / `open`, findings F-C061-03/F-C061-04. §2.1/§2.2 match
  exactly, including the "requires prior possession of the secret" / "Cloud Run inherently
  mitigated" nuances from the register's own text. No contradiction.
- B12 (UDS perms): register says `fixed` (re-verified live against a running stack, C044,
  2026-08-27). I explicitly did **not** list this as a limitation — §3 states outright that
  listing it would contradict a `fixed` row, and cites the same re-verification evidence
  the register gives, rather than silently omitting it with no explanation.
- B2 (real-filesystem projection / IPC-only workspaces): register says `fixed` via
  materialise-to-tmpfs (ADR-012 GO decision). Checked `docs/tasks/active-tasks.md`: C025
  (the IPC-only-limitation task) is still listed `pending`, gated on "C020 (NO-GO)" — a
  trigger condition that never occurred, since ADR-012 was a GO. Per the brief's own
  instruction, this is correctly omitted as a limitation (§3, third bullet, explains why
  rather than silently dropping it).
- P2-2 (Merkle indexer) and B11 (SWE-bench stub): both `v1.1`/`open` in the register,
  correctly listed only in §4 "Not in v1.0" (deferred-feature framing per roadmap §2.4),
  not duplicated as a §1/§2 disclosed-defect entry, since neither is a defect in shipped
  behavior — they are scope the roadmap never included in v1.0 at all.

No hedging language was used anywhere in the file (no "might not work perfectly" — every
entry states what does and does not work as a fact). Exactly one v1.1/remediation pointer
per §1/§2 entry (R-6 states "None" explicitly rather than inventing one). §4 quotes
`docs/SCOPE-v1.0.md` §2.4 verbatim (which itself quotes the roadmap) rather than
re-describing it, per the "no roadmap embedded in this file" rail.

**Acceptance criteria self-check:**
1. Every `documented-limitation` row (B1, R-1, R-6) has a full entry (what/why/blast
   radius/v1.1 pointer) — §1.1–§1.3. ✓
2. "Not in v1.0" section (§4) covers all nine roadmap §2.4 rows verbatim via
   `docs/SCOPE-v1.0.md`, including the Merkle-indexer cross-reference to P2-2. ✓
3. No contradiction with the register — checked row-by-row above, including the two
   rows (B12, B2/C025) that must NOT appear as limitations. ✓
4. Exactly one link each added: `README.md`'s "Documentation" section now has one new
   bullet, `**[Limitations (v1.0)](docs/LIMITATIONS.md)**`; `docs/user/README.md`'s "Known
   limitations" subsection now ends with one bullet linking
   `[`docs/LIMITATIONS.md`](../LIMITATIONS.md)`, replacing the prior "planned but not
   published yet" placeholder line (which was the file's only other candidate line and is
   now removed, so the substring "LIMITATIONS" should appear on exactly one line in each
   file). I do not have Bash/grep access to run the verification commands in this task
   brief myself; I re-read both edited files in full after editing and found no other line
   containing "LIMITATIONS" in either — the orchestrator should still run the three
   `grep`/`wc -l` verification commands listed above before merge as the authoritative
   check, since I could not execute them.

**Tooling note:** This agent (technical-writer) has no Bash access. All verification above
was done by reading the source files directly rather than running the brief's
`grep`/`wc -l` verification commands. No git operations were performed — all changes are
left uncommitted in the working tree for the orchestrator to commit.

**No blockers.** Task completed within file-ownership boundaries
(`docs/LIMITATIONS.md` created; one link added each to `README.md` and
`docs/user/README.md`; `docs/DEBT-REGISTER.md` and code untouched).
