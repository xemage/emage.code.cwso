# Task T060 — Enforce POST /mcp Content-Type

- Phase: **4 (Security Fix)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T051 · Blocks: T051
- Status: done

## Objective
Enforce strict request media-type validation for `POST /mcp` and reject unsupported content types.

## Inputs
- T051 security finding references
- [orchestrator/internal/transport/http.go](../../orchestrator/internal/transport/http.go)

## Constraints
- Require JSON payload media type for MCP POST endpoint.
- Return correct HTTP error (`415`) for invalid media types.
- Add/adjust tests for request validation behavior.

## Expected outputs
- Content-Type gate in transport handler.
- Tests for accepted/rejected content types.

## Acceptance criteria
1. Non-JSON POST requests to `/mcp` are rejected with `415`.
2. Valid JSON MCP requests still succeed.
3. Existing authentication/rate-limit behavior remains intact.

## Blocker protocol
If clients rely on non-standard media type variants, document compatibility handling and migration path.

## Completion notes (2026-05-16)
- Added strict media-type validation in `handlePOST` requiring `application/json` (including parameters such as charset).
- Requests with unsupported or invalid `Content-Type` now return `415 Unsupported Media Type` before body processing.
- Added dedicated transport test to verify non-JSON request rejection.

Validation evidence:
- `cd /home/emage/Code/emage/CWSO/orchestrator && go test ./internal/transport`: PASS
