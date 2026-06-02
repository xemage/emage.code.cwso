# Task T095 — Bump Go toolchain to 1.25 (clear `go:audit` stdlib advisories)

- **Status:** in_review
- **Owner:** devops-engineer
- **Priority:** P2
- **Depends on:** T094 (done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A) — follow-up
- **Based on:** `task-T094.md`

## Objective
Clear the standard-library vulnerabilities reported by the new `go:audit` job (T094) by
moving the Go toolchain off 1.23 to a current stable release.

## Context
The first `go:audit` run flagged 18 Go **standard-library** advisories, all of the form
"Found in go1.23.12 / Fixed in go1.24.x" (e.g. `GO-2025-4007/4008/4009`). These are not
dependency bugs — they are cleared by upgrading the toolchain. Verified empirically in
containers:

| Toolchain | `govulncheck ./...` |
|-----------|---------------------|
| go1.23.12 | 18 stdlib advisories |
| go1.24.13 | 7 stdlib advisories (now fixed only in go1.25.8) |
| **go1.25.10** | **No vulnerabilities found** |

So 1.25 is the first line that fully clears the current advisory set.

## Changes
- `orchestrator/go.mod`: `go 1.23.0` + `toolchain go1.23.12` → **`go 1.25.0`** (explicit
  `toolchain` line dropped so `GOTOOLCHAIN=local` uses the image's bundled 1.25.x, avoiding a
  pin-vs-image mismatch).
- `.gitlab-ci.yml`: `go:lint` / `go:test` / `go:audit` images `golang:1.23` → `golang:1.25`;
  `go:audit` now installs `govulncheck@latest` (v1.3.0 requires Go ≥ 1.25, now satisfied).
- `deploy/Dockerfile.orchestrator`: builder `golang:1.23-alpine` → `golang:1.25-alpine`.

## Acceptance Criteria
- [x] `go build` / `go vet` / `go test ./...` green on the bumped module (local Go 1.26.3).
- [x] `govulncheck ./...` reports no vulnerabilities under `golang:1.25` (verified in-container).
- [x] CI go-stage images and the orchestrator runtime image build on Go 1.25.

## Notes / Follow-ups
- `go:audit` and `rust:audit` remain `allow_failure: true` (PoC phase): a freshly-disclosed
  stdlib advisory should surface without wedging delivery. Flip to a hard gate at the start of
  production hardening (tracked in `task-T094.md`).
