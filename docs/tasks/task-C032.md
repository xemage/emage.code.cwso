# Task C032 — Execute the ADR-013 protocol decision

**ID:** C032
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C031 (ADR-013 approved)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B1); docs/plans/plan-cwso-v1.0-phase3-protocol-conformance-v1.md; docs/decisions/ADR-013-mcp-protocol-path.md

## Objective

Execute the path decided by the human on 2026-08-13 (roadmap Approval, decision 2) and
documented in ADR-013: **keep the hand-rolled kernel and prove it** — back it with a
conformance suite proving spec parity for every implemented method. v1.0 stops resting
on an undocumented subset. (SDK migration is recorded as considered-and-rejected in
ADR-013; do not revive it here.)

## Inputs

- `docs/decisions/ADR-013-mcp-protocol-path.md` (the approved decision — follow it)
- `docs/artifacts/mcp-gap-analysis-v1.md` (the work list)
- `orchestrator/internal/mcp/`, `orchestrator/internal/transport/`

## Rails (read before starting)

### You MUST
- Follow ADR-013's recorded decision (keep-and-prove) exactly
- Implement a conformance suite asserting spec-shaped requests, responses, and error codes for every method the gap table marks implemented/partial; unimplemented methods must return a correct "not supported" error, never a malformed response
- Close every "partial" row in the gap table or explicitly document it as unsupported-with-correct-error
- Remove the hand-rolled-subset `POC-DEBT` marker (protocol.go:10) when done and update `docs/DEBT-REGISTER.md` (B1 → `fixed`, closing task C032)
- Keep all existing tests green; add conformance tests

### You MUST NOT
- Change the tool surface semantics or schema shapes (that would break clients and C034)
- Expand the implemented method set beyond closing the gap table's "partial" rows — new methods are v1.1
- Start before ADR-013 is human-approved
- Touch the Rust services

## File ownership

- **May create/modify:** `orchestrator/**`, `docs/DEBT-REGISTER.md` (B1 row)
- **Must NOT touch:** `services/*`, `deploy/*`, `schemas/*` (shape must not change), docs

## Steps (execute in order)

1. Read ADR-013 and the gap table.
2. Execute the chosen path method by method.
3. Conformance tests for every implemented method + correct errors for unimplemented ones.
4. Remove the B1 marker; update DEBT-REGISTER.
5. Full test suite green.

## Expected outputs

- Protocol layer per ADR-013, with conformance coverage
- B1 marker removed; DEBT-REGISTER updated

## Acceptance criteria

1. Every implemented method has a conformance test asserting spec-shaped requests/responses/errors
2. Unimplemented methods return correct "not supported" errors
3. `go test ./...` passes in `orchestrator/`
4. No `POC-DEBT` hand-rolled marker remains; DEBT-REGISTER B1 = `fixed`

## Verification commands

```bash
cd orchestrator && go test ./... 
grep -n "POC-DEBT" internal/mcp/protocol.go   # = no hits
grep -c "not supported\|MethodNotFound" internal/mcp/*.go
```

## Git rails

- Branch: `agent/backend-developer/C032` from `develop`
- Commit: `feat(mcp): execute ADR-013 protocol conformance path`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If executing the ADR reveals the gap table was wrong, stop and report
`technical` / `major` — the ADR may need amendment; do not improvise a third path.

## Execution notes

<filled during execution>
