# CWSO Installation & Usage Guide — v1

> **Based on:** `develop` @ `v0.3.0` GA + post-GA docs (T154, T155)  
> **Audience:** operators and integrators getting CWSO running locally or in CI

## 1. Prerequisites

| Requirement | Version / notes |
|-------------|-----------------|
| Docker Engine | 24+ with Compose v2 |
| Git | clone with submodules if used |
| Python 3 | 3.10+ for `scripts/phase2-integration.py` (stdlib only) |
| Optional: Go 1.25, Rust stable | only for native dev; `make test` runs in containers |

Local toolchains are **optional** — the supported path is Docker Compose.

## 2. Quick start (Phase 2 core)

```bash
git clone https://gitlab.com/em-age/emage.code.cwso.git
cd emage.code.cwso

# Build orchestrator + git-shadow images
make build

# Start orchestrator + git-shadow sidecar
docker compose -f deploy/docker-compose.yml --profile phase2 up -d

# Health check
curl -sS http://127.0.0.1:8080/healthz

# End-to-end integration test
python3 scripts/phase2-integration.py
```

Expected result: `PHASE 2 INTEGRATION TEST: PASS`.

## 3. JWT authentication

HTTP transport requires HS256 JWT. For local Compose, the dev secret is in `.env.jwt.dev`
(mounted as a Docker secret) or injected via `CWSO_JWT_SECRET` in CI.

Claims:

| Claim | Value |
|-------|-------|
| `iss` | `cwso` |
| `aud` | `cwso-mcp` |
| `role` | `orchestrator` or `worker` |

Mint a token (example with Python):

```python
import base64, hashlib, hmac, json, time, secrets

secret = open(".env.jwt.dev").read().strip().encode()
header = {"alg": "HS256", "typ": "JWT"}
claims = {"sub": "dev", "role": "worker", "iss": "cwso", "aud": ["cwso-mcp"],
          "iat": int(time.time()), "exp": int(time.time()) + 3600}
def b64(b): return base64.urlsafe_b64encode(b).rstrip(b"=").decode()
msg = f"{b64(json.dumps(header,separators=(',',':')).encode())}.{b64(json.dumps(claims,separators=(',',':')).encode())}"
sig = b64(hmac.new(secret, msg.encode(), hashlib.sha256).digest())
print(f"{msg}.{sig}")
```

## 4. Calling MCP over HTTP

```bash
export TOKEN="<jwt>"
curl -sS http://127.0.0.1:8080/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Origin: http://localhost" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

Shadow workspace tools (`create_shadow_workspace`, `write_shadow_file`, `query_ast`, …) require
the **worker** role. Orchestrator-only tools include `dispatch_concurrent_jobs` and rollout APIs.

## 5. Phase 4 stack (merge engine)

```bash
docker compose -f deploy/docker-compose.yml \
  --profile phase2 --profile phase4 up -d

CWSO_COMPOSE_PROFILES=phase2,phase4 CWSO_PHASE4_MATRIX=1 \
  python3 scripts/phase2-integration.py
```

## 6. Next-Gen features (default-off)

All Phase 6–9 capabilities ship behind environment flags. Enable only what you need.

**Quick enable (local PoC):** `source scripts/cwso-enable-all-features.sh` loads
`deploy/cwso-all-features.env` with all orchestrator flags on (see
`scripts/cwso-enable-all-features.env.example`).

**IDE integration:** see [ide-integration-v1.md](ide-integration-v1.md) for Cursor / VS Code MCP
setup alongside CWSO.

### Hardware dispatch (Phase 6)

```bash
export CWSO_HAL_SOCKET=/run/cwso/hal.sock   # when cwso-hal sidecar is running
export CWSO_HHD_POLICY_ENGINE_V2_ENABLED=true
export CWSO_HAL_GPU_BASE_URL=http://localhost:8000/v1
export CWSO_HAL_GPU_MODEL=your-model
```

### Sparse micro-agents (Phase 7)

```bash
export CWSO_SPARSE_AGENTS_ENABLED=true
export CWSO_SPARSE_SOCKET=/run/cwso/sparse.sock
export CWSO_SPARSE_SLICE_MANIFEST=/path/to/manifest.json
```

### AST spike pipeline (Phase 7)

```bash
export CWSO_AST_SPIKE_MONITOR_ENABLED=true
export CWSO_AST_SPIKE_RESOURCES_ENABLED=true
```

### Rollout / Polar substrate (Phase 9)

```bash
export CWSO_ROLLOUT_SOCKET=/run/cwso/rollout.sock
export CWSO_ROLLOUT_PROXY_ENABLED=true
export CWSO_ROLLOUT_API_ENABLED=true
export CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED=true
export CWSO_ROLLOUT_REWARD_ENABLED=true
export CWSO_ROLLOUT_KV_PREFIX_ROUTER_ENABLED=true
```

Trainer REST endpoints (when `CWSO_ROLLOUT_API_ENABLED=true`):

| Method | Path |
|--------|------|
| POST | `/rollout/task/submit` |
| GET | `/rollout/task/{task_id}` |
| GET | `/rollout/status` |
| POST | `/callbacks/session_result` |

Point external coding harnesses at the **cwso-rollout proxy** URL for model traffic capture;
the orchestrator `/v1/chat/completions` stub returns 501 — use the sidecar proxy.

### Polar harness adapters (T144)

CWSO ships a harness adapter registry and Docker runtime launcher so external coding
harnesses (Codex, Claude Code, Qwen Code, shell-command) run unchanged with `base_url`
pointed at cwso-rollout.

Default proxy URL when the sidecar is enabled: `http://127.0.0.1:8787` (`CWSO_ROLLOUT_HTTP_BIND`).

