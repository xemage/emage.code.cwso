# ADR-010 — Rollout proxy boundary and Polar-compatible substrate

- **Status:** accepted (pending merge via T131)
- **Date:** 2026-06-04
- **Decision-maker:** solution-architect
- **Tasks:** T131 (design), T132–T138 (implementation + gate), T139 (release)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md` §5.2–5.4, §3.7; Phase 8 complete (`7dc4e7a`); ADR-001, ADR-006

## Context

Phases 6–8 delivered hardware-aware dispatch, sparse micro-agents, AST spike monitors, and a
sparse merge pre-filter. Phase 9 (**Rollout-as-a-Service**) turns every solved task into training
data by adopting Polar's central idea: **move the RL integration boundary to the model API
endpoint** — proxy all LLM traffic, capture completions, reconstruct token-faithful trajectories,
emit programmatic rewards from the merge state machine, and expose Polar-compatible REST endpoints
so external trainers (Ray RLlib, HF TRL, Slime) plug in unchanged.

CWSO is **not** a trainer. It implements only the rollout substrate (proxy + capture + storage +
reward + service API).

Constraints:

- **Go/Rust split (ADR-001).** Go orchestrator owns policy, jobs, MCP, and rollout task lifecycle;
  Rust owns the high-throughput proxy and trajectory I/O (mirrors `cwso-hal`, `cwso-merge-engine`,
  `cwso-sparse` sidecar pattern).
- **Deterministic rewards.** Merge outcomes already produce authoritative success/failure signals
  (ADR-006). Rewards must derive from the same state machine — no model-judged labels in v1.
- **Performance.** The proxy sits in front of fast LPU backends (Phase 6); overhead must stay bounded
  (blueprint target: not the CPU bottleneck). Rust `hyper` + zero-copy JSON borrowing on the hot path.
- **Security.** Provider API keys never reach sub-agents; the proxy is the sole egress for model
  traffic. Trajectory storage must not leak secrets from prompts.

## Decision

1. **New Rust sidecar `cwso-rollout`** (framed-JSON UDS to Go, `SO_PEERCRED` peer-auth — same
   envelope as `cwso-hal`). Hosts:
   - Transparent reverse proxy (`/v1/chat/completions`, Anthropic Messages, Google `generateContent`).
   - Four-step capture pipeline (detect provider → normalize to OpenAI Chat + `logprobs=true` →
     forward to upstream HAL/backend → store completion record → transform response to provider shape).
   - Synthetic SSE for streaming clients when upstream is non-streaming (Polar pattern).

2. **Go control plane owns rollout task API and trainer integration.** New package
   `orchestrator/internal/rollout` registers Polar-style HTTP routes on the existing daemon HTTP
   server (non-MCP side API per blueprint §3.7):
   - `POST /rollout/task/submit`, `GET /rollout/task/{task_id}`, `GET /rollout/status`
   - `POST /callbacks/session_result`, `POST /nodes/register`, `POST /nodes/{id}/heartbeat`
   - Proxied model routes delegate to `cwso-rollout` when `CWSO_ROLLOUT_PROXY_ENABLED=true`.

3. **Trajectory builder (Go) assembles token-faithful traces** from completion records stored by the
   Rust proxy, applying **prefix merging** (Polar §3.4.2): partition completions into append-only
   chains; copy only sampled assistant tokens as trainable (`loss_mask=1`); mask canonical
   interstitials. Sub-agents, context compaction, and parallel branches form separate chains.

4. **Trajectory store (Rust I/O thread)** writes Protobuf/Arrow + LZ4 to Parquet files (v1 PoC;
   ClickHouse deferred). Async via lock-free queue (`crossbeam-channel`) — capture path never blocks
   on disk.

5. **KV-cache prefix router (inside proxy, Feature F)** keys warm prefixes by shared base tree OID
   (shadow workspace base commit). Prewarm on `POST /rollout/task/submit`. Router is advisory;
   incorrect cache reuse must not change model output (strict prefix token check per Polar).

6. **Programmatic reward emission (Go)** hooks the merge state machine:
   - `+1.0` — workspace tests pass AND `merge_concurrent_results` completes with no AST conflicts.
   - `-1.0` — syntactically invalid output OR merge failure.
   Rewards attach to trajectory groups for trainer callbacks.

7. **Feature-flagged off by default.** `CWSO_ROLLOUT_PROXY_ENABLED`, `CWSO_ROLLOUT_API_ENABLED`,
   `CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED`, `CWSO_ROLLOUT_KV_PREFIX_ROUTER_ENABLED` — each
   independently gateable for incremental delivery.

## Alternatives considered

| Alt | Rejected because |
|-----|------------------|
| A — Proxy entirely in Go (`net/http` reverse proxy) | Hot-path JSON normalize/transform too slow in front of LPUs; violates ADR-001 data-plane split |
| B — Trainer embedded in CWSO | Out of scope; Polar explicitly separates rollout substrate from training |
| C — JSON trajectory storage | TB-scale explosion; blueprint mandates binary + LZ4 |
| D — MCP-only rollout API | RL trainers are not MCP clients; REST/gRPC side API required (blueprint §3.7) |
| E (chosen) — Rust `cwso-rollout` sidecar + Go task/reward API | Matches existing sidecar pattern; bounded proxy overhead; clear security boundary for API keys |

## Consequences

- (+) Every agent LLM call becomes capturable training data without harness changes (set `base_url`
  to CWSO proxy).
- (+) Programmatic rewards reuse deterministic merge semantics — no labeler model.
- (+) Prefix router can cut swarm inference cost when agents share repo context.
- (−) New sidecar to deploy, monitor, and secure (API keys at rest in orchestrator env only).
- (−) Provider shape normalization is ongoing maintenance (OpenAI/Anthropic/Google drift).
- (−) Trajectory storage growth requires retention policy (deferred to T134/T138 gate).

## Implementation mapping (active IDs)

| Roadmap | Active | Title |
|---------|--------|-------|
| T105 | T131 | Rollout architecture (this ADR + design v1) |
| T106 | T132 | Rust `hyper` reverse proxy + zero-copy capture |
| T107 | T133 | Trajectory builder + prefix merging |
| T108 | T134 | Trajectory store (Protobuf/Arrow + LZ4 + Parquet) |
| T109 | T135 | KV-cache prefix router (base tree OID key) |
| T110 | T136 | Programmatic reward emission from merge SM |
| T111 | T137 | Polar service API + external trainer e2e |
| T112 | T138 | Phase 9 integration QA + security gate |
| T113 | T139 | Next-Gen release readiness (v0.3.0) |
