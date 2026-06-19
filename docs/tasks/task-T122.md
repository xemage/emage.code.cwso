# Task T122 — `create_ephemeral_sparse_agent` + wasmtime lifecycle + agent telemetry

> **ID note:** roadmap **Feature B / placeholder T093** ("create_ephemeral_sparse_agent tool +
> telemetry resource"). Active ID **T122** (see numbering reconciliation in `active-tasks.md`).

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T120 (done), T121 (done)
- **Phase:** 7 — Sparse Micro-Agents & Spiking Monitors (**Feature B**)
- **Based on:** `docs/decisions/ADR-008-wasm-sparse-agent-tier.md`, `docs/artifacts/wasm-sparse-agent-design-v1.md` §6–§7

## Objective
Expose the sparse micro-agent tier via MCP: create ephemeral wasmtime-backed agents over pinned
`.cwsl` skill slices and stream per-agent telemetry through the existing MCP Resources / SSE layer.

## Changes
- **Rust `agent.rs`:** `AgentRegistry` + `AgentConfig::from_env()`; wasmtime instantiation with
  memory cap; slice mmap retained for agent lifetime; `create_agent` / `drop_agent` / `agent_stat`.
- **Rust IPC:** new ops on contract v2; `main.rs` boots registry when `CWSO_SPARSE_SLICE_MANIFEST`
  is set. `wasmtime = "29"` dependency added.
- **Go `internal/sparse`:** UDS client (`CreateAgent`, `DropAgent`, `AgentStat`, `Stat`).
- **Go dispatch:** `SparseAgentRegistry`, `AgentTelemetryEvent`, `TopicAgentTelemetry`,
  `AgentTelemetryFilter`, URI helpers.
- **Go tool:** `create_ephemeral_sparse_agent` (orchestrator-only) + schema JSON.
- **Server:** register tool when `CWSO_SPARSE_AGENTS_ENABLED` + socket configured; extend MCP
  Resources handlers + composite SSE subscription resolver for `cwso://agents/{id}/telemetry`.
- **Config:** `CWSO_SPARSE_AGENTS_ENABLED`, `CWSO_SPARSE_SOCKET`, `CWSO_SPARSE_HOST_RAM_CAP_MB`.

## Acceptance Criteria
- [x] Sidecar `create_agent` resolves manifest domain, mmap-pins slice, instantiates wasmtime, returns metrics.
- [x] Tool validates inputs, calls sidecar, registers agent, publishes telemetry, returns stream URI.
- [x] `resources/read` and scoped SSE deliver `agents/telemetry` events for the agent id.
- [x] Feature off by default; enabled only with explicit env configuration.
- [x] Unit tests pass (`cargo test -p cwso-sparse`, `go test ./...`).

## Notes / Follow-ups
- **Host-call `ternary_gemm` inside wasmtime** is deferred until an orchestration Wasm module with
  imports ships; the sidecar registers the allowlist path in T122 follow-up when that module exists.
  The standalone `ternary_gemm` IPC op (T120) remains the deterministic kernel contract.
- **T123** wires quality-floor breach → dense GPU escalation via existing policy guardrails.
