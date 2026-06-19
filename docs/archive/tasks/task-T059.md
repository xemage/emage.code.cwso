# Task T059 — Add baseline HTTP security headers

- Phase: **4 (Security Fix)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T051 · Blocks: T051
- Status: done

## Objective
Bring transport response headers into baseline security compliance (including HSTS and CSP where applicable).

## Inputs
- T051 security finding references
- [orchestrator/internal/transport/http.go](../../orchestrator/internal/transport/http.go)
- [SECURITY.md](../../SECURITY.md)

## Constraints
- Preserve API behavior and compatibility.
- Apply headers safely for deployment profile.
- Keep tests updated to assert expected headers.

## Expected outputs
- Middleware/header updates to include baseline security headers.
- Updated tests validating headers.

## Acceptance criteria
1. Required baseline headers are present on applicable responses.
2. Existing transport tests pass.
3. No regressions in e2e communication.

## Blocker protocol
If a header requires environment-specific handling, document policy and defaults clearly.

## Completion notes (2026-05-16)
- Updated HTTP security middleware to include baseline headers:
	- `Content-Security-Policy: default-src 'self'`
	- `Strict-Transport-Security: max-age=31536000; includeSubDomains`
	- `X-XSS-Protection: 0`
	- Existing `X-Content-Type-Options` and `X-Frame-Options` preserved
- Preserved `Cache-Control: no-store` behavior for POST requests.
- Added unit test coverage for header presence in transport middleware.

Validation evidence:
- `cd /home/emage/Code/emage/CWSO/orchestrator && go test ./internal/transport`: PASS
