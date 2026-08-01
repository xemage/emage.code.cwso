---
name: "checkpoint-protocol"
description: "Write, compress, and manage project checkpoints for context preservation. Use when saving progress, resuming work after context loss, compressing old checkpoints, or generating resume briefs."
---

# Checkpoint Protocol

## Purpose

Preserve project context across sessions and context window boundaries. Checkpoints capture progress, decisions, and state so that work can resume without loss after context resets or agent handoffs.

## When to Use

- At every phase boundary (e.g., completing Phase 0, starting Phase 1)
- After every 3–5 task completions
- Before major delegation to a sub-agent
- Before anticipated context window exhaustion
- When explicitly requested by the orchestrator or user

## File Location

```
docs/checkpoints/checkpoint-<N>.md
```

Where `<N>` is a sequential integer starting from `001`.

## Checkpoint Format

```markdown
# Checkpoint <N>

**Date:** YYYY-MM-DD HH:MM
**Author:** <agent-name or orchestrator>
**Phase:** <current phase>

## Progress Summary
<2-4 sentence summary of what was accomplished since the last checkpoint>

## Completed Tasks
| ID | Title | Completed |
|----|-------|-----------|
| TNNN | ... | YYYY-MM-DD |

## Active Tasks
| ID | Title | Status | Assignee |
|----|-------|--------|----------|
| TNNN | ... | in_progress | ... |

## Active Blockers
| Blocker ID | Type | Severity | Impacted Tasks | Description |
|------------|------|----------|----------------|-------------|
| BLK-NNN | ... | ... | ... | ... |

## Key Decisions
- **Decision:** <what was decided>
  **Rationale:** <why>
  **Alternatives Rejected:** <what was considered and why it was rejected>

## Next Steps
1. <immediate next action>
2. <following action>
3. <subsequent action>

## Token Spend
- **Estimated tokens used this session:** <number or range>
- **Estimated remaining context:** <percentage or tokens>
```

## Procedures

### 1. Write a Checkpoint

1. Determine the next checkpoint number by listing `docs/checkpoints/` and incrementing the highest `<N>`.
2. Fill in every section of the checkpoint format above.
3. For "Completed Tasks," query `active-tasks.md` for tasks marked `done` since the last checkpoint.
4. For "Active Blockers," query any open blocker reports.
5. For "Key Decisions," review conversation history for architectural or design decisions made.
6. Write the file to `docs/checkpoints/checkpoint-<N>.md`.

### 2. Compress Old Checkpoints

To keep the checkpoint directory manageable and reduce token cost when loading context:

1. **Keep the latest 2 checkpoints in full.**
2. For checkpoints older than the latest 2:
   - Extract only: Progress Summary, Key Decisions, and a one-line list of completed task IDs.
   - Rewrite the file as a compressed checkpoint.
3. Compressed format:

```markdown
# Checkpoint <N> (Compressed)

**Date:** YYYY-MM-DD
**Phase:** <phase>

## Summary
<original progress summary>

## Key Decisions
- <decision 1>
- <decision 2>

## Completed Tasks
T001, T002, T003
```

### 3. Generate a Resume Brief

When resuming after context loss:

1. Read the latest 2 full checkpoints.
2. Read compressed summaries of all older checkpoints.
3. Read `docs/tasks/active-tasks.md` for current task state.
4. Read `AGENTS.md` for project conventions.
5. Synthesize a resume brief:

```markdown
# Resume Brief

## Project State
<1-2 sentence overall status>

## Current Phase
<phase name and description>

## Active Work
| ID | Title | Status | Assignee |
|----|-------|--------|----------|
| ... | ... | ... | ... |

## Unresolved Blockers
- <blocker summary>

## Immediate Next Steps
1. <step>
2. <step>

## Key Context
- <critical decision or convention to remember>
```

## Examples

### Checkpoint Trigger: Phase Boundary

After completing Phase 0 (Foundation):
- Write `checkpoint-001.md` covering all Phase 0 work.
- Include decisions like technology choices, architecture patterns selected.
- Next steps should reference Phase 1 starting tasks.

### Checkpoint Trigger: Mid-Phase Progress

After completing T005, T006, T007 during Phase 1:
- Write `checkpoint-003.md` covering those task completions.
- Note any blockers encountered and resolved.

## Guidelines

- Never skip a checkpoint at a phase boundary — this is mandatory.
- Checkpoints are append-only until compression. Do not edit a checkpoint after the next one is written.
- Compression is irreversible. Ensure the latest 2 are always preserved in full.
- Resume briefs are ephemeral — generate them on demand, do not store them.
- Include token spend estimates to help plan future context windows.
- If unsure whether to checkpoint, checkpoint. Over-checkpointing is preferable to context loss.
