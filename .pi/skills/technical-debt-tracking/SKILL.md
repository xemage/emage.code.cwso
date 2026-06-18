---
name: "technical-debt-tracking"
description: "Document PoC shortcuts and production remediation plans using a consistent debt ledger."
---

# Technical Debt Tracking

## Categories
- Security
- Architecture
- Testing
- Data quality
- Operations

## Required fields
- Debt item
- Severity
- Impact
- Estimated remediation effort
- Risk if deferred
- Recommended owner
- Suggested target sprint

## Scorecard Requirements
- Produce a Technical Debt Scorecard summary by severity band.
- Mark each item as: `must_fix_pre_prod`, `can_defer_post_ga`, or `monitor_only`.
- Include top remediation sequence for the first production sprint.

---

## Protocol-Aware Enhancements

### POC-DEBT Tag Scanning Procedure

This skill is responsible for scanning codebases for `POC-DEBT` tags placed by the rapid-prototyping skill. The scanning procedure:

**Step 1: Discover all debt tags**
```bash
# Scan all project files for POC-DEBT tags
grep -rn "POC-DEBT:" --include="*.py" --include="*.ts" --include="*.js" --include="*.yaml" --include="*.yml" --include="*.md" --include="*.json" --include="*.html" --include="*.css" .
```

**Step 2: Parse each tag into a structured debt item**

For each discovered tag, extract:
- **File path** and **line number** where the tag appears
- **Description** from the tag content
- **Category** (infer from context: Security, Architecture, Testing, Data quality, Operations)
- **Severity** (assign based on risk assessment — see Debt Item Format below)

**Step 3: Cross-reference with evaluation findings**

Merge POC-DEBT scan results with:
- Code review findings (🔴 Must Fix and 🟡 Should Fix items)
- PoC evaluation refactoring backlog items
- Security scan findings

**Step 4: Produce consolidated debt ledger**

Output the full debt ledger as a versioned artifact (see below).

**Automation note:** This scan should be triggered:
- At the conclusion of every PoC (before evaluation)
- Before any production handoff
- As part of the QA validation gate

### Debt Item Format

Each debt item in the ledger uses the following structured format:

```markdown
### DEBT-{ID}: {Short title}

- **Category:** Security | Architecture | Testing | Data quality | Operations
- **Severity:** critical | high | medium | low
- **Impact:** [What breaks or degrades if this debt is not addressed]
- **Effort:** XS (< 1h) | S (1-4h) | M (4-16h) | L (16-40h) | XL (40h+)
- **Risk if deferred:** [What happens if we ship without fixing this]
- **Source:** POC-DEBT tag | code-review finding | security scan | poc-evaluation
- **Source location:** {file}:{line} (if from code)
- **Recommended owner:** {role}
- **Target sprint:** {sprint-name} | pre-production | post-GA
- **Disposition:** must_fix_pre_prod | can_defer_post_ga | monitor_only
- **Created:** {YYYY-MM-DD}
- **Status:** open | in-progress | resolved | accepted-risk
```

**Severity assignment guide:**

| Severity | Criteria |
|----------|----------|
| **critical** | Security vulnerability, data loss risk, or compliance violation |
| **high** | Performance degradation under load, missing error handling on critical paths, architectural violation that blocks scaling |
| **medium** | Missing tests for important paths, hardcoded configuration, suboptimal patterns |
| **low** | Code style issues, minor optimization opportunities, documentation gaps |

### Promotion Procedure for Debt Items to Production Backlog

When a PoC transitions to production implementation, debt items must be promoted from the debt ledger to the production task backlog:

**Promotion rules:**

| Disposition | Promotion Action |
|-------------|-----------------|
| `must_fix_pre_prod` | Create task in `docs/tasks/active-tasks.md` with priority `must` and target sprint = current or next |
| `can_defer_post_ga` | Create task in `docs/tasks/active-tasks.md` with priority `should` and target sprint = post-GA sprint |
| `monitor_only` | Do not create a task; add to risk register with monitoring criteria |

**Promotion procedure:**

1. Filter the debt ledger for items with disposition `must_fix_pre_prod` and `can_defer_post_ga`.
2. For each item, create a task entry:
   ```markdown
   ### TASK-{ID}: Resolve DEBT-{debt-id} — {title}
   - **Status:** pending
   - **Assignee:** {recommended owner from debt item}
   - **Priority:** {must | should — based on disposition}
   - **Points:** {mapped from effort: XS=1, S=2, M=5, L=8, XL=13}
   - **Sprint:** {target sprint from debt item}
   - **Depends on:** [any prerequisites]
   - **Artifact refs:** [debt-ledger-v{N}, poc-{name}-v{N}]
   - **Created:** {YYYY-MM-DD}
   ```
3. Sync promoted tasks to GitLab issues (see gitlab-management skill).
4. Update the debt ledger to mark promoted items with `promoted_to=TASK-{ID}`.

**Debt ledger versioning:**
```
docs/artifacts/debt-ledger-v1.md
docs/artifacts/debt-ledger-v2.md
```

Each version is immutable. Create a new version when items are added, resolved, or promoted.

### Technical Debt Scorecard (Enhanced)

The scorecard aggregates debt items by severity and disposition, providing a quick health view:

```markdown
# Technical Debt Scorecard v{N}

## Date: {YYYY-MM-DD}
## Source: debt-ledger-v{N}

## Summary by Severity

| Severity | Total | must_fix_pre_prod | can_defer_post_ga | monitor_only |
|----------|-------|-------------------|-------------------|--------------|
| Critical | {N} | {N} | {N} | {N} |
| High | {N} | {N} | {N} | {N} |
| Medium | {N} | {N} | {N} | {N} |
| Low | {N} | {N} | {N} | {N} |

## Summary by Category

| Category | Total | Critical/High | Estimated Total Effort |
|----------|-------|---------------|----------------------|
| Security | {N} | {N} | {effort} |
| Architecture | {N} | {N} | {effort} |
| Testing | {N} | {N} | {effort} |
| Data quality | {N} | {N} | {effort} |
| Operations | {N} | {N} | {effort} |

## Top Remediation Sequence (First Production Sprint)
1. DEBT-{ID}: {title} — {effort} — {owner}
2. DEBT-{ID}: {title} — {effort} — {owner}
3. DEBT-{ID}: {title} — {effort} — {owner}

## Risk Acceptance Register
[Items with disposition=monitor_only and their monitoring criteria]
```

The scorecard is included as part of the PoC evaluation output and referenced in production handoff checkpoints.
