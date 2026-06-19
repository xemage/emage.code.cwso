---
name: "rapid-prototyping"
description: "Create pragmatic scaffolding and integration patterns for fast proof-of-concept delivery."
---

# Rapid Prototyping

## When to use
- Bootstrapping PoC repositories
- Creating minimal architecture slices

## Outputs
- Minimal project structure
- Fast run instructions
- Scope boundaries for what not to build

---

## Protocol-Aware Enhancements

### Mandatory Debt Tagging

All PoC code MUST tag known shortcuts, workarounds, and deferred quality concerns using the standard debt tag format:

```html
<!-- POC-DEBT: description of the shortcut or deferred concern -->
```

**Placement rules:**
- Place the tag directly above or inline with the code that embodies the shortcut.
- For architectural shortcuts, place the tag in the relevant architecture or design document.
- For configuration shortcuts (e.g., hardcoded values, disabled security), place the tag in the config file.

**Examples:**
```python
# <!-- POC-DEBT: No input validation — add comprehensive validation before production -->
def create_user(data):
    return db.users.insert(data)

# <!-- POC-DEBT: Using in-memory store — replace with persistent database before production -->
cache = {}
```

```yaml
# <!-- POC-DEBT: CORS allow-all — restrict origins before production -->
cors:
  origin: "*"
```

**Enforcement:**
- Every file in a PoC that contains a shortcut MUST have at least one `POC-DEBT` tag.
- The technical-debt-tracking skill scans for these tags during PoC evaluation.
- Untagged shortcuts discovered during review are flagged as review findings (severity: 🟡 Should Fix).

### PoC Artifact Versioning

PoC artifacts follow the standard immutable versioning convention:

```
docs/artifacts/poc-{name}-v1.md
docs/artifacts/poc-{name}-v2.md
```

**PoC artifact structure:**
```markdown
# PoC: {Name} v{N}

## Version: {N}
## Date: {YYYY-MM-DD}
## Status: in-progress | complete | evaluated

## Hypothesis
[What this PoC is testing]

## Success Criteria
- [ ] Criterion 1
- [ ] Criterion 2

## Technology Stack
[Technologies selected, referencing tech-eval-v{N} if applicable]

## Scope Boundaries
### In scope:
- [what the PoC will demonstrate]

### Explicitly out of scope:
- [what the PoC will NOT build]

## Known Debt
[Summary of POC-DEBT tags — auto-populated during evaluation]

## Run Instructions
[How to run the PoC]
```

### Hypothesis Validation Checkpoint Format

At the conclusion of prototyping (or at significant milestones), publish a hypothesis validation checkpoint:

```
[CHECKPOINT] id=poc-{name}-validation | hypothesis="{hypothesis text}" | evidence=[{evidence items}] | debt_tags={count} | verdict=validated|invalidated|inconclusive | artifact_refs=[poc-{name}-v{N}] | next=[{next steps}]
```

**Evidence items** should be specific and measurable:
- "API response time < 200ms for 95th percentile" ✅
- "Integration with {service} successful via SDK v{X}" ✅
- "It seems to work" ❌ (too vague)

**Checkpoint triggers:**
- All success criteria have been tested (regardless of outcome).
- A showstopper is discovered that invalidates the hypothesis.
- The PoC time-box expires (document whatever evidence exists).
