# Task C050 — Write the single user guide (docs/user/README.md)

**ID:** C050
**Owner:** technical-writer
**Status:** pending
**Priority:** P1
**Depends on:** C040–C044 (Phase 4 complete); requires the post-Phase-1 flow (C010–C018)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B8, TODO quote); docs/plans/plan-cwso-v1.0-phase5-one-document-v1.md

## Objective

Write `docs/user/README.md` — the **single** guide: prerequisites → install → configure
client → verify → daily use → troubleshoot. Written against the *post-Phase-1* flow
(`make up`), not the current 7-step one. After this phase, `docs/user/` contains exactly
this guide.

## Inputs

- The post-Phase-1 flow: `make up` (C016), `cwso-doctor.sh` (C017), `cwso-token.sh` (C013), `CWSO_WORKSPACE_HOST` (C015), smoke test (C018)
- `docs/artifacts/mcp-client-compatibility-v1.md` (C033 — which clients/transports to document)
- `docs/SCOPE-v1.0.md` (C005 — what to promise and not promise)
- `docs/LIMITATIONS.md` if C025 or C044 landed limitations
- The five guides being replaced (for salvageable content only): `installation-v1/v2/v3.md`, `ide-integration-v1/v2.md`

## Rails (read before starting)

### You MUST
- Structure: Prerequisites → Install (`git clone && make up`) → Configure your MCP client (paste block) → Verify (`make doctor`, smoke test) → Daily use (workspace mounting, tokens, rollout opt-in) → Troubleshoot (doctor output → fixes)
- Write every command exactly as it will run post-Phase-1 — C054 will execute each one on a clean machine; an unrunnable command blocks CG4
- Document only what exists: no "coming soon" features, no v1.1 items
- Keep it client-agnostic with per-client config subsections for the clients C033 verified
- State limitations plainly (link `docs/LIMITATIONS.md` if it exists)

### You MUST NOT
- Carry forward any `--profile phase2/phase4` commands, heredocs, or source-the-script steps — those are gone post-Phase-1
- Delete the old guides (that is C051)
- Document internals/architecture (that is contributor docs, C053) — one cross-link, no more
- Exceed what a new user needs: this is a user guide, not a reference manual

## File ownership

- **May create/modify:** `docs/user/README.md` (new)
- **Must NOT touch:** the five old guides (C051 deletes them), code, other docs

## Steps (execute in order)

1. Read the post-Phase-1 flow artifacts (Makefile, scripts, compose).
2. Draft the guide section by section per the structure above.
3. Self-check every command against the actual scripts/Makefile.
4. Cross-link contributor docs (one link) and limitations.

## Expected outputs

- `docs/user/README.md` — the single guide

## Acceptance criteria

1. Follows the six-section structure
2. Every command is post-Phase-1-accurate (C054 will verify on a clean machine)
3. No version suffix in the filename; no references to deleted guides
4. Limitations stated plainly

## Verification commands

```bash
grep -c "make up\|make doctor" docs/user/README.md
grep -c "profile phase\|heredoc\|enable-all-features" docs/user/README.md   # = 0
ls docs/user/
```

## Git rails

- Branch: `agent/technical-writer/C050` from `develop`
- Commit: `docs(user): write the single user guide`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If a post-Phase-1 command does not yet exist when you write this, that is a
`dependency` / `critical` blocker — the guide documents reality, not aspiration.

## Execution notes

<filled during execution>
