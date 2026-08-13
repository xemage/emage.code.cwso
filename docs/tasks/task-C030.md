# Task C030 — MCP gap table: implementation vs spec

**ID:** C030
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C001–C005 (gate CG0)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B1); docs/plans/plan-cwso-v1.0-phase3-protocol-conformance-v1.md

## Objective

Enumerate exactly which MCP methods, notification types, and error codes
`orchestrator/internal/mcp/protocol.go` implements versus the MCP spec (`2025-03-26`),
and publish the gap table **before** anyone decides whether to adopt the official SDK.
The decision (C031) is only as good as this table.

## Inputs

- `orchestrator/internal/mcp/protocol.go` (the hand-rolled subset; marker at line 10)
- `orchestrator/internal/mcp/` (whole package), `orchestrator/internal/transport/` (stdio + HTTP)
- MCP specification `2025-03-26` (fetch the official spec; cite section numbers)
- `schemas/` (the tool surface)

## Rails (read before starting)

### You MUST
- Produce `docs/artifacts/mcp-gap-analysis-v1.md` with three tables:
  1. **Methods**: every spec method → implemented / partial / missing, with a code reference for implemented ones
  2. **Notifications**: every spec notification type → implemented / missing
  3. **Error codes**: spec error codes → used / unused / misused, with code references
- Cite the spec section for every row
- Record spec ambiguities as their own findings (a fourth "Ambiguities" section) instead of guessing intent
- Include lifecycle methods (`initialize`, `initialized`, `ping`, `tools/list`, `tools/call`, etc.) explicitly — they are the rows clients hit first

### You MUST NOT
- Change any code — this is analysis only
- Recommend a solution — that is C031's job; this table states facts
- Rely on memory for the spec — fetch it and cite sections
- Skip "boring" rows: a missing `notifications/cancelled` matters to real clients

## File ownership

- **May create/modify:** `docs/artifacts/mcp-gap-analysis-v1.md` (new)
- **Must NOT touch:** all code

## Steps (execute in order)

1. Inventory the implemented methods/notifications/error codes from the code.
2. Fetch the spec; enumerate the full method/notification/error surface.
3. Build the three tables with code + spec-section references.
4. Write the Ambiguities section.
5. Self-check: every spec method appears in the table exactly once.

## Expected outputs

- `docs/artifacts/mcp-gap-analysis-v1.md`

## Acceptance criteria

1. Three complete tables (methods, notifications, error codes)
2. Every row has a spec-section citation; implemented rows also have a code reference
3. Ambiguities recorded as findings, not resolved by guess
4. Published (merged) before C031 starts

## Verification commands

```bash
grep -c "spec §\|spec section\|2025-03-26" docs/artifacts/mcp-gap-analysis-v1.md
grep -c "initialize\|tools/list\|tools/call\|ping" docs/artifacts/mcp-gap-analysis-v1.md
git diff --stat   # exactly 1 new file
```

## Git rails

- Branch: `agent/backend-developer/C030` from `develop`
- Commit: `docs: publish MCP implementation-vs-spec gap analysis`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the spec version is ambiguous (README says `2025-03-26`; a newer spec exists),
analyze against `2025-03-26` as the spec of record and note the newer version as an
ambiguity finding — do not silently switch spec versions.

## Execution notes

<filled during execution>
