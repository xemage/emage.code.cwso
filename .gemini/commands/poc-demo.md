---
description: "Package the current PoC into a stakeholder-ready demo flow."
agent: "poc-orchestrator"
argument-hint: "Describe the audience and what outcome the demo should prove..."
---

Prepare a polished PoC demo package for stakeholder review.

## Demo Package

Please provide:
1. Demo walkthrough script
2. Required sample data and input sequence
3. Expected visible outcomes
4. Presenter notes and fallback path
5. Explicit PoC-only shortcuts shown in the demo
6. Production transition notes tied to the Technical Debt Scorecard

## Artifact References

7. **Reference all relevant artifacts in the demo**:
   - List artifact versions used/demonstrated (e.g., `poc-api-v0.3.md`, `poc-ui-v0.2.md`)
   - Link to the PoC plan document (`docs/plans/poc-<slug>.md`)
   - Link to the latest checkpoint (`docs/checkpoints/checkpoint-poc-<gate>.md`)
   - Reference the Technical Debt Scorecard (`docs/decisions/poc-debt-<slug>.md`)

## Hypothesis Validation Status

8. **Include hypothesis validation status in the demo**:

```
## Hypothesis Status

- **Hypothesis**: <restated hypothesis>
- **Validation status**: VALIDATED | INVALIDATED | INCONCLUSIVE | IN_PROGRESS
- **Key evidence demonstrated**: [list what the demo proves]
- **Evidence gaps**: [list what the demo does NOT prove]
- **Confidence level**: HIGH | MEDIUM | LOW
```

9. Highlight which parts of the demo directly validate the hypothesis
10. Call out any demo elements that are mocked/simulated vs. real implementation

Context:

{{input}}
