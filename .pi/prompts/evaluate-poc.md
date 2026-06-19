---
description: "Evaluate whether the current PoC validates the hypothesis."
argument-hint: "Describe what has been built and what hypothesis should be evaluated..."
---

Evaluate the current PoC and determine whether it validates the stated hypothesis.

## Evaluation Framework

Please return:
1. Hypothesis restatement
2. Evidence summary
3. Verdict (Validated, Invalidated, Inconclusive)
4. Recommended next step
5. Production recommendation (proceed, proceed_with_constraints, do_not_proceed)
6. Prioritized production refactoring backlog (top 5 minimum)
7. Technical Debt Scorecard summary (severity, effort, risk, owner)
8. Residual risks and assumptions

## Structured Evaluation Verdict

9. **Produce a structured VERDICT** at the end of the evaluation:

```
## POC VERDICT

- **Hypothesis**: <restated hypothesis>
- **Status**: VALIDATED | INVALIDATED | INCONCLUSIVE
- **Production recommendation**: proceed | proceed_with_constraints | do_not_proceed
- **Evidence strength**: strong | moderate | weak
- **Debt items**: <count> (CRITICAL: <n>, HIGH: <n>, MEDIUM: <n>, LOW: <n>)
- **Residual risks**: <count>
- **Evaluator**: poc-orchestrator
- **Timestamp**: <ISO-8601>
```

## Production Handoff Checklist

10. **If recommending `proceed` or `proceed_with_constraints`**, produce a handoff checklist:

- [ ] All CRITICAL debt items have remediation plans with owners
- [ ] Architecture decisions documented in `docs/decisions/`
- [ ] Security audit completed (or scheduled for Phase 1)
- [ ] Performance baselines established
- [ ] Test coverage plan defined for production code
- [ ] Data migration/seeding strategy defined (if applicable)
- [ ] Monitoring and alerting requirements captured
- [ ] Production infrastructure requirements documented

## Debt Summary

11. **Produce a debt summary table**:

| # | Debt Item | Severity | Effort | Risk | Owner | Production Impact |
|---|-----------|----------|--------|------|-------|-------------------|
| 1 | ... | CRITICAL/HIGH/MEDIUM/LOW | S/M/L/XL | ... | ... | blocks/degrades/cosmetic |

Context:

{{input}}
