# Task T202 — Fix dashboard rate-limit/logging gap (F-C061-01)

**ID:** T202
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** —
**Created:** 2026-08-29
**Completed:** —
**Based on:** `docs/artifacts/security-v1.0-audit-v1.md` finding F-C061-01 (SECURITY:MEDIUM),
found by C061's v1.0.0 security audit, independently reproduced live by Tech Lead review
(MR !199). Logged as a fast-follow task per security-guidelines.md's requirement that
MEDIUM findings carry a documented remediation plan — this task IS that plan, tracked and
dispatchable, not just a debt-register note, since it requires an actual code fix.

## Objective

`rateLimitMiddleware` (`orchestrator/internal/transport/http.go:757-790`) only throttles
`POST` requests — this exemption was originally scoped to keep long-lived `GET /mcp` SSE
streams from being rate-limited (`// Only rate-limit /mcp POST (not GET SSE)`, tracked as
debt item D6). The operator dashboard (`ADR-011`, added later) is mounted through the same
shared `mw` middleware chain (`mux.Handle("/dashboard", mw(o.dashboard))`,
`mux.Handle("/dashboard/status", mw(o.dashboard))`) and both dashboard routes are accessed
via `GET` per their own documented API contract — so they silently inherited the
SSE-specific POST-only exemption. `ADR-011`'s own "Security trade-offs" section explicitly
claims "Dashboard routes inherit `rateLimitMiddleware`," which is misleading: they inherit
the middleware wrapper, not its throttling effect.

Live-verified during the audit (independently reproduced by Tech Lead review, exact
match): 150 unauthenticated `GET /dashboard/status` requests with wrong bearer tokens →
all `401`, zero `429`. Contrast: `POST /mcp` under the same limiter → `401`×10 then
`429`×5 starting at request #11 (matching the documented `burst=10` token bucket exactly).
Combined with a static example dashboard token in documentation and zero auth-failure
logging for the dashboard path specifically (`ClientMetrics.RecordAuthFailure` only
increments a counter, never logs — contrast `/mcp`'s `log.Warn().Err(err).Msg("jwt
rejected")`), this is an unthrottled, unlogged brute-force surface against the dashboard
token. Impact is bounded to information disclosure (dashboard JSON confirmed secret-free
live during the audit) — not data exposure or privilege escalation — but it is a real gap.

## Inputs

- `docs/artifacts/security-v1.0-audit-v1.md` (F-C061-01's full finding + live-verification
  transcripts)
- `orchestrator/internal/transport/http.go` (`rateLimitMiddleware`, dashboard route wiring
  at lines 230-233)
- `orchestrator/internal/dashboard/dashboard.go` (`Handler.auth`, lines 259-276)
- `docs/decisions/ADR-011-operator-dashboard.md` ("Security trade-offs" section — needs a
  correction once this lands, since its "inherits rate limiting" claim will finally become
  literally true)

## Rails (read before starting)

### You MUST
- Make dashboard `GET` requests genuinely subject to rate limiting — either remove the
  blanket `POST`-only exemption in `rateLimitMiddleware` and replace it with a
  route-aware exemption (only `/mcp` GET/SSE bypasses; `/dashboard`/`/dashboard/status`
  GET does not), or add a dedicated rate-limit check specifically on the dashboard's own
  auth path. Your call on the cleanest implementation, but the SSE exemption's original
  purpose (don't throttle long-lived `/mcp` SSE connections) must not regress.
- Add logging for dashboard auth failures, matching `/mcp`'s existing pattern (a
  structured `log.Warn()` with the failing IP, not the attempted token), so an operator
  has a log-based signal of a brute-force attempt in progress, not just an atomic counter
- Add a test proving dashboard `GET` requests are genuinely throttled after N failed
  attempts (mirroring the audit's own live-verification methodology: repeated wrong-token
  requests, assert a `429` appears within the documented burst window)
- Add a regression test proving `/mcp`'s SSE GET exemption is NOT regressed by this fix
- Correct `ADR-011`'s "Security trade-offs" section to accurately describe the fix (its
  current claim was aspirational, not true, until this task)
- Add a CHANGELOG entry

### You MUST NOT
- Regress the `/mcp` GET/SSE exemption — long-lived SSE connections must still bypass
  rate limiting, only the dashboard's GET traffic changes
- Log the actual attempted bearer token value (only the failing IP and a generic "auth
  failed" message, matching this project's logging discipline — see
  `.claude/rules/security-guidelines.md`'s Logging section: "DO NOT log: passwords,
  tokens, session IDs")
- Touch the dashboard's actual authorization logic (`Handler.auth`) beyond what's needed
  to add the failure-logging call — this is a rate-limiting and logging fix, not a
  broader dashboard-auth rewrite

## File ownership

- **May create/modify:** `orchestrator/internal/transport/http.go`,
  `orchestrator/internal/transport/http_test.go` (or equivalent test file),
  `orchestrator/internal/dashboard/dashboard.go` (auth-failure logging call only),
  `docs/decisions/ADR-011-operator-dashboard.md` (Security trade-offs section only),
  `CHANGELOG.md`
- **Must NOT touch:** other services, `docs/artifacts/security-v1.0-audit-v1.md` (the
  audit artifact itself — immutable per this project's artifact-versioning convention),
  `docs/DEBT-REGISTER.md`

## Steps (execute in order)

1. Read F-C061-01's full finding and live-verification transcripts in the audit artifact.
2. Read `rateLimitMiddleware` and the dashboard route wiring to confirm the current
   mechanism.
3. Implement the fix (route-aware rate limiting for the dashboard).
4. Add dashboard auth-failure logging.
5. Tests: dashboard throttling, `/mcp` SSE exemption non-regression.
6. Correct ADR-011's Security trade-offs section.
7. CHANGELOG.

## Expected outputs

- Dashboard `GET` requests genuinely rate-limited
- Dashboard auth failures logged (IP + generic message, never the token)
- Regression tests for both the new throttling and the un-regressed SSE exemption
- Corrected ADR-011 section
- CHANGELOG entry

## Acceptance criteria

1. A live (or integration-test) repeat of the audit's methodology — repeated wrong-token
   `GET /dashboard/status` requests — shows a `429` within the documented burst window,
   not 150 unthrottled `401`s
2. `/mcp` GET/SSE requests remain unthrottled (regression test passes)
3. Dashboard auth failures produce a log line (IP + generic message only, no token value)
4. `go test ./internal/transport/... ./internal/dashboard/...` passes

## Verification commands

```bash
cd orchestrator
go test ./internal/transport/... ./internal/dashboard/... -count=1 -race
go vet ./...
grep -n "token" internal/dashboard/dashboard.go | grep -i "log\." # must show no token-value logging
```

## Git rails

- Branch: `agent/backend-developer/T202` from `develop`
- Commit: `fix(security): rate-limit and log dashboard auth failures (F-C061-01)`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries. If fixing
this exposes a genuine ambiguity in how `/mcp`'s SSE exemption should interact with a
route-aware rate limiter, report `unclear_requirements`/`minor` and propose your
best-judgment design rather than guessing silently.

## Execution notes

<filled during execution>
