# Task C031 — ADR-013: official SDK vs hand-rolled + conformance suite

**ID:** C031
**Owner:** solution-architect
**Status:** in_progress
**Priority:** P1
**Depends on:** C030
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B1, open question 2); docs/plans/plan-cwso-v1.0-phase3-protocol-conformance-v1.md; docs/artifacts/mcp-gap-analysis-v1.md

## Objective

Document, in ADR-013, the protocol path for v1.0 and scope its execution. **The human
decided on 2026-08-13 (roadmap Approval, decision 2): keep the hand-rolled kernel and
prove it** — the kernel is a deliberate determinism choice, and a rewrite at v0.9
carries real risk. ADR-013 records that decision with its rationale, records the
official-SDK option as considered-and-rejected, and scopes the conformance suite from
the C030 gap table. What remains your job: the *evidence-based scoping* — which methods,
notifications, and error codes the suite must prove, per the gap table.

## Rails addendum (decision already made)

- The ADR's "Decision" section states: keep hand-rolled + conformance suite, decided
  by the human on 2026-08-13; your analysis supports and scopes it, it does not reopen it
- The SDK option is documented as considered-and-rejected with the determinism and
  rewrite-risk rationale
- Reversal criteria still required: what future evidence would justify revisiting the SDK

## Inputs

- `docs/artifacts/mcp-gap-analysis-v1.md` (C030 — the evidence base)
- `orchestrator/internal/mcp/protocol.go` and package
- The official Go MCP SDK (evaluate its maturity, API stability, and determinism guarantees)
- `docs/decisions/_template.md`; ADR-012 for format reference

## Rails (read before starting)

### You MUST
- Ground every claim in the C030 gap table (e.g., "N of M lifecycle methods partial" — cite the table)
- Evaluate the SDK option on: API stability, determinism (does it preserve the kernel's deterministic behavior?), migration effort estimated from the gap table, and ongoing maintenance
- Evaluate the keep-and-prove option on: conformance-suite cost (from the gap table size), risk of spec drift, and the honesty of the resulting surface
- Give a plain recommendation with reasoning; number the ADR **ADR-013**, status `proposed`, with an "Approval required" section
- State what would change the decision in the future (reversal criteria)

### You MUST NOT
- Write implementation code
- Recommend a rewrite without an effort estimate derived from the gap table
- Treat "the SDK exists" as sufficient reason to adopt it — determinism is the kernel's raison d'être; weigh it
- Decide by preference; decide by the table

## File ownership

- **May create/modify:** `docs/decisions/ADR-013-mcp-protocol-path.md` (new)
- **Must NOT touch:** all code, the gap analysis (immutable input)

## Steps (execute in order)

1. Read the gap table fully.
2. Assess the SDK's current state (version, stability, determinism).
3. Estimate both paths' costs from the gap table.
4. Write ADR-013 with recommendation + reversal criteria + approval section.

## Expected outputs

- `docs/decisions/ADR-013-mcp-protocol-path.md` (status `proposed`)

## Acceptance criteria

1. Both options evaluated against the same criteria, grounded in the gap table
2. Plain recommendation + reversal criteria
3. ADR-013, house format, status `proposed`, approval section present

## Verification commands

```bash
grep -c "gap" docs/decisions/ADR-013-mcp-protocol-path.md
grep -c "reversal\|revisit" docs/decisions/ADR-013-mcp-protocol-path.md
git diff --stat   # exactly 1 new file
```

## Git rails

- Branch: `agent/solution-architect/C031` from `develop`
- Commit: `docs(adr): propose MCP protocol path decision`
- MR target: `develop`, squash and merge, delete source branch
- **MR description ends with the plain recommendation for fast human review**

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the SDK's determinism guarantees cannot be established from public docs, record
that as a finding weighing against adoption — do not assume them.

## Execution notes

<filled during execution>
