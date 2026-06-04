# Rollout Architecture v1 — Phase 9 Design (Features E + F + G)

**Based on:** `docs/decisions/ADR-010-rollout-proxy-boundary.md`, `cwso-nextgen-blueprint-v1.md`
§3.7, §5.2–5.4, Phase 8 gate (`gate-phase8-feature-d-2026-06-04.md`)
**Status:** accepted (spec via T131 MR !40 → `2d40413`) — gates T132–T138
**Phase:** 9 — Rollout-as-a-Service (Polar)

## 1. Problem

CWSO orchestrates multi-agent coding workflows but does not yet capture model I/O as training
signal. External RL trainers need:

1. A **transparent model API proxy** so all sub-agent LLM traffic flows through CWSO.
2. **Token-faithful trajectories** with prefix merging to avoid redundant training samples.
3. **Programmatic rewards** tied to merge/test outcomes (not human labels).
4. A **Polar-compatible REST API** for async rollout tasks and trainer callbacks.

Phase 9 delivers the rollout substrate; trainers remain external.

## 2. Component topology

```text
┌─────────────────────────────────────────────────────────────────┐
│ Go orchestrator (control plane)                                  │
│  • MCP tools (existing)                                          │
│  • /rollout/* REST API (T137)                                    │
│  • Trajectory builder + prefix merge (T133)                      │
│  • Reward hook on merge SM (T136)                                │
│  • Jobs manager (reuse for rollout task polling)                 │
└───────────────┬───────────────────────────────┬─────────────────┘
                │ UDS framed-JSON               │ HAL UDS (existing)
                ▼                               ▼
┌───────────────────────────┐       ┌─────────────────────────────┐
│ cwso-rollout (Rust)       │       │ cwso-hal (Rust)             │
│  • hyper reverse proxy    │──────▶│  gpu-accelerated / lpu / cpu│
│  • provider normalize     │       └─────────────────────────────┘
│  • completion capture     │
│  • KV prefix router (T135)│
│  • async trajectory I/O   │
│    (T134 Parquet writer)  │
└───────────────────────────┘
```

### 2.1 Process boundaries

| Component | Language | IPC | Owns |
|-----------|----------|-----|------|
| Rollout task API | Go | HTTP (trainer-facing) | Task lifecycle, callbacks, node registry |
| Trajectory builder | Go | in-process + UDS read | Prefix merge, loss masks, reward attachment |
| LLM proxy | Rust | HTTP inbound, UDS to HAL | Capture, normalize, forward, synthetic SSE |
| Trajectory store | Rust | lock-free channel | Parquet/LZ4 writes on dedicated thread |

## 3. Capture pipeline (Feature E — T132)

Four steps per Polar §3.2, implemented in `cwso-rollout`:

1. **Detect provider** — route by path + headers (`/v1/chat/completions`, `/v1/messages`,
   `/v1/models/:model:generateContent`).
2. **Normalize** — transform request to OpenAI Chat Completions shape; force `logprobs=true` when
   supported; strip provider-specific fields not needed upstream.
3. **Forward + store** — call upstream (HAL `gpu-accelerated` / `lpu-realtime` / configured URL);
   persist `CompletionRecord { request_id, prompt_token_ids, sampled_token_ids, logprobs,
   finish_reason, provider, timestamp_ns }` to the async store queue.
4. **Denormalize** — transform upstream response to the client's expected provider shape; for
   streaming clients, emit **synthetic SSE** chunks from the buffered completion.

### 3.1 Performance constraints

- Hot path uses borrowed JSON (`serde_json` + `RawValue` where possible) — no full DOM parse on
  pass-through fields.
- Capture enqueue is **non-blocking** (`try_send` + drop counter metric if saturated — PoC).
- Target: proxy overhead p95 ≤ 5 ms at 1k-token payloads (validated in T138).

### 3.2 Security

- Upstream API keys loaded from orchestrator env (`CWSO_ROLLOUT_UPSTREAM_*`); never forwarded to
  sub-agents.
- Request/response bodies logged at `trace` only; redact `Authorization` headers.
- TLS required for non-loopback upstream (reuse `cwso-hal` `validate_endpoint` pattern).

## 4. Trajectory builder (T133)

Input: ordered `CompletionRecord` stream for one rollout session.

Output: `TrajectoryGroup { chains[], rewards[], metadata }` where each chain is:

```text
Chain {
  chain_id: uuid
  prefix_token_ids: [...]      // shared context (system + repo AST)
  steps: [ { token_ids, loss_mask, logprobs } ]
}
```

**Prefix merging algorithm (Polar §3.4.2):**

1. Sort completions by `(session_id, timestamp_ns)`.
2. Build append-only chains: a new completion extends chain `C` iff its prompt token prefix equals
   `C.prefix || C.sampled_assistant_tokens` (strict token-prefix check).
3. For each step, set `loss_mask=1` only on **sampled assistant tokens**; system/user/context
   compaction tokens get `loss_mask=0`.
4. Parallel branches (e.g. two agents diverge) → separate chains automatically.

Determinism: stable sort keys; no randomness in merge logic.

## 5. Trajectory store (T134)

v1 PoC format:

```text
rollout_store/
  YYYY-MM-DD/
    {session_id}.parquet.lz4   # Arrow schema: completion + chain columns
```

