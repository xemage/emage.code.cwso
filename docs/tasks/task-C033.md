# Task C033 — Client compatibility matrix (3 clients × 2 transports)

**ID:** C033
**Owner:** qa-engineer
**Status:** in_progress
**Priority:** P1
**Depends on:** C032
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C033 row); docs/plans/plan-cwso-v1.0-phase3-protocol-conformance-v1.md

## Objective

Verify the post-C032 protocol surface against at least three real MCP clients over both
stdio and Streamable HTTP, and publish the results — **including failures**. A matrix
that only shows green is marketing, not verification.

## Inputs

- Post-C032 orchestrator
- `docs/artifacts/mcp-gap-analysis-v1.md` (what "correct" means per method)
- Three real MCP clients (e.g., Claude Desktop / Claude Code, Cursor, VS Code MCP — confirm availability with the orchestrator before starting)

## Rails (read before starting)

### You MUST
- Test each client × each transport (stdio, Streamable HTTP) = at least 6 cells
- Per cell, test: initialize handshake, `tools/list`, a `tools/call` happy path (e.g., `create_shadow_workspace`), and two error paths (unknown method, malformed params)
- Produce `docs/artifacts/mcp-client-compatibility-v1.md`: matrix with pass/fail per cell and, for every failure, the client, transport, method, expected vs actual, and a reproduction note
- Publish failures honestly — a documented failure is a valid cell result

### You MUST NOT
- Test only happy paths
- Patch the server mid-test to force a pass — a failure is a finding; file it and record it
- Substitute a hand-rolled script for a "real client" — the point is real-client quirks (a scripted JSON-RPC client may be a 4th row, not a substitute)
- Modify server code

## File ownership

- **May create/modify:** `docs/artifacts/mcp-client-compatibility-v1.md` (new), test harness scripts under `scripts/` if needed
- **Must NOT touch:** `orchestrator/*`, `services/*`

## Steps (execute in order)

1. Confirm the three clients with the orchestrator.
2. Build the per-cell test procedure (handshake, tools/list, happy call, 2 error paths).
3. Run all cells; record everything.
4. Write the matrix artifact with failures included.

## Expected outputs

- `docs/artifacts/mcp-client-compatibility-v1.md` (≥3 clients × 2 transports)

## Acceptance criteria

1. ≥6 cells tested with the 5-step per-cell procedure
2. Every failure recorded with reproduction detail
3. Matrix published (feeds C050, C063, and emage.code T422)

## Verification commands

```bash
grep -c "stdio\|Streamable HTTP" docs/artifacts/mcp-client-compatibility-v1.md
grep -c "FAIL\|fail" docs/artifacts/mcp-client-compatibility-v1.md   # failures recorded if any
git diff --stat
```

## Git rails

- Branch: `agent/qa-engineer/C033` from `develop`
- Commit: `test: publish MCP client compatibility matrix`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If a named client is unavailable in the test environment, report `external` / `major`
and propose a substitute real client — do not silently test fewer than three.

## Execution notes

<filled during execution>
