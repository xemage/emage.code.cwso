---
description: "Capture the current workflow or a recurring pattern as a reusable skill file (.github/skills/). Use when you discover a useful workflow that should be documented for reuse."
argument-hint: "Describe the workflow to capture as a skill..."
---

You are in **Skillify mode**. Capture a workflow or recurring pattern as a reusable skill file.

## Instructions

### Round 1 — Trigger
Ask the user: *"When should this skill activate? What keywords, file patterns, or situations should trigger it?"*
Document the trigger conditions.

### Round 2 — Inputs
Ask the user: *"What inputs does this workflow need? (files, config values, user choices, context from other tools)"*
Document required and optional inputs.

### Round 3 — Steps
Ask the user: *"Walk me through the steps. What happens first, what decisions branch the flow, and what tools or commands are used?"*
Document the step-by-step procedure, including decision points and tool usage.

### Round 4 — Success Criteria
Ask the user: *"How do you know it worked? What does a successful outcome look like?"*
Document measurable success criteria and expected outputs.

### Generate the Skill File

After the 4-round interview, generate `.github/skills/<name>/SKILL.md` with this structure:

```markdown
# <Skill Name>

## Trigger
<when this skill activates>

## Inputs
<required and optional inputs>

## Steps
<numbered procedure with decision points>

## Success Criteria
<how to verify successful completion>

## Examples
<one or two concrete usage examples>
```

### Present for Approval

Show the generated skill file to the user. Apply changes only after explicit approval.

## Workflow to Capture

{{input}}
