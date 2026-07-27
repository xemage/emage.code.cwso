# Task T165 - Author v0.5.0 changelog and release artifact

- **Status:** pending
- **Owner:** technical-writer
- **Priority:** P0
- **Depends on:** T164
- **Based on:** `docs/artifacts/release-v0.4.1.md`, `CHANGELOG.md`, `docs/plans/plan-v0.5.0-release.md`

## Objective

Produce the two release documents required before a v0.5.0 tag can be cut. This is a
**documentation-only** task — no code changes.

## Scope — files you MAY edit or create

- `docs/artifacts/release-v0.5.0.md` (create)
- `CHANGELOG.md` (edit: prepend one new section only)

## Files you MUST NOT edit

- Anything under `orchestrator/`, `services/`, `scripts/`, `deploy/`
- `.gitlab-ci.yml`
- Existing `docs/artifacts/release-v0.4.*.md` (immutable artifacts)

## Inputs — read these first

1. `docs/artifacts/release-v0.4.1.md` — copy its **structure and heading order exactly**
2. `CHANGELOG.md` — copy the style of the existing `## v0.4.0` section
3. Command to list the scope:
   ```bash
   git log v0.4.1..origin/develop --oneline --no-merges
   ```

## Release facts — use these verbatim, do not re-derive

| Field | Value |
|-------|-------|
| Version | `v0.5.0` |
| Date | `2026-07-27` |
| Prior GA tag | `v0.4.1` (2026-06-19) |
| develop tip | `4c0387f` |
| Breaking changes | none |

### Content that MUST appear in the changelog

**### Features**
- Phase 3.1 task assignment — executor node registry with round-robin task distribution (T235.1)

**### Fixes**
- `fix(transport)`: disable `WriteTimeout` on SSE connections so long-lived streams are not severed
- `fix(mcp)`: rate-limiting refinement with documented HTTP 429 behaviour; burst raised to 10 with localhost exemption
- `fix(jobs)`: `Manager.Close()` drains the queued-job channel before cancelling the root context, so queued jobs reach `cancelled` instead of racing to `completed`
- `fix(rollout)`: deterministic round-robin node ordering (T164)

**### Security** — this section is mandatory
- Go toolchain raised to **1.25.12** (`orchestrator/go.mod`, all three CI images) — remediates
  **GO-2026-5856**, an Encrypted Client Hello privacy leak in `crypto/tls`
- `crossbeam-epoch` pinned to **0.9.20** in `services/cwso-sparse/Cargo.toml` — remediates
  **RUSTSEC-2026-0204**, invalid pointer dereference in `fmt::Pointer` for `Atomic`/`Shared`.
  Transitive path: `wasmtime → rayon-core → crossbeam-deque → crossbeam-epoch`

**### Operations**
- `main` branch integrated into `develop`; production and integration lines are back in sync

### Required note on artifact location

Include this line under a `## Conventions` heading in `docs/artifacts/release-v0.5.0.md`:

> Release notes live at `docs/artifacts/release-v0.5.0.md`, following repository precedent
> (`release-v0.3.0.md`, `release-v0.4.0.md`, `release-v0.4.1.md`). The orchestrator instruction
> referencing `docs/releases/vX.Y.Z.md` and `scripts/verify-release-docs.py` does not apply —
> neither path exists in this repository.

### Version rationale — state this explicitly

Minor bump `v0.4.1 → v0.5.0` because Phase 3.1 task assignment is a new feature. No code-level
version constant needs changing: `services/Cargo.toml` is unmanaged at `0.1.0` and Go has no
version constant. The bump is documentation-only.

## Verification

```bash
test -f docs/artifacts/release-v0.5.0.md && echo OK
grep -q "^## v0.5.0" CHANGELOG.md && echo OK
grep -q "GO-2026-5856" docs/artifacts/release-v0.5.0.md && echo OK
grep -q "RUSTSEC-2026-0204" docs/artifacts/release-v0.5.0.md && echo OK
```

All four must print `OK`.

## Git workflow

Docs-only changes are exempt from feature branches per the git-workflow instruction, but
`develop` is protected, so an MR is still required:

```bash
git checkout develop && git pull origin develop
git checkout -b docs/T165-release-v0.5.0-notes
git add docs/artifacts/release-v0.5.0.md CHANGELOG.md
git commit -m "docs(release): add v0.5.0 changelog and release artifact

Refs T165"
git push origin docs/T165-release-v0.5.0-notes
glab mr create --source-branch docs/T165-release-v0.5.0-notes \
  --target-branch develop --title "docs(release): v0.5.0 notes (T165)" \
  --description "Refs T165" --yes
```

## Acceptance Criteria

- [ ] `docs/artifacts/release-v0.5.0.md` created, heading order matches `release-v0.4.1.md`
- [ ] `## v0.5.0 - 2026-07-27` section prepended to `CHANGELOG.md` above `## v0.4.0`
- [ ] `### Security` section present with both CVE identifiers
- [ ] `## Conventions` note on artifact location present
- [ ] Version rationale stated
- [ ] No files outside `docs/artifacts/` and `CHANGELOG.md` modified
- [ ] MR merged to `develop` with green CI

## STOP conditions

- `git log v0.4.1..origin/develop` shows commits not covered by the changelog content above →
  report the extra commits; do not invent entries for them.
- T164 is not yet merged → stop, this task depends on it.
