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

<filled during execution>
