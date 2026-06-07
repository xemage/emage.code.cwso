# Task T152 — v0.3.0 GA release readiness

- **Status:** pending
- **Owner:** release-manager
- **Priority:** P0
- **Depends on:** T142, T147, T141
- **Based on:** `release-v0.3.0-rc1.md`, `checkpoint-012-nextgen-ga-prep.md`

## Objective

Promote `v0.3.0-rc1` to **v0.3.0 GA** after post-RC hardening (T147, CI green) and
stakeholder validation. Publish release artifact, CHANGELOG, GitLab release, and checkpoint.

## Acceptance Criteria

- [ ] `develop` CI green on release commit
- [ ] `docs/artifacts/release-v0.3.0.md` with scope vs RC delta
- [ ] CHANGELOG section for v0.3.0
- [ ] Annotated tag `v0.3.0` on `develop`
- [ ] GitLab release published
- [ ] `checkpoint-013-v0.3.0-ga.md` written
- [ ] Task board marks T152 done; plan updated

## RC → GA delta (expected)

- T142 installation docs
- T147 OpenAI Responses API + proxy hardening
- T140/T135 post-RC (already in RC track)
- CI e2e hardening (MR !55)
- Polar parity T144–T151: **optional for GA** unless stakeholder requires

## Notes

Stakeholder RC sign-off remains external gate; document CONDITIONAL_PASS if sign-off pending.
