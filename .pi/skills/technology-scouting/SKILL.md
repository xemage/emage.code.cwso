---
name: "technology-scouting"
description: "Evaluate APIs, SDKs, and platforms quickly to select the fastest PoC implementation path."
---

# Technology Scouting

## When to use
- Selecting APIs or SDKs for PoCs
- Comparing managed service options

## Outputs
- Option matrix
- Recommendation with rationale
- Cost and risk notes

---

## Protocol-Aware Enhancements

### Technology Evaluation Artifact Versioning

Technology evaluation reports are versioned artifacts. When a technology evaluation is conducted, store it as an immutable artifact:

```
docs/artifacts/tech-eval-v1.md
docs/artifacts/tech-eval-v2.md
```

**Tech evaluation artifact structure:**
```markdown
# Technology Evaluation v{N}

## Version: {N}
## Date: {YYYY-MM-DD}
## Status: draft | approved | superseded
## Evaluator: {role}

## Context
[Problem being solved, constraints, requirements]

## Options Evaluated

### Option A: {Name}
- **Type:** API / SDK / Platform / Service
- **Maturity:** GA / Beta / Preview
- **Licensing:** {license type}
- **Cost model:** {cost description}
- **Pros:** [list]
- **Cons:** [list]
- **Risk level:** low | medium | high
- **PoC feasibility:** {estimate}

### Option B: {Name}
[Same structure]

## Comparison Matrix

| Criterion | Weight | Option A | Option B | Option C |
|-----------|--------|----------|----------|----------|
| API quality | 3 | ⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |
| Cost | 2 | ⭐⭐ | ⭐⭐⭐ | ⭐ |
| Community/support | 2 | ⭐⭐⭐ | ⭐⭐ | ⭐⭐ |
| Integration effort | 3 | ⭐⭐ | ⭐⭐⭐ | ⭐⭐ |
| Security posture | 3 | ⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |

## Recommendation
[Selected option with rationale]

## Residual Risks
[Risks that remain even with the selected option]
```

**Rules:**
- Each evaluation is immutable once approved — create a new version if assumptions change.
- Reference evaluations by version in checkpoint summaries: `artifact_refs=[tech-eval-v2]`.
- When a PoC is started based on an evaluation, link the PoC artifact back to the tech-eval version.

### Blocker Reporting for Technology Showstoppers

When a technology evaluation reveals a showstopper (e.g., licensing incompatibility, missing critical feature, unacceptable security posture), raise a blocker:

```
[BLOCKER] id=tech-{issue-id} | type=technology | severity=high|critical | description="..." | evaluated_option={name} | tech_eval_ref=tech-eval-v{N} | owner={role} | escalation={next-role}
```

**When to raise a technology blocker:**
- All evaluated options fail a critical requirement.
- The recommended option has a licensing or compliance conflict.
- A dependency is deprecated, end-of-life, or has known unpatched vulnerabilities.
- Cost projections exceed approved budget with no viable alternative.
- Integration complexity exceeds the time/resource constraints of the current phase.

**Blocker resolution paths:**
1. **Pivot:** Select a different technology not previously evaluated (requires new tech-eval version).
2. **Constraint relaxation:** Request stakeholder approval to relax the failing requirement (documented as decision artifact).
3. **Escalate:** Surface to orchestrator for cross-team resolution if the blocker impacts other workstreams.

Blockers must appear in the next checkpoint summary and be tracked in `docs/tasks/active-tasks.md`.
