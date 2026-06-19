---
description: "Start a new software project from an idea. Triggers the full orchestration workflow: planning, architecture, setup, development, testing, and deployment."
argument-hint: "Describe your project idea in detail..."
---

I have a new project idea. Please orchestrate the full team to turn this into a real software product:

## Phase 1: Plan-Approve-Execute

1. **Create a plan document** with task decomposition and dependency graph
   - Write the plan to `docs/plans/plan-<project-slug>.md`
   - Include: objective, scope, task DAG (Mermaid), risk assessment, estimated phases
   - Reference protocol: `04-protocols.md § Plan-Approve-Execute`
2. **Present the plan for my approval before executing**
   - Show the plan summary, task count, estimated phases, and identified risks
   - Wait for explicit approval (`APPROVED`, `APPROVED_WITH_CHANGES`, or `REJECTED`)
   - Do NOT proceed to execution until approval is received

## Phase 2: Requirements & Architecture

3. Analyze the idea and ask any clarifying questions
4. Have the Product Owner create requirements and user stories
5. Have the Solution Architect design the system
6. Have the Scrum Master plan sprints

## Phase 3: Setup & Execution

7. Set up the project with DevOps
8. Run an Architecture Briefing before parallel development
9. Coordinate development, testing, and deployment
10. Enforce task status callbacks for all delegated work
11. Run an Integration Checkpoint before QA execution

## Phase 4: Reporting & Governance

12. Report blockers and escalations explicitly
13. Use progressive context loading for delegations (only objective, dependency slice, artifact refs, blockers)
14. After each phase, publish a compact checkpoint summary:
	- `[CHECKPOINT] id=<phase_or_gate> | done=[...] | in_flight=[...] | blocked=[...] | decisions=[...] | artifact_refs=[...] | next=[...]`
	- Write checkpoint to `docs/checkpoints/checkpoint-<phase>.md`
15. Apply per-phase token budgets, model routing by complexity, and caching of repeated references
16. Include spend telemetry (`used`, `projected`, `variance`) in each phase checkpoint

## Phase 5: Validation & Artifacts

17. Run validation gates before phase transitions (reference: `04-protocols.md § Validation Gates`)
18. Produce versioned artifacts for all deliverables:
    - Format: `<artifact-name>-v<major>.<minor>.md`
    - Store in `docs/artifacts/`
    - Include artifact manifest in checkpoint summaries
19. Build an initial task dependency graph with lifecycle states
    - Track states: `pending → in_progress → review → done` (or `blocked`)
    - Write active tasks to `docs/tasks/active-tasks.md`

Here's my idea:

{{input}}
