# Task T071 — Phase 5 security gate and hardening

- Phase: **5 (Security Gate)** · Owner: **security-engineer** · Priority: **P0**
- Depends on: T070 · Blocks: T072
- Status: pending

## Objective
Run security gate for new dispatch surfaces, Wasm runtime interfaces, and telemetry instrumentation paths.

## Inputs
- [docs/tasks/task-T070.md](task-T070.md)
- Security guidelines and OWASP checklist references

## Constraints
- High/critical findings block progression.
- Any new privileged interfaces require explicit threat notes and controls.

## Expected outputs
- `docs/artifacts/security-phase5-audit-v1.md`
- Findings with severity and remediation status.

## Acceptance criteria
1. OWASP-relevant controls are reviewed for all new endpoints/surfaces.
2. Wasm host capability allowlist is validated and documented.
3. Gate verdict is PASS or CONDITIONAL_PASS with tracked conditions.

## Blocker protocol
If critical findings remain unresolved, report blocker type `technical` with remediation task proposals.
