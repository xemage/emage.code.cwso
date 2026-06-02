---
name: "blocker-escalation"
description: "Handle and escalate blockers using the structured escalation protocol. Use when an agent is blocked, needs to report a blocker, or when the orchestrator needs to route/resolve blockers."
---

# Blocker Escalation Protocol

## Purpose

Provide a structured process for reporting, routing, and resolving blockers. Ensure that blocked work is surfaced quickly and escalated to the right party when self-resolution fails.

## When to Use

- An agent encounters an obstacle that prevents task completion
- A task transitions to `blocked` status
- The orchestrator needs to triage and route a blocker
- Two self-resolution attempts have failed and user intervention is needed

## Blocker Types

| Type | Description | First Responder |
|------|-------------|-----------------|
| `technical` | Code error, build failure, environment issue | Implementing agent retries, then Tech Lead |
| `dependency` | Blocked by another task or external deliverable | Orchestrator re-prioritizes or reassigns |
| `unclear_requirements` | Ambiguous spec, missing acceptance criteria | Orchestrator asks user for clarification |
| `external` | Third-party API down, license issue, infrastructure | Orchestrator escalates to user immediately |

## Blocker Report Format

When filing a blocker, create a report with this structure:

```markdown
## Blocker Report

- **Blocker ID:** BLK-NNN
- **Type:** technical | dependency | unclear_requirements | external
- **Severity:** critical | high | medium | low
- **Reporter:** <agent-name>
- **Date:** YYYY-MM-DD
- **Impacted Tasks:** TASK-NNN, TASK-MMM
- **Description:** <clear description of what is blocked and why>
- **What Was Tried:**
  1. <attempt 1 and result>
  2. <attempt 2 and result>
- **Suggested Resolution:** <proposed fix or action>
- **Retry Count:** <0 | 1 | 2>
```

### Severity Definitions

| Severity | Criteria |
|----------|----------|
| `critical` | Blocks the critical path; no workaround; project timeline at risk |
| `high` | Blocks multiple tasks or a high-priority task; workaround is costly |
| `medium` | Blocks a single non-critical task; workaround available |
| `low` | Minor inconvenience; does not block progress |

## Escalation Path

```
Agent (self-resolve, up to 2 retries)
  │
  ▼ (retry count ≥ 2 OR severity = critical)
Orchestrator (triage, route, attempt resolution)
  │
  ▼ (orchestrator cannot resolve OR type = unclear_requirements/external)
User (final decision-maker)
```

## Procedures

### 1. Agent Reports a Blocker

1. Attempt to self-resolve the issue (retry count = 1).
2. If the first attempt fails, try an alternative approach (retry count = 2).
3. If both attempts fail, create a Blocker Report.
4. Transition the impacted task(s) to `blocked` status in `active-tasks.md`.
5. Send the Blocker Report to the orchestrator.

### 2. Orchestrator Triages a Blocker

1. Review the Blocker Report.
2. Route based on type:

| Type | Routing Action |
|------|---------------|
| `technical` | Assign to Tech Lead or a different agent with relevant expertise |
| `dependency` | Check the blocking task's status; re-prioritize or reassign if needed |
| `unclear_requirements` | Formulate a specific question and escalate to user |
| `external` | Escalate to user with impact assessment |

3. If the orchestrator can resolve directly (e.g., re-ordering tasks), do so and close the blocker.
4. If not, escalate to user.

### 3. Escalate to User

Present the blocker to the user with full context:

```
🚧 **Blocker Escalation**

**ID:** BLK-NNN
**Type:** <type>
**Severity:** <severity>
**Impacted Tasks:** <list>

**Problem:** <description>

**What was tried:**
1. <attempt 1>
2. <attempt 2>

**Suggested resolution:** <suggestion>

**Action needed:** <specific question or decision for user>
```

### 4. Resolve and Close a Blocker

1. Apply the resolution.
2. Transition impacted tasks from `blocked` back to `in_progress`.
3. Add a resolution note to the blocker report:

```markdown
- **Resolution:** <what was done>
- **Resolved By:** <agent or user>
- **Resolved Date:** YYYY-MM-DD
```

4. Update the checkpoint if the blocker was significant.

## The 2-Retry Rule

- Agents MUST attempt self-resolution at least once before escalating.
- Agents SHOULD attempt a second approach if the first fails.
- After 2 failed attempts, escalation is MANDATORY. Do not continue retrying the same approaches.
- Exception: `critical` severity blockers or `external` type blockers may be escalated immediately after 1 attempt.

## Examples

### Technical Blocker

```markdown
- **Blocker ID:** BLK-005
- **Type:** technical
- **Severity:** high
- **Reporter:** backend-dev
- **Impacted Tasks:** TASK-021
- **Description:** Token bucket implementation fails under concurrent access. Race condition in counter decrement.
- **What Was Tried:**
  1. Added mutex lock — deadlock under high load
  2. Switched to atomic operations — still inconsistent at >1000 rps
- **Suggested Resolution:** Use Redis-based distributed rate limiting instead of in-memory
- **Retry Count:** 2
```

### Unclear Requirements Blocker

```markdown
- **Blocker ID:** BLK-008
- **Type:** unclear_requirements
- **Severity:** medium
- **Reporter:** frontend-dev
- **Impacted Tasks:** TASK-030
- **Description:** Design spec does not specify behavior when user has no profile photo. Should we show initials, a default avatar, or leave blank?
- **What Was Tried:**
  1. Checked design system docs — no guidance
  2. Reviewed similar features in codebase — inconsistent patterns
- **Suggested Resolution:** Use initials derived from user's name as default
- **Retry Count:** 2
```

## Guidelines

- Always include what was tried. "I'm blocked" is not a valid report without attempted solutions.
- Severity should reflect actual project impact, not frustration level.
- The orchestrator should never sit on a blocker. Triage within the same session.
- Resolved blockers should inform future work — if a pattern causes repeated blockers, escalate it as a process improvement.
- Blocker IDs use the format `BLK-NNN` and are sequential, independent of task IDs.
