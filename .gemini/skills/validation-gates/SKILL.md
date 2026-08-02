---
name: "validation-gates"
description: "Define and execute validation gates with structured verdicts. Use when performing code review gates, QA gates, security audits, or release readiness checks."
---

# Validation Gates

## Purpose

Enforce quality checkpoints at critical project milestones. Each gate produces a structured verdict (PASS, CONDITIONAL_PASS, or FAIL) that determines whether work can proceed to the next phase.

## When to Use

- Before merging implementation work (implementation gate)
- Before declaring a feature integration-complete (integration gate)
- Before any release or deployment (security gate, release gate)
- When reviewing architectural decisions (architecture gate)
- At any phase transition in the project lifecycle

## Gate Types

| Gate | Executor | Trigger | Focus |
|------|----------|---------|-------|
| **Architecture** | Tech Lead | Before implementation starts | Design soundness, pattern compliance, scalability |
| **Implementation** | Tech Lead | After code complete, before merge | Code quality, test coverage, convention adherence |
| **Integration** | QA Agent | After feature merge, before release | End-to-end functionality, regression, compatibility |
| **Security** | Security Agent | Before any release | Vulnerabilities, auth flaws, data exposure, OWASP Top 10 |
| **Release** | Release Manager | Before deployment | All gates passed, docs complete, migration guides ready |

## Verdict Format

Every gate MUST produce a verdict in this format:

```markdown
## Gate Verdict: <Gate Name>

**Gate:** architecture | implementation | integration | security | release
**Executor:** <agent-name>
**Date:** YYYY-MM-DD
**Target:** <feature, task, or release being evaluated>

### Verdict: PASS | CONDITIONAL_PASS | FAIL

### Findings

| # | Severity | Category | Description | Recommendation |
|---|----------|----------|-------------|----------------|
| 1 | critical | ... | ... | ... |
| 2 | high | ... | ... | ... |
| 3 | medium | ... | ... | ... |
| 4 | low | ... | ... | ... |

### Conditions (if CONDITIONAL_PASS)
- [ ] <condition that must be met before proceeding>
- [ ] <condition>

### Summary
<1-3 sentence overall assessment>
```

### Verdict Rules

| Verdict | Criteria |
|---------|----------|
| **PASS** | No critical or high findings. Medium/low findings noted but non-blocking. |
| **CONDITIONAL_PASS** | No critical findings. High findings have documented mitigations. Medium/low with clear remediation plan. All conditions must be resolved before the next gate. |
| **FAIL** | Any critical finding unresolved, OR any high finding without mitigation. Work must return to the implementer. |

### Severity Definitions

| Severity | Definition |
|----------|-----------|
| `critical` | Broken functionality, security vulnerability, data loss risk. Blocks all progress. |
| `high` | Significant defect or design flaw. Must be addressed before release. |
| `medium` | Quality concern or technical debt. Should be addressed, can be tracked. |
| `low` | Minor style issue, optimization opportunity, or suggestion. |

## Gate Input Requirements

Each gate type requires specific inputs to perform its evaluation:

### Architecture Gate
- Plan document (`docs/plans/plan-<feature>.md`)
- Proposed design / tech stack
- Dependency analysis

### Implementation Gate
- Source code changes (diff or file list)
- Test results and coverage report
- Convention checklist (from `AGENTS.md`)

### Integration Gate
- Merged feature in integration branch
- End-to-end test results
- Regression test results
- API contract validation (if applicable)

### Security Gate
- Full codebase scan results
- Dependency audit (`npm audit`, `pip audit`, etc.)
- Authentication/authorization flow review
- Data handling review (PII, encryption, storage)

### Release Gate
- All prior gate verdicts (must be PASS or CONDITIONAL_PASS with conditions resolved)
- Changelog / release notes draft
- Documentation updates
- Migration guide (if breaking changes)
- Rollback plan

## Procedures

### 1. Execute a Gate

1. Identify the gate type and assign to the correct executor.
2. Gather all required inputs for that gate type.
3. The executor reviews each input against the gate's criteria.
4. The executor produces a verdict document.
5. Store the verdict in `docs/artifacts/gate-<type>-<target>-<date>.md`.

### 2. Handle a FAIL Verdict

1. Return the verdict to the task assignee with specific findings.
2. The assignee addresses all critical and high findings.
3. The assignee requests a re-evaluation.
4. The gate executor re-runs the gate, producing a new verdict.

### 3. Handle a CONDITIONAL_PASS Verdict

1. Proceed with work, but track all conditions as tasks.
2. Conditions MUST be resolved before the next gate in the pipeline.
3. When all conditions are resolved, update the verdict document:
   - Check off each condition.
   - Add a note: `All conditions resolved on YYYY-MM-DD`.

### 4. Gate Pipeline for a Release

```
Architecture Gate → Implementation Gate → Integration Gate → Security Gate → Release Gate
```

Each gate must achieve at least CONDITIONAL_PASS before the next gate can start. The Release Gate verifies that all prior conditions have been resolved.

## Examples

### Implementation Gate Verdict — CONDITIONAL_PASS

```markdown
## Gate Verdict: Implementation Review

**Gate:** implementation
**Executor:** tech-lead
**Date:** 2025-03-25
**Target:** T021 (Token Bucket Rate Limiting)

### Verdict: CONDITIONAL_PASS

### Findings

| # | Severity | Category | Description | Recommendation |
|---|----------|----------|-------------|----------------|
| 1 | high | testing | No load test for concurrent access | Add load test with 1000 concurrent requests |
| 2 | medium | code quality | Magic numbers in rate limit config | Extract to configuration constants |
| 3 | low | style | Inconsistent error message format | Align with project error format convention |

### Conditions
- [ ] Add load test covering concurrent access scenario
- [ ] Extract rate limit values to configuration

### Summary
Implementation is functionally correct with good unit test coverage. Load testing gap is the primary concern — must verify concurrent behavior before integration.
```

## Guidelines

- Gates are non-negotiable checkpoints. Do not skip gates to save time.
- Verdicts must be evidence-based. Every finding must reference specific code, tests, or documentation.
- CONDITIONAL_PASS is not a skip. Track conditions as real tasks.
- Gate executors should not review their own work. Cross-agent review is mandatory.
- Store all verdict documents for audit trail purposes.
