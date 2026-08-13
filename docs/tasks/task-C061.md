# Task C061 — Security pass: close T010

**ID:** C061
**Owner:** security-engineer
**Status:** pending
**Priority:** P0
**Depends on:** C050–C054 (gate CG4). May start earlier at orchestrator discretion — it needs no Phase-5 output.
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C061 row); docs/plans/plan-cwso-v1.0-phase6-release-v1.md; T010 (open since 2026-08-06)

## Objective

Complete the security audit that T010 opened on 2026-08-06 (auth, secret leakage) and
close it. Run the OWASP Top 10 checklist against the v1.0 surface. Per
security-guidelines.md: CRITICAL and HIGH findings block the release; MEDIUM needs a
remediation plan; LOW becomes tracked debt.

## Inputs

- T010 (`docs/tasks/task-T010.md` — the original audit scope)
- `SECURITY.md`, `docs/artifacts/security-baseline-v2.md`
- The full v1.0 surface: `orchestrator/`, `services/`, `deploy/`, `scripts/` (including the new C012/C013/C016/C017 scripts)
- OWASP Top 10 checklist (security-guidelines.md)

## Rails (read before starting)

### You MUST
- Audit: JWT auth (secret bootstrap C012, token script C013), secret handling (no secrets in code/logs/git), the new read-write workspace mount (C015), socket permissions (C044 outcome), container hardening posture, input validation at the MCP boundary
- Classify every finding `SECURITY:CRITICAL` / `HIGH` / `MEDIUM` / `LOW` per the workflow
- Produce `docs/artifacts/security-v1.0-audit-v1.md`: findings table + verdict
- Close T010 per the task-management archive procedure (orchestrator executes the ledger move; you supply the verdict)
- You are read-only for code: flag, annotate, create findings — do not fix

### You MUST NOT
- Modify any code — Security Engineer is read-only during audit
- Downgrade a finding to make the release date — CRITICAL/HIGH block; that is the rule, not a suggestion
- Audit v1.1-deferred surface (HAL, sparse, rollout internals beyond their compose reachability) — scope is the v1.0 default path
- Paste any discovered secret value into the findings report (describe location + type only)

## File ownership

- **May create/modify:** `docs/artifacts/security-v1.0-audit-v1.md` (new)
- **Must NOT touch:** all code, `SECURITY.md` (findings may *recommend* changes)

## Steps (execute in order)

1. Read T010's scope and the security baseline.
2. Run the OWASP checklist over the v1.0 surface.
3. Classify and write findings.
4. Deliver the verdict; hand T010 closure to the orchestrator.

## Expected outputs

- `docs/artifacts/security-v1.0-audit-v1.md`
- T010 closure verdict

## Acceptance criteria

1. OWASP checklist run over the v1.0 surface, findings classified
2. No unresolved CRITICAL/HIGH (or the release is correctly blocked)
3. T010 closed with the audit artifact as its outcome

## Verification commands

```bash
grep -c "SECURITY:CRITICAL\|SECURITY:HIGH\|SECURITY:MEDIUM\|SECURITY:LOW" docs/artifacts/security-v1.0-audit-v1.md
grep -rn "password\|secret\|token" orchestrator/ services/ --include="*.go" --include="*.rs" -i | grep -v "_test" | grep -iv "jwt_secret\|CWSO_JWT" | head   # leakage sweep
```

## Git rails

- Branch: `agent/security-engineer/C061` from `develop`
- Commit: `docs(security): v1.0.0 security audit closing T010`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
A CRITICAL/HIGH finding is not a blocker on *you* — it is a release blocker. Report it
plainly; do not soften it.

## Execution notes

<filled during execution>
