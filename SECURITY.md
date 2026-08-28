# Security Policy

## Reporting
Please report vulnerabilities privately to the maintainers. Do not open public issues for security bugs.

## Baseline
See [`docs/artifacts/security-baseline-v1.md`](docs/artifacts/security-baseline-v1.md) for the full threat model, OWASP Top-10 mapping, and immutable constraints.

## Immutable constraints
1. No secrets in source control.
2. No real PII in test data.
3. No bypass of Origin validation, JWT auth, or permission-tier gating — even in dev mode.
4. No untrusted code execution outside Firecracker microVMs.
5. No `--privileged` Docker except the Firecracker host runner.
6. No external network from worker sandboxes by default.

## Supported transports
- `stdio` — local use; trusted process boundary required.
- Streamable HTTP — mandatory `Origin` header validation + JWT auth.

## HAL accelerator endpoints (TLS)
The Hardware Abstraction Layer (`cwso-hal`) sends a bearer API key (`authorization: Bearer …`)
to OpenAI-compatible accelerator endpoints. To prevent that key (and prompt/response payload)
from traversing the network in cleartext:

- **`https://` endpoints** are always allowed.
- **`http://` to a loopback host** (`localhost`, `127.0.0.0/8`, `[::1]`) is allowed — typical
  for a co-located vLLM/Groq sidecar; the traffic never leaves the host.
- **`http://` to any non-loopback host is refused**: the adapter is not registered and an
  error is logged. Set `CWSO_HAL_ALLOW_INSECURE_ENDPOINTS=true` to override (a warning is then
  logged on every startup). The override is intended only for isolated, trusted networks.

Configured via `CWSO_HAL_{GPU,LPU}_BASE_URL`; enforced in `services/cwso-hal/src/security.rs`.

## Sidecar IPC authorization (Unix domain sockets)

The orchestrator talks to the `cwso-git-shadow` and `cwso-merge-engine` sidecars
exclusively over Unix domain sockets (UDS) on a shared `cwso-runtime` Docker
volume (`deploy/docker-compose.yml`), not TCP — neither sidecar publishes a
`ports:` entry or accepts any network connection (`network_mode: "none"`,
`cap_drop: ["ALL"]`). Authorization for these sockets is layered:

1. **Filesystem permission bits.** Each sidecar binds its socket and
   immediately tightens it to `0o660` (owner + group read/write, no world
   access) before accepting any connection:
   - `services/cwso-git-shadow/src/main.rs`, right after `UnixListener::bind`:
     `std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o660))`
   - `services/cwso-merge-engine/src/ipc.rs`, same pattern immediately after
     its own `UnixListener::bind`

2. **Peer credential allowlist (`uid` OR `gid`).** Filesystem bits alone are
   not sufficient authorization on a shared volume where multiple containers
   run as different, independently-assigned users — each sidecar additionally
   inspects the connecting peer's credentials (`SO_PEERCRED`) via
   `IpcAuthzPolicy::from_env()`/`allows()` and accepts the connection only if
   the peer's `uid` is in `CWSO_IPC_ALLOWED_UIDS` **or** its `gid` is in
   `CWSO_IPC_ALLOWED_GIDS` (both are comma-separated lists read from the
   environment at startup, one `OR` check — either allowlist matching is
   sufficient, neither is required to match on its own). This is what
   actually gates which processes may talk to the sidecar; the `0o660` bits
   are the first (filesystem-level) line of defense, not the only one.

