# Task T140 — CI audit hardening (promote audits to blocking gate)

- **Status:** in_review
- **Owner:** devops-engineer
- **Priority:** P1
- **Depends on:** T094 (done), T139 (done)
- **Phase:** GA hardening — v0.3.0 promotion prerequisite
- **Based on:** `docs/artifacts/release-v0.3.0-rc1.md`, `docs/tasks/task-T094.md`

## Objective

Promote the T094 dependency-audit jobs (`go:audit`, `rust:audit`) from advisory
(`allow_failure: true`) to blocking CI gates so known vulnerabilities cannot merge
to `develop` or `main` during the v0.3.0 GA hardening window.

## Changes

- Remove `allow_failure: true` from `go:audit` and `rust:audit` in `.gitlab-ci.yml`.
- Leave `rust:lint` `allow_failure: true` unchanged (PoC fmt/clippy advisory).
- Update `release-v0.3.0-rc1.md` deferred table and develop tip.
- Update `plan-cwso-nextgen-phase6plus.md` develop tip and GA hardening note.

## Acceptance Criteria

- [x] `go:audit` and `rust:audit` are blocking (no `allow_failure`).
- [x] `rust:lint` remains non-blocking.
- [x] Latest CI on develop showed both audit jobs green (`8670f04`, pipeline #2581299657).
- [ ] MR !50 CI all green; merge to `develop`.
- [ ] Task board marks T140 `done` after merge.

## Notes

- T114 (Go 1.25 bump) cleared `govulncheck` stdlib advisories; local `cargo audit` exit 0.
- If a non-actionable transitive advisory appears, add an ignore policy file rather than
  re-enabling `allow_failure`.
