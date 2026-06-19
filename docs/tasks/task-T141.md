# Task T141 — Publish GitLab release v0.3.0-rc1 + GA prep checkpoint

- **Status:** done
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** T139 (RC tag), T140 (post-RC audit hardening)
- **Phase:** Next-Gen GA prep — RC publication
- **Based on:** `release-v0.3.0-rc1.md`, `plan-cwso-nextgen-phase6plus.md`, checkpoints 007–011

## Objective

Publish the existing **`v0.3.0-rc1`** Git tag as a GitLab release and capture the phase boundary
checkpoint for GA preparation: Phases 6–9 complete, RC tagged @ `2032b33`, post-RC hardening
(T135 + T140) on `develop` @ `f5db055`.

## Deliverables

- **GitLab release** `v0.3.0-rc1` — CHANGELOG excerpt + link to `release-v0.3.0-rc1.md`
- **`docs/checkpoints/checkpoint-012-nextgen-ga-prep.md`** — GA prep phase boundary
- **`docs/artifacts/release-v0.3.0-rc1.md`** — next actions complete, GitLab release URL
- **`docs/plans/plan-cwso-nextgen-phase6plus.md`** — develop @ `f5db055`, T141 complete
- **`docs/tasks/active-tasks.md`** — T141 row done
- **`docs/tasks/task-T141.md`** — this brief

## Acceptance Criteria

- [x] GitLab release published for tag `v0.3.0-rc1` @ `2032b33` (tag not moved)
- [x] Release description includes CHANGELOG `v0.3.0-rc1` excerpt and artifact link
- [x] Checkpoint 012 records Phases 6–9 complete, RC published, GA path
- [x] `release-v0.3.0-rc1.md` next actions 1–3 marked complete with release URL
- [x] Plan and active-tasks board reconciled (`develop` @ `f5db055`, T141 done)
- [x] Doc changes merged via MR !51 with CI green

## Notes

- GA promotion (`v0.3.0`) requires stakeholder RC validation on published artifacts.
- Trainer fleet proxy benchmark remains deferred (non-blocking for RC).
