---
description: "Prepare a new release with version bump, changelog generation, and release checklist."
argument-hint: "Specify version type: major, minor, or patch..."
---

Please prepare a new release:

Release type: {{input}}

## Release Steps

1. Determine the next version number (semantic versioning)
2. **Generate changelog from completed tasks**:
   - Read `docs/tasks/completed-tasks.md` for all tasks completed since last release
   - Group by: Features, Fixes, Breaking Changes, Internal
   - Include task IDs and artifact version references
3. Review all included changes (features, fixes, breaking changes)
4. Create the release checklist
5. Prepare release notes

## Version Management

6. **Manage version artifacts**:
   - Update version in all relevant config files
   - Tag all current artifacts with release version
   - Create a release checkpoint: `docs/checkpoints/checkpoint-release-v<version>.md`
   - Archive completed tasks from `active-tasks.md` to `completed-tasks.md`

## Release Gate Verdict

7. **Produce a structured release gate VERDICT**:

```
## RELEASE VERDICT

- **Version**: v<major>.<minor>.<patch>
- **Status**: PASS | CONDITIONAL_PASS | FAIL
- **Features included**: <count>
- **Fixes included**: <count>
- **Breaking changes**: <count>
- **Open blockers**: <count>
- **Quality gates passed**: [list gates: code-review, security-audit, test-coverage, ...]
- **Quality gates failed**: [list any failed gates]
- **Blocker IDs**: [if FAIL — list blocking issues with owners]
- **Release manager**: orchestrator
- **Timestamp**: <ISO-8601>
```

8. If blocked, include blocker IDs, owners, and escalation path
9. If CONDITIONAL_PASS, list conditions that must be met before deployment

Ensure all quality gates are met before proceeding.
