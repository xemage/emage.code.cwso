---
description: "Start a proof-of-concept project focused on hypothesis validation and rapid delivery."
agent: "poc-orchestrator"
argument-hint: "Describe the hypothesis and what you need to prove..."
---

I want to start a proof-of-concept project.

## Phase 0: Lightweight Plan

1. **Create a lightweight PoC plan** before execution:
   - **Hypothesis**: Restate the hypothesis clearly with measurable success criteria
   - **Validation path**: Define what evidence proves/disproves the hypothesis
   - **3-step plan**: (a) Feasibility check → (b) Core build → (c) Evaluate & demo
   - Write to `docs/plans/poc-<slug>.md`
   - Reference protocol: `04-protocols.md § Plan-Approve-Execute (PoC variant)`
2. **Present the plan for approval** before proceeding

## Phase 1: Coordinate & Build

Please coordinate the PoC team and focus on:
3. Clarifying the hypothesis and success criteria
4. Running feasibility checks first
5. Building a lightweight dependency graph and lifecycle states
6. Running an Architecture Briefing before parallel work
7. Building only what is needed for validation
8. Enforcing task status callbacks for delegated streams
9. Running an Integration Checkpoint before final evaluation and demo

## Phase 2: Evaluate & Deliver

10. Preparing a stakeholder demo and evaluation report
11. **Capturing technical debt explicitly**:
    - Maintain a Technical Debt Scorecard: severity, effort, risk, owner
    - Write to `docs/decisions/poc-debt-<slug>.md`
    - Flag any debt items that would block production adoption
12. Reporting blockers and escalations explicitly

## Phase 3: Governance

13. Use progressive context loading for delegations (hypothesis slice, dependency slice, artifact refs, blockers)
14. After each PoC gate, publish a compact checkpoint summary:
	- `[CHECKPOINT] id=<poc_gate> | done=[...] | in_flight=[...] | blocked=[...] | decisions=[...] | artifact_refs=[...] | next=[...]`
	- Write checkpoint to `docs/checkpoints/checkpoint-poc-<gate>.md`
15. Apply strict PoC token envelope and model routing by risk/complexity
16. Include spend telemetry (`used`, `remaining`, `projected_total`) in each PoC checkpoint

PoC request:

{{input}}
