---
name: "memory-management"
description: "Manage the memory hierarchy: MCP Memory for runtime facts, AGENTS.md for project conventions, checkpoints for progress. Use when persisting important decisions, consolidating memory, or pruning stale entries."
---

# Memory Management

## Purpose

Manage the three-tier memory hierarchy to ensure important context is persisted at the right level, stale information is pruned, and agents can efficiently retrieve what they need.

## When to Use

- Persisting a key decision or architectural choice
- Consolidating scattered runtime notes into structured storage
- Promoting frequently referenced facts to higher-tier storage
- Pruning outdated or obsolete entries
- Setting up memory for a new project or phase

## Memory Tiers

| Tier | Storage | Scope | Lifetime | Content Type |
|------|---------|-------|----------|-------------|
| **Tier 1: MCP Memory** | MCP Memory tool (`/memories/`) | Runtime | Session or persistent | Runtime facts, quick notes, working state, temporary context |
| **Tier 2: AGENTS.md** | `AGENTS.md` file in repo | Project | Permanent (version controlled) | Project conventions, coding standards, architectural decisions, agent roles |
| **Tier 3: Checkpoints** | `docs/checkpoints/` | Phase/session | Semi-permanent (compressed over time) | Progress state, completed work, active blockers, decision history |

### Tier Characteristics

```
MCP Memory (fast, volatile)
    │
    ▼  promote if frequently referenced
AGENTS.md (permanent, project-scoped)
    │
    ▼  progress state captured periodically
Checkpoints (semi-permanent, compressed over time)
```

## Procedures

### 1. Persist a Key Decision

When a significant decision is made:

1. **Immediately:** Record in MCP Memory with tag `decision`:
   ```
   [decision] Use PostgreSQL over MongoDB for structured data — better relational support for the entity model.
   ```

2. **At next checkpoint:** Include in the "Key Decisions" section of the checkpoint.

3. **If the decision is a lasting convention:** Promote to `AGENTS.md` under the appropriate section.

### 2. Persist a Recurring Pattern

When a pattern is observed multiple times:

1. Record the first occurrence in MCP Memory.
2. On the second occurrence, flag it for promotion.
3. Promote to `AGENTS.md` as a convention:
   ```markdown
   ## Conventions
   - All API endpoints return `{ data, error, meta }` envelope format
   ```

### 3. Consolidation Procedure

Run consolidation periodically (every 2-3 checkpoints or when memory feels cluttered):

1. **Review MCP Memory entries.**
2. **Categorize each entry:**

   | Category | Action |
   |----------|--------|
   | **Keep** | Still relevant, still at the right tier |
   | **Promote** | Frequently referenced → move up to AGENTS.md |
   | **Demote** | Phase-specific → ensure captured in checkpoint, remove from MCP Memory |
   | **Prune** | Stale, obsolete, or superseded → remove |

3. **Execute the categorization:**
   - For promotions: Add to `AGENTS.md`, remove from MCP Memory.
   - For demotions: Verify captured in latest checkpoint, remove from MCP Memory.
   - For prunes: Remove from MCP Memory.

4. **Record the consolidation:**
   ```
   [memory-consolidation] Reviewed N entries: K kept, P promoted, D demoted, R pruned.
   ```

### 4. Promotion Rules

An entry should be promoted from MCP Memory to `AGENTS.md` when:

- It has been referenced 3+ times across different tasks or sessions
- It represents a project-wide convention or standard
- It is an architectural decision that affects multiple components
- It defines an agent role, responsibility, or workflow

### 5. Pruning Rules

An entry should be pruned (removed) when:

- The information is no longer accurate (superseded by a newer decision)
- The task or context it relates to has been completed and archived
- It duplicates information already in `AGENTS.md` or a checkpoint
- It has not been referenced in 2+ checkpoints

### 6. Setting Up Memory for a New Project

1. Create `AGENTS.md` with:
   - Project overview
   - Agent roles and responsibilities
   - Initial conventions and standards
   - File structure overview

2. Initialize MCP Memory with:
   - Current phase and objectives
   - Key context from project kickoff

3. Create `docs/checkpoints/` directory.
4. Write `checkpoint-001.md` as the initial state snapshot.

## Memory Retrieval Strategy

When an agent needs context, search in this order:

1. **MCP Memory** — for current session context and recent decisions
2. **AGENTS.md** — for project conventions, roles, and standards
3. **Latest checkpoint** — for recent progress and active state
4. **Older checkpoints** — for historical decisions and completed work (check compressed versions first)

## Examples

### Consolidation Session

Before consolidation (MCP Memory):
```
[decision] Use JWT for API auth
[note] Frontend prefers Tailwind CSS
[temp] Current branch: feature/auth
[decision] All errors use RFC 7807 format
[note] Frontend prefers Tailwind CSS  (duplicate)
[stale] Sprint 1 deadline is March 15  (past)
```

After consolidation:
- **Promoted to AGENTS.md:** JWT for API auth, RFC 7807 error format, Tailwind CSS
- **Pruned:** Duplicate Tailwind note, stale sprint deadline
- **Kept:** Current branch note (still relevant)

### Promotion Example

MCP Memory entry referenced in T005, T008, and T012:
```
[convention] All service classes use constructor injection for dependencies
```

Promote to `AGENTS.md`:
```markdown
## Code Conventions
- All service classes use constructor dependency injection.
```

Remove from MCP Memory after promotion.

## Guidelines

- MCP Memory is for **working state**. Do not let it become a permanent archive.
- `AGENTS.md` is the **source of truth** for project conventions. Keep it curated.
- Checkpoints are **snapshots**. They do not replace structured memory — they supplement it.
- Consolidate proactively. Cluttered memory slows down context retrieval.
- When in doubt about tier, start in MCP Memory. It's easy to promote; hard to un-promote.
- Never delete from `AGENTS.md` without consensus — it's version-controlled for a reason.
