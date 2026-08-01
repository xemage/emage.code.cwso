---
description: "Use when working with Git: committing, branching, merging, creating merge requests, managing release branches, or working with agent worktrees. Covers GitFlow branching strategy, conventional commits, and worktree lifecycle."
applyTo: "**"
---

# Git Workflow

## Branching Strategy (GitFlow)

```
main         ← production releases only (protected)
develop      ← integration branch (protected)
feature/*    ← new features (from develop)
bugfix/*     ← bug fixes (from develop)
release/*    ← release stabilization (from develop → main)
hotfix/*     ← emergency fixes (from main → main + develop)
```

### Branch Naming
```
feature/42-user-authentication
bugfix/58-fix-pagination
release/v1.2.0
hotfix/v1.2.1
```
Format: `type/[issue-number]-short-description`

## Agent Worktree Branch Naming

Agents operate in isolated worktrees. Agent branches follow a dedicated naming convention:

```
agent/<agent-name>/<task-id>
```

### Examples
```
agent/backend-engineer/T042
agent/frontend-engineer/T058
agent/security-engineer/T101
agent/devops-engineer/T033
```

### Rules
- `<agent-name>` is the kebab-case agent role name
- `<task-id>` matches the task identifier from the task management system
- Agent branches are always created from `develop`
- Agent branches merge back to `develop` via merge request after review

## Worktree Lifecycle

Each agent task follows a strict worktree lifecycle:

### 1. Create
```bash
git worktree add ../worktrees/agent-<name>-<task-id> -b agent/<agent-name>/<task-id> develop
```
- Create a new worktree from `develop`
- One worktree per agent per task — no sharing

### 2. Work
- Agent performs all implementation within its worktree
- Commits follow conventional commit format (see below)
- Agent must not modify files outside its assigned scope

### 3. Merge
- Agent signals task completion
- Tech Lead or Orchestrator reviews the worktree diff
- Merge to `develop` via squash-and-merge or standard merge
- All CI checks must pass before merge

### 4. Cleanup
```bash
git worktree remove ../worktrees/agent-<name>-<task-id>
git branch -d agent/<agent-name>/<task-id>
```
- Remove worktree directory after successful merge
- Delete the agent branch
- Never leave stale worktrees — cleanup is mandatory

## Commit Messages (Conventional Commits)

```
type(scope): description

[optional body]

[optional footer: Refs #issue]
```

### Conventional Commit Format Reference

The format is based on the [Conventional Commits 1.0.0](https://www.conventionalcommits.org/) specification:

- **type**: Required. One of the types listed below.
- **scope**: Optional. The module, feature, or area affected (e.g., `auth`, `api`, `ui`).
- **description**: Required. Imperative, lowercase, no period at the end.
- **body**: Optional. Explain *what* and *why*, not *how*. Wrap at 72 characters.
- **footer**: Optional. Reference issues, breaking changes (`BREAKING CHANGE:`).

### Types
| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `style` | Formatting, no code change |
| `refactor` | Code change that doesn't fix or add |
| `test` | Adding or fixing tests |
| `ci` | CI/CD changes |
| `chore` | Maintenance (deps, config) |
| `perf` | Performance improvement |
| `revert` | Revert a previous commit |

### Examples
```
feat(auth): add OAuth2 login flow

Implement Google and GitHub OAuth2 providers.
Includes token refresh and session management.

Refs #42
```

```
fix(api): prevent duplicate user registration

Add unique constraint check before insert to avoid
race condition on concurrent registrations.

Closes #58
```

## Merge Request Rules
- Always create MR from feature → develop
- Require at least 1 approval
- All CI checks must pass
- Branch must be up-to-date with target
- Use "Squash and merge" for feature branches
- Delete source branch after merge
