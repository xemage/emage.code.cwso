---
description: "List and recommend skills from the knowledge registry for the current task or intent."
agent: "orchestrator"
argument-hint: "Task intent, e.g. 'debug CI failure' or 'prepare release'..."
---

You are in **Skill Discovery** mode. Match the user's intent to canonical skills.

## Instructions

1. **Read registry** — load `implementation/registry/index.json` (or `registry/summary.md` for overview).
2. **Parse intent** — from `{{input}}` or active task brief; identify phase (plan, implement, debug, review, release).
3. **Recommend skills** — return a ranked table:

| Skill | Why | Mandatory? |
|-------|-----|------------|
| ... | one-line rationale | yes/no |

4. **Apply mandatory rules** from `AGENTS.md` Skill Workflow:
   - Implementation → `verification-before-completion` before done claims
   - Bugs/tests → `systematic-debugging` before fixes
   - Review feedback → `receiving-code-review` before edits
   - Phase transitions → relevant gate skill (`validation-gates`, `checkpoint-protocol`)
5. **Offer invocation** — tell the user which skill file to load or which slash command to run next.
6. **Optional packaging** — if a skill pack is installed (`packs/installed/`), merge pack registry entries into recommendations.

## Output format

```markdown
## Recommended skills for: <intent>

### Mandatory
- ...

### Suggested
- ...

### Next command
`/...` or read `skills/<name>/SKILL.md`
```
