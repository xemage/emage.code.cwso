# ADR-013 — MCP protocol path: keep the hand-rolled kernel, prove it with a conformance suite

> Filename: `ADR-013-mcp-protocol-path.md`

- **Status**: proposed
- **Date**: 2026-08-16
- **Decider(s)**: human (decision authority, roadmap Approval §, decision 2, 2026-08-13); solution-architect (this record — rationale write-up and conformance-suite scoping, not the decision itself)
- **Tasks**: C030 (gap table, done — MR !112), C031 (this ADR), C032 (execute the decision), C033 (client compatibility matrix), C034 (contract snapshot test)
- **Based on**: `docs/plans/plan-cwso-v1.0-roadmap.md` (blocker B1; §2.1 gate CG3; §2.5 risk 2; Approval §, decision 2); `docs/plans/plan-cwso-v1.0-phase3-protocol-conformance-v1.md`; `docs/artifacts/mcp-gap-analysis-v1.md` (C030 — the evidence base for every claim below)

## Format note

`docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md` — cited in the task
brief as the most recent format precedent — does not yet exist in this worktree: its
owning task, **C020**, is still `pending` per `docs/tasks/active-tasks.md`. This ADR
therefore follows `docs/decisions/_template.md` directly, with `docs/archive/decisions/ADR-002-streamable-http-transport.md`
consulted as a secondary, older-format reference for section naming. Recorded here so a
reader doesn't assume ADR-012 was checked and silently diverged from.

## Context

CWSO's MCP server implements the protocol with a hand-rolled kernel rather than the
official SDK: JSON-RPC envelope types and error constants live in
`orchestrator/internal/mcp/protocol.go`, and the actual method-dispatch switch lives in
`orchestrator/internal/server/server.go:789-827` (`Server.Handle`) — a single synchronous
function, one request in, one response out, no per-call concurrency in the dispatch path
itself. `protocol.go:10` carries a `POC-DEBT: Hand-rolled MCP subset; production must
adopt [the official SDK]` marker, which the v1.0 roadmap named blocker **B1** and gated
behind **CG3 — Protocol**: *"v1.0 cannot be declared on the hand-rolled MCP subset (B1)
unless a written conformance suite proves parity with the spec for every implemented
method."*

Task **C030** produced `docs/artifacts/mcp-gap-analysis-v1.md`, an exhaustive,
self-checked inventory of the hand-rolled implementation against MCP spec **2025-03-26**
(the spec of record per `docs/archive/decisions/ADR-002-streamable-http-transport.md`).
It contains three tables and is the sole evidence base for this ADR:

- **Methods** (16 rows: 15 spec request methods + `notifications/initialized` included
  for lifecycle coverage — self-checked exhaustive): **4 Implemented** (`ping`,
  `notifications/initialized`, `tools/list`, `tools/call`), **6 Partial** (`initialize`,
  `resources/list`, `resources/templates/list`, `resources/read`, `resources/subscribe`,
  `resources/unsubscribe`), **6 Missing** (`prompts/list`, `prompts/get`,
  `logging/setLevel`, `completion/complete`, `sampling/createMessage`, `roots/list`).
- **Notifications** (9 spec-defined rows, self-checked exhaustive): **1
  Implemented-lenient** (`notifications/initialized`), **8 Missing**, of which **1**
  (`notifications/resources/list_changed`) is flagged in the gap table as a genuine
  capability/behavior mismatch — the server advertises `listChanged: true` but never
  publishes the notification — rather than a plain not-built-yet gap.
- **Error codes** (10 constants across two sub-tables, self-checked exhaustive):
  standard JSON-RPC base — **4 of 5 used**, 1 unused (`ErrInvalidRequest`, which per the
  gap table's misuse finding *should* be used for 2 of `ParseRequest`'s 3 failure
  branches, currently all misrouted to -32700); custom reserved-range codes — **4 of 5
  used**, 1 unused/dead (`ErrUnauthorized`).

