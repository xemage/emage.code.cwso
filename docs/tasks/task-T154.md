# Task T154 — IDE integration guide (VS Code / Cursor)

- **Status:** in_review
- **Owner:** technical-writer
- **Priority:** P0
- **Depends on:** T142
- **Based on:** `installation-v1.md`, MCP HTTP transport

## Objective

Document how to connect VS Code, Cursor, or other MCP-capable coding tools to a running
CWSO orchestrator for shadow workspaces, AST tools, and rollout APIs.

## Acceptance Criteria

- [x] `docs/user/ide-integration-v1.md` — MCP config, JWT, roles, tool routing
- [x] Cursor and VS Code MCP examples (HTTP + env token)
- [x] Clarify orchestrator MCP vs cwso-rollout model proxy
- [x] Linked from `installation-v1.md` and README

## Notes

Streamable HTTP MCP at `POST /mcp` with HS256 JWT. Model traffic for Polar capture uses
cwso-rollout proxy, not orchestrator `/v1/chat/completions`.
