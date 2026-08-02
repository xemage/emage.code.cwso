---
name: "worktree-isolation"
description: "Manage git worktrees for parallel agent work. Use when running batch operations, isolating agent work into separate branches, or managing parallel development streams."
---

# Worktree Isolation

## Purpose

Use git worktrees to give each agent an isolated working directory and branch. This enables parallel development without merge conflicts during active work, and provides clean separation of concerns for batch operations.

## When to Use

- Running batch parallel execution with multiple agents
- Isolating experimental or risky work from the main branch
- Enabling agents to work on independent tasks simultaneously
- Managing parallel feature development streams

## Branch Naming Convention

```
agent/<agent-name>/<task-id>
```

Examples:
- `agent/backend-dev/T021`
- `agent/frontend-dev/T030`
- `agent/qa-agent/T025`

## Procedures

### 1. Create a Worktree for an Agent

```bash
# From the main repository root
git worktree add ../worktrees/<agent-name>-<task-id> -b agent/<agent-name>/<task-id>
```

This creates:
- A new directory at `../worktrees/<agent-name>-<task-id>`
- A new branch `agent/<agent-name>/<task-id>` based on the current HEAD

Example:
```bash
git worktree add ../worktrees/backend-dev-T021 -b agent/backend-dev/T021
```

### 2. Assign File Ownership

Each worktree should have a clear scope of files that the agent is allowed to modify:

```markdown
## Worktree Assignment

- **Worktree:** ../worktrees/backend-dev-T021
- **Branch:** agent/backend-dev/T021
- **Agent:** backend-dev
- **Task:** T021
- **Owned Files:**
  - `src/api/rate-limiter.ts`
  - `src/api/middleware/rate-limit.ts`
  - `tests/api/rate-limiter.test.ts`
- **Read-Only Files:**
  - `src/api/types.ts` (shared types — coordinate changes)
```

Agents MUST NOT modify files outside their ownership scope without coordination.

### 3. Work Within a Worktree

```bash
# Navigate to the worktree
cd ../worktrees/backend-dev-T021

# Work normally — all git operations are scoped to this worktree's branch
git add .
git commit -m "feat(T021): implement token bucket rate limiter"
```

### 4. Merge After Task Completion

Once the task is complete and has passed validation:

```bash
# Switch to the main integration branch
cd /path/to/main/repo
git checkout main

# Merge the agent's branch
git merge agent/backend-dev/T021 --no-ff -m "merge: T021 token bucket rate limiter"

# If conflicts arise, resolve them and commit
```

For batch merges with multiple agents:
```bash
# Merge in dependency order (use dependency graph from dependency-graphing skill)
git merge agent/backend-dev/T020 --no-ff
git merge agent/backend-dev/T021 --no-ff
git merge agent/qa-agent/T022 --no-ff
```

### 5. Clean Up Worktree

After a successful merge:

```bash
# Remove the worktree
git worktree remove ../worktrees/backend-dev-T021

# Delete the branch (it's been merged)
git branch -d agent/backend-dev/T021
```

### 6. List Active Worktrees

```bash
git worktree list
```

Output:
```
/path/to/main/repo          abc1234 [main]
/path/to/worktrees/backend-dev-T021  def5678 [agent/backend-dev/T021]
/path/to/worktrees/qa-agent-T025     ghi9012 [agent/qa-agent/T025]
```

## Example: Batch Parallel Execution Workflow

### Setup Phase

```bash
# Orchestrator creates worktrees for all parallel tasks
git worktree add ../worktrees/backend-dev-T020 -b agent/backend-dev/T020
git worktree add ../worktrees/frontend-dev-T030 -b agent/frontend-dev/T030
git worktree add ../worktrees/qa-agent-T025 -b agent/qa-agent/T025
```

### Execution Phase

Each agent works independently in their worktree:
- `backend-dev` works in `../worktrees/backend-dev-T020/`
- `frontend-dev` works in `../worktrees/frontend-dev-T030/`
- `qa-agent` works in `../worktrees/qa-agent-T025/`

### Merge Phase

```bash
# After all agents report task completion:
cd /path/to/main/repo

# Merge in dependency order
git merge agent/backend-dev/T020 --no-ff
git merge agent/frontend-dev/T030 --no-ff
git merge agent/qa-agent/T025 --no-ff
```

### Cleanup Phase

```bash
git worktree remove ../worktrees/backend-dev-T020
git worktree remove ../worktrees/frontend-dev-T030
git worktree remove ../worktrees/qa-agent-T025

git branch -d agent/backend-dev/T020
git branch -d agent/frontend-dev/T030
git branch -d agent/qa-agent/T025
```

## Guidelines

- Always create worktrees from the main integration branch to ensure a clean base.
- Never share a worktree between agents. One worktree = one agent = one task.
- Merge in dependency order to minimize conflicts (consult the dependency graph).
- Clean up worktrees promptly after merge. Stale worktrees waste disk space and cause confusion.
- If a merge conflict occurs, the orchestrator coordinates resolution — agents should not resolve cross-branch conflicts independently.
- Use `--no-ff` merges to preserve branch history for traceability.