The gap table also records **6 Ambiguities**, three of which bear directly on this
decision: **#2** — the spec has shipped 2025-06-18 and 2025-11-25 since CWSO pinned
2025-03-26 in ADR-002, with a 2026-07-28 RC in draft, and ADR-002's promised
re-evaluation ("as ADR-007 during Phase 4") was never completed — the number ADR-007 was
assigned to an unrelated topic (hardware dispatch), and no evidence of the promised
review exists anywhere in the repo; **#4** — there is no server-initiated
request/response correlation mechanism anywhere in the transport layer at all (no
outbound request-ID generator, no pending-request map), which is why
`sampling/createMessage` and `roots/list` are Missing — this is an architectural absence,
not a small per-method gap; **#1** — the actual dispatch table lives in `server.go`, not
`protocol.go` as the C030 brief's file list implied, which this ADR treats as
authoritative per C030's own resolution.

**The human already decided the path on 2026-08-13** (roadmap Approval §, decision 2):
*"Keep the hand-rolled MCP kernel and prove it. ADR-013 (C031) documents this decision
and scopes the conformance suite from the C030 gap table; C032 executes keep-and-prove.
SDK adoption is recorded as considered-and-rejected (determinism rationale, rewrite risk
at v0.9)."* This ADR does not reopen that choice. Its job is to record the rationale with
real reasoning (not a strawman) and to scope C032's conformance suite as a concrete,
gap-table-derived checklist.

## Decision

