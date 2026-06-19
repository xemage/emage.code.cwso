# Task T094 — CI dependency audit (`govulncheck` + `cargo audit`)

- **Status:** in_review
- **Owner:** devops-engineer
- **Priority:** P2
- **Depends on:** T089 (done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A) — follow-up
- **Based on:** `docs/artifacts/gate-phase6-feature-a-2026-06-02.md`

## Objective
Continuously scan the Go and Rust dependency trees for known vulnerabilities, surfacing
advisories in the pipeline without wedging PoC-phase delivery.

## Changes
- New `audit` stage in `.gitlab-ci.yml` (between `test` and `e2e`):
  - `go:audit` — `govulncheck ./...` over the orchestrator module (golang:1.23 image).
  - `rust:audit` — `cargo audit` over the `services` workspace (rust:1.86 image).
- Both jobs are `allow_failure: true` during the PoC phase: a freshly-disclosed advisory is
  reported but does not block the pipeline. Caches reuse the existing Go-mod / cargo keys.
- Runs on merge requests and on `develop` / `main`.

## Acceptance Criteria
- [x] Go vulnerability scan runs in CI (`govulncheck`).
- [x] Rust vulnerability scan runs in CI (`cargo audit`).
- [x] Non-blocking during PoC (`allow_failure: true`), but results are always visible.

## Notes / Follow-ups
- Flip `allow_failure` to `false` (hard gate) once the production hardening phase begins, and
  add an advisory-ignore policy file if a non-actionable transitive advisory needs waiving.
