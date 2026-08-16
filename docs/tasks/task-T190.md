# Task T190 — Bump pinned Go CI image to clear govulncheck stdlib advisories

**ID:** T190
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** —
**Created:** 2026-08-16
**Completed:** 2026-08-16
**Based on:** `docs/checkpoints/checkpoint-022-phase0-c001-c005-complete.md` (Blocker BLK-022-01)

## Objective

`go:audit` currently fails on **every** pipeline in this project, including `develop`'s
own tip with no application changes applied (confirmed: pipeline `2763043677` on commit
`e538023`). Root cause: `.gitlab-ci.yml` pins `image: golang:1.25.12` for `go:lint`,
`go:test`, and `go:audit`, and a new `govulncheck` advisory set was published against
Go stdlib `@go1.25.12` — all five findings are marked "Fixed in: ...@go1.25.13":

- `GO-2026-6218` — quadratic complexity in `net/url` `resolvePath`
- `GO-2026-6090` — post-handshake message limit in `crypto/tls`
- `GO-2026-6089` — missing `ReadHeaderTimeout` on unencrypted HTTP/2 check in `net/http`
- `GO-2026-5972` — recursion depth enforcement in `encoding/asn1`
- `GO-2026-5026` — Punycode label validation in `golang.org/x/net/idna` (via `net/http`)

None of the five are reachable via third-party dependencies we import; all five are
stdlib-only findings, so no `go.mod`/`go.sum` change is implicated — this is purely a
CI toolchain image version problem. Bump the pinned image to a patch version that
clears all five advisories so `go:audit` (and, by extension, every future MR gate) is
green on `develop` again.

This is a direct, narrowly-scoped follow-on to T172 (prior Rust/Go toolchain bump
precedent) and T114 (an earlier, different Go 1.25 bump) — same pattern, new advisory
set, do not conflate scope with either.

## Inputs

- `.gitlab-ci.yml` (lines 59–152: `go:lint`, `go:test`, `go:audit` jobs, each currently
  `image: golang:1.25.12`)
- `docs/checkpoints/checkpoint-022-phase0-c001-c005-complete.md` (blocker BLK-022-01 —
  root-cause diagnosis and evidence that this is unrelated to any Phase 0 change)
- Live `govulncheck` output from the failing job for the exact fixed-version claim per
  advisory (job `15917907673`, pipeline `2763043677`, or re-run your own to confirm
  current advisories are unchanged)

## Rails (read before starting)

### You MUST
- Confirm the actual advisory-clearing patch version before changing anything: run
  `govulncheck` (or trust the advisory `Fixed in:` field, but verify the Docker Hub tag
  exists) against candidate `golang:1.25.13` (or newer 1.25.x if `.13` does not clear
  all five or does not exist as a published tag) — do not guess
- Bump **all three** `golang:1.25.12` pins in `.gitlab-ci.yml` (`go:lint` line ~61,
  `go:test` line ~94, `go:audit` line ~141) to the same confirmed version — keep the
  three jobs on identical Go image versions, do not leave them inconsistent
- Prove the fix: run `go vet ./...`, `go test ./... -race`, and
  `govulncheck ./...` locally (or in a container using the new pinned image) from
  `orchestrator/`, and confirm `govulncheck` now reports zero vulnerabilities
- Push to `agent/devops-engineer/T190` (already created from `origin/develop`,
  worktree at `/home/emage/Code/emage/worktrees/agent-devops-engineer-T190`) and open
  an MR to `develop` referencing T190
- Report blockers per the Blocker Protocol (type/severity) if the advisories are not
  fully clearable by a same-minor-version patch bump (e.g. if 1.25.13 doesn't exist yet
  upstream, or doesn't clear all five) rather than silently picking an unverified tag

### You MUST NOT
- Touch `go.mod`/`go.sum`, application code, or any other CI job — this is a CI image
  version bump only
- Touch `deploy/Dockerfile.orchestrator` (`FROM golang:1.25-alpine`) — it is an
  unpinned floating tag, not pinned to the vulnerable `.12` patch, and out of scope
  unless you find concrete evidence it is also affected (report as a blocker if so,
  don't silently expand scope)
- Modify `rust:*` jobs or any Rust toolchain pin — unrelated to this advisory set
- Add `govulncheck -ignore`/allow-list flags to paper over the findings — the fix is
  the version bump, not suppression (contrast with T171's `cargo audit --ignore`
  precedent, which was used only because a real fix was MSRV-blocked; that does not
  apply here — a same-minor patch bump is available)

## File ownership

- **May create/modify:** `.gitlab-ci.yml` only
- **Must NOT touch:** everything else, including `docs/tasks/active-tasks.md` and
  `docs/tasks/completed-tasks.md` (orchestrator-only, updated separately)

## Expected outputs

- Modified `.gitlab-ci.yml` (three image-tag lines updated, otherwise unchanged)
- MR to `develop` from `agent/devops-engineer/T190`, CI green (`go:lint`, `go:test`,
  `go:audit`, and all other existing jobs unaffected/still passing)

## Acceptance criteria

- [ ] `go:audit` job passes in the MR's pipeline (govulncheck reports zero
      vulnerabilities against the five listed advisories)
- [ ] `go:lint` and `go:test` still pass on the new image (no regression from the
      version bump)
- [ ] All three `.gitlab-ci.yml` Go image pins are identical to each other
- [ ] No files other than `.gitlab-ci.yml` are modified
- [ ] MR references T190 in title or description

## Blocker protocol

Types: `technical` | `dependency` | `unclear_requirements` | `external`.
Severities: `critical` | `major` | `minor`. Max 2 retries before escalating to the
orchestrator. Do not silently fail — if the target patch version cannot be confirmed
or does not clear all five advisories, stop and report rather than merging a partial
or unverified fix.