**Keep the hand-rolled MCP kernel** (`orchestrator/internal/server/server.go`'s
`Handle()` dispatch + `orchestrator/internal/mcp/protocol.go`'s types/errors) as the
protocol implementation for CWSO v1.0. **Do not adopt** the official Go SDK
(`github.com/modelcontextprotocol/go-sdk`) at this time. Close blocker B1 and gate CG3
via a **conformance suite** (executed in C032) that, for the documented 35-row surface in
§"Conformance suite scope" below, asserts spec-shaped behavior for everything
Implemented/Partial and asserts a correct, spec-shaped "not supported" error — never a
malformed response — for everything Missing.

## Alternatives considered

| Option | Pros | Cons | Why not chosen |
|---|---|---|---|
| **A — Adopt the official Go SDK** (`modelcontextprotocol/go-sdk`) | Shifts spec-version tracking (Ambiguity #2) onto upstream maintainers; SDK already tracks newer spec versions (2025-11-25, 2026-07-28 RC per its own release history) so future drift work is partly absorbed; battle-tested transport/session handling for capabilities CWSO hasn't built (server-initiated correlation, session management) | **Determinism risk cannot be ruled out from public docs** — the SDK's own documentation states MCP itself offers no concurrency guarantees, and the Go SDK's stated model is that `tools/call`-class calls "are handled asynchronously with respect to each other" with one `ServerSession` per connection — i.e. concurrent-by-design dispatch, not the kernel's current synchronous single-path `Handle()`. No public SDK guarantee was found that preserves strict sequential dispatch while still using the SDK's session/transport machinery. Migration effort, estimated from the gap table, spans all 16 methods + 9 notifications + 10 error-code constants, plus re-hosting CWSO-specific role-based authorization and feature-flag gating that currently sits on top of 5 of the 6 Partial rows, plus building the Ambiguity #4 server-initiated-correlation plumbing on unfamiliar SDK primitives instead of known code — this is not a bounded swap, it is a rewrite of the request lifecycle at v0.9. New external dependency with its own fast-moving release cadence (v1.4.1 stable → v1.5.0-pre → v1.7.0 within months per its own release history) | **Rejected.** Determinism cannot be established as preserved (the crux — see below); migration size is a rewrite, not a port; the roadmap explicitly named this exact risk ("a rewrite at v0.9 carries real risk") |
| **B — Keep hand-rolled kernel + conformance suite** (CHOSEN) | Kernel's synchronous, single-path dispatch (`server.go:789-827`) is preserved exactly as-is — the property the roadmap calls "a deliberate determinism choice" is not touched at all, only tested; suite size is bounded and estimable directly from the gap table (~35 table rows, see scope below) rather than open-ended; converts "hand-rolled subset, gaps undocumented" (pre-C030 state) into "hand-rolled subset, gaps documented and tested" (post-C032 state) — a materially different, defensible position | Does not resolve Ambiguity #2 (spec-of-record still pinned to 2025-03-26 while 2025-06-18/2025-11-25/2026-07-28-RC exist) — inherits, not fixes, spec-drift risk; CWSO continues to hand-maintain protocol-layer code the SDK would otherwise absorb; the misuse finding (`ErrInvalidRequest` unused, should be used) and the genuine `notifications/resources/list_changed` capability mismatch are real defects that the suite must fix, not merely document | **Selected** — see Decision and the determinism/honesty analysis below |

### Determinism (the crux)

This is weighted most heavily because the roadmap names it explicitly: the hand-rolled
kernel is "a deliberate determinism choice." `Server.Handle()` (`server.go:789-827`) is a
flat synchronous `switch req.Method` — one request in, one response out, no goroutine
fan-out in the dispatch path itself. Public documentation for the official Go SDK states
that MCP as a protocol "offers no guarantees about concurrency semantics," and that the
SDK's own design handles calls such as `tools/call` "asynchronously with respect to each
other," with a single `Server` fanning out to many concurrent `ServerSession`s. No public
SDK documentation was found describing an opt-out that would let CWSO retain strict
sequential dispatch while still using the SDK's transport and session machinery. Per the
task's blocker protocol, this absence of a documented guarantee is recorded as a finding
that **weighs against adoption** — it is not assumed to be either safe or unsafe by
default; adopting the SDK without resolving this open question would mean trading a known
synchronous kernel for an unverified concurrency model in exactly the layer the project
has identified as determinism-critical.

### Migration effort, estimated from the gap table

Any SDK migration would need to re-plumb: 16 dispatch-table methods, 9 spec notification
types, and 10 error-code constants (5 standard + 5 custom) — the full C030 surface.
Beyond the 1:1 remap, the gap table surfaces three multiplying factors: (1) 5 of the 6
Partial rows (`resources/list`, `resources/templates/list`, `resources/read`,
`resources/subscribe`, `resources/unsubscribe`) carry CWSO-specific role-based
authorization and `spikeSubs`/`sparseAgents` feature-flag gating layered on top of
dispatch, which would need re-integration with the SDK's handler-registration model, not
a mechanical swap; (2) Ambiguity #4's architectural absence (zero server-initiated
request/response correlation anywhere) means `sampling/createMessage` and `roots/list`
can't be "added" cheaply under either path — under the SDK path this work would be done
on unfamiliar primitives; (3) two non-spec notifications with no spec analog
(`notifications/log`, `notifications/job-state`) would need a compatibility decision. This
is a full request-lifecycle rewrite, not a bounded port — consistent with the roadmap's
own risk register (§2.5: *"Phase 3 becomes a protocol rewrite that swallows the
release"*).

### Ongoing maintenance burden

SDK adoption would offload spec-version tracking (Ambiguity #2) to upstream, which is a
real, non-strawman advantage — CWSO would bump a dependency instead of hand-diffing
`schema.ts`. Against that: the SDK is young (stable since roughly mid-2025, moving
v1.4.1 → v1.5.0-pre → v1.7.0 within months per its own release history) and would add a
dependency whose determinism properties in the call-dispatch path could not be verified
from public documentation, plus its own breaking-change and CVE surface to track. Keeping
the hand-rolled kernel keeps CWSO fully in control of, and fully responsible for, its
determinism-critical dispatch path — the roadmap treats that trade as acceptable at v0.9.

### Honesty of the resulting surface (why keep-and-prove is defensible, not a cop-out)

Before C030, the situation was "hand-rolled subset, gaps undocumented" — a single
POC-DEBT comment with no enumerated surface. After C030 (merged) and this ADR, and once
C032 executes, the surface becomes: every one of the 16 methods / 9 notifications / 10
error codes is classified (Implemented / Partial / Missing), cited to a spec section and
a code location, and backed by a passing or explicitly-scoped-failing conformance test;
methods CWSO does not implement return a correct spec-shaped "not supported" error rather
than silence or a malformed response (Phase 3 exit criterion). That is "hand-rolled
subset, gaps documented and tested" — categorically different from, and the roadmap's own
bar for, what existed before C030: *"What is not defensible is shipping v1.0 with an
undocumented protocol subset."*

## Conformance suite scope (derived from the C030 gap table — for C032)

This is the concrete scoping deliverable this ADR is responsible for. Every row below
traces to a specific C030 gap-table row; nothing here is invented. Total surface: **16
method rows + 9 notification rows + 10 error-code rows = 35 table rows**, requiring an
estimated **~45–55 test cases** once documented deviations (which need both a happy-path
assertion and a deviation assertion) are counted separately.

### Methods (16 rows — C030 §1)

| Gap-table status | Count | Rows | Suite obligation |
|---|---|---|---|
| Implemented | 4 | `ping`, `notifications/initialized`, `tools/list`, `tools/call` | Assert spec-shaped request/response on the happy path. `ping`'s `{"pong":true}` deviation from spec `EmptyResult` (`{}}`) — C030 §1 row 1 — must be an explicit, named assertion, not silently normalized. |
| Partial | 6 | `initialize`, `resources/list`, `resources/templates/list`, `resources/read`, `resources/subscribe`, `resources/unsubscribe` | Each needs (a) the working-path assertion **and** (b) an explicit test of its documented deviation: `initialize`'s version-echo gap (server always returns its own version regardless of client's requested version — no negotiation/rejection path); the four `resources/*` rows' feature-flag gating (assert `ErrMethodNotFound` when `spikeSubs`/`sparseAgents` unset, spec behavior when set); `resources/subscribe`'s missing-notification gap (assert the subscription is accepted but that `notifications/resources/updated` is never sent — documenting, not silently fixing, unless C032 chooses to also close the companion notification gap below). |
| Missing | 6 | `prompts/list`, `prompts/get`, `logging/setLevel`, `completion/complete`, `sampling/createMessage`, `roots/list` | Assert `ErrMethodNotFound` (-32601) with a spec-shaped JSON-RPC error envelope for the first four. For `sampling/createMessage` and `roots/list`, the suite proves these are *correctly absent* (server never claims a capability it can't deliver) — per Ambiguity #4, actually implementing them requires new transport-layer request-correlation plumbing that is explicitly out of scope for C032 per the Phase 3 plan ("Out of scope: adding new MCP tools; changing tool semantics"). |

### Notifications (9 rows — C030 §2)

| Gap-table status | Count | Rows | Suite obligation |
|---|---|---|---|
| Implemented-lenient | 1 | `notifications/initialized` | Assert accepted-and-discarded; explicitly assert the *absence* of lifecycle gating (other methods remain callable before `initialized` is sent) so this leniency is a tested, known property rather than an untested one. |
| Missing (plain) | 7 | `notifications/cancelled`, `notifications/progress`, `notifications/roots/list_changed`, `notifications/message`, `notifications/resources/updated`, `notifications/tools/list_changed`, `notifications/prompts/list_changed` | Assert these are never emitted, consistent with the corresponding method/capability being unimplemented or (for `tools/list_changed`) with the capability being truthfully advertised as `false`. |
| Missing — capability/behavior mismatch | 1 | `notifications/resources/list_changed` | This is the one row C030 flags as a **genuine spec-conformance defect**, not a not-built-yet gap: capability is advertised (`listChanged: true`) but never published. The suite must not merely document this — C032 must either (a) stop advertising the capability until the notification is implemented, or (b) implement the publish path. Either way this is a **required fix**, tracked as a distinct line item in C032, not left as suite-documented debt. |

Non-spec notifications (`notifications/log`, `notifications/job-state`) are out of the
spec-conformance suite's scope by definition (they have no spec row) but should be
smoke-tested for stability so client-compatibility work (C033) isn't surprised by them.

### Error codes (10 constants — C030 §3)

| Sub-table | Count | Status | Suite obligation |
|---|---|---|---|
| 3a — standard JSON-RPC | 4 used (`ErrParse`, `ErrMethodNotFound`, `ErrInvalidParams`, `ErrInternal`) | Used | Assert each is returned in its documented trigger scenario. |
| 3a — standard JSON-RPC | 1 unused (`ErrInvalidRequest`, -32600) | **Misuse — required fix** | Per the gap table's misuse finding, `ParseRequest`'s 3 failure branches (malformed JSON; wrong `jsonrpc` version; missing `method`) are all currently mapped to -32700 (Parse error). Only the malformed-JSON branch should be -32700; the other two are Invalid Request (-32600) per JSON-RPC 2.0. This is a spec-correctness bug, not a documented gap — C032 must fix the branch mapping and the suite must assert all three branches independently. |
| 3b — custom reserved-range | 4 used (`ErrPermissionDenied`, `ErrToolNotFound`, `ErrToolExecution`, `ErrResourceNotFound`) | Used | Assert trigger scenario + assert the code value stays within the JSON-RPC-reserved server-error range (-32000..-32099) and does not collide with any spec-defined code. |
| 3b — custom reserved-range | 1 unused (`ErrUnauthorized`, -32001, dead code) | **Decision needed in C032** | Auth failures are fully handled at the HTTP transport layer (401/403) before a JSON-RPC envelope is ever parsed, so this code is currently unreachable. C032 must decide: wire it to a reachable JSON-RPC-level auth-failure path, or remove the dead constant. The suite should assert whichever outcome is chosen, not leave it silently unreachable. |

## Consequences

- **Positive**: The kernel's synchronous, deterministic dispatch path is untouched —
  zero risk to the property the roadmap calls "a deliberate determinism choice." The
  35-row surface above gives C032 a bounded, gap-table-derived checklist instead of an
  open-ended task. Two real defects the gap table found (the `ErrInvalidRequest` misuse
  and the `notifications/resources/list_changed` capability mismatch) get fixed as part
  of C032 rather than merely documented. CG3 closes on evidence, not on a rewrite.
- **Negative**: CWSO continues to hand-maintain protocol-layer code an SDK would
  otherwise absorb; the 6 Missing methods and 8 Missing notifications remain genuinely
  unimplemented in v1.0 (documented and correctly erroring, but not present) — any future
  client that needs `sampling/createMessage`, `roots/list`, prompts, logging, or
  completions cannot use CWSO's MCP server for that purpose without the Ambiguity #4
  transport-layer work, which this ADR does not authorize.
- **Risks introduced**: Ambiguity #2 (spec-of-record drift: 2025-03-26 pinned since
  ADR-002 while 2025-06-18/2025-11-25/2026-07-28-RC exist, with ADR-002's promised
  re-evaluation never completed) is inherited, not resolved, by this decision — the
  conformance suite proves parity with 2025-03-26 only. A future spec-version bump will
  require re-running or re-scoping this same class of gap analysis.
- **Follow-ups**: C032 (execute the scope above — required fixes flagged distinctly from
  documentation-only rows); C033 (client compatibility matrix, ≥3 clients × 2 transports);
  C034 (contract snapshot test in CI so protocol drift breaks the build); a **new,
  currently unnumbered follow-up task** to either explicitly re-affirm 2025-03-26 as
  spec-of-record with a stated reason or complete the re-evaluation ADR-002 promised
  (Ambiguity #2) — this ADR does not create that task number, it flags the gap for the
  orchestrator to schedule.

## Validation

This decision is correct if: (1) C032's conformance suite passes for the full 35-row
scope above, including the two required fixes; (2) C033's client compatibility matrix
(≥3 real MCP clients × stdio + Streamable HTTP) passes against the same kernel with no
client-observed protocol violations beyond what's already documented as Partial/Missing;
(3) C034's contract snapshot test is live in CI and demonstrably fails the build when the
protocol surface drifts (verified by a deliberate test-surface change during C034, per
that task's own acceptance criteria). If any of these three cannot be achieved on the
hand-rolled kernel without a scope explosion beyond the ~45–55 test-case estimate above,
that is itself evidence the "bounded cost" premise of this ADR was wrong, and it should
be escalated to the human rather than silently absorbed into C032.

## Reversal criteria — what would justify revisiting SDK adoption

None of the following exist today; any one of them occurring is grounds for a new ADR
proposal (not a silent reversal of this one):

1. **The official SDK documents a verifiable deterministic/sequential dispatch mode** —
   e.g. a published, opt-in guarantee that request handling can be constrained to strict
   sequential (non-concurrent) processing compatible with the kernel's current behavior,
   not merely the current default async-per-call model. Today this could not be
   established from public docs (see "Determinism" above) — that absence is what closes
   this option for now, and a documented guarantee would remove the blocking reason.
2. **CWSO commits to bumping the spec-of-record past 2025-03-26** to resolve Ambiguity
   #2. At that point the SDK's already-current spec-version tracking (2025-11-25,
   2026-07-28 RC per its release history) starts to overlap with work CWSO would have to
   do by hand anyway, materially changing the cost-benefit math computed here.
3. **The gap-table surface grows materially** — e.g. a future spec revision more than
   doubles the 6-of-16 Missing-method count, or adds required methods CWSO's dispatch
   model structurally cannot support — such that continued hand-maintenance stops being
   cheaper than a migration, per a future re-run of this same gap-table-driven analysis.
4. **C032, C033, or C034 surface defects traceable to hand-rolled-dispatch design
   flaws**, not just missing coverage — e.g. a class of protocol bugs a mature SDK's
   transport layer would have structurally prevented. Isolated bugs found and fixed by
   the conformance suite do **not** meet this bar; a *pattern* of dispatch-architecture
   defects would.
5. **The Ambiguity #4 architectural gap (no server-initiated request/response
   correlation) becomes a hard product requirement** — e.g. a v1.1+ feature needs
   `sampling/createMessage` or `roots/list`. At that point, building the correlation
   plumbing by hand vs. adopting the SDK specifically for that capability is a narrower,
   fresh decision deserving its own ADR — this document does not pre-decide it.

## Approval required

**This ADR's underlying direction (keep hand-rolled kernel, prove via conformance suite,
reject SDK adoption) was already approved by the human on 2026-08-13** (roadmap Approval
§, decision 2). This document does not ask the human to re-decide that; it is the written
record the roadmap promised, plus the conformance-suite scoping the roadmap deferred to
C031.

**What does need explicit sign-off before C032 starts:**

1. The 35-row conformance suite scope in "Conformance suite scope" above, including the
   two items flagged as **required fixes** rather than documentation-only
   (`ErrInvalidRequest` misuse; `notifications/resources/list_changed` capability
   mismatch) — confirm these should be fixed as part of C032 rather than deferred.
2. Whether `ErrUnauthorized` (dead code, -32001) should be wired to a reachable path or
   removed — C032 needs a decision, not a default.
3. Whether the new, unnumbered follow-up task (Ambiguity #2 — spec-of-record
   re-evaluation) should be scheduled now or explicitly deferred past v1.0; if deferred,
   whether 2025-03-26 should be re-affirmed in writing as the stated spec-of-record with a
   reason, closing the loop ADR-002 left open.

Once approved, status moves `proposed` → `accepted` and C032 may begin.
