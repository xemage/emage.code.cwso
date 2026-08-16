# MCP Implementation vs. Spec Gap Analysis (spec 2025-03-26)

**Based on:**
`orchestrator/internal/mcp/protocol.go` (marker at line 10), `orchestrator/internal/mcp/protocol_test.go`,
`orchestrator/internal/server/server.go` (JSON-RPC method dispatch — `Handle()`),
`orchestrator/internal/transport/http.go`, `orchestrator/internal/transport/stdio.go`,
`orchestrator/internal/eventbus/bus.go`, `schemas/README.md`,
`docs/archive/decisions/ADR-002-streamable-http-transport.md`,
MCP specification version **2025-03-26** (spec of record per task C030; schema:
`https://github.com/modelcontextprotocol/specification/blob/main/schema/2025-03-26/schema.ts`,
docs: `https://modelcontextprotocol.io/specification/2025-03-26/*`)

**Task:** C030 — precedes and gates C031 (SDK adoption decision). This document states facts
only; it makes no adoption recommendation.

**Date:** 2026-08-16

## Scope note on method dispatch location

`orchestrator/internal/mcp/protocol.go` (referenced in the task brief as "the hand-rolled
protocol implementation") in fact contains only JSON-RPC envelope types, MCP payload
structs, and error-code constants — it defines no method dispatch. The actual `switch
req.Method` routing that determines which MCP methods are implemented lives in
`orchestrator/internal/server/server.go:799-827` (`Server.Handle`). This gap analysis
inventories the implementation from `server.go`'s dispatch table, since that is where
"implemented vs. not" is actually decided; `protocol.go` is cited for the shared
types/constants each row depends on. This is recorded as Ambiguity #1 below since the
brief's file list did not name `server.go` explicitly.

---

## 1. Methods

Status legend: **Implemented** (dispatch case exists and returns spec-shaped result) /
**Partial** (dispatch case exists but deviates from spec behavior or is conditionally
available) / **Missing** (no dispatch case; falls through to `ErrMethodNotFound` or,
for server→client request methods, has no send/correlate mechanism at all).

| Method | Direction (spec) | Spec section | Status | Code reference | Notes |
|---|---|---|---|---|---|
| `ping` | either | `basic/utilities/ping` | Implemented | `server.go:816-817` | Static `{"pong":true}` reply; not a spec-shaped `EmptyResult` (`{}`) — payload shape differs from `PingRequest`'s expected empty result, but does not break clients that only check for a 200/result. |
| `initialize` | client→server | `basic/lifecycle` | Partial | `server.go:836-855`, `protocol.go:93-117,210-211` | `InitializeParams.ProtocolVersion` (`server.go:837-842`) is parsed but never read again — the server unconditionally returns `mcp.SupportedProtocolVersion` ("2025-03-26") regardless of what the client requested. Spec §Lifecycle/Version Negotiation requires the server to echo the client's version if supported, or explicitly respond with a different supported version so the client can decide whether to disconnect. No negotiation/rejection path exists (single-version server, so behavior is accidentally correct only when the client also requests 2025-03-26). Optional `instructions` result field (schema.ts `InitializeResult.instructions`) is never populated — allowed by spec (optional) but note it as unused capability. |
| `notifications/initialized` | client→server (notification) | `basic/lifecycle` | Implemented (lenient) | `server.go:818-819` | Accepted and discarded (`return nil, nil`). No session state machine enforces that `initialized` precedes other requests — `Handle()` accepts `tools/call`, `resources/*`, etc. even if `initialized` was never sent. Spec only constrains what the *server* should proactively send pre-`initialized` (pings/logging), so this is lenient rather than a hard violation, but it means the lifecycle gate is not implemented. |
| `tools/list` | client→server | `server/tools` | Implemented | `server.go:857-859` | No pagination (`cursor`/`nextCursor`) support; spec marks both as optional (`PaginatedRequest`/`PaginatedResult`), so this is spec-legal, not a gap. |
| `tools/call` | client→server | `server/tools` | Implemented | `server.go:861-893` | Includes role-based authorization (`ErrPermissionDenied`) beyond spec scope (spec leaves authZ to the host). Tool-level errors correctly reported inside `CallToolResult.isError` via `tool.Execute` results (`protocol.go:150-158`) rather than as protocol errors, matching spec's `CallToolResult` guidance. |
| `resources/list` | client→server | `server/resources` | Partial (feature-flagged) | `server.go:905-931`, gate at `server.go:898-903` | Only reachable when `spikeSubs` or `sparseAgents` is configured; otherwise every call — even though the method name is recognized in code — returns `ErrMethodNotFound` via `resourcesEnabled()`. This is consistent with the server only ever advertising the `resources` capability when one of those is enabled (`server.go:846-848`), so it is spec-conformant capability gating, not a bug — flagged Partial because "implemented" is conditional on runtime config, which matters for a reader deciding whether the method is available in a given deployment. |
| `resources/templates/list` | client→server | `server/resources` | Partial (feature-flagged) | `server.go:933-955` | Same gating as `resources/list`. |
| `resources/read` | client→server | `server/resources` | Partial (feature-flagged) | `server.go:957-969` | Same gating. Returns `TextResourceContents`-shaped content (`protocol.go:198-208`); no `BlobResourceContents` (binary) support — spec allows either, so text-only is spec-legal. |
| `resources/subscribe` | client→server | `server/resources` | Partial | `server.go:1071-1096` | Accepts a subscription and validates the URI exists, but the *result* of subscribing (spec: server later emits `notifications/resources/updated` when the resource changes — see Notifications §2 below) is never implemented anywhere in the codebase. The actual push mechanism is a separate, non-spec, subscription-scoped SSE stream (`transport.WithSubscriptionResolver`, `GET /mcp?subscription=<id>`) that streams raw event-bus topics, not `resources/updated` notifications. A spec-conformant client that calls `resources/subscribe` and then waits for `notifications/resources/updated` will never receive one. |
| `resources/unsubscribe` | client→server | `server/resources` | Partial (feature-flagged) | `server.go:1098-1123` | Same gating as `resources/list`; correctly errors on unknown subscription id. |
| `prompts/list` | client→server | `server/prompts` | Missing | — | No `prompts` capability is ever advertised (`server.go:843-848` never sets `caps["prompts"]`) and no dispatch case exists; falls through to `ErrMethodNotFound` (`server.go:820-825`). |
| `prompts/get` | client→server | `server/prompts` | Missing | — | Same as above. |
| `logging/setLevel` | client→server | `server/utilities/logging` | Missing | — | No `logging` capability advertised, no dispatch case. Server does emit a custom `notifications/log` event (see Notifications §3) unconditionally regardless of any client-requested level, since there is no level-filtering mechanism at all. |
| `completion/complete` | client→server | `server/utilities/completion` | Missing | — | No `completions` capability advertised, no dispatch case. |
| `sampling/createMessage` | server→client | `client/sampling` | Missing | — | Requires the server to *initiate* a request to the client and correlate an async response by request ID. No such mechanism exists anywhere in `mcp`, `server`, or `transport` — `Handle()` is purely reactive (inbound request → outbound response/notification). See Ambiguity #4. |
| `roots/list` | server→client | `client/roots` | Missing | — | Same architectural gap as `sampling/createMessage`: no server-initiated request/response correlation exists. `roots` is also never referenced anywhere in client-capability handling (the server never reads `InitializeParams.Capabilities` at all — `server.go:837-842` unmarshals it into a `map[string]any` field that is subsequently unused). |

**Self-check:** every method defined in `ClientRequest`/`ServerRequest` in schema.ts
(`ping`, `initialize`, `completion/complete`, `logging/setLevel`, `prompts/get`,
`prompts/list`, `resources/list`, `resources/templates/list`, `resources/read`,
`resources/subscribe`, `resources/unsubscribe`, `tools/call`, `tools/list`,
`sampling/createMessage`, `roots/list`) appears exactly once above — 15 rows for 15 spec
request methods. The table has one additional (16th) row, `notifications/initialized`,
which is a spec *notification* rather than a request method; it is included here per the
brief's explicit instruction to cover `initialize`/`initialized`/`ping`/`tools/list`/`tools/call`
as the lifecycle rows real clients hit first. It is not part of the 15-method self-check
count and is also covered in the Notifications table (§2) under its correct spec
classification, so it is not silently omitted from either table.

---

## 2. Notifications

| Notification | Direction (spec) | Spec section | Status | Code reference | Notes |
|---|---|---|---|---|---|
| `notifications/initialized` | client→server | `basic/lifecycle` | Implemented | `server.go:818-819` | See Methods table row — accepted, no state effect. |
| `notifications/cancelled` | either | `basic/utilities/cancellation` | Missing | — | No case in `server.go`'s dispatch switch; an unrecognized notification (no `id`) silently returns `nil, nil` (`server.go:820-823`) rather than erroring, so a client sending this gets no feedback either way. No in-flight-request cancellation mechanism exists for any tool (e.g. long-running `dispatch_concurrent_jobs`). |
| `notifications/progress` | either | `basic/utilities/progress` | Missing | — | No `progressToken` handling anywhere in the codebase (confirmed via repo-wide search). The custom `notifications/job-state` topic (see below) is a distinct, non-spec analog and does not carry a `progressToken` correlated to the originating request. |
| `notifications/roots/list_changed` | client→server | `client/roots` | Missing | — | Consistent with `roots/list` being entirely unimplemented (Methods table). |
| `notifications/message` (logging) | server→client | `server/utilities/logging` | Missing (non-conformant analog exists) | `orchestrator/internal/eventbus/bus.go:11`, `orchestrator/internal/transport/http.go:527-541` | The server publishes a custom event under topic/method name **`notifications/log`** (`TopicNotificationsLog`, `eventbus/bus.go:11`), sent to clients as a JSON-RPC notification whose `method` field is literally `"notifications/log"` (via `marshalJSONRPCNotification`, `transport/http.go:511-525`). This is a different method name than the spec's `notifications/message`, uses a flat `{request_id, method, state, error?}` payload rather than the spec's `{level, logger?, data}` shape, and is not gated by any `logging/setLevel`-negotiated severity. A spec-conformant client listening for `notifications/message` will not receive these events. |
| `notifications/resources/updated` | server→client | `server/resources` | Missing | — | Never published anywhere (confirmed via repo-wide search for `resources/updated`). See `resources/subscribe` note in Methods table — subscriptions are accepted but this notification, which is the entire point of subscribing per spec, is never sent. |
| `notifications/resources/list_changed` | server→client | `server/resources` | Missing — **capability/behavior mismatch** | `server.go:846-848` (capability advertised), no publish site found | `handleInitialize` advertises `capabilities.resources.listChanged: true` whenever `spikeSubs` or `sparseAgents` is configured (`server.go:846-848`), but no code path ever publishes this notification — confirmed via repo-wide search for `list_changed`. A client that trusts the advertised capability (e.g. to skip periodic re-polling of `resources/list`) will silently miss additions/removals of AST-spike or sparse-agent resources. This is the one row in this table that is a genuine spec-conformance defect rather than a straightforward "not built yet," since the server actively claims support it does not provide. |
| `notifications/tools/list_changed` | server→client | `server/tools` | Missing (consistent with advertised capability) | `server.go:844` | `handleInitialize` advertises `tools.listChanged: false` (`server.go:844`), matching the fact that this notification is never sent — capability and behavior are consistent here, unlike the `resources/list_changed` row above. |
| `notifications/prompts/list_changed` | server→client | `server/prompts` | Missing (consistent — no prompts capability) | — | Consistent with `prompts` never being advertised or implemented. |

**Non-spec notifications observed in code (not part of the table above, recorded for
completeness per the brief's "don't skip boring rows" instruction applied in reverse —
these are extra rows the spec doesn't define at all):**

- `notifications/job-state` (`TopicNotificationsJobState`, `eventbus/bus.go:12`, published
  at `transport/http.go:545-551`) — a private extension notification with no spec analog.
  Its method-name string is not namespaced away from the spec's `notifications/` prefix
  (e.g. as `notifications/x-cwso/job-state` might be), which could collide in spirit with
  future spec-defined notifications under that prefix.

**Self-check:** every notification defined in `ClientNotification`/`ServerNotification` in
schema.ts (`notifications/cancelled`, `notifications/progress`, `notifications/initialized`,
`notifications/roots/list_changed`, `notifications/message`, `notifications/resources/updated`,
`notifications/resources/list_changed`, `notifications/tools/list_changed`,
`notifications/prompts/list_changed`) appears exactly once above — 9 rows for 9 spec
notifications, `notifications/cancelled` counted once despite being bidirectional in spec.

---

## 3. Error codes

### 3a. Spec-defined (JSON-RPC 2.0 base, `basic` / schema.ts)

| Code | Name (spec) | Spec section | Status | Code reference |
|---|---|---|---|---|
| -32700 | Parse error | `basic` (schema.ts `PARSE_ERROR`) | Used (also misused — see below) | `protocol.go:25` (`ErrParse`), used at `server.go:790-794` |
| -32600 | Invalid Request | `basic` (schema.ts `INVALID_REQUEST`) | **Unused** | Defined at `protocol.go:26` (`ErrInvalidRequest`) — zero references anywhere in `server.go` or `transport/`. |
| -32601 | Method not found | `basic` (schema.ts `METHOD_NOT_FOUND`) | Used | `protocol.go:27` (`ErrMethodNotFound`), used at `server.go:824`, `server.go:900` |
| -32602 | Invalid params | `basic` (schema.ts `INVALID_PARAMS`) | Used | `protocol.go:28` (`ErrInvalidParams`), used at `server.go:840,864,867,963,1077,1104` |
| -32603 | Internal error | `basic` (schema.ts `INTERNAL_ERROR`) | Used | `protocol.go:29` (`ErrInternal`), used at `server.go:890,1016,1062` |

**Misuse finding:** `mcp.ParseRequest` (`protocol.go:78-91`) returns a plain Go `error` for
three distinct conditions — malformed JSON, `jsonrpc != "2.0"`, and a missing `method`
field — and `Handle()` maps *all* of them to `ErrParse` (-32700) at `server.go:790-794`
without distinguishing cases. Per JSON-RPC 2.0 (and MCP's adoption of it, `basic` §Messages,
"Error codes MUST be integers" / standard JSON-RPC semantics), Parse error (-32700) is
specifically for invalid JSON that cannot be parsed; a syntactically valid JSON object with
the wrong `jsonrpc` version or a missing `method` is an **Invalid Request** (-32600) case.
This means `ErrInvalidRequest` is not just unused but *should* be used for two of the three
`ParseRequest` failure branches (`protocol.go:84-89`).

*Naming collision note:* `orchestrator/internal/sandbox/runner.go:11-12` defines an
unrelated Go sentinel error also named `ErrInvalidRequest` (for malformed sandbox run
requests, package `sandbox`). It is a different identifier in a different package with no
relation to the JSON-RPC code; noted only to prevent a reader's `grep -r ErrInvalidRequest`
from being misread as evidence of usage of the MCP error code.

### 3b. Non-spec, implementation-defined (JSON-RPC reserved server-error range -32000..-32099)

The MCP 2025-03-26 schema does not define any MCP-specific numeric error codes (unlike,
e.g., LSP). The codes below are CWSO extensions that legitimately occupy the JSON-RPC
2.0 base-spec-reserved "implementation-defined server errors" range; they are not
"spec codes" in the sense the brief's required table covers, so they are broken out
separately rather than mixed into 3a.

| Code | Name (CWSO) | Status | Code reference |
|---|---|---|---|
| -32001 | `ErrUnauthorized` | **Unused** (dead code) | Defined at `protocol.go:32`; zero references outside the definition. Authentication failures are instead handled entirely at the HTTP transport layer as plain `401 Unauthorized` / `403 Forbidden` responses (`transport/http.go:811,821,826`) before a JSON-RPC envelope is ever parsed, so this JSON-RPC-level code has no reachable call site under the current architecture. |
| -32002 | `ErrPermissionDenied` | Used | `protocol.go:33`; `server.go:880-881` (role-based tool authorization) |
| -32010 | `ErrToolNotFound` | Used | `protocol.go:34`; `server.go:876` |
| -32011 | `ErrToolExecution` | Used | `protocol.go:35`; `server.go:887` |
| -32020 | `ErrResourceNotFound` | Used | `protocol.go:36`; `server.go:973,977,981,1027,1030,1081,1084,1090,1093,1108,1111,1117,1120` |

**Self-check:** every error code constant defined in `protocol.go:21-37` (10 total: 5
standard + 5 custom) appears in exactly one of the two sub-tables above.

---

## 4. Ambiguities

These are recorded as findings per the blocker protocol / brief instruction ("record spec
ambiguities as their own findings instead of guessing intent"). None of these are resolved
here — resolution belongs to C031 or a future architecture decision.

1. **Dispatch location vs. brief's file list.** The brief names `orchestrator/internal/mcp/protocol.go`
   as "the hand-rolled protocol implementation," but the file contains only types/constants;
   the actual method-dispatch switch is in `orchestrator/internal/server/server.go`
   (`Handle`, `server.go:789-827`). This document analyzed `server.go` as the source of
   truth for "implemented vs. missing" since that is where the behavior actually lives.
   Flagging in case the brief's author intended a narrower scope (types-only) that would
   change which file "counts" for the gap table.

2. **Newer spec versions exist; 2025-03-26 confirmed as spec of record, not silently
   switched.** Per the blocker protocol instruction, this analysis targets **2025-03-26**
   only, as directed. For context: the MCP project has since shipped **2025-06-18**
   (structured tool output, elicitation, resource links, OAuth-based authorization) and
   **2025-11-25** (current "Latest Stable" per `modelcontextprotocol.io` as of this
   analysis), with a **2026-07-28** release candidate for the next revision in draft. None
   of these were consulted for status determinations above. `docs/archive/decisions/ADR-002-streamable-http-transport.md`
   (2026-05-10) explicitly anticipated this: it pinned 2025-03-26 and stated "the
   2025-11-25 task semantics will be evaluated as ADR-007 during Phase 4." However,
   `docs/decisions/ADR-007-hardware-dispatch-provider-contract.md` (the ADR that number
   was actually assigned to) covers an unrelated topic (hardware dispatch), so there is no
   evidence in this repository that the promised newer-spec evaluation was ever completed
   or explicitly deferred. This is an open question for whoever owns spec-version strategy,
   not something this document resolves.

3. **`resources` and `logging`/`prompts`/`completions` capability semantics under
   feature-flag gating.** The server advertises `resources` capability (with
   `subscribe: true, listChanged: true`) only when `spikeSubs` or `sparseAgents` is
   configured at startup (`server.go:846-848`), and never advertises `prompts`, `logging`,
   or `completions` at all. It is ambiguous from the spec alone whether a server is
   expected to statically advertise capabilities per binary/build, or may legitimately vary
   them per deployment config the way CWSO does — the spec's capability-negotiation section
   (`basic/lifecycle`) describes negotiation per-connection but does not explicitly bless or
   forbid config-driven capability sets. Recorded as ambiguous rather than treated as
   automatically compliant or non-compliant.

4. **No server-initiated request/response correlation exists at all — architectural, not
   per-method.** `sampling/createMessage` and `roots/list` are both listed "Missing" in the
   Methods table, but the deeper fact is that the transport and dispatch layers
   (`mcp.Request`/`mcp.Response`, `Server.Handle`, `transport.RunStdio`,
   `transport.handlePOST`/`handleSSE`) implement only the "respond to inbound request"
   direction. There is no outbound request-ID generator, no pending-request map, and no
   mechanism to correlate an async client response arriving over a different channel (e.g.
   a later POST) with a server-initiated request. This means *any* future server→client
   request-type method (not just the two the spec currently defines) would require new
   transport-layer plumbing, not just a new dispatch case. Recorded as a finding because it
   affects how "gap size" for this pair of methods should be read — it is not a small
   per-method gap.

5. **Streamable HTTP transport-level conformance, outside the three required tables.** The
   brief's required tables cover methods/notifications/error-codes; the following
   transport-layer facts were observed while reading `orchestrator/internal/transport/http.go`
   per the brief's explicit instruction to review the transport package, and are recorded
   here since they don't fit the three table schemas but are directly relevant to spec
   §`basic/transports` conformance:
   - No `Mcp-Session-Id` header is ever set on the `initialize` response or read from
     subsequent requests (confirmed via repo-wide search) — session management (spec
     §Session Management) is entirely unimplemented. Since session management is
     spec-optional ("MAY assign a session ID"), this is not itself a violation, but it means
     related behaviors (400 for missing session id, 404 for expired session, session-scoped
     `DELETE`) are all absent as a consequence, not by independent design choice.
   - `POST /mcp` always responds with `Content-Type: application/json` synchronously
     (`transport/http.go:308-310`); it never opens an SSE stream in direct response to a
     POST. This is one of the two spec-permitted options (§Sending Messages to the Server,
     point 5), so it is compliant, not a gap — noted only so a reader doesn't assume the
     SSE-on-POST path was overlooked.
   - Authorization uses a custom HS256 JWT bearer scheme (`transport/http.go:792-884`)
     rather than the spec's OAuth 2.1-based authorization framework (`basic/authorization`).
     Per that spec page, "Authorization is OPTIONAL for MCP implementations," so a custom
     scheme is spec-legal; recorded because C031 (or a future authZ-focused task) may need
     this fact and it does not otherwise appear in the three required tables.

6. **`ping` response shape.** `server.go:817` replies to `ping` with `{"pong": true}`.
   Spec's `PingRequest` expects an `EmptyResult` (`{}`) — schema.ts does not define a
   `pong` field. This is likely harmless for real clients (which typically only check for a
   successful response), but it is a literal deviation from the schema's result shape for
   this method, and is recorded rather than silently normalized into the Methods table's
   "Implemented" status without comment.

---

## Summary counts

The table rows are the authoritative source; this section is a rough roll-up for the
orchestrator's completion report and C031's planning.

- **Methods table** (16 rows: 15 spec request methods + `notifications/initialized`
  included per the brief's lifecycle-coverage instruction — see §1 self-check):
  - Implemented: 4 (`ping`, `notifications/initialized`, `tools/list`, `tools/call`)
  - Partial: 6 (`initialize`, `resources/list`, `resources/templates/list`, `resources/read`,
    `resources/subscribe`, `resources/unsubscribe` — mostly feature-flag gating, plus the
    `initialize` version-negotiation gap and the `resources/subscribe` missing-notification gap)
  - Missing: 6 (`prompts/list`, `prompts/get`, `logging/setLevel`, `completion/complete`,
    `sampling/createMessage`, `roots/list`)
- **Notifications** (9 spec-defined, one row each — see §2): 1 Implemented-lenient
  (`notifications/initialized`), 8 Missing — of which one (`notifications/resources/list_changed`)
  is a genuine capability/behavior mismatch rather than a plain "not built yet" gap. Plus 1
  additional non-spec notification (`notifications/job-state`) observed with no spec analog,
  and 1 spec notification (`notifications/message`) with a non-conformant custom analog
  (`notifications/log`).
- **Error codes** (§3): standard JSON-RPC base — 4 of 5 used, 1 unused (`ErrInvalidRequest`,
  which should actively be used per the misuse finding in §3a); custom reserved-range codes
  — 4 of 5 used, 1 unused (`ErrUnauthorized`, dead code).
- **Ambiguities** (§4): 6 findings recorded, none resolved here.
