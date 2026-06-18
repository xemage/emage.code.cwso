# CWSO Installation & Usage Guide — v2 (comprehensive)

> **Based on:** `develop` targeting **v0.4.0**  
> **Audience:** operators, integrators, and developers adopting CWSO for agent orchestration + Polar-style rollout  
> **Supersedes:** [installation-v1.md](installation-v1.md) for v0.4.0+ (v1 retained for v0.3.0 reference)

## Table of contents

1. [Architecture](#1-architecture)
2. [Prerequisites & install](#2-prerequisites--install)
3. [Configuration reference](#3-configuration-reference)
4. [Workflow: MCP shadow agent](#4-workflow-mcp-shadow-agent)
5. [Workflow: IDE + coding tool](#5-workflow-ide--coding-tool)
6. [Workflow: Polar rollout capture](#6-workflow-polar-rollout-capture)
7. [Workflow: Gateway staging & evaluators](#7-workflow-gateway-staging--evaluators)
8. [Harness adapters](#8-harness-adapters)
9. [Operations & CI](#9-operations--ci)
10. [Troubleshooting](#10-troubleshooting)
11. [Further reading](#11-further-reading)

---

## 1. Architecture

CWSO is a **deterministic MCP orchestrator** with optional Rust sidecars. Three surfaces matter for adopters:

| Component | Default bind | Role |
|-----------|--------------|------|
| **cwso-orchestrator** | `:8080` HTTP | MCP tools, JWT auth, rollout REST API, job dispatch |
| **cwso-git-shadow** | UDS `/run/cwso/git-shadow.sock` | Isolated git shadow workspaces |
| **cwso-merge-engine** | UDS `/run/cwso/merge-engine.sock` | Concurrent merge (Phase 4) |
| **cwso-rollout** (sidecar) | proxy `:8787`, UDS `/run/cwso/rollout.sock` | Model API proxy + trajectory capture (Polar) |

```text
  Cursor / VS Code                External trainer
        |                                |
        v                                v
   /mcp (JWT)                    /rollout/task/submit
        |                                |
        +-------- cwso-orchestrator -----+
                        |
            +-----------+-----------+
            v           v           v
      git-shadow   merge-engine   rollout UDS
                                        |
                                        v
                              cwso-rollout proxy :8787
                                        |
                                        v
                              upstream LLM (OpenAI, etc.)
```

**Rule:** Model traffic for RL capture goes through the **rollout proxy**, not orchestrator `/v1/chat/completions` (501 by design). MCP tools stay on the orchestrator.

All Phase 6–9 capabilities are **default-off** behind `CWSO_*` environment variables.

---

## 2. Prerequisites & install

| Requirement | Notes |
|-------------|-------|
| Docker Engine 24+ | Compose v2 required |
| Python 3.10+ | `scripts/phase2-integration.py` (stdlib only) |
| Git | Clone + optional submodules |
| Go / Rust | Optional; CI and `make test` use containers |

### Quick start (Phase 2)

```bash
git clone https://gitlab.com/em-age/emage.code.cwso.git
cd emage.code.cwso
make build
docker compose -f deploy/docker-compose.yml --profile phase2 up -d
curl -sS http://127.0.0.1:8080/healthz
python3 scripts/phase2-integration.py   # expect: PASS
```

One-command deterministic local smoke:

```bash
make smoke-local
```

### Full local stack (Phase 2 + 4)

```bash
docker compose -f deploy/docker-compose.yml --profile phase2 --profile phase4 up -d
CWSO_COMPOSE_PROFILES=phase2,phase4 CWSO_PHASE4_MATRIX=1 \
  python3 scripts/phase2-integration.py
```

### Enable all orchestrator flags (PoC)

```bash
source scripts/cwso-enable-all-features.sh
# Recreate orchestrator with exported env (see script output)
docker compose -f deploy/docker-compose.yml --profile phase2 --profile phase4 \
  up -d --force-recreate orchestrator
```

Sidecars (HAL, sparse, rollout) require separate containers — see [rollout-architecture-v1.md](../artifacts/rollout-architecture-v1.md).

---

## 3. Configuration reference

### Core transport & auth

| Variable | Default | Description |
|----------|---------|-------------|
| `CWSO_TRANSPORT` | `stdio` | Set `http` for Docker/IDE |
| `CWSO_HTTP_ADDR` | `:8080` | Orchestrator listen address |
| `CWSO_JWT_SECRET` | — | HS256 secret (or mount `/run/secrets/jwt_secret`) |
| `CWSO_ALLOWED_ORIGINS` | localhost | Comma-separated MCP Origin allowlist |

JWT claims: `iss=cwso`, `aud=cwso-mcp`, `role=worker|orchestrator`.

### Sidecar sockets

| Variable | Sidecar |
|----------|---------|
| `CWSO_GIT_SHADOW_SOCKET` | git-shadow |
| `CWSO_MERGE_ENGINE_SOCKET` | merge-engine |
| `CWSO_HAL_SOCKET` | cwso-hal |
| `CWSO_SPARSE_SOCKET` | cwso-sparse |
| `CWSO_ROLLOUT_SOCKET` | cwso-rollout (trajectory drain) |

### Rollout / Polar (Phase 9)

| Variable | Default | Description |
|----------|---------|-------------|
| `CWSO_ROLLOUT_API_ENABLED` | `false` | `/rollout/*` REST API |
| `CWSO_ROLLOUT_REWARD_ENABLED` | `false` | Merge SM programmatic rewards |
| `CWSO_ROLLOUT_KV_PREFIX_ROUTER_ENABLED` | `false` | BLAKE3 prefix router on submit |
| `CWSO_ROLLOUT_PROXY_ENABLED` | `false` | Sidecar HTTP proxy (Rust) |
| `CWSO_ROLLOUT_KV_DIFFERENTIAL_PROMPTING_ENABLED` | `false` | Differential prompting on prefix-cache hit + `cache_salt` forwarding |
| `CWSO_ROLLOUT_HTTP_BIND` | `127.0.0.1:8787` | Proxy listen (sidecar) |
| `CWSO_ROLLOUT_UPSTREAM_URL` | — | Upstream inference base URL |

### Trajectory builder (T149)

| Variable | Default | Description |
|----------|---------|-------------|
| `CWSO_ROLLOUT_TRAJECTORY_BUILDER_ENABLED` | `false` | Enable Polar trajectory builder v2 when draining captures |
| `CWSO_ROLLOUT_TRAJECTORY_BUILDER_STRATEGY` | `prefix_merge` | Default strategy: `prefix_merge` or `per_request` |

When enabled, completion records from the rollout sidecar are assembled into trainer-facing
`TrajectoryGroup` chains. **`prefix_merge`** (Polar default) merges successive completions whose
prompts extend the same token prefix into one chain — fewer trainer samples, with EOT interstitial
tokens masked (`loss_mask=0`) and partition keys splitting sub-agent/compaction boundaries.
**`per_request`** emits one independent chain per completion (useful for debugging or when prefix
sharing is not valid). Override per task via `trajectory_builder_strategy` on submit (empty =
server default).

### Gateway staging (T146)

| Variable | Default | Description |
|----------|---------|-------------|
| `CWSO_ROLLOUT_GATEWAY_STAGING_ENABLED` | `false` | INIT→READY→RUNNING→POSTRUN pools |
| `CWSO_ROLLOUT_GATEWAY_INIT_WORKERS` | `2` | INIT pool size |
| `CWSO_ROLLOUT_GATEWAY_READY_BUFFER` | `4` | READY queue depth |
| `CWSO_ROLLOUT_GATEWAY_RUNNING_WORKERS` | `4` | RUNNING pool size |
| `CWSO_ROLLOUT_GATEWAY_POSTRUN_WORKERS` | `2` | POSTRUN pool size |
| `CWSO_ROLLOUT_GATEWAY_SESSION_TIMEOUT_SECONDS` | `300` | RUNNING timeout; partial trace on expiry |
| `CWSO_ROLLOUT_EVALUATOR_PREWARM_ENABLED` | `false` | Prewarm evaluators during RUNNING |

### Evaluator registry (T148)

| Variable | Default | Description |
|----------|---------|-------------|
| `CWSO_ROLLOUT_EVALUATOR_REGISTRY_ENABLED` | `false` | Post-run evaluator plugins |
| `CWSO_ROLLOUT_EVALUATOR_SESSION_REWARD_ENABLED` | `false` | Merge SM reward plugin |
| `CWSO_ROLLOUT_EVALUATOR_SWEBENCH_ENABLED` | `false` | SWE-bench stub hook |
| `CWSO_ROLLOUT_SWEBENCH_INSTANCE` | — | Instance id for PoC scoring |

See [installation-v1.md §6](installation-v1.md#6-next-gen-features-default-off) for Phase 6–8 HAL/sparse/spike flags.

---

## 4. Workflow: MCP shadow agent

1. Mint JWT with **worker** role (see [installation-v1.md §3](installation-v1.md#3-jwt-authentication)).
2. Call MCP over HTTP:

```bash
curl -sS http://127.0.0.1:8080/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Origin: http://localhost" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

3. Typical tool sequence:
   - `create_shadow_workspace` → `write_shadow_file` / `query_ast` → `commit_shadow`
   - Optional Phase 4: `merge_concurrent_results` after parallel workers

Orchestrator-only tools (`dispatch_concurrent_jobs`, rollout admin) require **orchestrator** role.

---

## 5. Workflow: IDE + coding tool

Full Cursor / VS Code MCP wiring: **[ide-integration-v1.md](ide-integration-v1.md)**.

Summary:

1. Start CWSO (`phase2` + optional `phase4`).
2. Export `CWSO_MCP_TOKEN` (JWT, worker role).
3. Point IDE MCP at `http://127.0.0.1:8080/mcp`.
4. For Polar capture, set `OPENAI_BASE_URL=http://127.0.0.1:8787` (rollout proxy).

---

## 6. Workflow: Polar rollout capture

Enable rollout API + sidecar proxy, then submit tasks from a trainer or script.

```bash
export CWSO_ROLLOUT_API_ENABLED=true
export CWSO_ROLLOUT_TRAJECTORY_BUILDER_ENABLED=true
export CWSO_ROLLOUT_TRAJECTORY_BUILDER_STRATEGY=prefix_merge   # or per_request
export CWSO_ROLLOUT_SOCKET=/run/cwso/rollout.sock   # when sidecar attached
# Sidecar env (separate container):
export CWSO_ROLLOUT_PROXY_ENABLED=true
export CWSO_ROLLOUT_UPSTREAM_URL=http://your-inference:8000
export OPENAI_BASE_URL=http://127.0.0.1:8787
```

### REST endpoints (orchestrator)

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/rollout/task/submit` | Enqueue task; optional `num_samples` (1–32), `trajectory_builder_strategy` |
| POST | `/rollout/task/offline_generate` | Build trajectories from existing session IDs (no callback) |
| GET | `/rollout/task/{task_id}` | Poll status + trajectories |
| GET | `/rollout/status` | Cluster summary |
| POST | `/callbacks/session_result` | Per-session callback (`session_id` when N>1) |

### Submit example (single session)

```bash
curl -sS http://127.0.0.1:8080/rollout/task/submit \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "task_spec": {"description": "fix flaky test", "workspace_id": "ws-1"},
    "num_samples": 1,
    "trajectory_builder_strategy": "prefix_merge",
    "trainer_callback_url": "http://trainer/callbacks/session_result"
  }'
```

`trajectory_builder_strategy` is optional (`prefix_merge` or `per_request`); when omitted the
orchestrator uses `CWSO_ROLLOUT_TRAJECTORY_BUILDER_STRATEGY`. Requires
`CWSO_ROLLOUT_TRAJECTORY_BUILDER_ENABLED=true` for builder v2 assembly at drain time.

### Multi-session fan-out (`num_samples` > 1)

Set `"num_samples": 3` to spawn three distinct `session_id`s. Callbacks must include `session_id`; task completes when all sessions report.

### Offline SFT generation mode

Use offline mode to generate trajectories from captured sessions without trainer callbacks:

```bash
curl -sS http://127.0.0.1:8080/rollout/task/offline_generate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "task_spec": {"description": "offline-sft", "workspace_id": "ws-1"},
    "source_session_ids": ["session-a", "session-b"],
    "drain_limit": 128,
    "trajectory_builder_strategy": "prefix_merge"
  }'
```

This path is intended for fixed-checkpoint batch generation and Parquet-backed capture replay.

### Reference harness

```bash
./scripts/shell-command-harness.sh   # one completion through proxy for capture
```

---

## 7. Workflow: Gateway staging & evaluators

For production-like Polar runs where init/eval must not block GPU harness execution:

```bash
export CWSO_ROLLOUT_API_ENABLED=true
export CWSO_ROLLOUT_GATEWAY_STAGING_ENABLED=true
export CWSO_ROLLOUT_EVALUATOR_PREWARM_ENABLED=true    # optional
export CWSO_ROLLOUT_EVALUATOR_REGISTRY_ENABLED=true
export CWSO_ROLLOUT_EVALUATOR_SESSION_REWARD_ENABLED=true
export CWSO_ROLLOUT_EVALUATOR_SWEBENCH_ENABLED=true   # PoC stub
export CWSO_ROLLOUT_SWEBENCH_INSTANCE=django-1234
```

**Gateway pools:** INIT (prep) → READY (buffer) → RUNNING (harness) → POSTRUN (eval + trajectory finalize). On **timeout**, POSTRUN still runs with **partial captures** and terminal failed status.

**Evaluators:** After trajectory construction, registered plugins attach scores to `TrajectoryGroup.Rewards` and `Metadata` (Polar §3.5). Session-reward plugin reads merge SM topic; SWE-bench plugin is a stub pending full harness launch.

---

## 8. Harness adapters

Registry + Docker runtime in `orchestrator/internal/harness/`. Default proxy: `http://127.0.0.1:8787`.

| Harness ID | Model env | Notes |
|------------|-----------|-------|
| `shell-command` | `OPENAI_BASE_URL` | Reference — `scripts/shell-command-harness.sh` |
| `codex` | `OPENAI_BASE_URL` | Stub launch (wire CLI image in production) |
| `claude_code` | `ANTHROPIC_BASE_URL` | Stub launch |
| `qwen_code` | `OPENAI_BASE_URL` | Stub launch |

Runtime API: `start`, `stop`, `exec`, `upload`, `download` via Docker HTTP (`CWSO_DOCKER_HOST`).

---

## 9. Operations & CI

- **Health:** `GET /healthz` on orchestrator.
- **Logs:** `docker compose -f deploy/docker-compose.yml logs -f orchestrator`
- **CI parity:** `.gitlab-ci.yml` runs lint → build → test → audit → e2e; socket-mounted DinD for e2e.
- **Tag pipelines:** Registry deploy uses `needs:optional` on e2e (T153) so release tags validate.

Local CI-like run:

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.ci.yml \
  --profile phase2 --profile phase4 up -d
python3 scripts/phase2-integration.py
```

---

## 10. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| 401 on `/mcp` | JWT mismatch | Check secret, `iss`/`aud`/`role`, expiry |
| 403 Origin | Origin not allowed | Add to `CWSO_ALLOWED_ORIGINS` |
| Shadow tools missing | Sidecar down | `--profile phase2`; check git-shadow UDS |
| Rollout 404 | API disabled | `CWSO_ROLLOUT_API_ENABLED=true`; restart |
| No model capture | Wrong URL | Use rollout proxy `:8787`, not orchestrator |
| Gateway tasks hang | Pool exhaustion | Increase `CWSO_ROLLOUT_GATEWAY_*_WORKERS` |
| Partial trajectory only | Session timeout | Expected POSTRUN behavior (T146); increase timeout |
| E2E merge-engine fail | Socket timing | Retry pipeline; ensure phase4 profile |
| Evaluator metadata empty | Registry off | Enable `CWSO_ROLLOUT_EVALUATOR_REGISTRY_ENABLED` |

---

## 11. Further reading

- [installation-v1.md](installation-v1.md) — v0.3.0 quick reference
- [ide-integration-v1.md](ide-integration-v1.md) — Cursor / VS Code
- [rollout-architecture-v1.md](../artifacts/rollout-architecture-v1.md) — Polar substrate design
- [polar-gap-analysis-v1.md](../artifacts/polar-gap-analysis-v1.md) — parity matrix
- [release-v0.3.0.md](../artifacts/release-v0.3.0.md) — prior GA scope
- [SECURITY.md](../../SECURITY.md) — auth and sandbox
