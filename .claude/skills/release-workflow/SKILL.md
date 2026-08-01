---
name: "release-workflow"
description: "Manage the release lifecycle: versioning, changelog generation, release gates, and hotfix procedures. Use when preparing a release, generating changelogs, managing version numbers, or handling hotfixes."
---

# Release Workflow

## Purpose

Manage the full release lifecycle — from version number selection through changelog generation, release gate verification, deployment, and hotfix handling. Ensures releases are consistent, auditable, and safe.

## When to Use

- Preparing a new release (major, minor, or patch)
- Generating a changelog from completed work
- Verifying release readiness through gates
- Handling a hotfix for a released version
- Deciding on version number increments

## Semantic Versioning

All releases follow [Semantic Versioning](https://semver.org/): `MAJOR.MINOR.PATCH`

| Component | Increment When | Example |
|-----------|---------------|---------|
| **MAJOR** | Breaking changes to public API or contracts | `1.0.0 → 2.0.0` |
| **MINOR** | New features, backward-compatible | `1.0.0 → 1.1.0` |
| **PATCH** | Bug fixes, backward-compatible | `1.0.0 → 1.0.1` |

### Version Decision Flowchart

```
Has any public API or contract changed in a breaking way?
  ├── Yes → MAJOR increment
  └── No
       Has new functionality been added?
         ├── Yes → MINOR increment
         └── No → PATCH increment
```

### Pre-release Versions

For release candidates: `MAJOR.MINOR.PATCH-rc.N` (e.g., `2.0.0-rc.1`)

## Procedures

### 1. Prepare a Release

1. **Determine the version number** using the semantic versioning rules above.
2. **Verify all planned tasks are complete:**
   - Check `docs/tasks/active-tasks.md` — no tasks for this release should be `in_progress` or `blocked`.
   - All planned tasks should be `done` or archived.
3. **Generate the changelog** (see procedure below).
4. **Run the release gate** (see gate checklist below).
5. **Create the release artifact** at `docs/artifacts/release-notes-v<VERSION>.md`.

### 2. Generate Changelog

Build the changelog from completed tasks and artifacts since the last release.

#### Changelog Format

```markdown
# Changelog — v<VERSION>

**Release Date:** YYYY-MM-DD
**Previous Version:** v<PREV_VERSION>

## Breaking Changes
- <description> (TNNN)

## New Features
- <description> (TNNN)

## Bug Fixes
- <description> (TNNN)

## Improvements
- <description> (TNNN)

## Documentation
- <description> (TNNN)

## Internal
- <description> (TNNN)
```

#### Changelog Generation Steps

1. Read `docs/tasks/completed-tasks.md` for tasks completed since the last release.
2. Categorize each task by type:
   - Tasks with breaking changes → "Breaking Changes"
   - Feature tasks → "New Features"
   - Bug fix tasks → "Bug Fixes"
   - Refactor/improvement tasks → "Improvements"
   - Documentation tasks → "Documentation"
   - CI/CD, tooling, internal tasks → "Internal"
3. Link each entry to its task ID.
4. Review gate verdict documents in `docs/artifacts/` for additional context.

### 3. Release Gate Checklist

The release gate is the final validation before deployment. All items must be checked:

```markdown
## Release Gate Checklist — v<VERSION>

### Prior Gates
- [ ] Architecture gate: PASS or CONDITIONAL_PASS (all conditions resolved)
- [ ] Implementation gate: PASS or CONDITIONAL_PASS (all conditions resolved)
- [ ] Integration gate: PASS or CONDITIONAL_PASS (all conditions resolved)
- [ ] Security gate: PASS or CONDITIONAL_PASS (all conditions resolved)

### Release Readiness
- [ ] No critical or high bugs open for this release
- [ ] All planned tasks completed and archived
- [ ] Changelog generated and reviewed
- [ ] Documentation updated (README, API docs, user guides)
- [ ] Migration guide written (if MAJOR version or breaking changes)
- [ ] Rollback plan documented

### Sign-off
- [ ] Tech Lead approval
- [ ] QA approval
- [ ] User/stakeholder approval
```

### 4. Create Release Artifact

```markdown
# Release Notes — v<VERSION>

**Release Date:** YYYY-MM-DD
**Release Manager:** <agent-name>

## Summary
<1-3 sentence summary of what this release contains>

## Changelog
<include full changelog>

## Migration Guide
<if applicable — steps to upgrade from previous version>

## Known Issues
- <any known issues shipping with this release>

## Gate Verdicts
- Architecture: <PASS/CONDITIONAL_PASS> on YYYY-MM-DD
- Implementation: <PASS/CONDITIONAL_PASS> on YYYY-MM-DD
- Integration: <PASS/CONDITIONAL_PASS> on YYYY-MM-DD
- Security: <PASS/CONDITIONAL_PASS> on YYYY-MM-DD
- Release: <PASS/CONDITIONAL_PASS> on YYYY-MM-DD

## Rollback Plan
<steps to roll back if critical issues are discovered post-release>
```

File location: `docs/artifacts/release-notes-v<VERSION>.md`

### 5. Hotfix Workflow

For critical issues discovered after a release:

```
1. TRIAGE
   ├── Confirm the issue is critical (production-impacting)
   └── Assign a severity (must be critical or high)

2. BRANCH
   ├── Branch from the release tag: hotfix/v<VERSION>-<issue>
   └── Example: hotfix/v1.2.0-auth-bypass

3. FIX
   ├── Implement the minimal fix (no feature work)
   ├── Write regression test
   └── Verify fix resolves the issue

4. FAST-TRACK GATES
   ├── Implementation gate (focused on the fix only)
   ├── Security gate (if security-related)
   └── Abbreviated integration gate (regression test + smoke test)

5. RELEASE
   ├── Increment PATCH version: v1.2.0 → v1.2.1
   ├── Generate hotfix changelog entry
   ├── Create release artifact
   └── Deploy

6. MERGE BACK
   ├── Merge hotfix branch into main
   └── Ensure the fix is included in future releases
```

#### Hotfix Gate Exceptions

Hotfixes use a fast-track gate process:
- Architecture gate: **SKIPPED** (minimal change, no design impact)
- Implementation gate: **REQUIRED** (focused review of the fix)
- Integration gate: **ABBREVIATED** (regression test + smoke test only)
- Security gate: **REQUIRED if security-related**, otherwise skipped
- Release gate: **ABBREVIATED** (checklist with reduced scope)

## Examples

### Minor Release

```
Version: 1.3.0 (was 1.2.0)
Reason: 3 new features added, no breaking changes
Tasks completed: T040 through T052
All gates passed
Changelog: 3 new features, 2 bug fixes, 1 improvement
```

### Hotfix

```
Version: 1.2.1 (hotfix for 1.2.0)
Reason: Authentication bypass vulnerability
Fix: Added token expiration validation
Fast-tracked through implementation + security gates
Merged back to main
```

## Guidelines

- Never release without passing the release gate checklist.
- Hotfixes are for critical/high issues only. Medium/low issues wait for the next regular release.
- Hotfixes must be minimal — fix the issue and nothing else.
- Always merge hotfixes back to main to prevent regression in future releases.
- Keep release notes user-facing. Internal changes go in the "Internal" changelog section.
- Tag releases in git: `git tag -a v<VERSION> -m "Release v<VERSION>"`.
- The release manager is responsible for the full release workflow, including gate coordination.
