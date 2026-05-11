# Task T029 — PoC-debt remediation pass

- Phase: **3 (Production hardening, gate-task before async work)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T028 · Blocks: T030 (and all subsequent Phase 3 tasks)
- Status: pending

## Objective
Eliminate the High-tier debt items recorded in [POC-DEBT-SCORECARD-phase1.md](../../POC-DEBT-SCORECARD-phase1.md) and Phase 2's scorecard before any async/SSE/event-bus work begins. After this task, no `POC-DEBT` tag with effort `M` or higher may remain in code paths touched by Phase 3.

## Inputs
- [POC-DEBT-SCORECARD-phase1.md](../../POC-DEBT-SCORECARD-phase1.md)
- Phase 2 scorecard (produced in T028)
- [security-baseline-v1.md](../artifacts/security-baseline-v1.md)

## Constraints
- Production guidelines apply (full validation gates resume from here).
- No breaking changes to public MCP tool schemas.
- All replaced subsystems must keep or improve test coverage.
- Each remediation lands as its own commit using Conventional Commits (`refactor:`, `fix:`, `chore:`).

## Mandatory remediations (Phase 1 debt)
1. **#1 MCP SDK** — Replace hand-rolled MCP subset with `github.com/modelcontextprotocol/go-sdk`. Preserve tool semantics and security gates. Update [orchestrator/internal/mcp/](../../orchestrator/internal/mcp/) and the server handler.
2. **#2 JWT verifier** — Replace `verifyHS256` with `github.com/golang-jwt/jwt/v5`. Support both HS256 (dev) and RS256 (prod) selected by `CWSO_JWT_ALG`. Add `iss` and `aud` validation, `nbf` + `exp` leeway (60s).
3. **#5 Secret mounting** — Update [deploy/docker-compose.yml](../../deploy/docker-compose.yml) to load `CWSO_JWT_SECRET` from a mounted file (`/run/secrets/jwt`) using compose `secrets:`. Document key rotation in [SECURITY.md](../../SECURITY.md).
4. **Phase 2 High items** — same treatment for any HIGH-tier debt found in T028.

## Recommended (Medium)
- **#7 Rate limiting** — token-bucket per IP on `/mcp` POST (`golang.org/x/time/rate`); 60 req/min default, configurable.
- **#3 SSE prep** — refactor `handleSSE` to accept a channel injected by the (still-stubbed) EventBus; clears the path for T030.
- **#4 Logger** — adopt `github.com/rs/zerolog`; keep `logging.Logger` API surface stable so tests don't churn.

## Expected outputs
- Updated source under `orchestrator/internal/mcp/`, `internal/transport/`, `internal/logging/`
- Updated `deploy/docker-compose.yml` with `secrets:` block
- All Phase 1 tests still PASS (`go test ./... -race`)
- `mcp-inspector` conformance run produces no protocol violations
- POC-DEBT tags remaining are only those marked Low

## Acceptance criteria
1. `grep -r "POC-DEBT" orchestrator/ | grep -vi "low"` returns **zero** results in code paths touched by Phase 3.
2. JWT validation rejects: expired, wrong-issuer, wrong-audience, wrong-algorithm tokens (new tests).
3. Compose stack starts using a file-mounted secret (no `CWSO_JWT_SECRET` in env vars).
4. Rate limiter returns HTTP 429 after burst threshold (new test).
5. `mcp-inspector` reports server as compliant for `2025-03-26`.
6. Tech Lead review (separate task at T036) PASS.

## Blocker protocol
Same as T020.
