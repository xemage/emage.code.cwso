# Task T162 - Remediate high-value reliability/security technical debt

- **Status:** pending
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T158
- **Based on:** `TECHNICAL-DEBT.md` (TD-05, TD-06, TD-08)

## Objective

Address the highest-value technical debt items affecting reliability/security confidence:
publish error observability, queued job close-state correctness, and sensitive error exposure
in SSE job-state notifications.

## Acceptance Criteria

- [ ] TD-05: publish failures emit DEBUG-level telemetry with safe context.
- [ ] TD-06: queued jobs at `Close()` transition to cancelled state deterministically.
- [ ] TD-08: job error text in broadcast paths is scrubbed/redacted appropriately.
- [ ] Unit/integration tests cover each behavior change.
- [ ] `TECHNICAL-DEBT.md` updated to reflect remediation status.
