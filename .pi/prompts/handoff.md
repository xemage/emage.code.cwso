---
description: "Create a structured session handoff for the next agent or human. Captures state, blockers, and next actions using the handoff schema."
argument-hint: "Optional: focus task ID or session summary..."
---

You are in **Session Handoff** mode. Produce a resumable handoff artifact for the next session.

## Instructions

1. **Gather context** — read `docs/tasks/active-tasks.md`, latest `docs/checkpoints/`, and open task briefs.
2. **Summarize state** — current phase, in-progress tasks, blockers, and decisions since last checkpoint.
3. **Draft handoff payload** conforming to `implementation/runtime/handoff/schema-v1.json`:
   - `fromAgent`: current role (usually `orchestrator`)
   - `toAgent`: next agent or `orchestrator` for resume
   - `taskId`: primary active task ID
   - `intent`: `delegate` | `request-review` | `request-data` | `escalate`
   - `payload`: objective, files touched, open questions
   - `constraints`: `writablePaths`, `forbiddenActions`, `maxToolCalls`, `deadlineUtc`
   - `trace`: `checkpointRef`, `correlationId`
4. **Write artifacts**:
   - JSON: `docs/checkpoints/handoff-<handoffId>.json`
   - Human summary: `docs/checkpoints/handoff-<handoffId>.md`
5. **Present to user** — show the markdown summary and ask for approval before treating handoff as final.
6. **On approval** — update checkpoint index; suggest `/discover-skills` or `/team-status` for the resuming session.

## Security

- Never include secrets, tokens, or `.env` contents in handoff payloads.
- Use `forbiddenActions` for destructive operations (see `security-guidelines`).

## Focus

{{input}}
