# Task C036 — Fix `resources/list` nil-slice-marshals-to-null bug

**ID:** C036
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** —
**Created:** 2026-08-26
**Completed:** —
**Based on:** `docs/artifacts/mcp-client-compatibility-v1.md` (C033) § "Cross-cutting findings," Finding A

## Objective

`orchestrator/internal/server/server.go`'s `handleResourcesList` declares its result
slice as `var resources []mcp.Resource` — a Go nil slice. When there are zero active
AST-spike/sparse-agent subscriptions (the common case), `resources` stays `nil` and
`encoding/json` marshals a nil slice as `null`, not `[]`. The response CWSO actually
sends is `{"resources": null}`, not the spec-shaped `{"resources": []}`.

This was independently discovered and reproduced by C033's client-compatibility
testing against **three real, independent MCP clients**, not a hand-rolled test — and
it has real, observed impact:

- **`wong2/mcp-cli`** (a real, published, SDK-based client): its default connection
  flow eagerly calls `resources/list` right after `initialize` because CWSO advertises
  the `resources` capability, and its underlying SDK's Zod-based response schema
  correctly requires `resources` to be an array. Receiving `null` throws a *synchronous,
  uncaught* validation error inside the SDK's own inbound-message handler — outside any
  `try/catch` in the client's own code — crashing the process before it prints anything
  usable. This is a schema-strict client correctly rejecting a spec-noncompliant
  response; it is not a bug in that client.
- **Claude Code** (both stdio and Streamable HTTP): does not crash, but logs a real
  `[ERROR] Failed to fetch resources` after 3 retries with backoff, because its resources
  fetch and tools fetch are independent (not bundled the way `wong2/mcp-cli`'s is).
- **MCP Inspector CLI**: unaffected in the tested cells, only because its `--method`
  invocations don't implicitly trigger a `resources/list` call the way the other two
  clients proactively do.

The fix is small and precisely scoped, and the correct pattern already exists in the
same file: the very next function, `handleResourceTemplatesList`, correctly
initializes with `make([]mcp.ResourceTemplate, 0, 2)` — a non-nil slice that marshals
to `[]` even when nothing is appended.

## Inputs

- `orchestrator/internal/server/server.go`'s `handleResourcesList` (the nil-slice bug)
  and `handleResourceTemplatesList` (the correct, adjacent pattern to mirror)
- `docs/artifacts/mcp-client-compatibility-v1.md` § "Cross-cutting findings," Finding A
  (the full reproduction, including the raw JSON-RPC repro command and per-client impact)
- `orchestrator/internal/server/mcp_conformance_test.go` (C032) and
  `mcp_contract_snapshot_test.go` (C034) — the existing conformance/snapshot test
  infrastructure this fix's regression test should extend, not duplicate

## Rails (read before starting)

### You MUST
- Fix `handleResourcesList` so that `resources/list` returns `{"resources": []}`, never
  `{"resources": null}`, when there are zero active spike/sparse-agent subscriptions —
  mirror `handleResourceTemplatesList`'s existing, correct initialization pattern in the
  same file rather than inventing a new one
- Add a regression test asserting the *empty* case specifically returns `[]` in the
  marshaled JSON, not just that the Go value is "falsy" or has length zero (the bug is
  specifically about JSON marshaling behavior, so the test must inspect the actual wire
  bytes/JSON, not just the Go slice's `len()`)
- Independently reproduce the raw JSON-RPC repro from `mcp-client-compatibility-v1.md`
  (`resources/list` with zero subscriptions) against your fix and confirm the response
  body is genuinely `{"resources":[]}`
- Keep the non-empty case's behavior byte-for-byte unchanged (when subscriptions exist,
  the response must be identical to before this fix)

### You MUST NOT
- Change the `resources` capability advertisement, the resource URI/shape for non-empty
  results, or any other method's behavior
- Touch `handleResourceTemplatesList` itself (it is already correct — cite it, don't
  "fix" it)
- Expand scope into `resources/read`/`resources/subscribe`/`resources/unsubscribe` or
  any other method not named in this bug report
- Touch `services/*`, `deploy/*`, or `schemas/*`

## File ownership

- **May create/modify:** `orchestrator/internal/server/server.go` (only
  `handleResourcesList`), `orchestrator/internal/server/*_test.go` (new/extended test)
- **Must NOT touch:** `services/*`, `deploy/*`, `schemas/*`, other `orchestrator/*` files

## Steps (execute in order)

1. Read `handleResourcesList` and `handleResourceTemplatesList` to confirm the bug and
   the correct sibling pattern yourself, don't take this brief's description on faith.
2. Apply the one-line-class fix (initialize `resources` as a non-nil, zero-length slice).
3. Add the regression test (inspect actual marshaled JSON for the empty case).
4. Reproduce the raw JSON-RPC repro against your fix.
5. Run the full existing test suite to confirm no regression.

## Expected outputs

- `resources/list` returns `{"resources":[]}` when empty
- A regression test proving it, asserting on the marshaled JSON shape

## Acceptance criteria

1. `resources/list` with zero active subscriptions returns `{"resources":[]}`, confirmed
   via the raw JSON-RPC repro
2. `resources/list` with one or more active subscriptions is byte-for-byte unchanged
   from pre-fix behavior
3. New regression test asserts on the actual marshaled JSON, not just Go-level `len()`
4. `go test ./...` passes in `orchestrator/`, including the existing C032/C034
   conformance/snapshot suites (confirm the snapshot test doesn't need a deliberate,
   reviewed update — if it does, that update must be deliberate, per C034's own "no
   auto-update in CI" rule, and explained in your MR, not silently regenerated)

## Verification commands

```bash
cd orchestrator && go test ./...
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"resources/list"}' \
  | docker exec -i cwso-orchestrator /usr/local/bin/cwso-orchestrator
# expect: {"jsonrpc":"2.0","id":1,"result":{"resources":[]}}  (zero subscriptions)
```

## Git rails

- Branch: `agent/backend-developer/C036` from `develop`
- Commit: `fix(mcp): resources/list returns empty array, not null, when empty`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If fixing this reveals the same nil-slice pattern exists elsewhere in a way that would
expand this task's scope meaningfully, report it as a separate finding — do not
silently widen this task's diff to cover it.

## Execution notes

<filled during execution>
