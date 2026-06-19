---
name: "cost-token-governance"
description: "Define token budgets, model routing, caching strategy, and spend telemetry for multi-agent workflows."
---

## Purpose
Keep orchestration quality high while controlling token spend and preventing runaway execution cost.

## Budget Envelope Defaults
- Initialization + setup: <= 80k tokens combined
- Development orchestration slice: <= 120k tokens
- QA/Security/Release slice: <= 60k tokens
- Trigger replanning when projected overrun > 20%

## Model Routing
- High-reasoning tier:
  - architecture, security, high-risk tradeoff decisions
- Standard-cost tier:
  - formatting, status updates, repetitive reporting

## Caching Rules
- Reuse checkpoint summaries for handoffs.
- Reuse previously fetched external docs when unchanged.
- Replace repeated policy blocks with skill references.

## Spend Telemetry Fields
Include in each major checkpoint:
- used
- projected
- variance
- remaining (for fixed-envelope PoC runs)

---

## Protocol-Aware Enhancements

### Phase-Specific Token Budgets

In addition to the envelope defaults above, the following per-phase budgets apply to structured workflows. These are targets; overrun triggers replanning, not hard failure.

| Phase | Budget | Includes |
|-------|--------|----------|
| **Phase 1 — Planning** | 20k tokens | Architecture decisions, plan documents, task decomposition, risk register |
| **Phase 2 — Implementation** | 120k tokens | Code generation, code review, integration, debugging, API design |
| **Phase 3 — QA/Security** | 60k tokens | Test execution, security scanning, QA gate verdicts, defect triage |
| **Phase 4 — Release** | 30k tokens | Release notes, deployment config, final gate checks, documentation |

**Budget enforcement rules:**
- At each checkpoint, report current spend against the phase budget.
- If projected spend exceeds the phase budget by >20%, trigger a replanning event before continuing.
- Inter-phase token transfers are allowed with orchestrator approval (document as a decision artifact).

### Checkpoint Spend Reporting Format

Every major checkpoint MUST include token spend telemetry in the following format:

```
[SPEND] phase={phase-name} | used={N}k | budget={M}k | projected={P}k | variance={+/-V}k | remaining={R}k
```

**Example:**
```
[CHECKPOINT] id=phase-2-midpoint | done=[auth-api, db-layer] | in_flight=[frontend] | blocked=[] | decisions=[] | artifact_refs=[api-contract-v2] | next=[integration]
[SPEND] phase=implementation | used=65k | budget=120k | projected=110k | variance=-10k | remaining=55k
```

**Spend status indicators:**
| Condition | Status | Action |
|-----------|--------|--------|
| projected ≤ budget | 🟢 On track | Continue |
| projected > budget by ≤20% | 🟡 Warning | Flag in checkpoint, consider optimization |
| projected > budget by >20% | 🔴 Overrun | Trigger replanning, document justification |

### Model Routing Alignment with Phases

Phase budgets inform model routing decisions:
- **Phase 1 (Planning):** Prefer high-reasoning tier — decisions here have cascading cost impact.
- **Phase 2 (Implementation):** Mix tiers — high-reasoning for architecture-sensitive code, standard for boilerplate and formatting.
- **Phase 3 (QA/Security):** High-reasoning for security analysis, standard for test execution reporting.
- **Phase 4 (Release):** Standard tier for most tasks; high-reasoning only for release-blocking decisions.
