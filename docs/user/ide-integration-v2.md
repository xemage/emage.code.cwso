# CWSO + IDE Integration — v2 (Cursor / VS Code, cross-platform)

> **Focus:** practical MCP setup with explicit env handling and OAuth fallback troubleshooting.

## 1. MCP endpoint

CWSO MCP server:

- URL: `http://127.0.0.1:8080/mcp`
- Auth: `Authorization: Bearer <HS256 JWT>`
- Required claim set: `iss=cwso`, `aud=cwso-mcp`, `role=worker|orchestrator`

## 2. VS Code setup

### Step A: export token in the same shell you will launch VS Code from

```bash
export CWSO_MCP_TOKEN='your-jwt'
```

### Step B: start VS Code from the same shell

```bash
code .
```

This ensures `${env:CWSO_MCP_TOKEN}` resolves in MCP configuration.

If you are using WSL2, launch VS Code from the WSL shell. If you are on native Linux, launch it from that terminal. On Windows, use the shell/session that provides the token to the editor process.

### Step C: configure `.vscode/mcp.json`

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

## 3. Why VS Code asks for OAuth client ID

If you see:

- "Dynamic Client Registration not supported"
- prompt for manual client ID

the MCP client did not send a usable Bearer token and entered OAuth fallback flow.

For local CWSO JWT usage, do not register an OAuth client ID. Fix token/env propagation instead.

This is true on Windows, Linux, and WSL2.

## 4. Fast diagnostic

Run this in WSL shell:

```bash
[[ -n "$CWSO_MCP_TOKEN" ]] && echo "token set" || echo "token missing"
```

Run authenticated MCP call:

```bash
curl -sS http://127.0.0.1:8080/mcp \
  -H "Authorization: Bearer $CWSO_MCP_TOKEN" \
  -H "Origin: http://localhost" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

If this succeeds, reload VS Code window and reconnect MCP.

## 5. Cursor config

`.cursor/mcp.json`:

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

## 6. Troubleshooting matrix

| Problem | Likely cause | Action |
|---|---|---|
| Server starts but tools unavailable | Missing token in client process | Export token and restart IDE from same shell |
| OAuth client ID prompt | Missing/invalid auth header | Keep Bearer header, fix env resolution |
| 401 Unauthorized | Expired token or claim mismatch | Mint fresh token; verify `iss`, `aud`, `role` |
| 403 Forbidden | Origin validation | Use `Origin: http://localhost`; update allowlist if customized |

## 7. Reference

- `docs/user/installation-v3.md`
- `docs/user/installation-v2.md`
- `README.md`