| Harness ID | Model env var | Notes |
|------------|---------------|-------|
| `shell-command` | `OPENAI_BASE_URL` | Reference PoC — `scripts/shell-command-harness.sh` |
| `codex` | `OPENAI_BASE_URL` | Stub launch command (wire real CLI image in production) |
| `claude_code` | `ANTHROPIC_BASE_URL` | Stub launch command |
| `qwen_code` | `OPENAI_BASE_URL` | Stub launch command |

**Runtime interface** (Polar §3.2.2): `start`, `stop`, `exec`, `upload`, `download` — implemented
via Docker HTTP API in `orchestrator/internal/harness` (`DockerRuntime`). Use
`CWSO_DOCKER_HOST` (same socket as sandbox runners).

Example: run the reference shell-command harness against a live proxy:

```bash
export CWSO_ROLLOUT_PROXY_ENABLED=true
export CWSO_ROLLOUT_UPSTREAM_URL=http://your-inference:8000
export OPENAI_BASE_URL=http://127.0.0.1:8787

# From repo root — posts one chat completion through the proxy for capture
./scripts/shell-command-harness.sh
```

Go launcher API (trainer/orchestrator integration):

```go
launcher, _ := harness.NewLauncher(harness.LauncherConfig{
    Registry: harness.DefaultRegistry(),
    Runtime:  dockerRuntime, // harness.NewDockerRuntime(...)
    ProxyURL: "http://127.0.0.1:8787",
})
result, _ := launcher.RunOnce(ctx, harness.LaunchRequest{
    HarnessID: harness.IDShellCommand,
    SessionID: "rollout-session-1",
    Prompt:    "fix the failing test",
})
```

Captured completions are drained from the sidecar UDS (`CWSO_ROLLOUT_SOCKET`) via
`rollout.Client.DrainCapture` and assembled into trajectories (T133).

## 7. CI parity

GitLab CI uses socket-mounted Docker runners (`.docker-socket` template). E2E jobs run
`scripts/phase2-integration.py` with `deploy/docker-compose.ci.yml` overrides.
See `.gitlab-ci.yml` stages: lint → build → test → audit → e2e.

## 8. Troubleshooting

| Symptom | Check |
|---------|-------|
| `401` on `/mcp` | JWT secret mismatch; verify `iss`/`aud`/`role` |
| `403` Origin | Add client origin to `CWSO_ALLOWED_ORIGINS` |
| Shadow tool errors | `docker compose logs cwso-git-shadow`; UDS at `/run/cwso/git-shadow.sock` |
| E2E connection refused | Ensure port 8080 free; run `compose down -v` between local runs |
| Rollout API 404 | `CWSO_ROLLOUT_API_ENABLED=true` and restart orchestrator |

## 9. Further reading

- [README.md](../../README.md) — project overview
- [ide-integration-v1.md](ide-integration-v1.md) — Cursor / VS Code + CWSO MCP
- [release-v0.3.0.md](../artifacts/release-v0.3.0.md) — GA scope and flags
- [rollout-architecture-v1.md](../artifacts/rollout-architecture-v1.md) — Polar substrate design
- [SECURITY.md](../../SECURITY.md) — auth and sandbox guidance
