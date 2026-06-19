# Checkpoint 016 — v0.4.1 Release Ready

**Phase:** Release → GA Pending  
**Date:** 2026-06-19  
**Previous checkpoint:** `checkpoint-015-v0.4.1-hardening-complete.md`  
**Status:** RELEASE_READY (awaiting stakeholder sign-off to promote to GA)

---

## Executive Summary

All v0.4.1 hardening and Polar parity work is delivered, validated, and packaged. The release/v0.4.1 branch is green on all CI jobs, the v0.4.1-rc1 tag is published, and MR !70 is open to main. The only remaining gate is stakeholder approval to merge and cut the GA release.

---

## Completed Since Checkpoint-015

| Task | Title | Artifact |
|------|-------|----------|
| T150 | KV differential prompting | `capture.rs` + tests |
| T151 | Offline SFT data generation | `service.go` + tests |
| T158 | Phase 2 integration auth fix | `make smoke-local` PASS |
| T159 | Smoke-local Makefile target | `Makefile` target |
| T160 | v0.4.0 documentation drift | checkpoint-014, installation-v2.md |
| T161 | Task board hygiene | completed-tasks.md (80 rows migrated) |
| T162 | Reliability/security debt | TD-05/06/08 resolved |
| T163 | Hardening validation gate | `gate-v0.4.1-hardening-2026-06-18.md` PASS |

### Release Packaging (this session)

| Action | Output | Status |
|--------|--------|--------|
| Create `release/v0.4.1` branch | `origin/release/v0.4.1` | ✅ |
| Publish `v0.4.1-rc1` tag | `origin/v0.4.1-rc1` | ✅ |
| Release notes artifact | `docs/artifacts/release-v0.4.1.md` | ✅ |
| Release MR to main | MR !70 (labels: release, hardening, polar-parity) | ✅ |
| Task board cleared | active-tasks.md empty; T150–T163 in completed-tasks.md | ✅ |

---

## CI Evidence

| Pipeline | Ref | Status |
|----------|-----|--------|
| #2611568664 | develop (a3c9c31) | **SUCCESS** (all 11 jobs) |
| #2611600722 | release/v0.4.1 (initial push) | canceled (superseded) |
| #2611602944 | release/v0.4.1 (7f058ef) | **SUCCESS** |
| #2611603061 | v0.4.1-rc1 tag | **SUCCESS** |
| #2611604388 | MR !70 (merge-request pipeline) | **SUCCESS** |

All CI contexts are green. No failing or skipped jobs.

---

## Key Decisions

- `release/v0.4.1` branched from `develop@a3c9c31` (post CI-fix commit)
- Release notes committed directly to `release/v0.4.1` (commit `7f058ef`) so the artifact is part of the branch, not re-merged separately
- MR !70 uses `--remove-source-branch` flag — release/v0.4.1 will be deleted after merge
- No breaking changes from v0.4.0; all new features are opt-in via env flags

---

## Active Tasks

**None.** Active task board is empty. All v0.4.1 work is done.

---

## Blocked / Deferred Items

| Item | Reason | Resolution path |
|------|--------|-----------------|
| GA merge to main | Awaiting stakeholder sign-off | User approves MR !70 |
| v0.4.1 final GA tag | Requires merge to main first | After MR merge: `git tag v0.4.1 main` |
| GitLab Release entry | Requires GA tag | After tag: `glab release create v0.4.1` |
| Staging deployment | Follows GA tag | DevOps to deploy from GA tag |

---

## Next Steps (Ordered)

1. **[USER ACTION REQUIRED]** Review and approve MR !70 at: https://gitlab.com/em-age/emage.code.cwso/-/merge_requests/70
2. After merge: tag `v0.4.1` on main
3. Create GitLab Release entry referencing `docs/artifacts/release-v0.4.1.md` and `installation-v2.md`
4. Trigger staging deployment from v0.4.1 GA tag
5. Archive release artifacts to `docs/archive/`

---

## Token Budget

| Phase | Budget | Used (est.) |
|-------|--------|-------------|
| Implementation (T150–T162) | 120k | ~110k |
| QA / Security / Release | 60k | ~45k |
| **Total this sprint** | 180k | **~155k** |

On-budget.
