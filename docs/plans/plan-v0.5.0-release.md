# Release Plan: v0.5.0

**Date**: 2026-07-27  
**Status**: PLANNING (awaiting approval)  
**Release Manager**: @orchestrator

## Overview

Release v0.5.0 to production main branch, incorporating Phase 3.1 enhancements, transport hardening, and MCP rate limiting improvements since v0.4.1 GA (2026-06-19).

## Scope

### Since v0.4.1 (v0.4.1 GA → develop at 4c0387f)
- **Phase 3.1 task assignment** (feat(T235.1)): Parallel task execution framework
- **Transport SSE hardening**: 
  - Disable WriteTimeout for SSE connections (fix performance)
  - Rate limit burst=10 for localhost exemption
  - Better 429 error handling documentation
- **MCP improvements**: Rate limiting refinement, callback documentation
- **Documentation**: Updated installation and artifact guides
- **Main integration**: Merged main into develop (commit 24a0b75), bringing v0.4.1 production code forward
- **Race condition fix**: Fixed TestCloseCancelsQueuedJobs in manager.Close() (fix(jobs): drain queue before cancelling)

- **Security fixes** (verified present on develop, must appear in the changelog):
  - Go toolchain `1.25.0 → 1.25.12` — GO-2026-5856, ECH privacy leak in `crypto/tls`
  - `crossbeam-epoch` pinned to `0.9.20` — RUSTSEC-2026-0204, invalid pointer dereference

### Version Bump Rationale
- v0.4.1 → v0.5.0 (minor bump) because Phase 3.1 task assignment is a new feature.
- Documentation-only bump: there is no code-level version constant. `services/Cargo.toml`
  is unmanaged at `0.1.0`; Go declares no version constant.

### Release-doc location deviation
Orchestrator instructions mandate `docs/releases/vX.Y.Z.md` and `scripts/verify-release-docs.py`.
**Neither exists in this repository.** This plan follows repo precedent —
`docs/artifacts/release-vX.Y.Z.md` — and the deviation is logged here deliberately.

## Release Blocking Conditions

✅ **Security Gate**: Passed (audit jobs green on MR !74)  
🔴 **Develop CI pipeline**: FAILED — pipeline #2708251138, job `go:test` (#15548635752)  
✅ **MR !74 (main→develop merge) pipeline**: PASSED all 11 jobs  

### Diagnosed blocker (not transient)

```
--- FAIL: TestNodeRegistry_AssignTask (0.00s)
    node_registry_test.go:39: assignments: map[executor-2:4]
    node_registry_test.go:40: expected tasks assigned to at least 2 nodes
FAIL  github.com/emage/cwso/orchestrator/internal/rollout
```

`internal/jobs` **passed** in that run — the `Manager.Close()` race fix is sound. The real
defect is in `getActiveNodesLocked()`, which builds its slice by ranging over a Go map.
Randomised map order means `activeNodes[lastAssignedIdx%len(activeNodes)]` indexes a shuffled
slice, so round-robin can assign every task to one node. MR !74 passed only by luck; this will
randomly fail the release MR too. Tracked as **T164**.

### Release Preflight Checklist
- [ ] T164 merged; `develop` pipeline green
- [ ] `CHANGELOG.md` v0.5.0 section added
- [ ] Release notes (`docs/artifacts/release-v0.5.0.md`) created
- [ ] Release branch `release/v0.5.0` cut from develop
- [ ] Release MR (release/v0.5.0 → main) merged with a **merge commit**
- [ ] Annotated tag `v0.5.0` pushed on the main merge commit
- [ ] GitLab release published from the artifact file
- [ ] Back-merge (main → develop) merged, release branch deleted

## Tasks

