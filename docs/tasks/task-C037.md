# Task C037 — Document that the OAuth-fallback/bearer-auth limitation is not VS-Code-specific

**ID:** C037
**Owner:** technical-writer
**Status:** done
**Priority:** P2
**Depends on:** —
**Created:** 2026-08-26
**Completed:** 2026-08-27
**Based on:** `docs/artifacts/mcp-client-compatibility-v1.md` (C033) § "Cross-cutting findings," Finding B; `docs/user/installation-v3.md` §7; `docs/artifacts/mcp-gap-analysis-v1.md`'s HS256-bearer-scheme note

## Objective

`docs/user/installation-v3.md` §7 ("Fix for 'Dynamic Client Registration not
supported'") already documents CWSO hitting an OAuth-discovery fallback when a
client's Streamable HTTP request arrives without a valid bearer token — framed
entirely as a VS Code troubleshooting step.

C033's client-compatibility testing independently reproduced the same underlying
class of failure against a second, unrelated real client (`@wong2/mcp-cli`) — and
worse: that client's OAuth-fallback path doesn't just prompt for a client ID, it
**crashes** trying to JSON-parse CWSO's plain-text `401` body as an OAuth error
document (`SyntaxError: Unexpected token 'm', "missing be"... is not valid JSON`).

This is not a new defect to fix — CWSO's custom HS256 JWT bearer scheme is spec-legal
(the MCP spec's authorization framework is optional) and was a deliberate choice
recorded in `mcp-gap-analysis-v1.md`. This task is purely about making the existing
documentation honestly reflect that the limitation is a general interoperability cost
of that choice, not a VS-Code-specific quirk — so a user hitting this with a
*different* client doesn't waste time assuming it's a VS Code bug or that switching
clients avoids it.

## Inputs

- `docs/user/installation-v3.md` §7 (the existing VS-Code-framed fix section)
- `docs/artifacts/mcp-gap-analysis-v1.md` (the existing note recording CWSO's HS256
  bearer scheme as spec-legal but an interoperability cost — find and reference the
  exact passage, don't paraphrase from memory)
- `docs/artifacts/mcp-client-compatibility-v1.md` (C033) § "Cross-cutting findings,"
  Finding B — the concrete, second-client reproduction this task is cross-referencing

## Rails (read before starting)

### You MUST
- Update `installation-v3.md` §7 to state plainly that this failure mode is a
  consequence of CWSO's bearer-only (no OAuth) authentication design, and can occur
  with **any** MCP client whose only remote-auth mechanism is OAuth — not just VS Code
  — citing C033's `@wong2/mcp-cli` finding as a concrete second example
- Add a cross-reference from `mcp-gap-analysis-v1.md`'s existing HS256/bearer-scheme
  note to `mcp-client-compatibility-v1.md`'s Finding B, so a reader following the gap
  analysis lands on the concrete client-impact evidence, not just the abstract
  spec-legality note
- Keep the existing §7 fix steps (they are still correct and still apply) — this task
  reframes *why* the problem happens and *who* it can happen to, not the remediation
  steps themselves

### You MUST NOT
- Reopen or re-litigate the HS256-bearer-scheme design decision itself (that was a
  considered choice, out of scope here)
- Modify any server code, config, or the `mcp-client-compatibility-v1.md` artifact
  itself (C033's artifact is final; cross-reference it, don't edit it)
- Expand into a general authentication-methods rewrite of the installation guide

## File ownership

- **May create/modify:** `docs/user/installation-v3.md` (§7 only),
  `docs/artifacts/mcp-gap-analysis-v1.md` (only the existing HS256/bearer-scheme note,
  to add the cross-reference)
- **Must NOT touch:** any code, `docs/artifacts/mcp-client-compatibility-v1.md`, other
  sections of `installation-v3.md`

## Steps (execute in order)

1. Read `installation-v3.md` §7, `mcp-gap-analysis-v1.md`'s HS256 note, and
   `mcp-client-compatibility-v1.md`'s Finding B yourself before editing anything.
2. Reframe §7's opening to name the general cause (bearer-only auth, OAuth-only
   clients) before the VS-Code-specific symptom text, citing the `@wong2/mcp-cli`
   finding as evidence this isn't VS-Code-specific.
3. Add the cross-reference in `mcp-gap-analysis-v1.md`.

## Expected outputs

- `installation-v3.md` §7 reframed to state the general cause, not just the VS Code
  symptom
- A cross-reference from `mcp-gap-analysis-v1.md`'s HS256 note to
  `mcp-client-compatibility-v1.md`'s Finding B

## Acceptance criteria

1. §7 explicitly states this can affect any OAuth-only client, citing C033's finding
2. `mcp-gap-analysis-v1.md`'s HS256 note cross-references `mcp-client-compatibility-v1.md`
3. Existing §7 remediation steps unchanged
4. No code, config, or other docs touched

## Verification commands

```bash
grep -n "wong2\|any MCP client" docs/user/installation-v3.md
grep -n "mcp-client-compatibility-v1" docs/artifacts/mcp-gap-analysis-v1.md
git diff --stat
```

## Git rails

- Branch: `agent/technical-writer/C037` from `develop`
- Commit: `docs: cross-reference OAuth-fallback limitation as client-general, not VS-Code-specific`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

<filled during execution>
