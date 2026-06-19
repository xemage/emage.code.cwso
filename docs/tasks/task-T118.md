# Task T118 — AST write-event feeder (`write_shadow_file` → monitor/filter)

> **ID note:** the runtime write-event **feeder** half of roadmap **Feature C / placeholder
> T097**. T117 delivered the MCP Resources surface; T118 wires a concrete producer so the
> stream carries live events. Active ID **T118** (see numbering reconciliation in
> `active-tasks.md`).

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T117 (done — `subscribe_ast_spikes` MCP Resources layer)
- **Phase:** 7 — Sparse Micro-Agents & Spiking Monitors (Feature C — Event-Driven Spiking AST Monitors)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md` §3.6 + Feature C, `task-T115.md`, `task-T116.md`, `task-T117.md`

## Objective
Drive the T115 volume monitor + T116 semantic filter from a real in-process write source so
the T117 `cwso://spikes` resources emit live events as agents edit code, instead of staying
empty until an external producer publishes to the broker.

## Context
T115/T116 built `ASTWriteSpikeMonitor` and `ASTSpikeFilter` with a source-agnostic
`ObserveWrite(WriteEvent)` API; T117 exposed the spike topics as subscribable MCP resources.
Nothing called `ObserveWrite` yet. The natural in-process write source is the
`write_shadow_file` MCP tool (worker-tier), which mutates the shadow workspace ODB. eBPF /
fanotify / fs-watch sources remain a later option; this task delivers the pragmatic feeder.

## Changes
- **`orchestrator/internal/dispatch/write_event_sink.go` (new):** `WriteEventSink` interface
  + `NewWriteEventFanout` (composes monitor + filter; nil-safe; error-tolerant).
- **`orchestrator/internal/tools/shadow_tools.go`:** `WriteShadowFile` gains an optional
  observer (`NewWriteShadowFileWithObserver`); after a successful write it emits a
  `dispatch.WriteEvent` (`Workspace`, `Path`, `Language` from extension, `At`, `Symbol` = path,
  `SignatureHash` = SHA-256(content)). Helpers `languageFromPath` + `contentSignature`.
- **`orchestrator/internal/server/server.go`:** `buildASTWriteSink` constructs monitor + filter
  (gated on `CWSO_AST_SPIKE_MONITOR_ENABLED`, sharing the HHD telemetry redaction policy);
  `registerShadowTools` injects the sink into `write_shadow_file`.
- **`orchestrator/internal/config/config.go`:** `ASTSpikeMonitorEnabled`, `ASTSpikePreferEBPF`,
  `ASTSpikeWindowMS`, `ASTSpikeThreshold`, `ASTSpikeDebounceMS`, `ASTSpikeMaxHotPaths`,
  `ASTSpikeSemanticThreshold`, `ASTSpikeConflictWindowMS`, `ASTSpikeSignatureTTLMS`,
  `ASTSpikeMaxConflictPeers` (+ validation when enabled).
- **Tests:** `dispatch/write_event_sink_test.go`, `tools/ast_feeder_test.go`,
  `server/ast_feeder_test.go` (end-to-end via a fake shadow sidecar), `config/config_test.go`.

## Acceptance Criteria
- [x] A successful `write_shadow_file` emits exactly one `WriteEvent`; a failed write emits none.
- [x] Language derived from extension (Go/Python/Rust/TS/JS); unknown → "".
- [x] `Symbol` defaults to the file path; `SignatureHash` = SHA-256(content).
- [x] Fanout delivers to monitor + filter; nil sinks drop; one sink's error doesn't starve others.
- [x] Monitor+filter constructed only when `CWSO_AST_SPIKE_MONITOR_ENABLED=true`; invalid
      `semantic_threshold` / non-positive window rejected at load.
- [x] End-to-end: worker writes crossing the threshold produce `ast/spike` on the broker (which
      the T117 `cwso://spikes` resources/SSE then surface).
- [x] `go build` / `go vet` / `go test -race ./...` + gofmt clean.

## Notes / Follow-ups
- **Symbol = file path is a deliberate PoC approximation.** True AST-symbol-level events
  (function/type granularity, real signature hashes) require parsing the edited file via
  `query_ast` (tree-sitter) on write — a later refinement that plugs into the same `WriteEvent`
  shape and `SemanticScorer` seam.
- Real **eBPF / fanotify / fs-watch** write sources (host-level, out-of-process edits) remain
  future work; `write_shadow_file` covers in-process agent edits.
- `dispatch_concurrent_jobs` / sandboxed agents that write outside `write_shadow_file` are not
  yet fed.
