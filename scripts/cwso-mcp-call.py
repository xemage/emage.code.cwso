#!/usr/bin/env python3
"""Minimal MCP tools/call client used by scripts/cwso-projection-e2e.sh (C024).

This is a thin, single-purpose CLI wrapper: `mcp_call.py <tool_name> <json_args>`
POSTs one `tools/call` JSON-RPC request to $CWSO_BASE_URL/mcp with a Bearer
token from $CWSO_TOKEN, and either:
  - prints the tool's text result to stdout and exits 0, or
  - prints the full JSON-RPC response (or transport error) to stderr and
    exits 1 -- never swallowed, never weakened.

Deliberately not a copy of scripts/cwso-smoke-test.sh's embedded stage
logic: that script hardcodes its own 7-stage MCP call sequence inline;
this is a small, generic, reusable single-call primitive so
cwso-projection-e2e.sh can interleave MCP calls with `docker exec`
filesystem operations stage-by-stage, which the smoke test's monolithic
heredoc block does not need to do.
"""
import json
import os
import sys
import urllib.error
import urllib.request


def rpc(base_url: str, token: str, method: str, params: dict) -> dict:
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).encode()
    req = urllib.request.Request(
        base_url + "/mcp",
        data=body,
        method="POST",
        headers={
            "Authorization": "Bearer " + token,
            "Content-Type": "application/json",
            "Origin": "http://localhost",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            return json.loads(r.read())
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "replace")
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return {"_http_status": e.code, "_body": raw}
    except urllib.error.URLError as e:
        return {"_transport_error": str(e)}


def main() -> int:
    if len(sys.argv) < 2:
        sys.stderr.write("usage: mcp_call.py <tool_name> [json_args]\n")
        return 2
    name = sys.argv[1]
    args = json.loads(sys.argv[2]) if len(sys.argv) > 2 else {}

    base_url = os.environ["CWSO_BASE_URL"]
    token = os.environ["CWSO_TOKEN"]

    resp = rpc(base_url, token, "tools/call", {"name": name, "arguments": args})
    if "result" not in resp or resp["result"].get("isError"):
        sys.stderr.write(json.dumps(resp, indent=2, default=str) + "\n")
        return 1
    text = resp["result"]["content"][0]["text"]
    sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
