# T029 Implementation Plan — Remaining Work

## Status: In Progress
- **Feature branch:** feature/T029-poc-debt-remediation
- **Commits so far:** 3 (JWT refactor, secret mounting, test suite)

## Completed Remediations
1. ✅ JWT library upgrade (golang-jwt/jwt/v5) with HS256+RS256 support
2. ✅ Secret mounting (docker-compose secrets)
3. ✅ Comprehensive JWT validation tests

## Remaining Mandatory Remediations

### #1 MCP SDK Replacement (Highest Priority)
**Scope:** Replace hand-rolled MCP subset in orchestrator/internal/mcp/ with official go-sdk.

**Files affected:**
- orchestrator/internal/mcp/protocol.go — use official SDK types
- orchestrator/internal/server/server.go — update initialize/tools handlers
- orchestrator/internal/tools/*.go — update tool schemas to match SDK

**Approach:**
1. Add `github.com/modelcontextprotocol/go-sdk` to go.mod
2. Refactor protocol.go to export SDK types + keep internal compat layer
3. Update handleInitialize to use official HandshakeResponse
4. Update handleTools to use official ToolCall/Result types
5. Preserve all security gates (role validation, path guards)
6. Run tests: `go test ./... -race`

**Estimated effort:** M (3-4 commits)

### P2-3 Rust+TypeScript Grammars (Phase 2 High-tier)
**Scope:** Add tree-sitter-rust and tree-sitter-typescript to git-shadow.

**Files affected:**
- services/cwso-git-shadow/Cargo.toml — add crate dependencies
- services/cwso-git-shadow/src/ast.rs — extend Lang enum
- services/cwso-git-shadow/src/repo.rs — update parser initialization

**Approach:**
1. Add tree-sitter-rust and tree-sitter-typescript to Cargo.toml
2. Extend Lang enum: pub enum Lang { Go, Python, Rust, TypeScript, ... }
3. Update load_parser() to instantiate Rust+TS parsers
4. Test end-to-end with new grammar: `cargo test --release`

**Estimated effort:** S (1-2 commits)

## Recommended Medium-tier (Optional but valuable)

### #7 Rate Limiting
**Scope:** Token-bucket per IP on /mcp POST, 60 req/min default.

**Approach:**
1. Add `golang.org/x/time/rate` to go.mod
2. Create rateLimiter middleware in transport/http.go
3. Extract IP from r.RemoteAddr, apply per-IP limit
4. Return HTTP 429 Too Many Requests
5. Add tests for burst + limit enforcement

**Effort:** S (1 commit)

### #3 SSE Channel Injection Prep
**Scope:** Refactor handleSSE to accept injected notification channel.

**Approach:**
1. Add channel field to serverContext (or create EventBus interface)
2. Update handleSSE signature to accept <-chan Notification
3. In Phase 1, pass nil channel (keep heartbeat-only)
4. In Phase 3 (T030), inject real EventBus channel

**Effort:** S (1 commit, minimal logic change)

## Testing & Validation

Before merging, ensure:
```bash
cd orchestrator
go test ./... -race          # All tests pass
go build -o /tmp/cwso ./cmd/cwso-orchestrator  # Compiles
```

Integration test:
```bash
docker compose -f deploy/docker-compose.yml --profile phase2 up
# Should start successfully with new JWT + secret mounting
```

## Merge & Gate

After T029 is complete:
1. Create MR from feature/T029-poc-debt-remediation → develop
2. CI pipeline must pass (lint, build, test, e2e)
3. Delegate to Tech Lead for review gate (T036)

## Next Phases

- **T030** depends on T029 complete: Streamable HTTP SSE (builds on #3 SSE prep)
- **T031** depends on T030: Async job runner pool
- **Phase 3 critical path:** T030 → T031 → T032/T033 → T035 (e2e tests) → T036 (Tech Lead gate)

---
*Last updated: 2026-05-12, during T029 remediation phase*
