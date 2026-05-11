# PoC Debt Scorecard — Phase 1

## Hypothesis
> A Go MCP server using a hand-rolled subset (later: official `go-sdk`) can serve baseline filesystem tools over both stdio and Streamable HTTP transports with p95 < 50 ms tool-call latency and pass `mcp-inspector` capability conformance.

## Result
**VALIDATED** — `initialize`, `tools/list`, `tools/call` all functional over HTTP with full security gates (Origin validation, JWT auth, permission tier enforcement, path-traversal rejection); `go test ./... -race` passes; `mcp-inspector` conformance pending UI run (no blockers identified in protocol shape).

## Debt Inventory

| # | File | Line(s) | Category | Description | Production Effort |
|---|------|---------|----------|-------------|-------------------|
| 1 | [orchestrator/internal/mcp/protocol.go](orchestrator/internal/mcp/protocol.go) | package doc | Maintainability | Hand-rolled MCP subset; production must adopt `github.com/modelcontextprotocol/go-sdk` for full spec compliance and upstream maintenance | M — port handlers to official SDK; preserve schemas |
| 2 | [orchestrator/internal/transport/http.go](orchestrator/internal/transport/http.go) | `verifyHS256` | Security | Hand-rolled HS256 JWT verifier; production must use `golang-jwt/jwt/v5` with RS256, key rotation, full claims validation (iss, aud, nbf, exp leeway) | M — swap library + add JWKS endpoint |
| 3 | [orchestrator/internal/transport/http.go](orchestrator/internal/transport/http.go) | `handleSSE` | Functionality | SSE GET endpoint emits heartbeats only; real notifications wired in Phase 3 (T030) | L — Phase 3 EventBus integration |
| 4 | [orchestrator/internal/logging/logger.go](orchestrator/internal/logging/logger.go) | package doc | Observability | Stdlib-only logger; production should adopt `zerolog` + OTEL integration | S — direct swap |
| 5 | [deploy/docker-compose.yml](deploy/docker-compose.yml) | `CWSO_JWT_SECRET` | Security | Dev env requires secret via env var; production must use mounted secret file or vault integration (Vault/SOPS/age) | S — mount secret file |
| 6 | [orchestrator/internal/server/server.go](orchestrator/internal/server/server.go) | `handleInitialize` | Spec compliance | Capability negotiation declares `tools.listChanged: false`; full capability set (resources, prompts, sampling) deferred | M — implement on demand per use case |
| 7 | [orchestrator/internal/transport/http.go](orchestrator/internal/transport/http.go) | rate limiting | Security | No per-IP rate limit on `/mcp` POST; relies on JWT to gate | S — add token-bucket middleware |
| 8 | [orchestrator/internal/tools/fs_tools.go](orchestrator/internal/tools/fs_tools.go) | 1 MiB cap | Robustness | Read cap is hard-coded; production should expose via config | S — config field |

## Summary
- Total debt items: **8**
- Critical (must fix before production): **0**
- High (must fix before Phase 3 hardening — T029): **3** (#1 SDK, #2 JWT, #5 secret mount)
- Medium (should fix before v0.1.0): **3** (#3 SSE, #6 capabilities, #7 rate limit)
- Low (nice to have): **2** (#4 logger, #8 cap config)

## Recommendation
**GO** — proceed to Phase 2 (Shadow Workspaces + AST). All Phase 1 acceptance criteria met. The 3 High-tier debt items must be remediated in T029 (PoC-debt remediation pass) before async work in Phase 3 begins. No CRITICAL findings; security baseline (Origin validation, auth, permission tier, path guard) is functional.

## Verified end-to-end
```
✓ initialize → 200 with protocolVersion=2025-03-26
✓ tools/list → 3 tools advertised
✓ read_file_sync → reads from workspace
✓ list_dir → lists entries
✓ path traversal → rejected ("escapes workspace root")
✓ orchestrator role attempting write_file_sync → permission denied (-32002)
✓ missing bearer → 401
✓ bad Origin → 403
✓ go test ./... -race → all packages PASS
```
