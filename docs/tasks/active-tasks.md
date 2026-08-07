# Active Tasks

| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
| T001 | PO: User story + acceptance criteria (operator-dashboard) | product-owner | done | P0 | — | 2026-08-06 |
| T002 | SA: Technical impact assessment + ADR-011 | solution-architect | done | P0 | — | 2026-08-06 |
| T003 | SM: Sprint plan + GitLab issues | scrum-master | done | P0 | T001, T002 | 2026-08-06 |
| T004 | BE: Expose Stats() on jobs.Manager | backend-developer | done | P0 | T003 | 2026-08-06 |
| T005 | BE: Client activity middleware (auth/tool counters) | backend-developer | done | P0 | T003 | 2026-08-06 |
| T006 | BE: Config validation + sidecar health aggregator | backend-developer | done | P0 | T003 | 2026-08-06 |
| T007 | BE: Dashboard HTTP routes + embedded HTML | backend-developer | done | P0 | T004, T005, T006 | 2026-08-06 |
| T008 | TL: Code review gate (CONDITIONAL_PASS → PASS after fixes) | tech-lead | done | P0 | T007 | 2026-08-06 |
| T009 | QA: 13 unit/integration tests for dashboard package | qa-engineer | done | P1 | T008 | 2026-08-06 |
| T010 | SE: Security audit (auth, secret leakage) | security-engineer | in_review | P1 | T008 | 2026-08-06 |

> Status values: `pending` · `in_progress` · `blocked` · `in_review` · `done` · `cancelled`
> Priority values: `P0` (critical path) · `P1` (important) · `P2` (nice-to-have)
> Owners are agent names from `knowledge/agents/`.

> Status values: `pending` · `in_progress` · `blocked` · `in_review` · `done` · `cancelled`
> Priority values: `P0` (critical path) · `P1` (important) · `P2` (nice-to-have)
> Owners are agent names from `knowledge/agents/`.

Per-task briefs live alongside this file as `task-T001.md`, `task-T002.md`, …
