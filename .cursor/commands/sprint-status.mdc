---
description: "Get a sprint status report showing progress, blockers, completed and remaining work."
agent: "orchestrator"
---

Generate a sprint status report for the current sprint:

## Sprint Metrics

1. **Completed Items**: List all done tasks with story points
2. **In Progress**: Current work with % complete estimate
3. **Remaining**: Not started items with priorities
4. **Blocked**: Any blocked items and why
5. **Velocity**: Points completed vs committed
6. **Risks**: Any at-risk items for the sprint goal
7. **Burndown**: Current trajectory (on track / behind / ahead)

## Task DAG Visualization

8. **Produce a task dependency graph** (Mermaid format):

```mermaid
graph TD
    TASK-001[Task 001: Description] -->|depends on| TASK-002[Task 002: Description]
    TASK-003[Task 003: Description] -->|blocked by| TASK-004[Task 004: Description]
    
    classDef done fill:#90EE90
    classDef inProgress fill:#FFD700
    classDef blocked fill:#FF6347
    classDef pending fill:#D3D3D3
    
    class TASK-001 done
    class TASK-002 inProgress
    class TASK-003 blocked
    class TASK-004 pending
```

- Read from `docs/tasks/active-tasks.md` for current task states
- Color code: done=green, in_progress=yellow, blocked=red, pending=gray

## Checkpoint Metrics

9. **Summarize checkpoint data since last sprint status**:
   - Checkpoints written this sprint: <count>
   - Phase transitions completed: [list]
   - Validation gates passed/failed: [list]
   - Artifact versions produced: [list]

## Token Spend Summary

10. **Aggregate spend telemetry for the sprint**:
    - Total tokens used this sprint
    - Projected remaining budget
    - Variance from plan (over/under by %)
    - Per-phase breakdown if available

Provide actionable recommendations for any issues found.
