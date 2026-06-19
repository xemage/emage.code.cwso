# Task T115 — AST write-spike monitor (generalize `anomaly_monitor`) + userspace fallback

> **ID note:** this is roadmap **Feature C / placeholder T095** ("eBPF AST write-spike
> monitor"). Active execution IDs continue sequentially from the board, so it lands as
> **T115** (T095 stays a roadmap placeholder). See the numbering-reconciliation section in
> `active-tasks.md`.

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T089 (done — Phase 6 gate)
- **Phase:** 7 — Sparse Micro-Agents & Spiking Monitors (Feature C — Event-Driven Spiking AST Monitors)
- **Based on:** `docs/plans/plan-cwso-nextgen-phase6plus.md`, `docs/artifacts/cwso-nextgen-blueprint-v1.md`

## Objective
Detect bursts of AST-affecting writes ("write spikes") inside a short sliding window and
emit early-warning telemetry, generalizing the event-driven core already proven in the
dispatch `anomaly_monitor`, with a privilege-free userspace fallback when an eBPF hook is
unavailable.

## Context
The blueprint's Feature C calls for event-driven monitors that fire *before* the merge
engine runs, warning that multiple concurrent agents are converging on the same code. The
dispatch `anomaly_monitor` already implements the hard part — an eBPF-preferred /
userspace-fallback signal-path abstraction with advisory-vs-measured detection-latency
semantics. T115 extracts that core and reuses it for filesystem write activity.

## Changes
- **`orchestrator/internal/dispatch/signal_path.go` (new):** extracted `signalPathResolver`
  (eBPF preference + injectable availability checker → path / privilege / notes),
  `detectionLatency`, the `defaultEBPFChecker`, and the `signal_path*` / `detection_mode*`
  vocabulary out of `anomaly_monitor.go`.
- **`orchestrator/internal/dispatch/anomaly_monitor.go`:** now consumes `signalPathResolver`
  instead of its own `preferEBPF`/`checkEBPF` fields and helpers. Behaviour unchanged
  (existing anomaly tests are the guard).
- **`orchestrator/internal/dispatch/ast_spike_monitor.go` (new):** `ASTWriteSpikeMonitor`
  with a source-agnostic `ObserveWrite(WriteEvent)` ingestion API, per-workspace sliding
  window, threshold + debounce, and `ASTSpikeEvent` publication on topic `ast/spike`
  (`TopicASTSpike`). Reports hot paths, distinct-path count, languages, severity
  (`warning` → `critical` at 2× threshold), and the resolved signal-path characterization.
- **`orchestrator/internal/dispatch/telemetry.go`:** added `TopicASTSpike = "ast/spike"`.
- **`orchestrator/internal/dispatch/ast_spike_monitor_test.go` (new):** 9 unit tests.

## Acceptance Criteria
- [x] Spike fires when window write count ≥ threshold; stays silent below threshold.
- [x] Writes outside the sliding window are pruned (no spike when writes never co-occur).
- [x] Debounce collapses a sustained burst into a single event by default.
- [x] Severity escalates to `critical` at 2× threshold.
- [x] eBPF-preferred path reports `ebpf-hook` + advisory latency when available, and degrades
      to `fallback-userspace` (measured latency, reason in notes) when not.
- [x] Redaction policy `anomaly_notes_mode=drop` drops both notes and hot paths.
- [x] Per-workspace windows are isolated.
- [x] `go build` / `go vet` / `go test ./...` + gofmt clean; existing anomaly tests still green.

## Notes / Follow-ups
- **Source-agnostic by design:** `ObserveWrite` is fed by either an eBPF write probe or a
  userspace filesystem watcher. The concrete feeders + the `subscribe_ast_spikes` MCP SSE
  resource are roadmap **T096** (sparse spike filter + semantic-conflict pre-warning) and
  **T097** (`subscribe_ast_spikes` resource + write-event wiring) — not in T115 scope.
- `queue_depth`-style backpressure for the write stream is out of scope; the monitor only
  caps memory via window pruning.
