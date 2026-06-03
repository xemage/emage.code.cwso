# Task T116 — Spike filter (semantic classifier) + semantic-conflict pre-warning

> **ID note:** roadmap **Feature C / placeholder T096** ("Spike filter (sparse mini-model) +
> semantic-conflict pre-warning"). Active IDs continue from the board, so this lands as
> **T116** (T096 stays a roadmap placeholder). See the numbering-reconciliation section in
> `active-tasks.md`.

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T115 (done — AST write-spike monitor)
- **Phase:** 7 — Sparse Micro-Agents & Spiking Monitors (Feature C — Event-Driven Spiking AST Monitors)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md` §3.6 + Feature C steps 2 & 5, `task-T115.md`

## Objective
On an AST write-spike, decide whether the edit is *semantically significant* (changes a
symbol surface / function signature) and, when concurrent workspaces touch the same symbol,
emit an early conflict pre-warning — before `merge_concurrent_results` is invoked.

## Context
T115 delivered the volume monitor (write-rate spikes). The blueprint's Feature C step 2 adds
a "spike filter": a tiny sparse mini-model that classifies whether a write changes a
function signature / symbol surface, so monitoring fires only on relevant events (the
neuromorphic "spike or sleep" energy story). Step 5 feeds the merge loop with pre-warnings.

The intended scorer is a Wasm sparse micro-agent from Feature B — which is **not yet built**
(its roadmap IDs T090–T094 collided with the Phase 6 gate follow-ups). T116 therefore ships a
deterministic, dependency-free classifier behind a **pluggable `SemanticScorer` seam** so the
Wasm model can replace it later with no change to the correlation logic.

## Changes
- **`orchestrator/internal/dispatch/ast_spike_filter.go` (new):**
  - `SpikeKind` ordering (`none` < `cosmetic` < `symbol_added`/`symbol_removed` <
    `signature_change`) + threshold-rank gating (`any` accepts cosmetic-or-stronger).
  - `SemanticScorer` func seam + default `HeuristicSemanticScorer` (trusts feeder
    `ChangeKind`, else diffs `SignatureHash` against the last-seen signature for the symbol).
  - `ASTSpikeFilter.ObserveWrite`: classify → gate on `SemanticThreshold` → publish
    `SemanticSpikeEvent` (topic `ast/semantic-spike`); maintain a TTL-pruned per-symbol
    signature memory + a windowed per-symbol recent-writers index → publish
    `SemanticConflictWarning` (topic `ast/conflict-warning`) on cross-workspace overlap.
  - Reuses `signalPathResolver` + `detectionLatency`; drops symbol/path/node-path under
    `anomaly_notes_mode=drop` redaction (after correlation).
- **`orchestrator/internal/dispatch/ast_spike_monitor.go`:** `WriteEvent` gains optional
  `Symbol`, `NodePath`, `SignatureHash`, `ChangeKind` (T115 volume monitor ignores them).
- **`orchestrator/internal/dispatch/telemetry.go`:** `TopicASTSemanticSpike`,
  `TopicASTConflictWarning`.
- **`orchestrator/internal/dispatch/ast_spike_filter_test.go` (new):** 9 unit tests.

## Acceptance Criteria
- [x] Semantic spike emitted only when classified kind ≥ `SemanticThreshold` (default
      `signature_change`); `any` fires for cosmetic-or-stronger; non-semantic writes stay silent.
- [x] Signature delta classification: first touch → `symbol_added`; changed hash →
      `signature_change`; unchanged hash → `cosmetic`.
- [x] Conflict pre-warning fires only when ≥2 distinct workspaces hit the same symbol within
      the correlation window; lists `potential_conflict_with` + both workspaces; `critical`
      for `signature_change`.
- [x] Stale writers age out of the correlation window (no false warning).
- [x] eBPF-preferred path degrades to userspace fallback with reason in notes.
- [x] `anomaly_notes_mode=drop` blanks symbol/path/node-path + notes; workspace correlation
      still works.
- [x] Custom `SemanticScorer` overrides the heuristic.
- [x] `go build` / `go vet` / `go test -race ./...` + gofmt clean.

## Notes / Follow-ups
- **Not wired to a runtime feeder yet.** `ObserveWrite` is source-agnostic; the
  `subscribe_ast_spikes` MCP SSE resource and the concrete eBPF / userspace fs-watcher feeders
  are roadmap **T097**.
- `queue_depth`/backpressure for the write stream and a real sparse-model scorer are deferred
  (the scorer seam is in place for the latter).
