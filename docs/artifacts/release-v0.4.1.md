# Artifact: release-v0.4.1 (RC)

## Metadata
- Producer agent: backend-developer / release-manager
- Created: 2026-06-19
- Based on: `gate-v0.4.1-hardening-2026-06-18.md`, v0.4.0 baseline, T150–T163
- **develop tip:** `a3c9c31` (post-CI-fix)
- **Prior GA tag:** `v0.4.0` @ published 2026-06-09

## Release intent

**v0.4.1** is a hardening and Polar parity release that delivers:
- KV-cache differential prompting for prefix-cache optimization
- Offline SFT batch trajectory generation endpoint
- Integration testing and auth flow determinism
- Jobs manager reliability and security hardening
- Validation gate **PASS** confirmation

**Primary user documentation:** [`docs/user/installation-v2.md`](../user/installation-v2.md)

## Scope vs v0.4.0

| Item | Task | Status |
|------|------|--------|
| KV differential prompting | T150 | **Included** |
| Offline SFT data generation | T151 | **Included** |
| Fix Phase 2 integration auth | T158 | **Included** |
| Add smoke-local validation target | T159 | **Included** |
| Reconcile v0.4.0 GA documentation drift | T160 | **Included** |
| Clean task board hygiene | T161 | **Included** |
| Remediate reliability/security technical debt (TD-05/06/08) | T162 | **Included** |
| Hardening validation gate | T163 | **PASS** |

## Changelog — v0.4.1

**Release Date:** 2026-06-19  
**Previous Version:** v0.4.0

### New Features
- **KV differential prompting** (T150): Enabled via `CWSO_ROLLOUT_KV_DIFFERENTIAL_PROMPTING_ENABLED` flag. On cache hit, strips prefix tokens, forwards cache_salt to reduce redundant prompt processing.
- **Offline SFT batch generation** (T151): New `POST /rollout/task/offline_generate` endpoint for batch trajectory assembly from existing session captures without trainer callback infrastructure. Supports up to 32 concurrent session drains.

### Hardening
- **Jobs manager reliability** (T162): Added queued-job cancellation on manager close, publish failure logging, and error redaction for SSE broadcasts (sensitive string masking).
- **Integration testing** (T158–T159): Implemented deterministic JWT secret resolution with `.env.jwt.dev` source-of-truth and `make smoke-local` one-command smoke target for local validation.
- **Documentation drift reconciliation** (T160): Updated checkpoint and installation guide to reflect v0.4.0 GA state and v0.4.1 new features.

### Documentation
- [`docs/user/installation-v2.md`](../user/installation-v2.md): Updated with new env vars and endpoints
- [`docs/checkpoints/checkpoint-015-v0.4.1-hardening-complete.md`](../checkpoints/checkpoint-015-v0.4.1-hardening-complete.md): Complete execution record of v0.4.1 hardening sprint
- [`docs/artifacts/gate-v0.4.1-hardening-2026-06-18.md`](./gate-v0.4.1-hardening-2026-06-18.md): Validation gate artifact with PASS verdict

### Operations
- Board hygiene: Migrated 80 completed tasks (T080–T157) to `completed-tasks.md`
- Active task board now contains only 8 pending tasks with clear dependency graph
- All changes formatted per project standards and pass CI green

## Feature flag matrix (v0.4.1 additions)

| Flag | Default | Enables |
|------|---------|---------|
| `CWSO_ROLLOUT_KV_DIFFERENTIAL_PROMPTING_ENABLED` | `false` | KV-cache differential prompting on prefix-cache hits |

All other flags unchanged from v0.4.0.

## Validation and CI evidence

- **Hardening validation gate:** PASS (QA + security + tech-lead) per `gate-v0.4.1-hardening-2026-06-18.md`
- **End-to-end smoke test:** `make smoke-local` PASS on clean stack lifecycle
- **All unit tests:** Go tests 4/4 pass, Rust tests 33/33 pass (race-free)
- **CI pipeline:** `develop` pipeline #2611568664 all jobs green (lint, build, test, e2e)
- **Commit:** `a3c9c31` published to origin/develop and origin/release/v0.4.1

## RC verdict

**PASS (RC_READY — ready for v0.4.1-rc1 tag)**

Rationale:
- All v0.4.1 hardening and parity tasks completed and validated
- CI pipeline green on full test matrix
- Smoke test and validation gate PASS
- Documentation and checkpoint prepared
- No breaking changes or regressions vs v0.4.0

## Release actions

1. **Tag v0.4.1-rc1** on `release/v0.4.1` branch (or `develop`)
2. **Create GitLab release** with CHANGELOG excerpt and link to `installation-v2.md`
3. **Deploy to staging** for end-to-end validation
4. **Collect stakeholder sign-off** and final approval for GA
5. **Tag v0.4.1** (final) when ready for production release

## Known issues

None blocking v0.4.1-rc1. See TECHNICAL-DEBT.md for post-GA deferred items.

## Migration guide

No breaking changes from v0.4.0. All new features are **opt-in** via environment flags.

To enable new features:
```bash
export CWSO_ROLLOUT_KV_DIFFERENTIAL_PROMPTING_ENABLED=true
# Offline SFT generation is available at POST /rollout/task/offline_generate by default
```
