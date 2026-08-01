@AGENTS.md

## Claude Code

This project follows the emage.code multi-agent protocol defined in the
imported `AGENTS.md` above. Claude Code-specific notes:

- Subagents live in `.claude/agents/*.md` — invoke one explicitly with
  `@<agent-slug>` (e.g. `@orchestrator`, `@backend-developer`) or let Claude
  delegate automatically based on each subagent's `description`.
- Skills live in `.claude/skills/*/SKILL.md`; slash commands live in
  `.claude/commands/*.md`. Both are invoked with `/<name>`.
- Path-scoped instructions live in `.claude/rules/*.md` (frontmatter `paths:`).
- MCP servers are declared in `.mcp.json` at the project root. Set the
  required environment variables before starting a session (see
  `.mcp.json` for the `env` keys each server expects).
- Start every new project with `/new-project` or `/discover-skills`, exactly
  as with every other supported platform.
