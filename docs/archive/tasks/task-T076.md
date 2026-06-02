# Task T076 — Release candidate v0.2.0-rc1 readiness gate

- Phase: **5 (Release Candidate Closure)** · Owner: **release-manager** · Priority: **P0**
- Depends on: T075 · Blocks: —
- Status: done (2026-05-24)

## Objective
Promote completed Phase 5 hardening work (T073-T075) into an explicit v0.2.0-rc1 release-candidate readiness state with operator-facing evidence.

## Inputs
- [docs/tasks/task-T073.md](task-T073.md)
- [docs/tasks/task-T074.md](task-T074.md)
- [docs/tasks/task-T075.md](task-T075.md)
- [docs/artifacts/release-v0.2.0-hardware-aware-v1.md](../artifacts/release-v0.2.0-hardware-aware-v1.md)
- [docs/artifacts/security-phase5-audit-v1.md](../artifacts/security-phase5-audit-v1.md)

## Constraints
- Preserve HTTPS-only push and CI-gated release workflow.
- RC readiness must reflect the post-hardening state, not pre-hardening conditional notes.
- Keep deployment model unchanged (feature-flagged rollout, baseline fallback available).

## Expected outputs
- Updated changelog entry for v0.2.0-rc1.
- Updated README operator controls for newly enforced hardening env vars.
- RC readiness artifact with PASS/CONDITIONAL_PASS verdict and evidence.
- Phase checkpoint documenting RC-ready state.

## Acceptance criteria
1. RC documentation reflects closure of F-071-01, F-071-02, and F-071-03 follow-ups.
2. Operator docs enumerate active hardening controls and advisory telemetry semantics.
3. CI pipeline evidence for latest hardening line is recorded in RC artifact.

## Completion notes (2026-05-24)
- Added v0.2.0-rc1 changelog section in `CHANGELOG.md`.
- Updated README phase-5 operator section with:
  - Wasm integrity env controls (`SHA256` + trusted dir)
  - telemetry redaction env controls
  - explicit advisory semantics for `ebpf-hook` detection latency
- Added RC readiness artifact and checkpoint.
- Recorded latest green pipeline evidence after hardening closure.

### Evidence
- `docs/artifacts/release-v0.2.0-rc1.md`
- `docs/checkpoints/checkpoint-022-phase5-rc1-ready.md`
