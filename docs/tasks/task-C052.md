# Task C052 — Receive emage.code deployment docs (T403 ⇄ C052)

**ID:** C052
**Owner:** technical-writer
**Status:** pending
**Priority:** P1
**Depends on:** C050; emage.code T403 (paired handover)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C052 row, §2.7); docs/plans/plan-cwso-v1.0-phase5-one-document-v1.md

## Objective

Receive the six deployment guides relocated from emage.code (their T403) and fold them
into this repo's single documentation tree. This is a **paired handover**: neither repo
lands its side until both are ready — the docs must not be dropped between the repos.

## Inputs

- emage.code T403 (the sending side — coordinate timing with the orchestrator)
- The six incoming deployment guides (from emage.code)
- `docs/user/README.md` (C050 — the tree they fold into)

## Rails (read before starting)

### You MUST
- Confirm with the orchestrator that emage.code's T403 is ready before starting (hard ordering constraint, roadmap §2.7)
- Place the received guides under `docs/user/deployment/` and link them from the single guide's deployment section
- Normalize filenames to this repo's conventions (no version suffixes; kebab-case)
- Record the provenance of each received file (source repo + original path) in a `docs/user/deployment/README.md` index
- Add a CHANGELOG entry

### You MUST NOT
- Land this before emage.code's T403 side is ready (and vice versa — coordinate via orchestrator)
- Rewrite the received content substantively — fold in, normalize names, link; content edits are a separate task if needed
- Leave duplicates: if a received guide overlaps the single guide, link rather than duplicate
- Touch code

## File ownership

- **May create/modify:** `docs/user/deployment/` (new files), `docs/user/README.md` (deployment section links), `CHANGELOG.md`
- **Must NOT touch:** code, other docs, the emage.code repo (that is T403's side)

## Steps (execute in order)

1. Confirm T403 readiness with the orchestrator.
2. Receive the six guides; place under `docs/user/deployment/`.
3. Normalize names; write the provenance index.
4. Link from the single guide; CHANGELOG.
5. Confirm with the orchestrator that both sides landed.

## Expected outputs

- `docs/user/deployment/` with the six guides + provenance index
- Single guide links them
- CHANGELOG entry

## Acceptance criteria

1. Six guides present, normalized, indexed with provenance
2. Single guide's deployment section links them
3. Both repos agree the handover completed (orchestrator confirms T403 landed)

## Verification commands

```bash
ls docs/user/deployment/ | wc -l   # = 7 (6 guides + index)
grep -c "deployment/" docs/user/README.md
git diff --stat
```

## Git rails

- Branch: `agent/technical-writer/C052` from `develop` (rebased on merged C050)
- Commit: `docs(user): receive deployment guides from emage.code (T403)`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If T403 is not ready, this task is `blocked` / `dependency` / `critical` — do not
proceed with a partial handover.

## Execution notes

<filled during execution>