| ID | Task | Owner | Depends on | Brief |
|----|------|-------|-----------|-------|
| T164 | Fix flaky `TestNodeRegistry_AssignTask` | backend-developer | — | [task-T164.md](../tasks/task-T164.md) |
| T165 | Author v0.5.0 changelog + release artifact | technical-writer | T164 | [task-T165.md](../tasks/task-T165.md) |
| T166 | Cut release/v0.5.0, merge to main | release-manager | T165 | [task-T166.md](../tasks/task-T166.md) |
| T167 | Tag v0.5.0, publish GitLab release | release-manager | T166 | [task-T167.md](../tasks/task-T167.md) |
| T168 | Back-merge main → develop, cleanup | release-manager | T167 | [task-T168.md](../tasks/task-T168.md) |

## Git Workflow (GitFlow)

```
develop (4c0387f)
  ↓ create branch
release/v0.5.0 (new)
  ↓ create MR → main
main (892075e)
  ↓ merge MR
main (merge commit)
  ↓ tag v0.5.0
v0.5.0 tag
  ↓ hotfix merge back
develop
```

## Release Actions

### Phase 1: Documentation (Release Manager)
1. Read `docs/artifacts/release-v0.4.1.md` as template reference
2. Create `docs/artifacts/release-v0.5.0.md` with:
   - Metadata (producer, created date, based-on, develop tip, prior GA tag)
   - Release intent (Phase 3.1, transport hardening, main sync)
   - Scope table (commits, features, fixes)
   - Changelog per Conventional Commit categories
   - Feature flag matrix (unchanged from v0.4.1 except new Phase 3.1 flags if any)
   - Validation evidence (CI pipeline pass, test results)
   - RC verdict (PASS)
   - Migration guide (no breaking changes)

### Phase 2: Branch Creation
1. `git checkout -b release/v0.5.0 origin/develop`
2. Push to origin: `git push origin release/v0.5.0`

### Phase 3: Merge to Main (MR process)
1. Create MR: release/v0.5.0 → main
2. Title: `release(v0.5.0): Phase 3.1 + transport hardening`
3. Description: Link to release artifact
4. Gate: all 11 CI jobs green (sole maintainer — CI green is the effective approval gate)
5. Merge with a **standard merge**, not squash — `v0.4.1` sits on `Merge branch 'release/v0.4.1' into 'main'` (`03bbf25`). Squashing breaks main↔develop ancestry and turns the back-merge into a conflict replay.
6. Keep the release branch until Phase 5 completes

### Phase 4: Tag, then publish
```bash
git checkout main && git pull origin main
git tag -a v0.5.0 -m "release: v0.5.0"
git push origin v0.5.0
# wait for the tag pipeline to go green, then:
glab release create v0.5.0 --ref v0.5.0 --name v0.5.0 -F docs/artifacts/release-v0.5.0.md
```
The explicit `git tag` is required — `--ref v0.5.0` resolves an existing tag and cannot create one.

### Phase 5: Back-merge (main → develop)
1. Create MR: main → develop (GitFlow back-merge, **not** a `hotfix/*` branch)
2. Title: `chore: back-merge v0.5.0 release into develop`
3. Standard merge, preserve history
4. Delete `release/v0.5.0` only after this lands

## Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|-----------|
| Flaky `TestNodeRegistry_AssignTask` randomly fails the release MR | CRITICAL | T164 sorts nodes by ID; verify with `go test -race -count=20` |
| Release MR squashed by project default | HIGH | Explicitly disable squash before merging (T166) |
| Tag created before the merge commit exists | HIGH | Tag only after the MR lands on main (T167) |
| Release branch deleted before back-merge | MEDIUM | Cleanup is the last step of T168 |
| Git push rejected on protected branch | LOW | All changes go through MRs |

## Success Criteria

✅ Release artifact (release-v0.5.0.md) published in docs/artifacts/  
✅ v0.5.0 tag created on main with proper release notes  
✅ All CI jobs green on release MR  
✅ Develop pipeline green and synced with main  
✅ Release notes published to GitLab releases page  
✅ No commits stranded on release/* branch after merge  

## Approval Required

**User decision needed:**
1. Approve v0.5.0 as the version (vs v0.4.2)?
2. Authorize release to proceed once develop pipeline is green?
3. Any additional validation gates required before tagging?