- Protobuf schema `rollout.v1.CompletionRecord` (canonical); Arrow for columnar trainer ingest.
- Dedicated I/O thread drains `crossbeam-channel`; proxy thread never awaits fsync.
- Retention: `CWSO_ROLLOUT_STORE_RETENTION_DAYS` (default 7, PoC).

ClickHouse sink deferred post-v0.3.0.

## 6. KV-cache prefix router (T135 — Feature F)

**Prefix key:** `blake3(base_tree_oid || system_prompt_hash || shared_read_files_hash)` where
`base_tree_oid` is the shadow workspace shared base commit (blueprint §1.3).

Flow:

1. `POST /rollout/task/submit` includes `workspace_id` → orchestrator resolves base OID → sends
   prewarm request to proxy.
2. Proxy checks cache; on miss, sends prefix-only request to HAL/vLLM with `cache_salt=prefix_key`.
3. Agent requests include `prefix_key` metadata; proxy strips cached prefix tokens from upstream
   prompt (differential prompting).

Invalidation: base OID change → new prefix key; stale entries age out via LRU.

## 7. Programmatic rewards (T136 — Feature G)

Hook points in Go merge state machine (`merge_concurrent_results` completion path):

| Event | Reward | Condition |
|-------|--------|-----------|
| `merge_success` | +1.0 | No semantic conflicts; output syntactically valid |
| `merge_conflict` | -1.0 | `MergeError::SemanticConflict` or unresolved conflict |
| `syntax_fail` | -1.0 | Post-merge validation fails (future: tree-sitter parse check) |
| `test_pass` bonus | +0.0 (v1) | Tests green — optional composite in v2 |

Rewards publish to topic `rollout/reward` and attach to trajectory groups returned by
`GET /rollout/task/{task_id}`.

## 8. Polar service API (T137)

Non-MCP REST routes on orchestrator HTTP server (port shared with MCP Streamable HTTP):

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/rollout/task/submit` | Enqueue rollout; prewarm KV prefix |
| GET | `/rollout/task/{task_id}` | Poll status, partial results, trajectories |
| GET | `/rollout/status` | Fleet: nodes, cache hit rate, pending sessions |
| POST | `/callbacks/session_result` | Gateway → trainer callback |
| POST | `/nodes/register` | Register rollout worker node |
| POST | `/nodes/{id}/heartbeat` | Liveness |
| POST | `/v1/chat/completions` | Transparent proxy (when enabled) |

JSON schemas: `schemas/rollout_task_submit.json`, `schemas/rollout_task_status.json`.

gRPC deferred; REST sufficient for PoC trainer e2e (T138).

## 9. Configuration (feature flags)

| Env var | Default | Gates |
|---------|---------|-------|
| `CWSO_ROLLOUT_PROXY_ENABLED` | false | Proxy routes + sidecar |
| `CWSO_ROLLOUT_API_ENABLED` | false | `/rollout/*` REST |
| `CWSO_ROLLOUT_SOCKET` | — | UDS path to `cwso-rollout` |
| `CWSO_ROLLOUT_UPSTREAM_URL` | — | Default HAL/upstream for proxy |
| `CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED` | false | Parquet writer |
| `CWSO_ROLLOUT_STORE_PATH` | `./rollout_store` | Storage directory |
| `CWSO_ROLLOUT_KV_PREFIX_ROUTER_ENABLED` | false | Prefix cache |
| `CWSO_ROLLOUT_REWARD_ENABLED` | false | Merge SM reward hook |

## 10. Implementation breakdown

| Active ID | Roadmap | Title | Owner | Pri | Depends |
|-----------|---------|-------|-------|-----|---------|
| T131 | T105 | Rollout architecture (this doc + ADR-010) | solution-architect | P0 | T130 |
| T132 | T106 | Rust `hyper` reverse proxy + capture | backend-developer | P0 | T131 |
| T133 | T107 | Trajectory builder + prefix merging | backend-developer | P0 | T132 |
| T134 | T108 | Trajectory store (Arrow + LZ4 + Parquet) | backend-developer | P1 | T133 |
| T135 | T109 | KV-cache prefix router | backend-developer | P1 | T132 |
| T136 | T110 | Programmatic reward emission | backend-developer | P0 | T133 |
| T137 | T111 | Polar REST API + trainer e2e | backend-developer | P0 | T134, T136 |
| T138 | T112 | Phase 9 integration QA + security gate | qa / security | P0 | T137 |
| T139 | T113 | v0.3.0 release readiness | release-manager | P0 | T138 |

Recommended delivery order: T132 → T133 → T136 (parallel T135) → T134 → T137 → T138 → T139.

## 11. Security & determinism

- Proxy is the **only** model egress for sandboxed agents (configure `base_url` → CWSO).
- Trajectory files must not contain raw API keys; redact `Authorization` at capture.
- Reward signals derive from deterministic merge SM — no LLM-as-judge in v1.
- UDS peer-auth on all sidecar IPC (reuse T058 hardening).

## 12. Out of scope (v1)

- Embedded trainer / GRPO implementation inside CWSO.
- gRPC rollout API (REST only for PoC).
- ClickHouse trajectory warehouse.
- Multi-region rollout gateway replication.
