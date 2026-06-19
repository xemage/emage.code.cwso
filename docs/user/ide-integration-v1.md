# CWSO + IDE Integration — v1

> **Based on:** `installation-v1.md`, `develop` @ v0.3.0  
> **Audience:** developers using **Cursor**, **VS Code** (MCP), or Claude Desktop with CWSO

## Overview

CWSO exposes two integration surfaces:

| Surface | URL | Purpose |
|---------|-----|---------|
| **Orchestrator MCP** | `http://127.0.0.1:8080/mcp` | Shadow workspaces, AST, merge, rollout REST, dispatch |
| **Rollout proxy** | `http://127.0.0.1:8787` (default) | OpenAI/Anthropic/Google model traffic capture (Polar) |

Point your **coding tool's MCP client** at the orchestrator. Point **model API** settings
(`OPENAI_BASE_URL`, Cursor model override, etc.) at the rollout proxy when training/capture
is enabled — not at the orchestrator (which returns 501 on `/v1/chat/completions`).

## 1. Start CWSO locally

```bash
make build
docker compose -f deploy/docker-compose.yml --profile phase2 --profile phase4 up -d
curl -sS http://127.0.0.1:8080/healthz
```

Optional: enable all orchestrator feature flags:

```bash
source scripts/cwso-enable-all-features.sh
# Restart orchestrator with exported env (see script output)
```

## 2. Mint a JWT (worker role)

Coding agents need the **worker** role for shadow file tools. Orchestrator-only tools
(`dispatch_concurrent_jobs`, rollout admin) use **orchestrator** role.

```bash
python3 - <<'PY'
import base64, hashlib, hmac, json, os, time
secret = open(".env.jwt.dev").read().strip().encode()
role = os.environ.get("CWSO_MCP_ROLE", "worker")
header = {"alg": "HS256", "typ": "JWT"}
claims = {"sub": "cursor-dev", "role": role, "iss": "cwso", "aud": ["cwso-mcp"],
          "iat": int(time.time()), "exp": int(time.time()) + 86400}
def b64(b): return base64.urlsafe_b64encode(b).rstrip(b"=").decode()
msg = f"{b64(json.dumps(header,separators=(',',':')).encode())}.{b64(json.dumps(claims,separators=(',',':')).encode())}"
print(f"{msg}.{b64(hmac.new(secret, msg.encode(), hashlib.sha256).digest())}")
PY
```

Save as `CWSO_MCP_TOKEN` in your shell or IDE env.

## 3. Cursor configuration

Add to `.cursor/mcp.json` (project) or **Cursor Settings → MCP**:

```json
{
  "mcpServers": {
    "cwso": {
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "Authorization": "Bearer ${env:CWSO_MCP_TOKEN}",
        "Origin": "http://localhost"
      }
    }
  }
}
```

Export the token before launching Cursor:

```bash
export CWSO_MCP_TOKEN="<jwt-from-step-2>"
cursor .
```

### Cursor + Polar model capture

In Cursor settings (or harness env), route model calls through cwso-rollout when the
sidecar is running:

```bash
export OPENAI_BASE_URL=http://127.0.0.1:8787
# Anthropic-compatible clients: ANTHROPIC_BASE_URL=http://127.0.0.1:8787
```

Use `scripts/shell-command-harness.sh` as a minimal reference for proxy traffic.

## 4. VS Code configuration

Use an MCP extension that supports **HTTP/SSE** servers (e.g. MCP client extensions
following the Model Context Protocol streamable HTTP transport).

Example `.vscode/mcp.json`:

```json
{
  "servers": {
    "cwso": {
      "type": "http",
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "Authorization": "Bearer ${input:cwso-token}",
        "Origin": "http://localhost"
      }
    }
  }
}
```

Provide the JWT when prompted, or set `CWSO_MCP_TOKEN` in your workspace `.env` and
reference it per extension docs.

## 5. Typical agent workflow

1. **tools/list** — confirm shadow tools (`create_shadow_workspace`, `write_shadow_file`, `query_ast`, …).
2. **create_shadow_workspace** — isolated git shadow for the task.
3. **write_shadow_file** / **query_ast** — edit and analyze code in the shadow.
4. **commit_shadow** — persist agent changes.
5. Optional: **merge_concurrent_results** (Phase 4) after parallel workers.
6. Optional: **POST /rollout/task/submit** when `CWSO_ROLLOUT_API_ENABLED=true`.

## 6. Troubleshooting

| Issue | Fix |
|-------|-----|
| MCP connection refused | `docker compose ps`; check port 8080 |
| 401 Unauthorized | Refresh JWT; check `iss`/`aud`/`role` |
| 403 Origin | Add your IDE origin to `CWSO_ALLOWED_ORIGINS` |
| Tools missing | Enable sidecars (phase2 profile); check logs |
| Model calls not captured | Use rollout proxy URL, enable `CWSO_ROLLOUT_PROXY_ENABLED` |

## 7. Further reading

- [installation-v1.md](installation-v1.md)
- [rollout-architecture-v1.md](../artifacts/rollout-architecture-v1.md)
