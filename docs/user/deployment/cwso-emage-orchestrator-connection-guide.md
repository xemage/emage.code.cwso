# Connect CWSO to emage.code Orchestrator (Tested Guide)

Version: 1.0
Last updated: 2026-08-05
Audience: Local operators running CWSO via Docker Compose who want emage.code orchestration/runtime code to use that stack.

## What this guide does

This guide wires emage.code runtime clients to your running CWSO stack by exporting the two required environment variables:

- `CWSO_BASE_URL`
- `CWSO_JWT_SECRET`

It then validates connectivity with:

1. a direct authenticated MCP smoke check (tools listing + workspace lifecycle), and
2. one live functional test from this repo.

## Preconditions

- You are in the emage.code repository root.
- CWSO is already running from `deploy/docker-compose-t226.yml`.
- The CWSO orchestrator endpoint is reachable on `http://127.0.0.1:8080`.

Quick health check:

```bash
cd ~/Code/emage/emage.code
docker compose -f deploy/docker-compose-t226.yml ps
curl -sS http://127.0.0.1:8080/healthz
```

Expected liveness response is `ok`.

## Step 1: Export connection variables

Set base URL:

```bash
export CWSO_BASE_URL=http://127.0.0.1:8080
```

Set JWT secret (choose one option):

### Option A (matches currently running Docker stack)

```bash
export CWSO_JWT_SECRET="$(docker exec cwso-sia-executor sh -lc 'cat /run/secrets/jwt_secret')"
```

### Option B (from sibling CWSO checkout)

```bash
cd ~/Code/emage/emage.code
source ../CWSO/.env.jwt.dev
export CWSO_JWT_SECRET="$JWT_SECRET"
```

Notes:

- Do not commit secrets.
- Do not print the token in logs.
- `CWSO_JWT_SECRET` must match the secret CWSO was started with, or MCP calls return 401.

## Step 1.5: Mint a bearer token with the helper script

This repo now includes a helper that mints a valid HS256 bearer token for CWSO MCP:

```bash
cd ~/Code/emage/emage.code
python3 scripts/mint-cwso-jwt.py
```

By default it creates a token with:

- `role=orchestrator`
- `sub=vscode-mcp`
- `ttl=3600` seconds

Useful variants:

```bash
# Worker token
python3 scripts/mint-cwso-jwt.py --role worker

# Custom TTL (15 min)
python3 scripts/mint-cwso-jwt.py --ttl 900

# Explicit secret source file
python3 scripts/mint-cwso-jwt.py --secret-file ../CWSO/.env.jwt.dev
```

Secret resolution order used by the script:

1. `--secret`
2. `CWSO_JWT_SECRET` environment variable
3. `--secret-file`
4. default sibling file `../CWSO/.env.jwt.dev`

## Step 2: Run authenticated MCP smoke check

Run this from repo root:

```bash
cd ~/Code/emage/emage.code
python3 - <<'PY'
from implementation.runtime.cwso.mcp_client import CwsoMcpClient

client = CwsoMcpClient.from_env()

worker_tools = client.list_tools(role='worker')
orch_tools = client.list_tools(role='orchestrator')

print('worker_tools_count', len(worker_tools))
print('orchestrator_tools_count', len(orch_tools))

created = client.call_tool(role='worker', name='create_shadow_workspace', arguments={})
workspace_uuid = created.get('workspace_uuid')
if not workspace_uuid:
    raise SystemExit('workspace_uuid missing')

client.call_tool(role='worker', name='drop_shadow_workspace', arguments={'workspace_uuid': workspace_uuid})
print('workspace_lifecycle_ok', True)
PY
```

Success criteria:

- Both tool counts are returned as non-zero.
- Workspace create/drop finishes without exceptions.

## Step 3: Run one repository live test

```bash
cd ~/Code/emage/emage.code
CWSO_LIVE_CONTRACT_TEST=1 \
CWSO_BASE_URL="$CWSO_BASE_URL" \
CWSO_JWT_SECRET="$CWSO_JWT_SECRET" \
python3 -m unittest tests.functional.test_cwso_client_live.TestCwsoClientLiveIntegration.test_tools_list_returns_11_tools -v
```

Expected result:

- `... ok`
- `Ran 1 test`
- `OK`

## Step 4: Use from emage.code scripts/runtime

With env vars exported in the same shell session, emage.code runtime code will use CWSO via `from_env()` defaults:

- `implementation/runtime/cwso/mcp_client.py`
- `implementation/runtime/cwso/client.py`

Example command that now uses your live CWSO wiring:

```bash
cd ~/Code/emage/emage.code
python3 -c "from implementation.runtime.cwso.client import CwsoClient; c=CwsoClient.from_env(); print(len(c.tools_list()['tools']))"
```

## Step 5: Use it in VS Code MCP config prompt

If your `.vscode/mcp.json` CWSO entry is configured with:

- URL `http://127.0.0.1:8080/mcp`
- header `Authorization: Bearer ${input:cwso_jwt_token}`

then paste a freshly minted token from:

```bash
python3 scripts/mint-cwso-jwt.py
```

into the `cwso_jwt_token` prompt when VS Code asks for it.

## Troubleshooting

### 401 Unauthorized / missing bearer token

- `CWSO_JWT_SECRET` is unset or mismatched.
- Re-export it and rerun Step 2.

### 403 Forbidden on specific tool calls

- You are using the wrong role for the tool.
- Retry with `role='worker'` or `role='orchestrator'` as required.

### Connection refused

- CWSO stack is down or endpoint/port differs from `CWSO_BASE_URL`.
- Check `docker compose -f deploy/docker-compose-t226.yml ps` and health endpoints.

## Tested evidence

Validated on 2026-08-05 in this workspace with:

- `docker compose -f deploy/docker-compose-t226.yml ps` showing healthy `cwso-orchestrator` and `cwso-rollout`.
- `curl http://127.0.0.1:8080/healthz` returning `ok`.
- MCP smoke check completing with workspace create/drop success.
- Live test command passing:

```text
test_tools_list_returns_11_tools ... ok
Ran 1 test in 0.049s
OK
```
