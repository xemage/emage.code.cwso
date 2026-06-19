# Task T061 — Clarify/implement RS256 support path

- Phase: **4 (Security Follow-up)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T051 · Blocks: T051 (documentation/clarity condition)
- Status: done

## Objective
Resolve RS256 configuration ambiguity by either implementing RS256 verification path or constraining configuration/documentation to validated HS256-only operation.

## Inputs
- T051 security finding references
- [orchestrator/internal/transport/http.go](../../orchestrator/internal/transport/http.go)
- [deploy/docker-compose.yml](../../deploy/docker-compose.yml)
- [SECURITY.md](../../SECURITY.md)

## Constraints
- Avoid misleading configuration options.
- Keep cryptographic behavior explicit and test-covered.
- Align runtime configuration, docs, and deployment defaults.

## Expected outputs
- Implemented RS256 support with tests, or explicit removal/deferral with hardened docs/config.

## Acceptance criteria
1. No unimplemented auth algorithm appears as production-ready config.
2. Behavior is documented and test-verified.
3. Security gate can evaluate this item as closed or tracked debt with rationale.

## Blocker protocol
If key-management requirements for RS256 are out of scope, propose explicit deferral artifact and safe HS256 constraints.

## Completion notes (2026-05-16)
- Clarified current-build algorithm support as HS256-only:
	- Config validation now rejects non-HS256 values for `CWSO_JWT_ALG`.
	- Removed RS256-ready ambiguity from config/runtime comments.
	- Updated compose comment to state RS256 is not supported in current build.
- Added config regression test ensuring `CWSO_JWT_ALG=RS256` is rejected.

Validation evidence:
- `cd /home/emage/Code/emage/CWSO/orchestrator && go test ./internal/config`: PASS
