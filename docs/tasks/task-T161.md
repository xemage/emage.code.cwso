# Task T161 - Clean active/completed task board hygiene

- **Status:** pending
- **Owner:** scrum-master
- **Priority:** P1
- **Depends on:** T160
- **Based on:** `docs/tasks/active-tasks.md`, `docs/tasks/completed-tasks.md`

## Objective

Restore task-board readability by keeping `active-tasks.md` focused on non-done work and moving
done rows to `completed-tasks.md` while preserving historical traceability.

## Acceptance Criteria

- [ ] `active-tasks.md` contains only `pending`, `in_progress`, `blocked`, or `in_review` rows.
- [ ] Done rows are moved to `completed-tasks.md` with date and outcome summary.
- [ ] No task ID loss or duplication.
