---
name: "context-window-management"
description: "Compact orchestration context and checkpoint summaries for long-running multi-agent workflows."
---

## Purpose
Keep delegations focused and token-efficient while preserving decision continuity.

## Minimal Delegation Packet
Use only:
- objective
- dependency slice
- artifact references
- active blockers
- expected outputs

## Checkpoint Summary Format
After each phase or gate, publish:
- `[CHECKPOINT] id=<phase_or_gate> | done=[...] | in_flight=[...] | blocked=[...] | decisions=[...] | artifact_refs=[...] | next=[...]`

## Compaction Rules
- Do not replay full transcripts when checkpoint summaries exist.
- Carry forward only unresolved blockers, active tasks, and decision deltas.
- Keep summaries terse and reference immutable artifacts by version.

---

## Protocol-Aware Enhancements

### Checkpoint Compression Rules

To prevent unbounded context growth over long-running workflows, apply the following checkpoint compression strategy:

**Retention policy:**
- **Latest 2 checkpoints:** Keep in full, uncompressed form.
- **Older checkpoints:** Compress to a single-line summary preserving only:
  - Checkpoint ID
  - Key decisions made
  - Unresolved blockers carried forward
  - Artifact versions produced

**Compressed checkpoint format:**
```
[CHECKPOINT-SUMMARY] id={id} | decisions=[{key decisions}] | blockers_carried=[{ids}] | artifacts=[{refs}] | status=compressed
```

**Example progression:**
```
# Full (kept — latest 2)
[CHECKPOINT] id=phase-2-integration | done=[auth-api, db-schema] | in_flight=[frontend-routing] | blocked=[] | decisions=[use-jwt-v2] | artifact_refs=[api-contract-v3] | next=[integration-test]

[CHECKPOINT] id=phase-2-testing | done=[integration-test] | in_flight=[e2e-smoke] | blocked=[blocker-perf-01] | decisions=[defer-caching] | artifact_refs=[test-plan-v2] | next=[qa-gate]

# Compressed (older)
[CHECKPOINT-SUMMARY] id=phase-1-planning | decisions=[monorepo, postgres, REST] | blockers_carried=[] | artifacts=[architecture-v1, api-contract-v1] | status=compressed
[CHECKPOINT-SUMMARY] id=phase-1-review | decisions=[approved-arch-v1] | blockers_carried=[] | artifacts=[architecture-v1] | status=compressed
```

### Checkpoint Protocol Skill Reference

For full checkpoint lifecycle management (creation, validation, gate integration), reference the **checkpoint protocol** skill. This skill (context-window-management) focuses specifically on compression and token efficiency of checkpoint data.

Cross-reference: When creating or consuming checkpoints, defer to the checkpoint protocol for:
- Required checkpoint fields and validation rules
- Gate-checkpoint linkage (which gates trigger checkpoints)
- Checkpoint archival and retrieval procedures

### Skill References Over Inline Policy

To minimize context window consumption, follow this principle:

> **Never inline full policy text when a skill reference suffices.**

Instead of copying policy rules into a delegation or checkpoint, use a skill reference:

```
[SKILL-REF] skill=context-window-management | section=checkpoint-compression-rules
[SKILL-REF] skill=code-review | section=verdict-format
[SKILL-REF] skill=cost-token-governance | section=budget-envelope
```

**When to inline vs. reference:**
| Situation | Action |
|-----------|--------|
| Agent needs to apply the rule right now | Inline the minimal relevant excerpt |
| Agent needs to be aware a rule exists | Use `[SKILL-REF]` |
| Checkpoint summary mentions a policy | Use `[SKILL-REF]` |
| Delegation to a sub-agent | Include only the objective + skill references, not full skill text |

This reduces per-delegation token cost by 60-80% compared to inlining full skill content.
