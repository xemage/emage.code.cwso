# Task C033 — Client compatibility matrix (3 clients × 2 transports)

**ID:** C033
**Owner:** qa-engineer
**Status:** done
**Priority:** P1
**Depends on:** C032
**Created:** 2026-08-12
**Completed:** 2026-08-27
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

Before dispatch, the orchestrator ran a real environment check (no assumption): no X
server reachable in this sandbox at all; Cursor's CLI is only a non-functional
agent-wrapper stub, not a real IDE install; VS Code Server is genuinely present and
live-connected (confirmed via `code --status`) but has no scriptable/headless path to
drive its MCP-capable extensions; Claude Code CLI is a confirmed-real, installed MCP
client; Node/npx are available for the official MCP Inspector.

Produced `docs/artifacts/mcp-client-compatibility-v1.md`: 3 genuinely real,
independently-implemented clients — Claude Code CLI, MCP Inspector (`--cli` mode,
v1.0.2), and `@wong2/mcp-cli` (third-party, SDK-based) — over both stdio and
Streamable HTTP (6 cells, 30 checks). Cursor ruled out as a confirmed stub; VS Code
investigated in genuine depth and correctly ruled out on evidence (a live, connected
Desktop session with real MCP-relevant extensions installed, but no way to drive it
from this session) rather than silently substituted. A plausible bonus 4th client
(`cline` npm CLI) was identified but deliberately not pursued once 3 real clients
already satisfied the requirement, to avoid spending a live billed credential on a
non-required row — flagged explicitly as a judgment call.

Surfaced two genuine, reproducible protocol-conformance bugs, neither fixed here
(out of scope) but logged as new tasks: **C036** (`resources/list` nil-slice-marshals-
to-null bug, crashes `wong2/mcp-cli` outright) and **C037** (the OAuth-fallback/401-
parsing failure mode already documented for VS Code is not VS-Code-specific).

**VERDICT: CONDITIONAL_PASS → resolved** across two independent Tech Lead review
rounds (MR !166). First round independently reproduced both bugs byte-for-byte and
confirmed the matrix/client-selection reasoning, with two narrow conditions: an
arithmetic error in the acceptance-criteria table (11 of 30 → correctly 8 of 30,
22 PASS/6 FAIL/2 N/A) and an MCP Inspector `2.4.0` exclusion claim that didn't
reproduce for the reviewer. Both corrected directly, independently re-verified by
the orchestrator before pushing (personally re-tallied the 30 cells; personally
reproduced `2.4.0` launching cleanly twice from a local package cache). Edited the
artifact in place under its existing `-v1` filename rather than bumping to `-v2`,
reasoned and disclosed in a corrigendum note (not yet merged/published at correction
time; the two dependent task briefs cite it only for Findings A/B, unchanged by
either fix). Second round confirmed both fixes and the versioning call, and found
one further self-referential arithmetic slip in the fix's own explanatory text —
resolved by iterating to an actual, verified fixed point rather than guessing a
digit, applied directly given its purely mechanical nature.

MR !166 (`agent/qa-engineer/C033`), merged to `develop` via merge commit `8cdf4566`.
Closes gate CG3 ("Protocol") — the entire C030→C034→C033 chain is complete. Both
CG2 and CG3 are now closed, fully unblocking Phase 4 (C040–C044, "Correctness").
