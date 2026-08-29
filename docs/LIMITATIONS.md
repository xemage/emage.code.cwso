# CWSO v1.0 — Limitations

**Last Updated:** 2026-08-29
**Based on:** `docs/DEBT-REGISTER.md` (post-C060 release classification, 2026-08-29 —
the `documented-limitation` rows below are this file's required content);
`docs/artifacts/mcp-gap-analysis-v1.md` (C030 gap analysis); `docs/decisions/ADR-013-mcp-protocol-path.md`
(C031, the MCP kernel decision); `docs/SCOPE-v1.0.md` §2.4 (quoting `docs/plans/plan-cwso-v1.0-roadmap.md`);
`docs/user/README.md`'s "Known limitations" section (C050)

## Purpose

This file states what CWSO v1.0 does **not** do. A limitation recorded here is a fact
about the current release, not an apology and not a roadmap promise — where a v1.1
remediation is planned, one pointer is given per entry; this file does not itself
describe future work in any detail beyond that pointer.

This file exists to close a loop opened by `docs/DEBT-REGISTER.md`'s C060 re-classification
pass (2026-08-29): three register rows (B1, R-1, R-6) were classified `documented-limitation`
— a real, disclosed gap that is allowed to stay open for v1.0 only because it is disclosed
here — specifically because this file did not yet exist. Section 1 below is that
disclosure; C060's cross-check depends on it.

---

## 1. Documented limitations (required disclosures, v1.0 release-gate)

The three entries in this section correspond 1:1 to `docs/DEBT-REGISTER.md`'s three
`documented-limitation` rows (B1, R-1, R-6, per the register's 2026-08-29 C060
classification pass). Each is a conscious, reviewed decision to ship the gap rather than
an oversight, and each was open specifically pending this file's existence.

### 1.1 MCP protocol: the hand-rolled kernel implements a documented subset of the spec

**What:** CWSO's MCP server (`orchestrator/internal/server/server.go`'s `Handle()`
dispatch, `orchestrator/internal/mcp/protocol.go`'s types/errors) is a hand-rolled
implementation of MCP spec **2025-03-26**, not the official `go-sdk`. Of the spec's 16
request methods (15 request methods + `notifications/initialized` counted for lifecycle
coverage), **6 are Missing**: `prompts/list`, `prompts/get`, `logging/setLevel`,
`completion/complete`, `sampling/createMessage`, `roots/list`. Of the spec's 9 defined
notifications, **8 are Missing**. Every implemented and partially-implemented method
returns spec-shaped behavior; every missing method returns a correct, spec-shaped
"not supported" error (`-32601 Method not found`) rather than silence or a malformed
response. Full row-by-row inventory, including the 6 Partial rows and every notification:
`docs/artifacts/mcp-gap-analysis-v1.md`.

**Why:** `docs/decisions/ADR-013-mcp-protocol-path.md` records the human-approved
decision (2026-08-13) to keep the hand-rolled kernel rather than adopt the official Go
SDK. The deciding factor was determinism: the kernel's dispatch is a single synchronous
`switch req.Method` (one request in, one response out), and no public SDK documentation
was found describing a way to preserve that property while using the SDK's own
transport/session machinery, which handles calls concurrently by design. Migrating would
also mean re-plumbing all 16 methods, 9 notifications, and 10 error-code constants, plus
building new server-initiated request/response correlation from scratch (needed for
`sampling/createMessage` and `roots/list`) on unfamiliar SDK primitives — a rewrite of the
request lifecycle, not a bounded port.

**Blast radius:** Any MCP client that specifically needs prompts, `logging/setLevel`
severity filtering, completions, server-initiated sampling requests, or client-provided
roots cannot use CWSO's MCP server for those capabilities in v1.0. Every other method
(`ping`, `initialize`, `tools/list`, `tools/call`, the `resources/*` family) works, with
documented deviations (e.g. `initialize` does not implement version negotiation; it
always returns its own supported version) called out in the gap analysis. No malformed
response is ever returned for a missing method — a client always gets a correct
JSON-RPC error it can act on.

**v1.1 pointer:** Implementing `sampling/createMessage` or `roots/list` requires new
transport-layer request/response correlation plumbing not built for v1.0 (ADR-013,
Ambiguity #4). Re-opening the SDK-adoption question at all is gated on ADR-013's own
"Reversal criteria" section (e.g. the SDK publishing a verified deterministic dispatch
mode, or the correlation plumbing becoming a hard product requirement).

---

### 1.2 Dev/Compose JWT signing secret is file-based

**What:** The default local Docker Compose deployment signs JWTs using a secret staged
from a plaintext file (`.env.jwt.dev`) into the `cwso-jwt-secret` named Docker volume via
the `jwt-secret-fix` service (`deploy/docker-compose.yml`).

**Why:** CWSO v1.0's supported deployment model is local-only (a single operator on
their own machine, loopback-first). A full external secret-management integration
(Vault/SOPS) was never started for v1.0 — tracked separately as the production half of
this same debt (`DEBT-REGISTER.md` row R-2).

**Blast radius:** Anyone with filesystem access to `.env.jwt.dev` or the
`cwso-jwt-secret` volume (i.e., anyone who already has local access to the host or its
containers) can mint arbitrarily-scoped JWTs for CWSO's MCP server. This is not remotely
exploitable on its own; it requires access that already implies a compromised host. See
§2.1 below for a related, lower-severity gap in how long a minted token can live.

**v1.1 pointer:** External secret management via Vault or SOPS (`DEBT-REGISTER.md` row
R-2, T029).

---

### 1.3 `git-shadow`'s shadow-workspace projection is not executable, with no compiler in the runtime image

**What:** `git-shadow`'s tmpfs-backed shadow-workspace projection mount is `noexec`, and
the sidecar's runtime container image ships with no compiler or language toolchain. It is
not possible to compile and run a test binary at the real, materialized workspace path
from inside the container.

**Why:** This is a direct, intended consequence of a deliberate hardening decision (C019):
`cap_drop: ["ALL"]`, `network_mode: "none"`, `read_only: true`, and a minimal base image
for the `git-shadow` runtime container. An independent Tech Lead review (of C024's MR)
explicitly recommended **deferring** any change here — loosening `noexec` or adding a
toolchain to the runtime image would trade away already-reviewed security posture for
marginal test-harness convenience, with no consumer anywhere in the roadmap.

**Blast radius:** Test-harness convenience only. CI (C024) works around the constraint,
not around its substance: it pre-compiles a static test binary (`CGO_ENABLED=0`) outside
the container and runs it from an exec-allowed volume, passing in the real materialized
workspace path so the test's own assertions still validate real content at the real
location — only the compiled binary's own inode location changes to satisfy `noexec`.
Production code paths are entirely unaffected; a running CWSO instance never needs to
compile or execute anything inside `git-shadow`'s projection mount.

**v1.1 pointer:** None. This is a permanent, reviewed constraint, not deferred work — the
Tech Lead review that assessed it explicitly recommended against opening a follow-up task.

---

## 2. Additional known gaps (tracked as v1.1 debt, disclosed for transparency)

`DEBT-REGISTER.md` classifies both entries below as `v1.1` — real debt, explicitly
deferred past v1.0, not required by the release gate — which is a different disposition
than the three `documented-limitation` entries in §1 above. They are surfaced here anyway,
at the coordinator's direction, because both are real, already-evidenced, low-severity
findings from C061's v1.0.0 security audit that a reader of this file should know about
even though the release gate does not require their disclosure here.

### 2.1 `scripts/cwso-token.sh`'s `--ttl` has no upper bound (F-C061-03, LOW)

**What:** `--ttl` is validated only as a positive integer number of seconds — there is no
maximum. A developer holding the JWT signing secret can mint a token valid for years.

**Blast radius:** Minting still requires prior possession of the signing secret file
itself (already `chmod 600`, gitignored, generated only via
`scripts/cwso-bootstrap-secrets.sh`), so this does not grant any new privilege — it only
widens the blast radius of an already-compromised secret file, or lets a long-lived local
token accidentally outlive its intended dev session.

**v1.1 pointer:** `DEBT-REGISTER.md` row R-11. Suggested remediation: cap `--ttl` at a
documented ceiling (e.g. 24h) with an explicit opt-out for legitimate long-running local
test scenarios.

### 2.2 No TLS-termination guidance for network-reachable deployments (F-C061-04, LOW)

**What:** The default v1.0 stack terminates plain HTTP directly on `:8080` — JWT bearer
tokens and the dashboard token traverse the wire in cleartext.
`docs/user/deployment/proxmox-lxc-guide.md` documents deploying CWSO inside a
network-reachable LXC container/VM and suggests adding a reverse proxy (HAProxy/Nginx)
for multi-instance load-balancing, but never mentions TLS termination as part of that
recommendation.

**Blast radius:** Low as currently documented — the guide's primary path is a
single-operator homelab deployment over loopback, and the GCP Cloud Run deployment path is
inherently mitigated (Cloud Run always terminates TLS at the platform edge). The gap
widens for any reader who follows the Proxmox guide's reverse-proxy suggestion toward a
genuinely network-reachable, multi-instance setup without separately adding TLS.

**v1.1 pointer:** `DEBT-REGISTER.md` row R-12. Suggested remediation: add an explicit
TLS-termination warning/recommendation to the Proxmox guide's reverse-proxy section.

---

## 3. Cross-referenced elsewhere (not duplicated here)

- **Firecracker microVM sandbox tier ships as a documented fallback, not promoted.**
  Already documented in `docs/user/README.md`'s "Known limitations" section (C050): without
  `/dev/kvm` and `/dev/vhost-net` on the host, sandboxed execution degrades to a supported
  gVisor-only fallback, not a broken state. See that section rather than this file for the
  operator-facing detail; `docs/SCOPE-v1.0.md` §2.4 (quoted in §4 below) records the same
  fact at the scope-decision level.
- **UDS socket permissions and cross-service GID alignment are not a limitation.**
  `DEBT-REGISTER.md` row B12 is classified `fixed`, re-verified live against a running
  compose stack (C044, 2026-08-27): both `git-shadow` and `merge-engine` sockets are bound
  `0o660` with GIDs correctly aligned to the orchestrator image's live GID. This is
  deliberately not listed as a limitation here, since doing so would contradict a `fixed`
  register row.
- **IPC-only shadow workspaces (no real-filesystem projection).** This would only apply if
  ADR-012 had reached a NO-GO and the conditional task C025 had activated. It did not:
  ADR-012 chose the "materialise-to-tmpfs" path (a GO), and `DEBT-REGISTER.md` row B2 is
  `fixed` on that basis — every shadow workspace is a real, tmpfs-backed directory reachable
  by ordinary tooling. `docs/tasks/active-tasks.md` still lists C025 as `pending`
  (dependency: "C020 (NO-GO)"), meaning its trigger condition never occurred; there is
  nothing to disclose here.

---

## 4. Not in v1.0 (deferred features)

Quoted verbatim from `docs/SCOPE-v1.0.md` §"Explicitly not in v1.0" (itself a verbatim
quote of `docs/plans/plan-cwso-v1.0-roadmap.md` §2.4). Each of these is real, working code;
none is required by v1.0's own definition of done (`docs/SCOPE-v1.0.md` §"What 'v1.0'
should mean"). This section states what is deferred, not when or how it will land — see
`docs/SCOPE-v1.0.md` for the re-entry column and change-control rule.

| Deferred | Status |
|---|---|
| HAL / hardware-aware dispatch | built (`services/cwso-hal`), off by default |
| Sparse micro-agents | built (`services/cwso-sparse`) |
| Rollout / Polar trajectory capture | built (`services/cwso-rollout`), opt-in profile only |
| SWE-bench evaluator (`DEBT-REGISTER.md` row B11) | stub — registry hook exists, harness launch deferred |
| Terminal-Bench evaluator | not started |
| Firecracker microVM tier | implemented with a documented gVisor-only fallback; not promoted (see §3 above) |
| Kubernetes operator, CRDs, autoscaling | not started |
| Merkle incremental AST indexer (`DEBT-REGISTER.md` row P2-2) | not started — every AST query re-parses the file; fine at v1.0 scale (<1k LOC), will not scale |
| Vault/SOPS secret management (`DEBT-REGISTER.md` row R-2) | not started (see §1.2 above) |

---

## See also

- `docs/DEBT-REGISTER.md` — the full, live debt inventory this file's §1 and §2 entries
  are drawn from, including every `fixed` row.
- `docs/user/README.md`'s "Known limitations" section — operator-facing gaps encountered
  during day-to-day use.
- `docs/SCOPE-v1.0.md` — the authoritative statement of what "v1.0" means and what it
  explicitly excludes.
