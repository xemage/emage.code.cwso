---
description: "Create a structured bug report with reproduction steps, expected vs actual behavior, and severity assessment."
argument-hint: "Describe the bug you found..."
---

Create a structured bug report for the following issue:

{{input}}

Use this format:
## Bug Report

### Title
[Clear, descriptive title]

### Severity
[Critical | High | Medium | Low] — with justification

### Environment
[Identify from context: OS, browser, version, environment]

### Steps to Reproduce
1. [Precise step 1]
2. [Precise step 2]
3. ...

### Expected Behavior
[What should happen]

### Actual Behavior
[What actually happens]

### Root Cause Analysis
[Investigate the codebase to identify the likely cause]

### Suggested Fix
[Recommend a fix approach based on root cause analysis]

### Related Files
[List relevant source files]

### Task Creation

After creating the bug report:
1. **Create a TASK entry** for tracking this bug:
   - Add to `docs/tasks/active-tasks.md` with state `pending`
   - Assign severity-appropriate priority
   - Use the NEXT sequential `T<NNN>` ID. NEVER invent a `BUG-` prefix — non-`T` rows are silently deleted by `install.sh --update`.
   - Format (7 columns, exact order):
     `| T<NNN> | BUG: <title> | <owner-slug> | pending | P0\|P1\|P2 | <dep-ids or —> | YYYY-MM-DD |`
   - Map severity → priority: critical→P0, high→P0, medium→P1, low→P2
2. Reference any related task IDs or artifact versions affected

### Blocker Classification

3. **Assess blocker impact**:
   - Does this bug block any in-flight tasks? List blocked task IDs
   - Does this bug block a release gate? Flag with `[RELEASE_BLOCKER]`
   - Does this bug block other team members? Flag with `[TEAM_BLOCKER]`
   - If blocking, escalate severity and recommend immediate triage
