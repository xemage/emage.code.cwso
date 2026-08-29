# Task T203 — Wire missing baseline-required CI security tools (F-C061-02)

**ID:** T203
**Owner:** devops-engineer
**Status:** pending
**Priority:** P1
**Depends on:** —
**Created:** 2026-08-29
**Completed:** —
**Based on:** `docs/artifacts/security-v1.0-audit-v1.md` finding F-C061-02
(SECURITY:MEDIUM), found by C061's v1.0.0 security audit, independently re-verified by
Tech Lead review (MR !199) by reading `.gitlab-ci.yml` directly. Logged as a fast-follow
task per security-guidelines.md's requirement that MEDIUM findings carry a documented
remediation plan — this task IS that plan.

## Objective

`docs/artifacts/security-baseline-v2.md` §5 ("Required CI checks") lists six required CI
security checks: `gosec`, `govulncheck`, `cargo audit`, `cargo deny`, a Trivy container
image scan, and `gitleaks`/`trufflehog` secret scanning. Verified against the actual
`.gitlab-ci.yml` `audit` stage: only `govulncheck` (`go:audit`) and `cargo audit`
(`rust:audit`) are implemented, both correctly wired as blocking gates on MR/develop/main
pipelines. `gosec`, `cargo deny`, a Trivy scan, and `gitleaks`/`trufflehog` do not appear
anywhere in `.gitlab-ci.yml`. There is also no `.pre-commit-config.yaml` in the repo, so
the baseline's Deployment Checklist item "Pre-commit secret scan hooks installed locally"
is unmet by default.

This is a control-gap finding, not an active-leak finding — a manual secret sweep of code
and full git history during the audit found zero live secrets, and Tech Lead review
independently re-ran an equivalent sweep and corroborated this. The gap is that a *future*
secret leak or a newly-introduced Go/Rust vulnerability pattern that `gosec`/`cargo deny`
would catch currently has no automated CI signal.

## Inputs

- `docs/artifacts/security-v1.0-audit-v1.md` (F-C061-02's full finding)
- `docs/artifacts/security-baseline-v2.md` §5 (the required-tools list, and the
  Deployment Checklist referencing pre-commit hooks)
- `.gitlab-ci.yml` (the `audit` stage, lines ~162-215 — read the existing `go:audit`/
  `rust:audit` jobs as the pattern to follow for consistency)

## Rails (read before starting)

### You MUST
- Add `gosec` as a new CI job in the `audit` stage, following the same pattern as
  `go:audit` (blocking on MR/develop/main, same `rules:` shape)
- Add `cargo deny` as a new CI job in the `audit` stage, following the same pattern as
  `rust:audit`
- Add a Trivy container image scan — decide whether this runs per-image in the existing
  `build` stage or as a dedicated `audit`-stage job scanning already-built images; either
  is fine, but it must actually scan the real images this project builds
  (`cwso/orchestrator`, `cwso/git-shadow`, `cwso/merge-engine`, `cwso/rollout`), not a
  placeholder
- Add `gitleaks` or `trufflehog` as a CI job scanning the diff/commit range on every
  MR/push (your choice of tool; document which and why briefly in your MR)
- For each new tool: run it for real against the current codebase first, before wiring it
  as blocking — if it surfaces genuine findings on the current code, do NOT silently
  suppress them to make the job pass; report them as a new finding for the orchestrator to
  route, and land the CI job as `allow_failure: true` initially with a tracked follow-up
  to fix the findings and flip it to blocking, mirroring this project's own precedent
  (`rust:lint`'s `allow_failure: true` for exactly this kind of "new gate, pre-existing
  drift" situation)
- Add a `.pre-commit-config.yaml` with at least a secret-scan hook (matching whichever
  tool you chose for CI), closing the Deployment Checklist gap
- Add a CHANGELOG entry

### You MUST NOT
- Add a CI job that's a no-op or doesn't actually scan real content (a "security theater"
  job is worse than an honestly-absent one)
- Silently suppress or ignore a genuine finding surfaced by any of these new tools just to
  land this task with `allow_failure: false` — report it instead, per the rail above
- Remove or weaken `go:audit`/`rust:audit`'s existing blocking behavior
- Touch application code — this is a CI/tooling-only task

## File ownership

- **May create/modify:** `.gitlab-ci.yml`, `.pre-commit-config.yaml` (new),
  `docs/artifacts/security-baseline-v2.md` (only to update the "Required CI checks"
  table's status column once tools are wired), `CHANGELOG.md`
- **Must NOT touch:** application code, `docs/artifacts/security-v1.0-audit-v1.md` (the
  audit artifact itself, immutable), `docs/DEBT-REGISTER.md`

## Steps (execute in order)

1. Read F-C061-02's full finding and `security-baseline-v2.md` §5.
2. Read the existing `go:audit`/`rust:audit` jobs in `.gitlab-ci.yml` for the pattern.
3. Add `gosec`, `cargo deny`, Trivy, and `gitleaks`/`trufflehog` as new CI jobs.
4. Run each tool for real against current code before finalizing; route any genuine
   findings rather than suppressing them.
5. Add `.pre-commit-config.yaml`.
6. Update `security-baseline-v2.md`'s status table; CHANGELOG.

## Expected outputs

- Four new CI jobs (gosec, cargo-deny, Trivy, gitleaks-or-trufflehog), each genuinely
  scanning real content
- `.pre-commit-config.yaml`
- Updated `security-baseline-v2.md` status table
- CHANGELOG entry
- Any genuine findings the new tools surface, reported as new tracked items (not
  suppressed)

## Acceptance criteria

1. All four missing tools genuinely wired into `.gitlab-ci.yml`, each verified to scan
   real content (not a placeholder/empty-scope job)
2. `.pre-commit-config.yaml` exists with at least a secret-scan hook
3. No genuine finding from any new tool was silently suppressed to make this task "pass"
4. `glab ci lint` (or equivalent YAML validation) confirms the pipeline is syntactically
   valid

## Verification commands

```bash
glab ci lint
# Run each new tool locally against the current codebase to confirm real scan behavior
# before wiring into CI, per the tool's own CLI usage
```

## Git rails

- Branch: `agent/devops-engineer/T203` from `develop`
- Commit: `ci(security): wire gosec, cargo-deny, Trivy, gitleaks into CI (F-C061-02)`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries. If any new
tool surfaces a genuine CRITICAL/HIGH finding against current code, treat it the same as
any other security gate finding in this roadmap — report it plainly, do not soften it or
suppress it to land this task cleanly.

## Execution notes

<filled during execution>
