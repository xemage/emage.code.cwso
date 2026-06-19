# Task T117 — `subscribe_ast_spikes` MCP Resources layer (SSE, threshold-gated)

> **ID note:** roadmap **Feature C / placeholder T097** ("`subscribe_ast_spikes` MCP SSE
> resource + write-event feeders"). Active IDs continue from the board, so this lands as
> **T117** (T097 stays a roadmap placeholder). The runtime write-event **feeder** half of
> T097 is split into **T118**. See the numbering-reconciliation section in `active-tasks.md`.

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T116 (done — semantic spike filter + conflict pre-warning)
- **Phase:** 7 — Sparse Micro-Agents & Spiking Monitors (Feature C — Event-Driven Spiking AST Monitors)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md` §3.6 + Feature C step 3, `task-T115.md`, `task-T116.md`

## Objective
Expose the T115/T116 AST spike topics (`ast/spike`, `ast/semantic-spike`,
`ast/conflict-warning`) as subscribable MCP **Resources** under the `cwso://` scheme, so an
agent can `subscribe_ast_spikes` and receive a threshold/path/workspace-scoped live SSE stream
(or read a snapshot) instead of consuming the unfiltered broker firehose.

## Context
Before T117 the server's MCP surface was **tools-only** — no `resources/*` handlers, no
`cwso://` scheme, and SSE on `GET /mcp` was a single broker-wide firehose. The user selected
the **faithful MCP Resources layer** approach (over a pragmatic reuse), so T117 implements the
real protocol surface. The spike monitors (T115/T116) already publish to the broker topics;
the resource layer filters those records per-subscription. A concrete write-event **feeder**
(so live edits actually drive the topics) is intentionally split into **T118** — the resource
machinery is producer-agnostic and ships/tests independently.

## Changes
- **`orchestrator/internal/mcp/protocol.go`:** `Resource`, `ResourceTemplate`,
  `ResourcesListResult`, `ResourceTemplatesListResult`, `ResourceURIParams`, `ResourceContents`,
  `ResourceReadResult`, and `ErrResourceNotFound` (-32020).
- **`orchestrator/internal/dispatch/spike_subscriptions.go` (new):**
  `SpikeSubscriptionRegistry` (Create/Get/List/Remove, random ids) + `SpikeSubscription` with
  `Allow(topic, payload)` — the shared filter predicate (semantic-threshold rank gating, path
  glob via `path.Match`, workspace scope; volume `ast/spike` events only pass the `any`
  threshold and match against hot paths). `ParseSpikeResourceID` + `ASTSpikeTopics`.
- **`orchestrator/internal/tools/ast_spike_tools.go` (new):** `subscribe_ast_spikes` tool
  (orchestrator + worker), validates `path`/`semantic_threshold`/`workspace_scope`, returns
  `{subscription_id, stream_resource, topics, transport_hint}`.
- **`orchestrator/internal/server/server.go`:** registry field + construction gated on config;
  tool registration; `resources/list|templates/list|read|subscribe|unsubscribe` handlers;
  `resources` capability advertised in `initialize`; SSE resolver wired into `RunHTTP`.
  `resources/read` replays a filtered snapshot from the broker (`Query` + `Allow`).
- **`orchestrator/internal/transport/http.go`:** `RecordFilter` + `SubscriptionResolver` +
  `WithSubscriptionResolver` option (variadic, avoids growing the wide signature — TD-02);
  `GET /mcp?subscription=<id>` resolves + filters the stream (unknown id → 404).
- **`orchestrator/internal/config/config.go`:** `CWSO_AST_SPIKE_RESOURCES_ENABLED` (default false).
- **Tests (new/updated):** `dispatch/spike_subscriptions_test.go`,
  `tools/ast_spike_tools_test.go`, `server/spike_resources_test.go`,
  `transport/http_spike_sse_test.go`, `config/config_test.go`.

## Acceptance Criteria
- [x] `subscribe_ast_spikes` returns a `cwso://spikes/<id>` handle; subscription is registered.
- [x] Invalid `semantic_threshold` / disabled server return a clean error result.
- [x] `resources` capability advertised only when enabled; `resources/*` are method-not-found
      when disabled.
- [x] `resources/list` + `resources/templates/list` expose the subscription + URI template.
- [x] `resources/read` returns a snapshot filtered by the subscription's threshold/path/scope.
- [x] `resources/subscribe`/`unsubscribe` validate the URI; unsubscribe removes the subscription.
- [x] `GET /mcp?subscription=<id>` streams only matching spike events; unknown id / no resolver → 404.
- [x] `go build` / `go vet` / `go test -race ./...` + gofmt clean.

## Notes / Follow-ups
- **No runtime feeder yet (T118).** In a live server the spike topics stay empty until a write
  source calls `ObserveWrite`; tests publish events directly. T118 wires `write_shadow_file`
  (and optionally eBPF / fs-watch) into the monitor + filter, config-gated.
- Live resource updates use the existing SSE transport scoped by subscription id; the
  spec's `notifications/resources/updated` re-read model is not implemented (the stream
  delivers event payloads directly). Adopting the official MCP go-sdk (POC-DEBT) would
  reconcile this.
- `resources/read` snapshot is bounded to the last 100 matching broker records.