3. **Where the allowed uid/gid values come from.** `CWSO_IPC_ALLOWED_UIDS`
   and `CWSO_IPC_ALLOWED_GIDS` are set per-service in
   `deploy/docker-compose.yml` (`git-shadow` and `merge-engine` blocks) as
   literal CSV strings. These values must **cover** the orchestrator
   container's **actual, live** `cwso` user identity — which is assigned
   dynamically by Alpine's `addgroup -S`/`adduser -S` in
   `deploy/Dockerfile.orchestrator` at image build time, not a value
   hardcoded or guaranteed stable across rebuilds. The uid and gid are
   assigned independently and are not guaranteed to match each other
   numerically (see T197: the gid drifted to `101` while the uid stayed
   `100`, and the compose file's hardcoded GID allowlist had gone stale at
   `100` as a result). Because filesystem permissions and the
   peer-credential check are both scoped to a real running container's
   identity, this value must always be looked up live against the built
   image (`docker run --rm --entrypoint id cwso/orchestrator:dev cwso`) —
   never assumed or copied from documentation.

   **Correction (SEC-C044-003, follow-up security review of C044,
   2026-08-28):** the phrasing above previously read "must match the
   orchestrator container's actual, live identity" without qualification,
   which overstated what the allowlist check actually establishes. The
   `uid`/`gid` allowlist is a **numeric identity check, not an
   orchestrator-specific one** — it authorizes *any* peer presenting a
   matching uid or gid, not uniquely "the orchestrator". SEC-C044-001 (below)
   found exactly this: the opt-in `rollout` service coincidentally ran under
   the identical uid=100/gid=101 as the orchestrator (a Debian
   `addgroup --system`/`adduser --system` assignment landing on the same
   numbers as Alpine's independently-assigned `cwso` identity), and would
   have passed both sidecars' authorization check on that basis alone had it
   been able to reach either socket. The allowlist is therefore necessary
   but not sufficient reasoning on its own — reachability also matters: a
   container must both (a) have a numerically-allowlisted identity **and**
   (b) have a filesystem path to the socket (i.e., be mounted on the shared
   `cwso-runtime` volume) to actually authenticate. Keeping both conditions
   narrow — not just the allowlist — is part of this authorization model, not
   an incidental detail.

4. **Drift regression check.** `scripts/check-ipc-gid-drift.sh` builds (or
   reuses) the `cwso/orchestrator:dev` image, looks up its live `cwso`
   uid/gid, and asserts that both are present in `git-shadow`'s and
   `merge-engine`'s `CWSO_IPC_ALLOWED_UIDS`/`CWSO_IPC_ALLOWED_GIDS` entries in
   `deploy/docker-compose.yml`. It exits non-zero on any mismatch so a future
   orchestrator image rebuild that shifts the `cwso` uid/gid again cannot
   silently reintroduce the T197 class of drift. Run it with:
   ```
   bash scripts/check-ipc-gid-drift.sh
   ```
   As of this fix (SEC-C044-002, follow-up security review of C044,
   2026-08-28), this script is also wired into CI as its own job
   (`.gitlab-ci.yml`, `security:ipc-gid-drift`) in the `audit` stage,
   gated the same way as `go:audit`/`rust:audit` (MR pipelines, and
   `develop`/`main` branch pipelines) — a future orchestrator-image rebuild
   that shifts the `cwso` uid/gid now gets an automatic CI failure signal on
   the same class of drift T197 previously had to discover and fix by hand,
   rather than relying on someone remembering to run the script locally.

5. **Why `uid=0` (root) is in the allowlist.** Both `CWSO_IPC_ALLOWED_UIDS`
   and `CWSO_IPC_ALLOWED_GIDS` on `git-shadow` and `merge-engine` are
   `"0,100"` / `"0,101"` — the `0` entry (root) is not currently satisfied by
   any process in the default stack: every service's Dockerfile (
   `deploy/Dockerfile.orchestrator`, `deploy/Dockerfile.git-shadow`,
   `deploy/Dockerfile.merge-engine`) ends in `USER cwso`, so nothing defined
   in `deploy/docker-compose.yml` connects to either sidecar as root, and no
   built-in healthcheck runs as root either (`HEALTHCHECK`/`CMD` inherits the
   image's `USER`). The `0` entry exists to admit an **operator running
   diagnostic tooling as root inside a container** (e.g. `docker exec -u 0
   cwso-orchestrator ...` invoking something that dials the socket directly,
   or an equivalent root-context debugging session) without that session
   being unexpectedly denied by the peer-credential check — it broadens the
   allowlist for manual/administrative access, not for any process that
   currently runs unattended. It is not itself a live risk in the default
   profile (no reachable process presents uid=0), but is called out here so a
   future reader does not mistake it for dead configuration or add a process
   that runs as root without realizing this allowlist would silently accept
   it too.

6. **Fail-open vs. fail-closed behavior of `IpcAuthzPolicy::from_env()`**
   (`services/cwso-git-shadow/src/main.rs`,
   `services/cwso-merge-engine/src/ipc.rs` — identical logic in both):
   - **Unset.** If `CWSO_IPC_ALLOWED_UIDS`/`CWSO_IPC_ALLOWED_GIDS` are not set
     at all, `from_env()` falls back to the sidecar process's **own**
     `geteuid()`/`getegid()` (i.e. it allows only its own identity — that
     is, effectively no external peer at all, since a client dialing in
     would need to share the sidecar's own uid or gid). This is the
     restrictive/fail-closed direction: an operator who forgets to set these
     env vars gets a socket that in practice authorizes nobody but the
     sidecar itself, not one that is silently wide open.
   - **Malformed (unparseable entries or an all-empty CSV).**
     `parse_id_csv()` returns an `Err` — either from a failed `u32` parse on
     a non-numeric token, or explicitly via `anyhow::bail!` when the parsed
     set ends up empty (e.g. `""` or all-whitespace/empty-comma entries).
     That `Err` propagates through `from_env()` and, in `main()`, through
     `IpcAuthzPolicy::from_env()?` — the `?` causes the sidecar process to
     **exit at startup with an error before ever binding the socket**. This
     is also fail-closed: a malformed allowlist prevents the sidecar from
     starting at all rather than starting in a permissive or ambiguous
     state.
   - **Non-Linux targets only** (`authorize_stream()`'s
     `#[cfg(not(target_os = "linux"))]` branch): `SO_PEERCRED` is a
     Linux-specific socket option, so on any non-Linux build the function
     unconditionally returns `Ok(true)` — authorization is skipped entirely.
     This is a genuine fail-open path, but is not a live production concern
     for this project: every deployed target (`deploy/Dockerfile.*`) builds
     and runs on Linux base images (`debian:bookworm-slim`, `alpine:3.20`),
     so this branch is unreachable in any shipped container. Flagged here for
     completeness, not routed as a new finding — if a non-Linux build target
     is ever added to the deployment matrix, this branch would need
     revisiting before that target could be considered equivalently secured.

7. **Live verification (C044, 2026-08-27).** Both the `0o660` permission
   bits and the uid/gid allowlist alignment were independently re-verified
   against a real running `docker compose -f deploy/docker-compose.yml up -d
   --build` stack (not source inspection alone):
   `docker exec cwso-git-shadow stat -c '%a' /run/cwso/git-shadow.sock` and
   the equivalent for `cwso-merge-engine` both report `660`;
   `scripts/check-ipc-gid-drift.sh` exits `0`; and
   `scripts/cwso-smoke-test.sh`'s `merge_concurrent_results` stage exercises
   both sockets end-to-end (orchestrator → git-shadow, orchestrator →
   merge-engine) and passes. See `docs/DEBT-REGISTER.md` row B12 for the full
   evidence transcript.

8. **Resolved risk: `rollout`'s coincidental identity match (SEC-C044-001,
   HIGH, follow-up security review of C044, 2026-08-28).** The opt-in
   `rollout` service (`deploy/docker-compose.yml`, `profiles: ["rollout"]`,
   not part of the default stack) previously mounted the same
   `cwso-runtime` named volume that backs both the `git-shadow` and
   `merge-engine` UDS sockets. Its own image
   (`deploy/Dockerfile.rollout`, Debian `addgroup --system`/
   `adduser --system`) coincidentally assigns it the identical
   uid=100/gid=101 identity as the orchestrator's Alpine-assigned `cwso`
   user — which is exactly what `CWSO_IPC_ALLOWED_UIDS`/
   `CWSO_IPC_ALLOWED_GIDS` allow-list on both sidecars. Because the check is
   `uid OR gid` (point 2 above), `rollout` would have passed both sidecars'
   authorization check purely on that numeric coincidence, despite having
   zero code today that dials either socket (this was speculative,
   provisioned wiring for a not-yet-built future trajectory-drain feature,
   not an active integration).
   **Fix:** the `cwso-runtime:/run/cwso` volume mount was removed from the
   `rollout` service block. `rollout` now has no filesystem path to either
   `.sock` file at all — even a compromised `rollout` container has nothing
   to dial. This is deliberately a reachability fix (remove the unused
   mount), not an identity fix (pin `rollout` to a distinct uid/gid) or a
   documented-risk-acceptance, because nothing in the codebase currently uses
   this access; see `deploy/docker-compose.yml`'s `rollout` service block for
   the full reasoning, including why `CWSO_ROLLOUT_SOCKET` was deliberately
   left set (rather than removed) even though it is now unreachable.
   **Going forward:** any future feature that legitimately needs `rollout` to
   reach these sockets must mount a distinct, narrowly-scoped volume (or
   subset of this one) under an identity **deliberately pinned** to be
   distinct from uid=100/gid=101 — never a reuse of this coincidental match.
   See `docs/DEBT-REGISTER.md` row R-7 for the closing evidence.
