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
| `docs/tasks/active-tasks.md` | All tasks not yet archived (pending through done) |
| `docs/tasks/completed-tasks.md` | Archived tasks that have been completed and verified |

## Task Table Format

Both files use this table schema:

```markdown
| ID | Title | Status | Assignee | Blocks | BlockedBy | Priority | Created |
|----|-------|--------|----------|--------|-----------|----------|---------|
| TASK-001 | Implement auth module | in_progress | backend-dev | TASK-003 | — | high | 2025-01-15 |
```

### Field Definitions

| Field | Format | Description |
|-------|--------|-------------|
| **ID** | `TASK-NNN` | Zero-padded 3-digit sequential identifier |
| **Title** | Free text | Short, descriptive title |
| **Status** | Enum | One of: `pending`, `in_progress`, `blocked`, `in_review`, `done` |
| **Assignee** | Agent/role name | Who owns this task |
| **Blocks** | Comma-separated IDs | Tasks that cannot proceed until this task is done |
| **BlockedBy** | Comma-separated IDs | Tasks that must complete before this task can start |
| **Priority** | `critical` / `high` / `medium` / `low` | Task priority |
| **Created** | `YYYY-MM-DD` | Date the task was created |

## Status Lifecycle

```
pending → in_progress → blocked → in_progress → in_review → done
                  │                                          │
                  └──────────────────────────────────────────┘
                           (can regress if review fails)
```

Valid transitions:

| From | To | Trigger |
|------|----|---------|
| `pending` | `in_progress` | Agent picks up the task |
| `in_progress` | `blocked` | Dependency or blocker encountered |
| `in_progress` | `in_review` | Implementation complete, ready for validation |
| `blocked` | `in_progress` | Blocker resolved |
| `in_review` | `done` | Validation gate passed |
| `in_review` | `in_progress` | Review rejected, rework needed |

## Procedures

### 1. Create a Task

1. Open `docs/tasks/active-tasks.md`.
2. Determine the next available `TASK-NNN` ID (increment the highest existing ID).
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

### 4. Archive a Task

1. Confirm the task status is `done`.
2. Cut the task row from `active-tasks.md`.
3. Paste it into `completed-tasks.md`, preserving all fields.
4. Remove the task ID from `Blocks`/`BlockedBy` fields of remaining active tasks.
5. Update any dependency graphs.

## Examples

### Creating a Task

```markdown
| TASK-012 | Add rate limiting to API | pending | backend-dev | — | TASK-010 | high | 2025-03-20 |
```

### Transitioning a Task

Before:
```markdown
| TASK-012 | Add rate limiting to API | pending | backend-dev | — | TASK-010 | high | 2025-03-20 |
```

After (TASK-010 completed):
```markdown
| TASK-012 | Add rate limiting to API | in_progress | backend-dev | — | — | high | 2025-03-20 |
```

## Guidelines

- Never skip statuses in the lifecycle (e.g., do not go directly from `pending` to `in_review`).
- Always update both sides of a dependency relationship.
- Archive promptly — do not leave `done` tasks in `active-tasks.md` longer than one checkpoint cycle.
- Use the `dependency-graphing` skill to visualize complex dependency chains.
- Task IDs are immutable once assigned. Never reuse an ID.
