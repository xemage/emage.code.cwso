# Task C032 — Execute the ADR-013 protocol decision

**ID:** C032
**Owner:** backend-developer
**Status:** done
**Priority:** P1
**Depends on:** C031 (ADR-013 approved)
**Created:** 2026-08-12
**Completed:** 2026-08-20
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

Added `orchestrator/internal/server/mcp_conformance_test.go` (16 test functions):
spec-shaped request/response/error assertions for the 4 gap-table "Implemented"
methods (`ping`, `notifications/initialized`, `tools/list`, `tools/call`), the
6 "Partial" methods (`initialize`, `resources/list`, `resources/templates/list`,
`resources/read`, `resources/subscribe`, `resources/unsubscribe`), correct
spec-shaped `MethodNotFound` (-32601) errors for the 6 genuinely "Missing"
methods (`prompts/list`, `prompts/get`, `logging/setLevel`,
`completion/complete`, `sampling/createMessage`, `roots/list`), and a sweep
confirming 7 notification types are never emitted on the event bus across a
representative call sequence.

Two required fixes surfaced and applied during implementation (not scope
creep — both directly required to make the conformance suite's own
assertions true): (1) `capabilities.resources.listChanged` flipped `true` →
`false` in `handleInitialize`, because `notifications/resources/list_changed`
is never actually published anywhere in the codebase — the server was
advertising a capability it doesn't deliver; (2) a new `mcp.RequestError{Code,
Err}` type distinguishes malformed-JSON (-32700, Parse error) from
wrong-protocol-version/missing-method (-32600, Invalid Request), previously
collapsed into a single code. `ErrUnauthorized` (-32001) removed as genuinely
dead code — auth is fully handled at the HTTP transport layer; confirmed via
a whole-repo grep (not just `internal/mcp/`) for any reachable call site,
none found. `POC-DEBT` hand-rolled-subset marker removed from `protocol.go:10`
only once the above was verified true; `docs/DEBT-REGISTER.md` B1 flipped
`open`/`v1.0-blocker` → `closed`/`fixed`, referencing C030–C032. Zero tool
schema/semantics changes, zero new MCP methods added, zero touches to
`services/*`/`deploy/*`/`schemas/*`.

**VERDICT: CONDITIONAL_PASS → resolved** (independent Tech Lead review, MR
!141). First-round review confirmed the `listChanged:false` fix and
`ErrUnauthorized` removal were both correct and in-scope (not unrelated
scope creep), confirmed 100% of the existing test suite green with zero
weakened assertions, and confirmed the DEBT-REGISTER B1 claim was actually
true (not just relabeled) — but attached two blocking conditions: (1) the
MR's own gating pipeline hadn't yet finished green, and (2) a genuine
test-coverage gap: `resources/unsubscribe` was previously only incidentally
exercised inside an unrelated notification-sweep test with its own response
discarded — no dedicated assertion existed for its response shape or the
unknown-subscription-id error case. The orchestrator independently
re-derived condition 2 directly from the diff and the actual
`handleResourcesUnsubscribe` handler (confirming the handler logic itself
was already correct — a genuine test gap, not an implementation bug) before
dispatching a fix. A follow-up commit (`177283d`) added
`TestConformanceResourcesUnsubscribeSpecShapeAndUnknownID` (happy path +
unknown-id error case + a bonus double-unsubscribe check). A second,
independent Tech Lead re-review verified this new test non-vacuously via
**mutation testing** — temporarily broke the handler's not-found path in a
disposable worktree, confirmed the new test actually failed, reverted,
confirmed it passed again — and cross-checked the test's expectations
directly against the real handler code. Condition 2 fully closed.

Condition 1 (CI green) was hit twice by transient, content-independent
shared-runner flakiness during the review/merge cycle: a `docker compose up`
container-name collision between concurrently-scheduled `e2e:smoke`/
`e2e:phase4-swarm` jobs (`Conflict. The container name "/cwso-git-shadow" is
already in use`, same class as prior C018/T191 incidents this roadmap), and
separately, on a retry of `e2e:phase4-swarm`, the previously-characterized
"RPC connection-refused under concurrent load" pattern. Both independently
confirmed transient by re-running each failed job individually to a clean
pass (not assumed) — full 14/14 green before merge. Also corrected,
non-blocking: the MR description's inaccurate "~21 test functions" claim
fixed to the real, grep-confirmed 16.

MR !141 (`agent/backend-developer/C032`), merged to `develop` via merge
commit `69bb3720`. **Unblocks C033** (client compatibility matrix) and
**C034** (contract snapshot test in CI), both listed as depending on C032,
not yet dispatched.
