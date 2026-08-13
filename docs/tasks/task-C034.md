# Task C034 — Contract snapshot test: protocol drift breaks CI

**ID:** C034
**Owner:** qa-engineer
**Status:** pending
**Priority:** P1
**Depends on:** C032
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C034 row); docs/plans/plan-cwso-v1.0-phase3-protocol-conformance-v1.md

## Objective

Add a contract snapshot test so that any drift in the MCP protocol surface fails CI.
Align with emage.code's `test_cwso_mcp_contract_snapshot.py` so both repos test the
same surface from both ends.

## Inputs

- Post-C032 orchestrator (the surface to snapshot)
- `schemas/` (the tool-shape baseline)
- emage.code's `test_cwso_mcp_contract_snapshot.py` (fetch from the emage.code repo — align expectations; coordinate via the orchestrator)
- `docs/artifacts/mcp-gap-analysis-v1.md` (the intended surface)

## Rails (read before starting)

### You MUST
- Generate the snapshot from the **post-C032** surface (not before)
- Snapshot: method set, notification set, error-code behavior, and tool schema shapes (from `schemas/`)
- Review the snapshot against the C030 gap table before merging (the snapshot must match the *intended* surface, not calcify a mistake)
- Wire the test into CI so a surface change without a snapshot update fails the pipeline
- Document how to regenerate the snapshot deliberately (a `make` target or script flag)

### You MUST NOT
- Generate the snapshot before C032 merges
- Auto-update the snapshot in CI (drift must fail, not self-heal)
- Snapshot timestamps, version strings, or other volatile fields
- Modify server code

## File ownership

- **May create/modify:** test files (Go test or script), the snapshot file, `.gitlab-ci.yml` (one job), `Makefile` (regen target if used)
- **Must NOT touch:** `orchestrator/internal/*` implementation, `services/*`, `schemas/*`

## Steps (execute in order)

1. Fetch emage.code's snapshot test; note the surface it asserts.
2. Generate the CWSO-side snapshot from the post-C032 surface.
3. Review against the gap table.
4. Write the test + CI job + regen path.
5. Prove drift fails CI: change a method shape locally, watch the test fail, revert.

## Expected outputs

- Contract snapshot + test + CI job
- Regen documentation

## Acceptance criteria

1. Snapshot matches the post-C032 intended surface (reviewed against gap table)
2. A deliberate surface change fails the test (evidence in MR)
3. CI runs the test
4. Both repos' snapshot tests assert the same surface (noted in MR)

## Verification commands

```bash
cd orchestrator && go test ./... -run Contract
# deliberate-drift proof:
# (edit a method shape) → test fails → revert → test passes
git diff --stat
```

## Git rails

- Branch: `agent/qa-engineer/C034` from `develop`
- Commit: `test: add MCP contract snapshot test`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If emage.code's snapshot test is unreachable, report `dependency` / `major` — do not
invent its expectations from memory.

## Execution notes

<filled during execution>
