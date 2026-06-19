# Checkpoint 012 — Next-Gen GA Prep (RC Published)

**Date:** 2026-06-07  
**Phase:** GA preparation — post Phases 6–9 RC  
**develop tip:** `f5db055` (post T140 MR !50)  
**RC tag:** `v0.3.0-rc1` @ `2032b33`

## Completed tasks (Phases 6–9 + RC closure)

| Phase | Feature(s) | Gate | Checkpoint |
|-------|------------|------|------------|
| 6 | A — HAL | PASS/PASS | `checkpoint-007-phase6-complete.md` |
| 7 | B + C — Sparse + Spikes | PASS/PASS | `checkpoint-009-phase7-complete.md` |
| 8 | D — Sparse merge | PASS/PASS | `checkpoint-010-phase8-complete.md` |
| 9 | E + F + G — Rollout | PASS/PASS | `checkpoint-011-phase9-complete.md` |

| ID | Title | Merge / artifact |
|----|-------|------------------|
| T139 | v0.3.0-rc1 release readiness | !48 → `d693c3f`; tag `2032b33` |
| T135 | KV prefix router (post-RC) | !49 → `0685893` |
| T140 | CI audit hardening (post-RC) | !50 → `130a254` |
| T141 | GitLab release publish + GA prep checkpoint | !51 |

## Key decisions

- RC tag **`v0.3.0-rc1`** pinned @ `2032b33` (T139 merge); GitLab release published with CHANGELOG excerpt.
- Post-RC hardening landed on `develop` after tag: T135 (KV prefix router, default-off) and T140
  (promote `go:audit` / `rust:audit` to blocking gates).
- GA path requires stakeholder RC validation; no open MRs on `develop`; CI green (#2581768352).

## Gate summary (RC)

| Gate | Status | Artifact |
|------|--------|----------|
| QA (Phases 6–9) | PASS | `qa-phase7-report-v1.md`, `qa-phase9-report-v1.md` |
| Security (Phases 6–9) | PASS | gate artifacts + security checklists |
| CI/CD Pipeline | GREEN | #2581768352 on `develop` @ `f5db055` |
| RC Release | PUBLISHED | https://gitlab.com/em-age/emage.code.cwso/-/releases/v0.3.0-rc1 |

## Deferred / debt (GA path)

- Stakeholder RC validation on published artifacts (blocking GA).
- Trainer fleet proxy p95 benchmark — unit-tested, fleet validation deferred.
- Orchestrator `/v1/chat/completions` stub (501); trainers use `cwso-rollout` proxy.
- Trajectory chain columns (store v2) — raw completion records sufficient for RC.

## Blockers

None for RC publication. GA blocked on stakeholder sign-off.

## Next steps

1. Stakeholder RC validation cycle on `v0.3.0-rc1` artifacts.
2. Open GA hardening tasks if feedback requires fixes before `v0.3.0`.
3. Trainer fleet proxy benchmark (ops, non-blocking).

## Token metrics

Release + checkpoint tracked within QA/Sec/Release budget per AGENTS.md.
