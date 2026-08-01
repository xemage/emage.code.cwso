---
description: "Decompose a sweeping change into independent units for parallel execution across worktrees. Use for large refactors, migrations, or cross-cutting changes."
argument-hint: "Describe the change to batch-process..."
---

You are in **Batch Processing mode**. Decompose a large change into independent units that can be worked on in parallel across isolated worktrees.

## Instructions

1. **Analyse the change** — understand the full scope and identify natural boundaries (by module, feature, layer, or file group).
2. **Decompose into independent units** — each unit must be:
   - Self-contained (no cross-unit dependencies within a batch)
   - Independently testable
   - Small enough for a single agent session
3. **Create worktree isolation** — for each unit:
   - Create a dedicated worktree and branch using the worktree-isolation skill
   - Branch naming: `batch/<slug>/<unit-number>-<short-description>`
4. **Delegate to agents** — assign each unit to the appropriate agent (backend, frontend, qa, devops, docs) with clear instructions and acceptance criteria.
5. **Track progress** — update `docs/tasks/active-tasks.md` with the full batch manifest:
   - Unit ID, description, assigned agent, branch, status
6. **Collect results** — as units complete, verify each passes its acceptance criteria.
7. **Guide PR creation** — once all units are verified:
   - Help the user create a PR for each branch
   - Suggest a merge order that respects any integration constraints
   - Recommend a final integration test after all merges

## Important

- Never modify the main branch directly. All work happens in worktree branches.
- If a unit turns out to have a dependency on another unit, flag it immediately and re-plan.
- Present the decomposition plan for user approval before creating worktrees.

## Change Description

{{input}}
