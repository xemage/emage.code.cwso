---
description: "Plan-only mode: produce a task decomposition, dependency graph, and plan document without executing. Use this to review and refine a plan before committing to execution."
argument-hint: "Describe what you want to plan..."
---

You are in **Plan-Only mode**. Execute ONLY the **Plan** phase of the Plan-Approve-Execute workflow. Do NOT proceed to execution.

## Instructions

1. **Analyse the request** — break the goal into discrete tasks with clear acceptance criteria.
2. **Build a dependency graph** — identify which tasks block others. Render the graph as a Mermaid diagram.
3. **Assign resources** — map each task to the most appropriate agent (backend, frontend, qa, devops, docs, or yourself).
4. **Assess risks** — for each task, note complexity (S/M/L), likelihood of rework, and any unknowns.
5. **Write the plan document** — save to `docs/plans/<slug>-plan.md` using this structure:
   - **Goal** — one-sentence summary
   - **Task Decomposition** — numbered list with acceptance criteria
   - **Dependency Graph** — Mermaid flowchart
   - **Resource Assignments** — table of task → agent
   - **Risk Assessment** — table of task → complexity, risk, mitigation
   - **Open Questions** — anything that needs clarification before execution
6. **Present the plan for review** — output a summary and wait for user approval. Do NOT execute any tasks.

## Important

- Stop after the plan is written and presented. Execution requires explicit user approval.
- If the request is ambiguous, list your assumptions in the Open Questions section.
- Keep task granularity small enough that each task can be completed in a single agent session.

## Request

{{input}}
