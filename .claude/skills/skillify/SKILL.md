---
name: "skillify"
description: "Capture a workflow or recurring pattern as a reusable skill file. Use when a useful workflow emerges that should be documented for reuse."
---

# Skillify

## Purpose

Capture emerging workflows, recurring patterns, or successful procedures as reusable skill files. Skills are self-contained instruction sets that any agent can follow to perform a specific task consistently.

## When to Use

- A workflow has been performed successfully 2+ times and is likely to recur
- A complex procedure needs to be standardized across agents
- The user or orchestrator identifies a pattern worth preserving
- After a post-mortem reveals a process that should be formalized

## File Location

```
.github/skills/<skill-name>/SKILL.md
```

## Procedure: The 4-Round Interview

Skillification follows a structured 4-round interview to extract all necessary information.

### Round 1: Trigger & Name

Ask and determine:
- **What triggers this workflow?** (What situation or request initiates it?)
- **What should the skill be named?** (Short, kebab-case identifier)
- **One-line description:** What does this skill do?

Output:
```yaml
---
name: <skill-name>
description: "<one-line description>"
---
```

### Round 2: Inputs & Context

Ask and determine:
- **What inputs does this skill need?** (Files, data, parameters)
- **What context must be available?** (Project state, prerequisites)
- **What tools or permissions are required?**

Output:
```markdown
## Prerequisites
- <input 1>
- <input 2>

## Required Context
- <context item>
```

### Round 3: Step-by-Step Procedure

Ask and determine:
- **Walk through the workflow step by step.** (Every action, decision point, and output)
- **What are the decision points?** (If X, do Y; otherwise do Z)
- **What are the outputs at each step?**

Output:
```markdown
## Procedure

### 1. <First Step>
<detailed instructions>

### 2. <Second Step>
<detailed instructions>

...
```

### Round 4: Success Criteria & Edge Cases

Ask and determine:
- **How do you know the skill executed successfully?**
- **What can go wrong?** (Edge cases, failure modes)
- **What should happen when it fails?**

Output:
```markdown
## Success Criteria
- <criterion 1>
- <criterion 2>

## Edge Cases
| Scenario | Handling |
|----------|----------|
| ... | ... |
```

## SKILL.md Template

After completing all 4 rounds, generate the skill file:

```markdown
---
name: <skill-name>
description: "<one-line description>"
---

# <Skill Title>

## Purpose
<2-3 sentences explaining what this skill does and why it exists>

## When to Use
- <trigger 1>
- <trigger 2>
- <trigger 3>

## Prerequisites
- <input/context requirement>

## Procedure

### 1. <Step Title>
<instructions>

### 2. <Step Title>
<instructions>

### 3. <Step Title>
<instructions>

## Examples

### <Example Name>
<walkthrough of the skill applied to a concrete scenario>

## Edge Cases

| Scenario | Handling |
|----------|----------|
| <edge case> | <what to do> |

## Guidelines
- <guideline 1>
- <guideline 2>
- <guideline 3>
```

## Review and Approval

After generating the skill file:

1. Present the draft to the user:
   ```
   I've drafted a new skill: **<skill-name>**
   
   **Purpose:** <purpose>
   **Location:** `.github/skills/<skill-name>/SKILL.md`
   
   Key steps:
   1. <step summary>
   2. <step summary>
   3. <step summary>
   
   Would you like to:
   1. ✅ Approve and save
   2. ✏️ Revise (specify changes)
   3. ❌ Discard
   ```

2. If approved, write the file to the skills directory.
3. If revised, incorporate feedback and re-present.
4. If discarded, note the reason for future reference.

## Examples

### Skillifying a Database Migration Workflow

**Round 1:**
- Trigger: Need to run database schema migrations
- Name: `db-migration`
- Description: "Execute database schema migrations safely with rollback support."

**Round 2:**
- Inputs: migration SQL files, target database connection
- Context: current schema version, migration history
- Tools: database client, schema version tracker

**Round 3:**
1. Check current schema version
2. Identify pending migrations
3. Create a backup/snapshot
4. Execute migrations in order
5. Verify schema state
6. Update version tracker

**Round 4:**
- Success: all migrations applied, schema matches expected state
- Edge cases: migration fails mid-batch (rollback to snapshot), concurrent migration attempts (acquire lock first)

## Guidelines

- Skills should be self-contained. An agent unfamiliar with the project should be able to follow a skill.
- Prefer specificity over generality. A skill that does one thing well is better than a vague multi-purpose skill.
- Include concrete examples. Abstract procedures are harder to follow correctly.
- Keep skills updated. If a workflow changes, update the skill file.
- Skill names use kebab-case: `task-management`, `checkpoint-protocol`, `db-migration`.
- Every skill must have the YAML frontmatter with `name` and `description`.
