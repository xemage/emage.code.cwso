---
name: "project-planning"
description: "Create structured project plans, work breakdown structures (WBS), milestone timelines, feature decomposition, and sprint roadmaps. Use when starting a new project, planning a major feature, or creating a development roadmap."
---

# Project Planning

Skill for creating comprehensive project plans from high-level ideas, including work breakdown structures, milestone timelines, and sprint roadmaps.

## When to Use
- Starting a new project from an idea
- Planning a major feature or epic
- Creating a development roadmap
- Breaking down complex work into manageable tasks

## Procedure

### 1. Idea Analysis
Analyze the project idea and extract:
- Core problem being solved
- Target users/audience
- Key features and capabilities
- Constraints (time, budget, technology)
- Success criteria

### 2. Work Breakdown Structure (WBS)
Decompose the project into a hierarchy:

```markdown
# WBS: [Project Name]

## 1. Project Setup
  1.1 Repository & CI/CD setup
  1.2 Development environment
  1.3 Architecture design
  1.4 Database schema design

## 2. Core Features (MVP)
  2.1 [Feature Area 1]
    2.1.1 [Sub-feature / Task]
    2.1.2 [Sub-feature / Task]
  2.2 [Feature Area 2]
    2.2.1 [Sub-feature / Task]

## 3. Extended Features
  3.1 [Feature Area]

## 4. Quality & Security
  4.1 Testing implementation
  4.2 Security audit
  4.3 Performance optimization

## 5. Documentation
  5.1 API documentation
  5.2 User guide
  5.3 Architecture docs

## 6. Deployment & Release
  6.1 Staging deployment
  6.2 Production deployment
  6.3 Monitoring setup
```

### 3. Milestone Planning
Map WBS items to time-boxed milestones:

```markdown
# Milestone Plan

## Milestone 1: Foundation (Week 1-2)
- Goal: Project infrastructure and architecture ready
- Deliverables:
  - [ ] Repository with CI/CD pipeline
  - [ ] Architecture document
  - [ ] Database schema v1
  - [ ] Development environment runnable

## Milestone 2: MVP Core (Week 3-6)
- Goal: Core features functional
- Deliverables:
  - [ ] [Core Feature 1] complete with tests
  - [ ] [Core Feature 2] complete with tests
  - [ ] Basic UI functional

## Milestone 3: MVP Complete (Week 7-8)
- Goal: MVP ready for internal testing
- Deliverables:
  - [ ] All MVP features integrated
  - [ ] 80%+ test coverage
  - [ ] Security audit passed
  - [ ] Documentation complete

## Milestone 4: Release (Week 9-10)
- Goal: Production deployment
- Deliverables:
  - [ ] Staging validated
  - [ ] Production deployed
  - [ ] Monitoring active
  - [ ] User documentation published
```

### 4. Sprint Backlog Generation
Convert milestones into 2-week sprints with estimated tasks:

```markdown
# Sprint 1: Foundation

## Sprint Goal
Set up project infrastructure and complete architecture design.

## Tasks
| ID | Task | Assignee | Points | Priority |
|----|------|----------|--------|----------|
| 1 | Create GitLab project and CI/CD | DevOps | 3 | Must |
| 2 | Design system architecture | Architect | 8 | Must |
| 3 | Design database schema | DB Engineer | 5 | Must |
| 4 | Set up dev environment (Docker) | DevOps | 3 | Must |
| 5 | Create initial project structure | Tech Lead | 2 | Must |
| 6 | Write project README | Tech Writer | 2 | Should |

## Capacity: 30 points
## Committed: 23 points
```

### 5. Risk Assessment
```markdown
# Risk Register

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| [Risk 1] | High/Med/Low | High/Med/Low | [Strategy] |
```

## Templates
See the planning templates in [.github/skills/project-planning/references/](./references/) for reusable planning templates.

## Output
The skill produces:
1. Project vision summary
2. Work Breakdown Structure
3. Milestone timeline
4. Sprint backlogs with estimated tasks
5. Risk register
6. RACI matrix (who does what)

---

## Protocol-Aware Enhancements

### Plan-Approve-Execute Protocol Reference

All plans produced by this skill feed into the **plan-approve-execute** protocol. The workflow is:

1. **Plan** — This skill produces the plan document (WBS, milestones, sprint backlogs).
2. **Approve** — The plan is submitted for review. Approval is a validation gate that produces a verdict:
   ```
   [VERDICT] gate=plan-review | result=PASS|CONDITIONAL_PASS|FAIL | reviewer={role} | artifact_ref=plan-v{N} | date={YYYY-MM-DD}
   ```
3. **Execute** — Only after a `PASS` or `CONDITIONAL_PASS` verdict does execution begin. `CONDITIONAL_PASS` items become tracked tasks.

**No work begins without an approved plan.** If a plan is rejected (`FAIL`), it must be revised and resubmitted.

### Plan Document Format and Versioning

Plan documents are versioned artifacts stored under `docs/plans/`:

```
docs/plans/project-plan-v1.md
docs/plans/project-plan-v2.md
```

**Plan document structure:**
```markdown
# Project Plan v{N}

## Version: {N}
## Date: {YYYY-MM-DD}
## Status: draft | approved | superseded

## Changes from v{N-1}
- [List of changes]

## Vision Summary
[1-2 paragraphs]

## Work Breakdown Structure
[WBS content]

## Milestone Timeline
[Milestone content]

## Sprint Backlogs
[Sprint content]

## Risk Register
[Risk content]

## Approval
- Reviewer: {role}
- Verdict: PASS | CONDITIONAL_PASS | FAIL
- Conditions (if CONDITIONAL_PASS): [list]
```

### Task Decomposition to `docs/tasks/active-tasks.md`

When a plan is approved, the sprint backlog tasks MUST be decomposed into entries in `docs/tasks/active-tasks.md`. This is the canonical task register consumed by all agents and synced to GitLab.

**Decomposition procedure:**

1. For each task in the approved sprint backlog, create an entry in `active-tasks.md`:
   ```markdown
   ### T{ID}: {Title}
   - **Status:** pending
   - **Assignee:** {role}
   - **Priority:** must | should | could
   - **Points:** {N}
   - **Sprint:** {sprint-name}
   - **Depends on:** [T{ID}, ...] or none
   - **Artifact refs:** [{artifact-version}, ...]
   - **Created:** {YYYY-MM-DD}
   ```

2. Assign sequential TASK IDs continuing from the last used ID.
3. Record dependency links between tasks (e.g., frontend task depends on API contract task).
4. Sync each new task to a GitLab issue (see gitlab-management skill).

**Task lifecycle:**
- `pending` → `in-progress` → `review` → `done` (moved to `completed-tasks.md`)
- `pending` → `blocked` (when a `[BLOCKER]` is raised) → `in-progress` (when blocker resolved)
