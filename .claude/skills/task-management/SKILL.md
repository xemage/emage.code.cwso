---
name: "task-management"
description: "Manage the task lifecycle: create, read, update, transition, and archive tasks in docs/tasks/. Use when creating tasks, updating task status, building task dependency graphs, or archiving completed work."
---

# Task Management

## Purpose

Manage the full lifecycle of project tasks — creation, status tracking, dependency management, and archival — using structured markdown tables in `docs/tasks/`.

## When to Use

- Creating a new task for planned or discovered work
- Updating task status (e.g., starting work, marking blocked, submitting for review)
- Querying active tasks, filtering by status or assignee
- Building or updating task dependency graphs
- Archiving completed tasks to `completed-tasks.md`

## File Locations

| File | Purpose |
|------|---------|
| `docs/tasks/active-tasks.md` | Tasks in `pending`, `in_progress`, `blocked`, `in_review` ONLY |
| `docs/tasks/completed-tasks.md` | Archived tasks that have been completed and verified |

> ## INVARIANT (never violate)
> `active-tasks.md` MUST NEVER contain a row whose Status is `done` or `cancelled`.
> The row is removed in the SAME edit that sets the terminal status.
> Writing `done` into `active-tasks.md` is a protocol violation.

## Task Table Format

### active-tasks.md — 7 columns, in this exact order
| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
| T042 | Add rate limiting | backend-developer | in_progress | P1 | T040 | 2026-07-27 |

### completed-tasks.md — 5 columns, in this exact order
| ID | Title | Owner | Done on | Outcome / artifact |
|----|-------|-------|---------|--------------------|
| T042 | Add rate limiting | backend-developer | 2026-07-27 | src/mw/ratelimit.ts; docs/tasks/task-T042.md |

### Field rules
| Field | Rule |
|-------|------|
| ID | `T` + 3 or more digits. `T001`, `T042`, `T1001`. NEVER `T001`. NEVER `BUG-7`. |
| Owner | Exact agent slug, kebab-case, from the installed agents folder. |
| Status | `pending` \| `in_progress` \| `blocked` \| `in_review` \| `done` \| `cancelled` |
| Priority | `P0` \| `P1` \| `P2`. NEVER `critical`/`high`/`medium`/`low`. |
| Depends on | Comma-separated task IDs, or `—` |
| Last update / Done on | `YYYY-MM-DD`, a real date |

### Field mapping when archiving
| active column | goes to |
|---------------|---------|
| ID, Title, Owner | copied as-is |
| Status | DROPPED (implied `done`) |
| Priority | DROPPED |
| Depends on | DROPPED |
| Last update | becomes `Done on` |
| — | new `Outcome / artifact`: semicolon-separated paths, MUST include `docs/tasks/task-<ID>.md` |

## Status Lifecycle

```
pending → in_progress → blocked → in_progress → in_review → done
                  │                                          │
                  └──────────────────────────────────────────┘
                           (can regress if review fails)
```

`done` and `cancelled` are TERMINAL → archive immediately (see § "Complete a Task").

Valid transitions:

| From | To | Trigger |
|------|----|---------|
| `pending` | `in_progress` | Agent picks up the task |
| `in_progress` | `blocked` | Dependency or blocker encountered |
| `in_progress` | `in_review` | Implementation complete, ready for validation |
| `blocked` | `in_progress` | Blocker resolved |
| `in_review` | `done` | Validation gate passed |
| `in_review` | `in_progress` | Review rejected, rework needed |
| any | `cancelled` | Work abandoned — orchestrator decision |
| `in_review` | `cancelled` | Rejected outright |

## Procedures

### 1. Create a Task

1. Open `docs/tasks/active-tasks.md`.
2. Determine the next available `TNNN` ID (increment the highest existing ID).
3. Add a new row to the table with status `pending`.
4. If the task depends on other tasks, populate `BlockedBy`.
5. If other tasks depend on this task, update their `BlockedBy` field and this task's `Blocks` field.

### 2. Update Task Status

1. Locate the task row by ID in `active-tasks.md`.
2. Validate the transition is allowed (see lifecycle table above).
3. Update the `Status` field.
4. If transitioning to `blocked`, file a blocker report (see `blocker-escalation` skill).
5. If transitioning to `done`, proceed to the archive procedure.

### 3. Manage Dependencies

1. When adding a dependency, update **both** sides:
   - Add the dependency ID to the dependent task's `BlockedBy` field.
   - Add the dependent task's ID to the dependency's `Blocks` field.
2. Before transitioning a task to `in_progress`, verify all `BlockedBy` tasks are `done`.
3. When a task reaches `done`, check its `Blocks` field and evaluate if blocked tasks can be unblocked.

### 4. Complete a Task (ATOMIC — all 4 steps in one edit session)

Performed by the ORCHESTRATOR ONLY. Other agents report completion; they never move rows.

1. Verify acceptance criteria are met (skill: `verification-before-completion`).
2. APPEND one row to `docs/tasks/completed-tasks.md` using the 5-column schema
   and the field mapping above. Append at the BOTTOM.
3. DELETE the task's row from `docs/tasks/active-tasks.md`.
4. In `docs/tasks/task-<ID>.md`, set the header lines to:
       **Status:** done
       **Completed:** YYYY-MM-DD

Never do step 3 without step 2. Never do step 2 without step 4.

### 5. Cancel a Task

Same 4 steps, except step 2's `Outcome / artifact` MUST start with
`CANCELLED: <reason>;` and step 4 sets `**Status:** cancelled`.

### 6. Dependency bookkeeping

When T-x is completed, for every active row whose `Depends on` contains T-x,
rewrite that cell as `T-x (done)`. NEVER delete the reference — it is the audit trail.

## Examples

### Creating a Task

```markdown
| T012 | Add rate limiting to API | pending | backend-dev | — | T010 | high | 2025-03-20 |
```

### Transitioning a Task

Before:
```markdown
| T012 | Add rate limiting to API | pending | backend-dev | — | T010 | high | 2025-03-20 |
```

After (T010 completed):
```markdown
| T012 | Add rate limiting to API | in_progress | backend-dev | — | — | high | 2025-03-20 |
```

## Guidelines

- Never skip statuses in the lifecycle (e.g., do not go directly from `pending` to `in_review`).
- Always update both sides of a dependency relationship.
- Archive promptly — do not leave `done` tasks in `active-tasks.md` longer than one checkpoint cycle.
- Use the `dependency-graphing` skill to visualize complex dependency chains.
- Task IDs are immutable once assigned. Never reuse an ID.
