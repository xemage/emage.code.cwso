---
description: "Review MCP Memory entries, prune stale items, and promote valuable items to AGENTS.md or project docs. Use periodically to keep memory clean and relevant."
agent: "orchestrator"
argument-hint: "Optional: focus area or category to consolidate..."
---

You are in **Memory Consolidation mode**. Review, organize, and clean up MCP Memory entries to keep the project knowledge base accurate and lean.

## Instructions

1. **Read all MCP Memory entries** — use `mcp__memory` to retrieve the full set of stored memories. If a focus area is provided, filter to that category.
2. **Categorize each entry** — assign one of:
   - **Keep** — still relevant, correctly scoped, leave in memory
   - **Promote** — frequently referenced or broadly useful; should be elevated to `AGENTS.md`, project docs, or skill files
   - **Prune** — stale, obsolete, superseded, or duplicated; safe to remove
3. **Present recommendations** — show a table with:
   - Memory key/identifier
   - Current content summary (one line)
   - Recommended action (Keep / Promote / Prune)
   - Rationale (why this action)
4. **Wait for user approval** — do NOT execute changes until the user confirms which recommendations to apply.
5. **Execute approved changes**:
   - **Promote**: append the content to the appropriate section of `AGENTS.md` or the relevant doc, then remove from memory
   - **Prune**: delete the memory entry
   - **Keep**: no action needed
6. **Report results** — summarize what was promoted, pruned, and kept. Note the new memory entry count.

## Important

- Never delete a memory entry without explicit user approval.
- When promoting to `AGENTS.md`, place the content in the most relevant section and format it consistently with existing entries.
- If you find contradictory memories, flag them for user resolution rather than choosing one silently.

## Focus Area

{{input}}
