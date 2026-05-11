#!/usr/bin/env python3
"""Phase 2 integration test for CWSO.

Spins up the orchestrator + cwso-git-shadow sidecar via docker compose, then
drives the orchestrator's MCP HTTP transport from a worker JWT and exercises
the shadow tools end-to-end.

Validates Phase 2 hypothesis: a Go orchestrator can drive an in-memory
libgit2-backed shadow workspace through a Rust sidecar over UDS, and
tree-sitter AST queries return correct results for Go and Python.

Prereqs: docker, docker compose, python3 (stdlib only).
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
import secrets
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
COMPOSE = REPO / "deploy" / "docker-compose.yml"
BASE_URL = "http://127.0.0.1:8080"

if "CWSO_JWT_SECRET" not in os.environ:
    os.environ["CWSO_JWT_SECRET"] = base64.b64encode(secrets.token_bytes(32)).decode()
SECRET = os.environ["CWSO_JWT_SECRET"].encode()


def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def mint_jwt(role: str) -> str:
    header = {"alg": "HS256", "typ": "JWT"}
    now = int(time.time())
    claims = {"sub": "phase2-test", "role": role, "iat": now, "exp": now + 600}
    h = b64url(json.dumps(header, separators=(",", ":")).encode())
    p = b64url(json.dumps(claims, separators=(",", ":")).encode())
    sig = hmac.new(SECRET, f"{h}.{p}".encode(), hashlib.sha256).digest()
    return f"{h}.{p}.{b64url(sig)}"


def rpc(role: str, method: str, params: dict) -> dict:
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).encode()
    req = urllib.request.Request(
        BASE_URL + "/mcp",
        data=body,
        method="POST",
        headers={
            "Authorization": "Bearer " + mint_jwt(role),
            "Content-Type": "application/json",
            "Origin": "http://localhost",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            return json.loads(r.read())
    except urllib.error.HTTPError as e:
        return {"_http_status": e.code, "_body": e.read().decode("utf-8", "replace")}


def call_tool(role: str, name: str, args: dict) -> dict:
    return rpc(role, "tools/call", {"name": name, "arguments": args})


def tool_text(resp: dict) -> str:
    return resp["result"]["content"][0]["text"]


def assert_ok(cond: bool, label: str, ctx: object = "") -> None:
    if cond:
        print(f"  OK  {label}")
    else:
        print(f"  FAIL {label}: {ctx}")
        sys.exit(1)


def compose(*args: str, check: bool = True, capture: bool = False) -> subprocess.CompletedProcess:
    cmd = ["docker", "compose", "-f", str(COMPOSE), "--profile", "phase2", *args]
    return subprocess.run(cmd, check=check, capture_output=capture, text=True)


def wait_for(predicate, label: str, timeout: float = 30.0) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            if predicate():
                print(f"  OK  {label}")
                return
        except Exception:
            pass
        time.sleep(0.5)
    print(f"  FAIL waiting for {label}")
    compose("logs", check=False)
    sys.exit(1)


def healthz_up() -> bool:
    try:
        with urllib.request.urlopen(BASE_URL + "/healthz", timeout=2) as r:
            return r.status == 200
    except Exception:
        return False


def socket_present() -> bool:
    cp = subprocess.run(
        ["docker", "compose", "-f", str(COMPOSE), "exec", "-T", "orchestrator",
         "test", "-S", "/run/cwso/git-shadow.sock"],
        capture_output=True,
    )
    return cp.returncode == 0


def main() -> None:
    print("--- building images ---")
    compose("build")

    print("--- starting stack ---")
    compose("up", "-d")

    try:
        print("--- waiting for orchestrator /healthz ---")
        wait_for(healthz_up, "/healthz reachable")

        print("--- waiting for git-shadow socket ---")
        wait_for(socket_present, "/run/cwso/git-shadow.sock present")

        print("--- 1. tools/list shows shadow tools ---")
        resp = rpc("orchestrator", "tools/list", {})
        if "result" not in resp:
            print(f"  unexpected response: {resp}")
            sys.exit(1)
        names = {t["name"] for t in resp["result"]["tools"]}
        required = {"create_shadow_workspace", "drop_shadow_workspace",
                    "read_shadow_file", "write_shadow_file",
                    "commit_shadow", "query_ast"}
        assert_ok(required.issubset(names), "shadow tools registered", names)

        print("--- 2. create 3 isolated shadow workspaces ---")
        wsids: list[str] = []
        for i in range(3):
            r = call_tool("worker", "create_shadow_workspace", {})
            if "result" not in r or r["result"].get("isError"):
                print(f"  create failed: {r}")
                sys.exit(1)
            payload = json.loads(tool_text(r))
            ws = payload["workspace_uuid"]
            wsids.append(ws)
            print(f"  ws{i+1}: {ws}")
        assert_ok(len(set(wsids)) == 3, "workspaces are unique")

        print("--- 3. write Go to ws1, Python to ws2; ws3 stays empty ---")
        go_src = (
            'package main\n\nimport "fmt"\n\n'
            'func Greet(name string) string { return fmt.Sprintf("hi %s", name) }\n\n'
            'func main() { fmt.Println(Greet("world")) }\n'
        )
        py_src = (
            'def add(a, b):\n    return a + b\n\n'
            'class Calc:\n    def mul(self, a, b):\n        return a * b\n\n'
            'if __name__ == "__main__":\n    print(add(2, 3))\n'
        )
        r = call_tool("worker", "write_shadow_file",
                      {"workspace_uuid": wsids[0], "path": "main.go", "content": go_src})
        assert_ok(not r["result"].get("isError"), "ws1 main.go written", r)

        r = call_tool("worker", "write_shadow_file",
                      {"workspace_uuid": wsids[1], "path": "calc.py", "content": py_src})
        assert_ok(not r["result"].get("isError"), "ws2 calc.py written", r)

        r = call_tool("worker", "read_shadow_file",
                      {"workspace_uuid": wsids[2], "path": "main.go"})
        assert_ok(r["result"].get("isError") is True,
                  "ws3 isolated (read of unknown file rejected)", r)

        print("--- 4. AST: find Greet definition in Go ---")
        r = call_tool("worker", "query_ast", {
            "workspace_uuid": wsids[0], "path": "main.go",
            "query_type": "find_definition", "target_symbol": "Greet"})
        payload = json.loads(tool_text(r))
        assert_ok(len(payload["hits"]) >= 1, "Greet definition found", payload)

        print("--- 5. AST: detect Go entrypoint (main) ---")
        r = call_tool("worker", "query_ast", {
            "workspace_uuid": wsids[0], "path": "main.go",
            "query_type": "detect_entrypoints", "target_symbol": ""})
        payload = json.loads(tool_text(r))
        assert_ok(len(payload["hits"]) >= 1, "Go main detected", payload)

        print("--- 6. AST: list Python exports ---")
        r = call_tool("worker", "query_ast", {
            "workspace_uuid": wsids[1], "path": "calc.py",
            "query_type": "list_exports", "target_symbol": ""})
        payload = json.loads(tool_text(r))
        names = {h["name"] for h in payload["hits"]}
        assert_ok({"add", "Calc"}.issubset(names), "exports include add+Calc", names)

        print("--- 7. commit ws1 ---")
        r = call_tool("worker", "commit_shadow",
                      {"workspace_uuid": wsids[0], "message": "test commit"})
        commit_oid = json.loads(tool_text(r))["commit_oid"]
        assert_ok(len(commit_oid) == 40, "commit oid is a sha1", commit_oid)

        print("--- 8. permission gate: orchestrator role cannot write ---")
        r = call_tool("orchestrator", "write_shadow_file",
                      {"workspace_uuid": wsids[0], "path": "x.txt", "content": "x"})
        # Permission denied returns JSON-RPC error code -32002.
        assert_ok(r.get("error", {}).get("code") == -32002,
                  "orchestrator role denied write_shadow_file", r)

        print("--- 9. drop all workspaces ---")
        for ws in wsids:
            call_tool("worker", "drop_shadow_workspace", {"workspace_uuid": ws})
        print("  OK  dropped")

        print()
        print("=" * 60)
        print("  PHASE 2 INTEGRATION TEST: PASS")
        print("=" * 60)
    finally:
        print("--- tearing down ---")
        compose("down", "-v", check=False)


if __name__ == "__main__":
    main()
