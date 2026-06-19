---
description: "Request a thorough code review of specified files or the current changes, following the code review skill checklist."
agent: "tech-lead"
argument-hint: "Specify files or describe what to review..."
---

Please perform a thorough code review on the following:

{{input}}

## Review Checklist

Review against:
1. Correctness and acceptance criteria compliance
2. Security (OWASP Top 10)
3. Performance concerns
4. Code quality and maintainability
5. Test coverage
6. Documentation
7. Gate status recommendation (`pass`, `conditional_pass`, `fail`)

Provide structured feedback with severity levels (Must Fix / Should Fix / Nice to Have) and specific remediation suggestions.
If status is `fail`, include blocker details, owner, and retry attempt guidance.

## Artifact Version References

8. Reference the specific artifact versions being reviewed:
   - List artifact files and their versions (e.g., `api-design-v1.2.md`)
   - Note if reviewing against a specific checkpoint
9. Check that implementation matches the approved plan artifact

## Verdict Output

10. **Produce a structured VERDICT** at the end of the review:

```
## VERDICT

- **Status**: PASS | CONDITIONAL_PASS | FAIL
- **Reviewed artifacts**: [list artifact versions reviewed]
- **Must Fix count**: <n>
- **Should Fix count**: <n>
- **Nice to Have count**: <n>
- **Blocker IDs**: [if FAIL — list blocking issues]
- **Reviewer**: tech-lead
- **Timestamp**: <ISO-8601>
```

If CONDITIONAL_PASS, list the conditions that must be met before merge.
If FAIL, include blocker details, owner, and retry attempt guidance.
