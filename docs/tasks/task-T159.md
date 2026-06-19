# Task T159 - Add deterministic one-command local smoke target

- **Status:** pending
- **Owner:** devops-engineer
- **Priority:** P1
- **Depends on:** T158
- **Based on:** `Makefile`, `docs/user/installation-v2.md`

## Objective

Provide a single deterministic local smoke command that builds required images, starts required
profiles, runs health and integration validation, and tears down cleanly.

## Acceptance Criteria

- [ ] New documented `Makefile` target runs full local smoke flow end-to-end.
- [ ] Target is repeatable and exits non-zero on failures.
- [ ] Target does not require hidden manual pre-steps.
- [ ] Installation docs reference the new command.
