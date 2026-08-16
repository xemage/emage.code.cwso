# Task C002 — Reconcile quick-start commands

**ID:** C002
**Owner:** technical-writer
**Status:** done
**Priority:** P0
**Depends on:** —
**Created:** 2026-08-12
**Completed:** 2026-08-15
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B10); docs/plans/plan-cwso-v1.0-phase0-honest-baseline-v1.md

## Objective

`README.md:51-56` quick-starts with `--profile phase2`; `docs/user/installation-v3.md:29`
uses `--profile phase2 --profile phase4`. Following the README yields a system without
the merge engine. Make the two quick-start command sequences **byte-identical**, using
the command set that starts the full stack as it exists today (Phase 0, before C010
removes the profile gates).

## Inputs

- `README.md` (quick-start code block, lines 51–56)
- `docs/user/installation-v3.md` (quick-start, ~line 29)
- `deploy/docker-compose.yml` (to confirm which profiles gate which services: `phase2` → git-shadow, `phase4` → merge-engine)

## Rails (read before starting)

### You MUST
- Use this exact command sequence in **both** files (it reflects the pre-C010 compose file):
  ```bash
  make build
  docker compose -f deploy/docker-compose.yml --profile phase2 --profile phase4 up -d
  curl -sS http://127.0.0.1:8080/healthz
  python3 scripts/phase2-integration.py
  ```
- Verify the two blocks are byte-identical with `diff` after editing
- Add a one-line HTML comment above each block: `<!-- NOTE: profiles removed in v0.8.0 (C010); this block must stay identical in README.md and installation-v3.md -->`

### You MUST NOT
- Delete or rewrite any installation guide (that is C051, Phase 5)
- Remove the `--profile` flags yet — C010 removes the gates; until then the flags are required for a full stack
- Touch any other section of either file
- "Improve" the commands (e.g., adding `docker compose ps`) — consistency, not enhancement, is the goal

## File ownership

- **May create/modify:** `README.md` (quick-start block only), `docs/user/installation-v3.md` (quick-start block only)
- **Must NOT touch:** everything else

## Steps (execute in order)

1. Read both quick-start blocks and the compose file profile lines.
2. Replace both blocks with the exact sequence above.
3. Add the HTML comment above each block.
4. Run the verification command.

## Expected outputs

- Byte-identical quick-start blocks in `README.md` and `docs/user/installation-v3.md`

## Acceptance criteria

1. The `diff` verification below produces no output
2. Both blocks carry the HTML comment
3. No other lines in either file changed (`git diff --stat` shows exactly 2 files)

## Verification commands

```bash
sed -n '/^```bash$/,/^```$/p' README.md | head -8 > /tmp/qs-readme.txt
sed -n '/^```bash$/,/^```$/p' docs/user/installation-v3.md | head -8 > /tmp/qs-install.txt
diff /tmp/qs-readme.txt /tmp/qs-install.txt && echo "PASS: byte-identical"
git diff --stat
```

## Git rails

- Branch: `agent/technical-writer/C002` from `develop`
- Commit: `docs: reconcile quick-start commands between README and installation-v3`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the two files disagree about more than the quick-start (e.g., contradictory
prerequisites), do not reconcile by guesswork — record the conflict in the MR
description and flag `unclear_requirements` / `minor`.

## Execution notes

<filled during execution>
