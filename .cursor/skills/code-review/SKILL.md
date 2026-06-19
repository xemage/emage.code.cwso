---
name: "code-review"
description: "Perform structured code reviews with checklists for correctness, security, performance, and maintainability. Use when reviewing merge requests, pull requests, checking code quality, or performing peer review."
---

# Code Review

Skill for performing thorough, structured code reviews following best practices.

## When to Use
- Reviewing a merge request or pull request
- Performing peer code review
- Checking code quality before merge
- Evaluating third-party code or libraries

## Procedure

### 1. Context Gathering
- Read the MR/PR description and linked issue
- Understand the feature or bug being addressed
- Review the acceptance criteria
- Check the architecture constraints

### 2. Review Checklist

#### Correctness
- [ ] Code does what the issue/story requires
- [ ] Edge cases handled
- [ ] Error handling is appropriate
- [ ] No off-by-one errors or boundary issues
- [ ] Concurrency issues addressed (if applicable)

#### Security (OWASP)
- [ ] Input validation on all user inputs
- [ ] No SQL/command/XSS injection vectors
- [ ] Authentication/authorization checks present
- [ ] Sensitive data not logged or exposed
- [ ] No hardcoded secrets or credentials

#### Performance
- [ ] No N+1 query issues
- [ ] Appropriate use of indexes (database)
- [ ] No unnecessary allocations or copies
- [ ] Efficient algorithms for the data size
- [ ] Caching used where appropriate

#### Maintainability
- [ ] Code is readable and self-documenting
- [ ] Functions are focused (single responsibility)
- [ ] DRY — no unnecessary duplication
- [ ] Naming is clear and consistent
- [ ] No commented-out code left behind

#### Testing
- [ ] Tests added for new functionality
- [ ] Tests cover happy path and error cases
- [ ] Tests are deterministic (no flaky tests)
- [ ] Mocking is appropriate (not over-mocked)

#### Documentation
- [ ] Public APIs documented
- [ ] Complex logic has explanatory comments
- [ ] Breaking changes documented

### 3. Feedback Format

```markdown
## Review: [MR Title]

### Verdict: [Approved | Changes Requested | Needs Discussion]

### Summary
[1-2 sentence overall assessment]

### Findings

#### 🔴 Must Fix
1. **[file:line]** — [Issue description]
   > Suggestion: [How to fix]

#### 🟡 Should Fix
1. **[file:line]** — [Issue description]
   > Suggestion: [How to fix]

#### 🔵 Nice to Have
1. **[file:line]** — [Improvement suggestion]

#### ✅ Well Done
- [Positive observations about the code]
```

### 4. Severity Guide
| Level | Criteria | Action |
|-------|----------|--------|
| 🔴 Must Fix | Bug, security issue, data loss risk | Block merge |
| 🟡 Should Fix | Performance, maintainability concern | Request changes |
| 🔵 Nice to Have | Style, minor improvement | Comment only |
| ✅ Well Done | Excellent pattern, clean code | Acknowledge |

## Guidelines
- Be constructive — explain WHY, suggest HOW
- Praise good code, not just critique problems
- Focus on the code, not the person
- Ask questions when you're uncertain
- Don't nitpick style issues that linters should catch

---

## Protocol-Aware Enhancements

### VERDICT Format for Validation Gates

Code review is a **validation gate** in the emage.code workflow. Every review MUST conclude with a structured verdict that downstream gates (CI/CD, release) can consume:

```
[VERDICT] gate=code-review | result=PASS|CONDITIONAL_PASS|FAIL | reviewer={role} | artifact_ref={artifact-version} | date={YYYY-MM-DD}
```

**Verdict definitions:**

| Verdict | Meaning | Pipeline Effect |
|---------|---------|-----------------|
| `PASS` | No must-fix findings. Code is merge-ready. | Pipeline proceeds. |
| `CONDITIONAL_PASS` | Only should-fix findings. Code may merge with tracked follow-ups. | Pipeline proceeds; follow-up items logged to `docs/tasks/active-tasks.md`. |
| `FAIL` | One or more must-fix findings. Code must not merge. | Pipeline halts. Re-review required after fixes. |

### Artifact Version Awareness

When reviewing code, always note which artifact versions are relevant to the review:

- **API contracts:** Confirm the code conforms to the referenced `api-contract-vN`.
- **Architecture decisions:** Confirm the code aligns with `architecture-decision-vN`.
- **Pipeline config:** Confirm CI/CD changes match `pipeline-config-vN`.

Include artifact references in the review output:

```markdown
### Artifacts Reviewed
- api-contract-v3 — endpoints conform ✅
- architecture-decision-v2 — pattern followed ✅
```

### Review as Validation Gate

The code-review verdict is consumed by the CI/CD pipeline at the `approve` stage. The following rules apply:

1. A `FAIL` verdict blocks the merge request — no override without Tech Lead waiver (documented as a decision artifact).
2. A `CONDITIONAL_PASS` verdict allows merge but **requires** that each should-fix item is logged as a task in `docs/tasks/active-tasks.md` with an assigned owner and target sprint.
3. A `PASS` verdict allows merge with no additional conditions.
4. Every verdict must be recorded in the next checkpoint summary under `decisions=[...]`.
