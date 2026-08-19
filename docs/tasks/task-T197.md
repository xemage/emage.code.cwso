# Task T197 — Fix `CWSO_IPC_ALLOWED_GIDS` drift (hardcoded gid 100 vs orchestrator's live gid 101)

**ID:** T197
**Owner:** devops-engineer
**Status:** pending
**Priority:** P2 — not blocking, defense-in-depth hygiene (see severity note below)
**Depends on:** —
**Created:** 2026-08-19
**Completed:** —
**Based on:** Discovered incidentally during T191 (`.env.jwt.dev` permission fix, MR
!132) — the worker looked up the orchestrator image's live `cwso` uid/gid while
implementing T191's fix and found it doesn't match this pre-existing compose config.
Not part of T191's scope, logged separately per the orchestrator's defect-disposition
call.

## Objective

`deploy/docker-compose.yml` hardcodes `CWSO_IPC_ALLOWED_GIDS: "0,100"` on both
`git-shadow` and `merge-engine` (lines ~56, ~77) — the allowlist of GIDs permitted to
connect to their Unix domain socket IPC servers. T191's live verification confirmed
the `orchestrator` container's actual `cwso` user is `uid=100` but **`gid=101`**, not
`100` — so the hardcoded GID entry does not match the orchestrator's real group.

## Independently re-verified severity assessment (do not skip this before scoping a fix)

**This is real config drift, but it is currently latent, not an active
access-control failure — get this right before treating it as more urgent than it
is.** `services/cwso-git-shadow/src/main.rs` and
`services/cwso-merge-engine/src/ipc.rs` both implement the allowlist check as:

```rust
fn allows(&self, cred: &PeerCred) -> bool {
    self.allowed_uids.contains(&cred.uid) || self.allowed_gids.contains(&cred.gid)
}
```

This is an **OR**, not an AND. `CWSO_IPC_ALLOWED_UIDS` is also hardcoded to `"0,100"`
on both services, and `100` **does** match the orchestrator's real live UID — so
today, the orchestrator's IPC connections are correctly permitted via the UID branch
of the check, independent of the broken GID entry. The GID allowlist entry is
currently dead weight: wrong, but not gating anything that's actually failing right
now.

**Why it's still worth fixing, not just noting and ignoring:** the GID check exists
as defense-in-depth / an intended fallback (e.g. if a future deployment ran the
orchestrator under a different UID but the same group, or supplementary-group-based
access patterns), and a config value that silently doesn't do what its name claims is
exactly the kind of drift that causes confusing incidents later, once someone
actually depends on it. Fix it for correctness and to stop the drift from compounding,
but do not describe this task (in commits, MR, or ledger) as fixing an active
security hole — it isn't one today, and overstating it would misrepresent the actual
risk to future readers.

## Inputs

- `deploy/docker-compose.yml` (`CWSO_IPC_ALLOWED_GIDS`/`CWSO_IPC_ALLOWED_UIDS` on
  `git-shadow` and `merge-engine`)
- `deploy/Dockerfile.orchestrator` (`addgroup -S cwso && adduser -S -G cwso cwso` —
  the source of the orchestrator's actual, image-assigned uid/gid; note this can
  legitimately differ across image rebuilds if earlier system users/groups are added
  or removed in the same build stage, which is exactly how a hardcoded gid drifts)
- `services/cwso-git-shadow/src/main.rs` (`PeerCred`/`allows()` — read-only, do not
  weaken the OR-based check itself as part of this fix)
- `services/cwso-merge-engine/src/ipc.rs` (same pattern)
- T191's live-verified finding (`uid=100`, `gid=101`) as the originating evidence —
  re-confirm it yourself rather than assuming it's still accurate by the time you
  work this, since image rebuilds could shift it again

## Rails (read before starting)

### You MUST
- Re-verify the orchestrator image's actual live `cwso` uid/gid yourself before
  changing anything (do not assume T191's numbers are still current)
- Fix `CWSO_IPC_ALLOWED_GIDS` so it actually reflects the orchestrator's real gid —
  prefer a solution that can't drift again silently (e.g. deriving the value at
  container-start/compose-build time rather than a second hardcoded literal) if
  that's achievable within reasonable scope; a corrected hardcoded literal is an
  acceptable minimum fix if a dynamic solution is out of proportion for this task
- Apply the fix consistently to both `git-shadow` and `merge-engine` (and check
  `cwso-sparse`/`cwso-hal`/`cwso-rollout`'s `ipc.rs` too — they share the same
  `CWSO_IPC_ALLOWED_GIDS`/`allows()` pattern per a repo-wide grep; confirm whether
  they're also affected and either fix them too or explicitly note why they're out
  of scope, e.g. if they're not currently wired into compose with this env var at
  all)
- Add a regression check (test, or a compose-config assertion, your call on the most
  proportionate mechanism) that would catch this exact class of drift recurring
- Add a CHANGELOG `## Unreleased` entry that accurately describes this as a
  **latent, not active**, fix (per the severity note above — do not overstate it)

### You MUST NOT
- Weaken the `allows()` OR-logic itself, or remove the GID check as "unnecessary
  since UID already works" — the GID branch is intentional defense-in-depth, fix its
  value, don't delete it
- Touch `orchestrator/internal/config/config.go`'s secret-handling logic (T191's
  territory, unrelated)
- Overstate this as an active vulnerability in commit messages/CHANGELOG/MR — it
  isn't one today, per the severity analysis above; misrepresenting severity is its
  own kind of drift

## File ownership

- **May create/modify:** `deploy/docker-compose.yml`, `CHANGELOG.md` (Unreleased),
  plus whatever test file is appropriate for the regression check (identify the
  right location — likely alongside existing `ipc.rs` tests in the affected
  service(s), or a compose-config-level check if that's more proportionate)
- **Must NOT touch:** `orchestrator/internal/config/config.go`, sandbox tiering,
  MCP tool surface

## Acceptance criteria

1. `CWSO_IPC_ALLOWED_GIDS` matches the orchestrator's actual live gid on both
   `git-shadow` and `merge-engine`
2. A regression mechanism exists that would catch this drift recurring (test or
   config-check, proportionate to the fix)
3. The `allows()` OR-logic and UID allowlist are unchanged
4. CHANGELOG/commit language accurately reflects this as latent drift, not an active
   vulnerability
5. `git diff --stat` touches only the files justified above

## Verification commands

```bash
docker run --rm cwso/orchestrator:dev id cwso   # confirm current live uid/gid
grep -n "CWSO_IPC_ALLOWED_GIDS" deploy/docker-compose.yml
cd services && cargo test -p cwso-git-shadow -p cwso-merge-engine
```

## Git rails

- Branch: `agent/devops-engineer/T197` from `develop`
- Commit: `fix(deploy): correct CWSO_IPC_ALLOWED_GIDS drift for orchestrator's live gid`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
This is not expected to be a hard task — if it turns out to be more involved than
expected (e.g. the gid genuinely can't be determined statically and needs real
dynamic derivation plumbing), report `technical` / `minor` rather than force a
low-quality fix, given the low current severity.

## Execution notes

<filled during execution>
