# CWSO Installation & Usage Guide — v3

> **Audience:** users running CWSO on Ubuntu 22 or Linux with VS Code.
> **Goal:** provide a reliable, easy path from clone to working MCP tools.
> **Supersedes for this setup:** `installation-v2.md` and `ide-integration-v1.md`.

## 1. Prerequisites

- Ubuntu 22.04 or another Linux distribution
- Docker Engine with Docker Compose v2
- Python 3.10+
- VS Code

Validate runtime:

```bash
docker --version
docker compose version
python3 --version
```

## 2. Start CWSO

From a shell on the machine where CWSO is running:

```bash
cd <repo-root>
source scripts/cwso-enable-all-features.sh
docker compose -f deploy/docker-compose.yml --profile phase2 --profile phase4 up -d
curl -sS http://127.0.0.1:8080/healthz
```

Expected health output:

```text
OK
```

## 3. Create JWT for MCP

```bash
python3 - <<'PY'
import base64, hashlib, hmac, json, os, time
secret = open('.env.jwt.dev').read().strip().encode()
role = os.environ.get('CWSO_MCP_ROLE', 'worker')
header = {'alg': 'HS256', 'typ': 'JWT'}
claims = {
    'sub': 'vscode-dev',
    'role': role,
    'iss': 'cwso',
    'aud': ['cwso-mcp'],
    'iat': int(time.time()),
    'exp': int(time.time()) + 86400,
}
def b64(b):
    return base64.urlsafe_b64encode(b).rstrip(b'=').decode()
msg = f"{b64(json.dumps(header, separators=(',', ':')).encode())}.{b64(json.dumps(claims, separators=(',', ':')).encode())}"
print(f"{msg}.{b64(hmac.new(secret, msg.encode(), hashlib.sha256).digest())}")
PY
```

Copy the token value.

## 4. Critical env step

Set and **export** the token in the shell that will launch VS Code (not just assign):

```bash
export CWSO_MCP_TOKEN='paste-your-token-here'
```

Quick check:

```bash
[[ -n "$CWSO_MCP_TOKEN" ]] && echo "token exported" || echo "token missing"
```

Open VS Code from the **same shell** so the extension host inherits the env:

```bash
cd <workspace-root>
code .
```

If VS Code was already open before export:

1. Re-run `export CWSO_MCP_TOKEN=...` in the same shell.
2. In VS Code, run command: `Developer: Reload Window`.

## 5. VS Code MCP config

Create/update `.vscode/mcp.json` in your project:

```json
{
  "servers": {
    "cwso": {
      "type": "http",
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "Authorization": "Bearer ${env:CWSO_MCP_TOKEN}",
        "Origin": "http://localhost"
      }
    }
  }
}
```

## 6. Verify MCP connectivity

Before testing VS Code, verify token + endpoint directly:

```bash
curl -sS http://127.0.0.1:8080/mcp \
  -H "Authorization: Bearer $CWSO_MCP_TOKEN" \
  -H "Origin: http://localhost" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

If this returns JSON tool data, server/auth are good.

## 7. Fix for "Dynamic Client Registration not supported"

If VS Code shows:

- "Dynamic Client Registration not supported"
- asks for OAuth "client ID"

that means your MCP request reached CWSO **without a valid Bearer token**, so the client fell back to OAuth discovery flow.

Use this fix sequence:

1. Confirm token is exported: `echo ${CWSO_MCP_TOKEN:+set}` should print `set`.
2. Ensure VS Code is running from the same shell session that exported the token.
3. Reload VS Code window after export.
4. Re-test with `curl` command above.
5. Retry MCP connection.

Important: CWSO does not require you to manually register an OAuth client ID for this local JWT flow.

## 7a. Rate limiting & development

CWSO enforces rate limiting to protect against abuse:

- **Localhost (127.0.0.1, ::1):** Exempt from rate limiting (development convenience)
- **Remote IPs:** 60 requests/minute with burst capacity for connection initialization
- **VS Code MCP client behavior:** May show `[info] Error 429 ...` during initial connection attempts as it probes the endpoint

**For local development (typical scenario):**
- No rate limiting applies when connecting from the same machine
- Connection should succeed on first attempt

**If you see 429 errors:**
1. Confirm you're running CWSO and VS Code on the same machine (localhost)
2. If connecting remotely, wait 60 seconds before retrying
3. The server logs `rate limit exceeded` for remote clients only

## 8. Common issues

| Symptom | Cause | Fix |
|---|---|---|
| `${env:CWSO_MCP_TOKEN}` not resolved | Variable not exported to extension host env | `export ...`, then start `code .` from same shell or reload window |
| OAuth client ID prompt | Missing/invalid `Authorization` header | Fix token export; keep Bearer header in `mcp.json` |
| Error 429 (too many requests) | Rate limiting (remote IPs only) | Localhost is exempt; if still occurs, wait 60s and retry |
| 401 on `/mcp` | JWT mismatch/expired | Mint new token, check `iss`, `aud`, `role` |
| 403 on `/mcp` | Origin not allowed | Ensure header `Origin: http://localhost` and allowlist settings |
| `curl /healthz` OK but MCP fails | Auth path issue, not server uptime | Validate with authenticated `tools/list` curl |
| `[info] ... Error 429 ...` logged | VS Code extension logging level (not server) | This is informational; no action needed if connection succeeds after retry |

## 9. About VS Code MCP extension logging

VS Code's MCP extension logs all connection-related events (including errors) at `[info]` level rather than `[error]`. This is expected behavior:

- `[info] Starting server cwso` — server start attempt
- `[info] Connection state: Error 429 ...` — connection error message, logged at info level
- `[info] Connection state: Running` — successful connection

**Don't be alarmed:** Information-level messages with error content are normal during connection establishment. The client automatically retries on 429 (rate limit) and other transient errors.

**Expected flow:**
1. `Starting server cwso` — attempt 1
2. Possibly one or more `Error 429` messages as client probes endpoint
3. `Connection state: Running` — connection established

This is not a bug; it's how VS Code's extension logs connection lifecycle events.

## 10. Recommended daily workflow

```bash
cd <repo-root>
source scripts/cwso-enable-all-features.sh
docker compose -f deploy/docker-compose.yml --profile phase2 --profile phase4 up -d
export CWSO_MCP_TOKEN='...'
cd <workspace-root>
code .
```

Then use MCP tools from VS Code.

## 11. Related docs

- `docs/user/installation-v2.md` (full feature and rollout reference)
- `docs/user/ide-integration-v2.md` (IDE-focused setup and troubleshooting)
- `README.md`
