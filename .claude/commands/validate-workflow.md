---
description: "Validate the multi-agent workflow using scenario-based tests, handoff checks, and command regression checks."
argument-hint: "Optional scope override (small|medium|complex|all)"
---

Run a workflow validation pass for the AI development team configuration.

## Validation Scope

1. Scenario suites: `small`, `medium`, `complex` (or filtered by user input)
2. Role behavior checks for Product Owner, Solution Architect, Tech Lead
3. Handoff integration checks:
   - API contract parity
   - QA defect routing closure
   - Security-to-release blocking consistency
4. Command regression checks:
   - `new-project`, `new-poc`, `evaluate-poc`, `code-review`, `prepare-release`, `team-status`

## Validation Gate References

5. **Check all validation gates** defined in `04-protocols.md § Validation Gates`:
   - Plan approval gate
   - Architecture briefing gate
   - Integration checkpoint gate
   - Code review gate
   - Security audit gate
   - Release gate
6. For each gate, verify:
   - Gate is reachable in workflow
   - Gate produces a VERDICT
   - Gate blocks progression on FAIL
   - Gate escalation path is defined

## Output Requirements

7. Use the Workflow Validation Scorecard format from `testing-strategy` skill
8. Return verdict per scenario: `pass`, `conditional_pass`, `fail`
9. Aggregate a final verdict with prioritized remediation backlog
10. Include owners and ETA for each remediation item

## Verdict Output

11. **Produce a structured VERDICT** at the end of validation:

```
## WORKFLOW VALIDATION VERDICT

- **Status**: PASS | CONDITIONAL_PASS | FAIL
- **Scenarios tested**: <count>
- **Scenarios passed**: <count>
- **Scenarios failed**: <count>
- **Gates validated**: <count>/<total>
- **Handoff checks**: passed=<n>, failed=<n>
- **Command regressions**: passed=<n>, failed=<n>
- **Blocker IDs**: [if FAIL — list blocking issues]
- **Validator**: orchestrator
- **Timestamp**: <ISO-8601>
```

If FAIL, the remediation backlog must include:
- Issue description
- Severity (CRITICAL/HIGH/MEDIUM/LOW)
- Owner
- Estimated fix effort
- Blocked workflows

Scope override:
{{input}}
