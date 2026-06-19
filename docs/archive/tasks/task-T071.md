# Task T071 — Phase 5 security gate and hardening

- Phase: **5 (Security Gate)** · Owner: **security-engineer** · Priority: **P0**
- Depends on: T070 · Blocks: T072
- Status: done (2026-05-23, CONDITIONAL_PASS)

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

## Completion notes (2026-05-23)
- Produced security gate artifact `docs/artifacts/security-phase5-audit-v1.md` covering scoped review for dispatch policy v2, sparse/quantized assist, SSM assist, Wasm scoring runtime, telemetry monitor/emitter, config validation, and server wiring.
- Verified Wasm host-call controls are deny-by-default with explicit allowlist enforcement and unknown-call rejection.
- Gate verdict: `CONDITIONAL_PASS`.
- Findings summary:
	- No `CRITICAL`/`HIGH` findings.
	- One `MEDIUM` hardening item: Wasm module integrity verification is not yet enforced.
	- Two `LOW` hardening items: telemetry minimization policy and eBPF latency semantics.
- Proposed follow-up hardening tasks: T073 (Wasm integrity controls), T074 (telemetry redaction policy), T075 (measured eBPF latency/advisory semantics).

### Evidence
- Security audit artifact: `docs/artifacts/security-phase5-audit-v1.md`
