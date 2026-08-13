# Task C025 — CONDITIONAL: document the IPC-only limitation (escape hatch)

**ID:** C025
**Owner:** technical-writer
**Status:** pending
**Priority:** P0
**Depends on:** C020 (ADR-012 approved: **NO-GO** — only then does this task activate)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C025 row, §2.5 risk 1); docs/plans/plan-cwso-v1.0-phase2-real-filesystem-v1.md

## Objective

**Conditional task — activates only if ADR-012 concludes NO-GO.** If the filesystem
projection cannot be delivered on the target host matrix, say so plainly: scope v1.0 to
IPC-only shadow workspaces with the limitation stated in the README, `SCOPE-v1.0.md`,
and a new `docs/LIMITATIONS.md` seed. An honest limitation beats a silent one.

## Inputs

- `docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md` (the NO-GO record)
- `README.md` (What CWSO is / quick-start sections)
- `docs/SCOPE-v1.0.md` (C005)

## Rails (read before starting)

### You MUST
- State the limitation in user-facing terms: shadow workspaces are reachable only via MCP tool calls (`read_shadow_file`/`write_shadow_file`); sub-agents cannot `cd` into them; tools requiring a real path will not work
- Update: README (a "Known limitations" note near the top), `docs/SCOPE-v1.0.md` (amend the v1.0 definition with a dated addendum citing ADR-012 — do not rewrite the verbatim quote; add an addendum section), and seed `docs/LIMITATIONS.md` with this as the first entry
- Reference ADR-012 as the decision record in every touched file
- Add a CHANGELOG entry

### You MUST NOT
- Activate this task if ADR-012 says GO — check with the orchestrator first
- Soften the limitation into marketing language ("coming soon" is allowed only as a clearly-marked v1.1 pointer, not as a hedge)
- Modify ADR-012 (it is immutable once accepted)
- Delete the projection code or P2-1 marker — the limitation is documented, the code path stays for v1.1

## File ownership

- **May create/modify:** `README.md`, `docs/SCOPE-v1.0.md` (addendum section only), `docs/LIMITATIONS.md` (new), `CHANGELOG.md`
- **Must NOT touch:** code, ADR-012, other docs

## Steps (execute in order)

1. Confirm with the orchestrator that ADR-012 concluded NO-GO.
2. Write the limitation text; apply to the three files.
3. Seed `docs/LIMITATIONS.md`.
4. CHANGELOG.

## Expected outputs

- README limitation note
- SCOPE-v1.0.md addendum
- `docs/LIMITATIONS.md` seeded
- CHANGELOG entry

## Acceptance criteria

1. A new user reading only the README learns that shadow workspaces are IPC-only in v1.0
2. Every touched file cites ADR-012
3. The §1.5 definition's fate is addressed explicitly (which clause is deferred to v1.1)

## Verification commands

```bash
grep -c "IPC-only\|ADR-012" README.md docs/SCOPE-v1.0.md docs/LIMITATIONS.md
git diff --stat
```

## Git rails

- Branch: `agent/technical-writer/C025` from `develop`
- Commit: `docs: document IPC-only shadow workspace limitation for v1.0`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

<filled during execution>
