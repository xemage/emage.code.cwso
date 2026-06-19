---
name: "gitlab-management"
description: "Manage GitLab projects, issues, milestones, merge requests, labels, and boards. Use when creating GitLab issues from user stories, setting up milestones for sprints, managing merge request workflows, configuring project labels, or interacting with GitLab API."
---

# GitLab Project Management

Skill for interacting with GitLab to manage projects, issues, milestones, merge requests, and other project management artifacts.

## When to Use
- Creating or managing GitLab projects
- Creating issues from user stories or tasks
- Setting up sprint milestones
- Managing merge request workflows
- Configuring labels and boards
- Querying project status and metrics

## Prerequisites
- GitLab API token available as environment variable `GITLAB_PERSONAL_ACCESS_TOKEN`
- GitLab API URL configured as `GITLAB_API_URL` (defaults to `https://gitlab.com/api/v4`)
- Project ID or path available

## Procedure

### 1. Project Setup
Create a new GitLab project with standard configuration:

```bash
# Create project
curl --header "PRIVATE-TOKEN: $GITLAB_PERSONAL_ACCESS_TOKEN" \
  --data "name=$PROJECT_NAME&description=$PROJECT_DESC&visibility=private&initialize_with_readme=true" \
  "$GITLAB_API_URL/projects"
```

### 2. Label Setup
Create standard labels for the project:

```bash
# Standard label set
LABELS=(
  "type::feature,#428BCA"
  "type::bug,#DC3545"
  "type::task,#6C757D"
  "type::spike,#FFC107"
  "type::docs,#17A2B8"
  "priority::critical,#DC3545"
  "priority::high,#FD7E14"
  "priority::medium,#FFC107"
  "priority::low,#28A745"
  "status::todo,#6C757D"
  "status::in-progress,#007BFF"
  "status::review,#FFC107"
  "status::done,#28A745"
  "status::blocked,#DC3545"
  "team::backend,#9B59B6"
  "team::frontend,#3498DB"
  "team::devops,#E67E22"
  "team::qa,#1ABC9C"
)

for label_info in "${LABELS[@]}"; do
  IFS=',' read -r name color <<< "$label_info"
  curl --header "PRIVATE-TOKEN: $GITLAB_PERSONAL_ACCESS_TOKEN" \
    --data "name=$name&color=$color" \
    "$GITLAB_API_URL/projects/$PROJECT_ID/labels"
done
```

### 3. Milestone Creation
Create milestones for sprints:

```bash
curl --header "PRIVATE-TOKEN: $GITLAB_PERSONAL_ACCESS_TOKEN" \
  --data "title=Sprint 1&description=Sprint goal: MVP core features&start_date=2026-03-30&due_date=2026-04-13" \
  "$GITLAB_API_URL/projects/$PROJECT_ID/milestones"
```

### 4. Issue Creation
Create issues from user stories:

```bash
curl --header "PRIVATE-TOKEN: $GITLAB_PERSONAL_ACCESS_TOKEN" \
  --header "Content-Type: application/json" \
  --data '{
    "title": "Implement user registration API",
    "description": "## User Story\nAs a new user, I want to register an account so that I can access the platform.\n\n## Acceptance Criteria\n- [ ] POST /api/v1/auth/register endpoint\n- [ ] Email validation\n- [ ] Password strength check\n- [ ] Duplicate email prevention\n\n## Story Points: 5\n## Priority: Must Have",
    "labels": "type::feature,priority::high,team::backend",
    "milestone_id": 1
  }' \
  "$GITLAB_API_URL/projects/$PROJECT_ID/issues"
```

### 5. Merge Request Management
```bash
# Create MR
curl --header "PRIVATE-TOKEN: $GITLAB_PERSONAL_ACCESS_TOKEN" \
  --data "source_branch=feature/user-auth&target_branch=develop&title=feat(auth): implement user registration&description=Closes #1&remove_source_branch=true" \
  "$GITLAB_API_URL/projects/$PROJECT_ID/merge_requests"
```

### 6. Query Project Status
```bash
# Open issues by milestone
curl --header "PRIVATE-TOKEN: $GITLAB_PERSONAL_ACCESS_TOKEN" \
  "$GITLAB_API_URL/projects/$PROJECT_ID/issues?milestone=Sprint%201&state=opened"

# MR status
curl --header "PRIVATE-TOKEN: $GITLAB_PERSONAL_ACCESS_TOKEN" \
  "$GITLAB_API_URL/projects/$PROJECT_ID/merge_requests?state=opened"
```

## Reference

See [GitLab API documentation](./references/gitlab-api-reference.md) for detailed API usage.

## Notes
- Always use labels consistently for tracking and board management
- Link issues to milestones for sprint tracking
- Use issue weights for story points
- Close issues with MR references: `Closes #N`

---

## Protocol-Aware Enhancements

### Task Synchronization with `docs/tasks/active-tasks.md`

The canonical task list lives in `docs/tasks/active-tasks.md`. GitLab issues are a secondary projection of this list. The following synchronization rules apply:

**Source of truth:** `docs/tasks/active-tasks.md` is the primary task register. GitLab issues mirror it for visibility and team collaboration.

**Sync procedure:**
1. When a task is added to `active-tasks.md`, create a corresponding GitLab issue with the same ID in the title (e.g., `[TASK-007] Implement auth middleware`).
2. When a task status changes in `active-tasks.md`, update the GitLab issue labels to match.
3. When a task is moved to `completed-tasks.md`, close the corresponding GitLab issue.
4. When a GitLab issue is updated externally (e.g., by a human team member), sync the change back to `active-tasks.md` at the next checkpoint.

**Conflict resolution:** If `active-tasks.md` and a GitLab issue disagree, `active-tasks.md` wins unless the GitLab update was made by a human (identifiable by author).

### Merge Request Linking to Task IDs

Every merge request MUST reference the task ID it addresses:

**MR title format:**
```
feat(scope): description [TASK-{ID}]
```

**MR description format:**
```markdown
## Task Reference
Addresses: TASK-{ID}
Artifact refs: [api-contract-v2, architecture-decision-v1]

## Changes
[Description of changes]

Closes #{gitlab-issue-number}
```

This ensures traceability from task → MR → code changes → artifacts.

### Label Conventions for Task Status

In addition to the standard labels defined above, the following labels are used specifically for protocol-aware task tracking:

| Label | Meaning | Sync Rule |
|-------|---------|-----------|
| `status::todo` | Task not started | Maps to `pending` in active-tasks.md |
| `status::in-progress` | Task actively being worked | Maps to `in-progress` in active-tasks.md |
| `status::review` | Task complete, awaiting review | Maps to `review` in active-tasks.md |
| `status::blocked` | Task blocked by a dependency or blocker | Maps to `blocked` in active-tasks.md; must have a linked `[BLOCKER]` |
| `status::done` | Task complete and verified | Triggers move to completed-tasks.md |
| `gate::pending` | Awaiting validation gate result | Used for tasks gated on code-review or QA verdict |
| `gate::passed` | Validation gate passed | Gate cleared; proceed with merge |
| `gate::failed` | Validation gate failed | Gate blocked; requires remediation |
