# Security Audit Report: CWSO v1.0.0 Release Gate (closes T010)

Date: 2026-08-28
Owner: security-engineer
Task: C061
Based on: `docs/tasks/task-C061.md`, `SECURITY.md`, `docs/artifacts/security-baseline-v2.md`,
`docs/plans/feature-operator-dashboard.md` (T010's original scope reference),
`docs/decisions/ADR-011-operator-dashboard.md`, `docs/DEBT-REGISTER.md`

## Summary

- **Risk Level**: Medium
- **Findings**: 0 Critical, 0 High, 2 Medium, 2 Low
- **Status**: **PASS** (no unresolved CRITICAL/HIGH findings; MEDIUM findings below carry
  remediation plans per security-guidelines.md)

This is a fresh, full-v1.0-surface OWASP audit — not a resumption of T010's original
narrow scope (dashboard auth / no-secret-leakage). T010 was opened 2026-08-06 as a
security gate for the operator-dashboard feature and was **never run**: the feature
shipped without it (confirmed by the orchestrator's 2026-08-28 investigation, logged in
`docs/tasks/active-tasks.md`). This audit supersedes T010 by covering the entire v1.0
default-path surface, including the dashboard feature T010 was originally scoped to gate.

## Scope

In scope (v1.0 default path only, per the task brief's rails):
- JWT auth: secret bootstrap (`scripts/cwso-bootstrap-secrets.sh`, C012), token minting
  (`scripts/cwso-token.sh`, C013), verification (`orchestrator/internal/transport/http.go`)
- Secret handling across the repo (code, logs, git history)
- The read-write workspace mount (C015) and its path-traversal defenses
  (`orchestrator/internal/tools/fs_tools.go`)
- Sidecar IPC socket permissions and uid/gid allowlist (C044 outcome) — re-verified live,
  not taken on trust from `completed-tasks.md`
- Container hardening posture across `deploy/docker-compose.yml`
- MCP boundary input validation (`orchestrator/internal/server/`, `orchestrator/internal/tools/`)
- The operator dashboard (`orchestrator/internal/dashboard/`) — T010's original target
- CI security tooling posture against `security-baseline-v2.md` §5's required-checks list

Explicitly out of scope (per the brief): v1.1-deferred surface (HAL inference-endpoint
internals beyond their compose reachability, sparse/quantized/SSM assist internals,
rollout/Polar trajectory pipeline internals beyond its compose reachability).

## Methodology

1. Static review of the full v1.0 surface listed above, tracing every trust boundary
   (JWT auth middleware → tool registry role gate → tool execution; HTTP origin/rate-limit
   middleware chain; sidecar UDS peer-credential authorization).
2. Live re-verification against a real running stack: bootstrapped a fresh JWT secret
   (`scripts/cwso-bootstrap-secrets.sh`), brought up `deploy/docker-compose.yml` with
   `docker compose up -d` (see Blocker note below on the `--build` path), and exercised
   the running containers directly — not source inspection alone. See transcripts below.
3. Manual secret-leakage sweep: `grep -rni` for `password|secret|token` across
   `orchestrator/` and `services/` (excluding tests and known-safe JWT config identifiers),
   plus a full `git log --all --diff-filter=A` sweep for ever-added `.env`/`.pem`/`.key`/
   `id_rsa`-pattern files.
4. OWASP Top 10 checklist run against the in-scope surface (table below).
5. Cross-checked every claim in `SECURITY.md`'s "Sidecar IPC authorization" section and
   `ADR-011-operator-dashboard.md`'s "Security trade-offs" section against the actual
   current source and live container behavior, rather than taking either document's
   claims on trust — this is the standing discipline this roadmap holds itself to
   (SEV-C041-001, SEC-C044-001 were both found this way).

## OWASP Top 10 checklist

| Category | Reviewed | Findings | Notes |
|---|---|---|---|
| A01 Broken Access Control | Yes | 0 | Tool registry (`orchestrator/internal/tools/registry.go`) enforces `AllowedRoles()` per tool on every `tools/call`; role is sourced only from a JWT-verified session for HTTP transport (never client-supplied). Path-traversal defenses in `fs_tools.go` reviewed in depth — see below. |
| A02 Cryptographic Failures | Yes | 1 (LOW) | JWT HS256 correctly implemented, secret sourced from a mounted file (preferred) or env var, fails closed if unset. Dashboard token SHA-256 hashed + constant-time compared. Gap: no operator guidance to add TLS termination for non-loopback self-hosted deployments — see F-C061-04. |
| A03 Injection | Yes | 0 | All subprocess execution (`os/exec`) uses argv arrays, never a shell string (`sandbox/runner_firecracker.go`, `harness/local_runtime.go`). All MCP tool inputs are typed/schema-validated (enums, required fields, `additionalProperties: false` where appropriate). |
| A04 Insecure Design | Yes | 1 (MEDIUM) | Dashboard auth endpoints have no rate limiting — see F-C061-01. Dispatch tools enforce `maxBatch`/`maxTimeoutSeconds`/`maxFieldLength` bounds against resource exhaustion. |
| A05 Security Misconfiguration | Yes | 0 | Live-verified: `read_only: true`, `cap_drop: [ALL]`, `no-new-privileges:true`, non-root `USER cwso` across all default-stack services; `CWSO_DOCKER_NETWORK=host` is explicitly rejected at config-validation time; sidecars run `network_mode: none`. |
| A06 Vulnerable Components | Yes | 1 (MEDIUM) | `govulncheck` and `cargo audit` run as blocking CI gates. `gosec`, `cargo-deny`, and a Trivy image scan — all listed as required in `security-baseline-v2.md` §5 — are absent from CI. See F-C061-02. |
| A07 Auth Failures | Yes | 1 (MEDIUM, shared with F-C061-01) | `/mcp` JWT auth verified live (missing token → 401, bad origin → 403, valid token → 200). Dashboard token brute-force is unthrottled — same finding as A04 above. |
| A08 Data Integrity Failures | Yes | 0 | JSON unmarshaling throughout uses typed structs, not open-ended `interface{}` deserialization, on all security-relevant paths. No package/artifact signing gap beyond what `security-baseline-v2.md` itself scopes as "encouraged," not required. |
| A09 Logging & Monitoring Failures | Yes | 1 (LOW, folded into F-C061-01) | `/mcp` auth failures are logged (`log.Warn().Err(err).Msg("jwt rejected")`); dashboard auth failures are only counted via an atomic counter (`ClientMetrics.RecordAuthFailure`), never logged — an operator has no log-based signal that a brute-force attempt against the dashboard token is in progress. |
| A10 SSRF | Yes | 0 | Out-of-scope services (HAL, rollout) already implement allow-list/loopback validation per `SECURITY.md`; in-scope v1.0 default path makes no user-controlled outbound HTTP calls. |

## Findings

### F-C061-01: Dashboard auth endpoints have no rate limiting (live-verified)
- **Severity**: SECURITY:MEDIUM
- **Category**: OWASP A04 (Insecure Design — rate limiting on auth endpoints) / A07 (Auth
  Failures) / A09 (Logging gap, folded in as a contributing factor)
- **Location**: `orchestrator/internal/transport/http.go:757-790` (`rateLimitMiddleware`),
  `orchestrator/internal/transport/http.go:230-233` (dashboard route wiring),
  `orchestrator/internal/dashboard/dashboard.go:259-276` (`Handler.auth`)
- **Description**: `rateLimitMiddleware` only throttles `POST` requests
  (`if r.Method != http.MethodPost { next.ServeHTTP(w, r); return }`) — this exemption was
  originally scoped to keep long-lived `GET /mcp` SSE streams from being rate-limited
  (comment: `// Only rate-limit /mcp POST (not GET SSE)`, tracked as debt item D6). The
  dashboard handler (`ADR-011`, added later) is mounted through the same shared `mw` chain
  (`mux.Handle("/dashboard", mw(o.dashboard))`, `mux.Handle("/dashboard/status", mw(o.dashboard))`)
  and both dashboard routes are accessed via `GET` per their own documented API contract —
  so they silently inherited the SSE-specific exemption. `ADR-011`'s own "Security
  trade-offs" section explicitly claims "Dashboard routes inherit `rateLimitMiddleware`,
  bounding the request rate from any single IP" — this claim does not hold for the actual
  GET-based access pattern the same ADR documents.
- **Live verification**: with the stack running (`docker compose -f deploy/docker-compose.yml
  up -d`, `CWSO_DASHBOARD_TOKEN=live-verify-weak-token`), 150 consecutive
  `GET /dashboard/status` requests with wrong bearer tokens all returned `401` — zero `429`
  responses at any point. Under the same running instance, `POST /mcp` with no token hit
  `429 Too Many Requests` starting at request #11, matching the documented `burst=10`
  token-bucket config exactly. Transcript:
  ```
  # 150x GET /dashboard/status with wrong tokens:
  401 401 401 ... (150 times, no 429)

  # contrast: POST /mcp, same limiter, same process:
  401 401 401 401 401 401 401 401 401 401 429 429 429 429 429
  ```
  A subsequent `GET /dashboard/status` with the correct token returned `200` with a JSON
  body confirmed (both by source review of `ConfigSnapshot`'s allowlist-only field set and
  by inspecting the live response) to leak no secrets — only redacted socket basenames,
  boolean feature flags, and counters, consistent with `ADR-011`'s intended design and
  T010's original "no secret leakage in JSON" gate.
- **Compounding factors**:
  1. `CWSO_DASHBOARD_TOKEN` is a static, non-expiring, operator-supplied secret (unlike the
     JWT, which is short-lived and signed). Nothing in `dashboard.go`'s `New()` enforces a
     minimum length or entropy.
  2. The only documented examples of this token, across `docs/artifacts/release-v0.6.0.md`
     and `release-v0.6.1.md`, are literal low-entropy strings (`my-operator-token`,
     `<your-operator-token>`) — unlike `CWSO_JWT_SECRET`, which is always generated via
     `openssl rand -hex 32` by `scripts/cwso-bootstrap-secrets.sh` and never hand-typed.
  3. Failed dashboard-auth attempts are counted (`ClientMetrics.RecordAuthFailure`) but
     never logged — there is no structured-log signal an operator could alert on while a
     brute-force run is in progress (contrast with `/mcp`'s `authMiddleware`, which does
     `log.Warn().Err(err).Msg("jwt rejected")` on every failure).
- **Impact**: An attacker with network reachability to the published `8080` port (default
  in `deploy/docker-compose.yml`) can send unlimited, unthrottled, unlogged
  `Authorization: Bearer <guess>` attempts against `/dashboard/status`. Impact is bounded
  to information disclosure of non-secret operational metadata (job counts, feature flags,
  redacted socket names, auth-failure/rate-limit counters) — the dashboard token grants no
  MCP tool-call privilege and the JSON response is allowlist-serialized. Exploitability
  requires the operator to have both (a) explicitly opted into the dashboard (off by
  default: unset token → `501`) and (b) chosen a low-entropy token, which the project's own
  documentation examples make more likely than for `CWSO_JWT_SECRET`.
- **Remediation**:
  1. Extend rate limiting to cover dashboard GET routes — either widen
     `rateLimitMiddleware` to apply per-route rather than per-method (e.g. rate-limit any
     non-SSE request), or add a dedicated limiter inside `dashboard.Handler.auth()`.
  2. Enforce a minimum `CWSO_DASHBOARD_TOKEN` length (e.g. reject <32 chars) at startup in
     `dashboard.New()`, mirroring the entropy bar already implied for `CWSO_JWT_SECRET`.
  3. Add a `log.Warn(...)` call on dashboard auth failure (with source IP and request ID,
     no token value) so this becomes visible to log-based alerting, not just an
     after-the-fact counter an attacker-blocked operator cannot even see without the token.
  4. Update `docs/artifacts/release-v0.6.*.md` and any future dashboard docs to show a
     `CWSO_DASHBOARD_TOKEN=$(openssl rand -hex 32)`-style example instead of a literal
     low-entropy placeholder string.
- **Code Example**:
  ```go
  // Before (vulnerable) — orchestrator/internal/transport/http.go
  func rateLimitMiddleware(store *rateLimiterStore, log *logging.Logger, m RequestMetrics) middleware {
      return func(next http.Handler) http.Handler {
          return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              // Only rate-limit /mcp POST (not GET SSE)
              if r.Method != http.MethodPost {
                  next.ServeHTTP(w, r)
                  return
              }
              // ... token-bucket check ...
          })
      }
  }

  // After (illustrative fix) — scope the SSE exemption to /mcp specifically,
  // not to every GET route sharing the same middleware chain:
  if r.Method != http.MethodPost && r.URL.Path == "/mcp" {
      next.ServeHTTP(w, r) // SSE stream — exempt by path, not by method alone
      return
  }
  // dashboard GET requests now fall through to the limiter below
  ```

### F-C061-02: Required CI security-scanning tooling is not implemented
- **Severity**: SECURITY:MEDIUM
- **Category**: OWASP A06 (Vulnerable Components) / secret-scanning hygiene (A09-adjacent)
- **Location**: `.gitlab-ci.yml` (`audit` stage, lines 162-215); `docs/artifacts/security-baseline-v2.md` §5 ("Required CI checks")
- **Description**: `security-baseline-v2.md` §5 lists six required CI security checks for
  v0.4.1 GA and beyond: `gosec`, `govulncheck`, `cargo audit`, `cargo deny`, a Trivy
  container image scan, and `gitleaks`/`trufflehog` secret scanning. Verified against the
  actual `.gitlab-ci.yml` `audit` stage: only `govulncheck` (`go:audit`) and `cargo audit`
  (`rust:audit`) are implemented, both correctly wired as blocking gates on MR/develop/main
  pipelines. `gosec`, `cargo deny`, a Trivy scan, and `gitleaks`/`trufflehog` do not appear
  anywhere in `.gitlab-ci.yml`. There is also no `.pre-commit-config.yaml` in the repo, so
  the baseline's Deployment Checklist item "Pre-commit secret scan hooks installed locally"
  is unmet by default. (Note: `.github/` in this repo is the emage.code platform-projection
  skill/agent framework per `AGENTS.md`, not real GitHub Actions workflows — `.gitlab-ci.yml`
  is confirmed to be the only live CI pipeline.)
- **Impact**: This is a control-gap finding, not an active-leak finding — a manual
  secret-leakage sweep for this audit (grep across `orchestrator/`+`services/` for
  `password|secret|token`, and a full `git log --all --diff-filter=A` sweep for ever-added
  `.env*`/`.pem`/`.key`/`id_rsa`-pattern files) found zero live secrets in the repository.
  However, without `gosec` (Go SAST) and a Trivy image scan gating CI, future Go
  vulnerability classes outside `govulncheck`'s known-CVE database (e.g. hardcoded
  credentials, weak crypto usage, SSRF-shaped code patterns) and base-image CVEs in the
  four `Dockerfile.*` images will not be caught automatically before merge — matching
  exactly how `SEC-C044-001`/`SEC-C044-002` were each found by an independent human
  security review rather than an automated gate, which is a real but slower detection path.
- **Remediation**: Add `gosec`, `cargo-deny`, a `trivy image` scan (post-build, in the
  `build` or a new post-build stage against each of the four built images), and
  `gitleaks`/`trufflehog` as blocking jobs in the `audit` stage of `.gitlab-ci.yml`,
  following the exact pattern already established by `go:audit`/`rust:audit`/
  `security:ipc-gid-drift` (same `rules:` gating MR/develop/main). Route to the debt
  register (C060) for tracking and prioritization, since none of the four missing tools
  represent an active exploit today.

### F-C061-03: `cwso-token.sh --ttl` has no upper bound
- **Severity**: SECURITY:LOW
- **Category**: OWASP A07 (Auth Failures) — informational / tracked debt
- **Location**: `scripts/cwso-token.sh` (`--ttl` argument parsing)
- **Description**: `--ttl` is validated only as "a positive integer number of seconds" —
  there is no maximum. A developer holding `.env.jwt.dev` can mint an `orchestrator`-role
  JWT valid for years. Minting still requires prior possession of the signing secret file
  (itself `chmod 600`, gitignored, generated only via `cwso-bootstrap-secrets.sh`), so this
  does not grant any new privilege — it only widens the blast radius of an
  already-compromised secret file or an accidentally-retained long-lived local token
  outliving its intended dev session.
- **Impact**: Low — local dev tooling only, gated behind an existing secret.
- **Remediation**: Cap `--ttl` at a documented ceiling (e.g. 24h) with an explicit
  `--ttl-unsafe-long` opt-out for legitimate long-running local test scenarios, or simply
  document the risk in the script's own usage text. Route to the debt register (C060) as
  LOW/non-blocking.

### F-C061-04: No operator guidance to add TLS for self-hosted, non-loopback deployments
- **Severity**: SECURITY:LOW
- **Category**: OWASP A02 (Cryptographic Failures)
- **Location**: `docs/user/deployment/proxmox-lxc-guide.md`; `deploy/docker-compose.yml`
  (`orchestrator` service, `ports: ["8080:8080"]`, no TLS termination)
- **Description**: The v1.0 default stack terminates plain HTTP directly on `:8080`; JWT
  bearer tokens and the dashboard token traverse the wire in cleartext. This is a
  reasonable default for the documented local/loopback use case (and is inherently
  mitigated for the `gcp-cloud-run-guide.md` path, since Cloud Run always terminates TLS at
  the platform edge). The `proxmox-lxc-guide.md`, however, documents deploying CWSO inside
  a network-reachable LXC container/VM and even suggests adding "HAProxy or Nginx as
  reverse proxy" for load-balancing multi-instance setups (line 483) — but never mentions
  TLS termination as part of that recommendation, and there is no warning anywhere in that
  guide that a non-loopback deployment transmits both credential types in cleartext.
- **Impact**: Low as currently documented (the guide's primary path is still a
  single-operator homelab deployment), but the gap widens for any reader who follows the
  Proxmox guide's own "expose beyond loopback" and "multi-container + reverse proxy"
  sections without independently knowing to add TLS.
- **Remediation**: Add an explicit TLS/reverse-proxy note to `proxmox-lxc-guide.md`
  (and any future non-GCP, non-loopback deployment guide) — mirroring the loopback-vs.-
  non-loopback distinction `SECURITY.md` already documents for the HAL accelerator
  endpoint's `CWSO_HAL_ALLOW_INSECURE_ENDPOINTS` behavior.

## Live verification transcripts

### JWT auth + Origin validation (orchestrator/internal/transport/http.go)
```
$ curl -sS -o /dev/null -w "healthz=%{http_code}\n" http://127.0.0.1:8080/healthz
healthz=200
$ curl -sS -o /dev/null -w "mcp-no-auth=%{http_code}\n" -X POST http://127.0.0.1:8080/mcp \
    -H 'Content-Type: application/json' -H 'Origin: http://localhost' \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
mcp-no-auth=401
$ curl -sS -o /dev/null -w "mcp-bad-origin=%{http_code}\n" -X POST http://127.0.0.1:8080/mcp \
    -H 'Content-Type: application/json' -H 'Origin: http://evil.example' \
    -H 'Authorization: Bearer garbage' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
mcp-bad-origin=403
$ TOKEN=$(scripts/cwso-token.sh --role orchestrator --ttl 300)
$ curl -sS -o /dev/null -w "mcp-valid-token=%{http_code}\n" -X POST http://127.0.0.1:8080/mcp \
    -H 'Content-Type: application/json' -H 'Origin: http://localhost' \
    -H "Authorization: Bearer $TOKEN" -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
mcp-valid-token=200
```

### Sidecar IPC socket permissions + uid/gid allowlist (C044 outcome, re-verified live)
```
$ docker exec cwso-git-shadow stat -c '%a' /run/cwso/git-shadow.sock
660
$ docker exec cwso-merge-engine stat -c '%a' /run/cwso/merge-engine.sock
660
$ bash scripts/check-ipc-gid-drift.sh
Live orchestrator 'cwso' identity: uid=100 gid=101
OK: git-shadow CWSO_IPC_ALLOWED_UIDS="0,100" contains live uid=100
OK: git-shadow CWSO_IPC_ALLOWED_GIDS="0,101" contains live gid=101
OK: merge-engine CWSO_IPC_ALLOWED_UIDS="0,100" contains live uid=100
OK: merge-engine CWSO_IPC_ALLOWED_GIDS="0,101" contains live gid=101
```

### Container hardening posture (deploy/docker-compose.yml, live docker inspect)
```
$ docker inspect cwso-orchestrator --format \
    'ReadOnly={{.HostConfig.ReadonlyRootfs}} CapDrop={{.HostConfig.CapDrop}} SecurityOpt={{.HostConfig.SecurityOpt}} User={{.Config.User}} NetworkMode={{.HostConfig.NetworkMode}}'
ReadOnly=true CapDrop=[ALL] SecurityOpt=[no-new-privileges:true] User=cwso NetworkMode=cwso_default
$ docker inspect cwso-git-shadow --format \
    'ReadOnly={{.HostConfig.ReadonlyRootfs}} CapDrop={{.HostConfig.CapDrop}} SecurityOpt={{.HostConfig.SecurityOpt}} NetworkMode={{.HostConfig.NetworkMode}}'
ReadOnly=true CapDrop=[ALL] SecurityOpt=[no-new-privileges:true] NetworkMode=none
$ docker inspect cwso-merge-engine --format \
    'ReadOnly={{.HostConfig.ReadonlyRootfs}} CapDrop={{.HostConfig.CapDrop}} SecurityOpt={{.HostConfig.SecurityOpt}} NetworkMode={{.HostConfig.NetworkMode}}'
ReadOnly=true CapDrop=[ALL] SecurityOpt=[no-new-privileges:true] NetworkMode=none
```

### `rollout`'s SEC-C044-001 fix (unused-mount removal), re-verified live
```
$ docker compose -f deploy/docker-compose.yml --profile rollout up -d rollout
$ docker exec cwso-rollout sh -c 'ls -la /run/cwso; test -S /run/cwso/git-shadow.sock && echo REACHABLE || echo "NOT REACHABLE"'
total 8
drwxr-xr-x 2 cwso cwso 4096 ... .
drwxr-xr-x 1 root root 4096 ... ..
NOT REACHABLE
```
Confirms `rollout` has no filesystem path to either sidecar socket — the fix documented in
`deploy/docker-compose.yml` and `SECURITY.md` point 8 holds under live inspection.

### F-C061-01's rate-limit gap, live proof
```
$ for i in $(seq 1 150); do curl -sS -o /dev/null -w "%{http_code} " \
    http://127.0.0.1:8080/dashboard/status -H "Authorization: Bearer guess-$i"; done
401 401 401 ... [150 times total, zero 429s]

$ curl -sS -o /dev/null -w "dashboard-correct-token=%{http_code}\n" \
    http://127.0.0.1:8080/dashboard/status -H "Authorization: Bearer live-verify-weak-token"
dashboard-correct-token=200

$ for i in $(seq 1 15); do curl -sS -o /dev/null -w "%{http_code} " -X POST http://127.0.0.1:8080/mcp \
    -H 'Content-Type: application/json' -H 'Origin: http://localhost' \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'; done
401 401 401 401 401 401 401 401 401 401 429 429 429 429 429
```

### Dashboard response confirmed secret-free (allowlist serialization holds live)
```
$ curl -sS http://127.0.0.1:8080/dashboard/status -H "Authorization: Bearer live-verify-weak-token"
{"version":"0.1.0","timestamp":"...","overall":"degraded","system":{"uptime_seconds":290},
 "sidecars":{"git_shadow":{"connected":true,"socket":"git-shadow.sock"}, ...},
 "config":{"transport":"http","sandbox_runner":"none","feature_flags":{...},"warnings":null},
 "jobs":{"workers":4,"queue_capacity":64,"queue_depth":0,"active":0,"total_completed":0,"total_failed":0},
 "clients":{"total_requests":162,"auth_failures":160,"rate_limit_hits":5,"tool_calls":{}},
 "rollout":{"enabled":true,"last_reward_signal":null}}
```
No JWT secret, dashboard token hash, or sandbox credential present — only basenames of
socket paths, boolean feature flags, and counters, satisfying T010's original "no secret
leakage in JSON" gate.

Full stack was torn down cleanly after verification (`docker compose down -v`); no code,
config, or committed files were modified during this audit (worktree confirmed clean via
`git status --short` both before and after live testing); the locally-generated
`.env.jwt.dev` secret file used for the live run was deleted afterward.

## Blocker encountered (resolved, informational — not blocking this audit)

**Type**: external | **Severity**: minor | **Retry attempts**: 1

`docker compose -f deploy/docker-compose.yml up -d --build` failed in this sandbox: the
Docker daemon's credential helper could not resolve/pull `docker/dockerfile:1.6` and
`debian:bookworm-slim` from the registry (`error getting credentials`,
`UtilAcceptVsock:273: accept4 failed 110` — a WSL/vsock-level registry-connectivity issue
in this specific sandbox, unrelated to CWSO's own code or Dockerfiles). **Mitigation
applied**: cached images (`cwso/orchestrator:dev`, `cwso/git-shadow:dev`,
`cwso/merge-engine:dev`, `cwso/rollout:dev`, all built 2026-08-28 from this same `develop`
tip by a prior agent session in this environment) were already present in the shared
Docker image store; `docker compose up -d` (without `--build`) succeeded using those images
and produced a fully live stack, against which every live-verification transcript above
was captured. No retry against a second network path was needed once this was found. Not
escalated further since it did not block completion of any acceptance criterion.

## Recommendations (priority order)

1. Fix F-C061-01 (dashboard rate limiting + token entropy + auth-failure logging) before
   any deployment that sets `CWSO_DASHBOARD_TOKEN` in a network-reachable environment —
   MEDIUM, not release-blocking for the default (dashboard-disabled) profile, but should
   land promptly.
2. Fix F-C061-02 (add `gosec`, `cargo-deny`, Trivy, `gitleaks`/`trufflehog` to CI) — MEDIUM,
   close the gap between the documented security baseline and actual CI enforcement.
3. Track F-C061-03 and F-C061-04 in the debt register (C060) as LOW/non-blocking.
4. No action required on R-1 (file-based JWT secret) beyond what C063 already owns
   (documenting it as an accepted v1.0-local-only limitation) — confirmed still accurate
   by this audit's live verification.

## Compliance Checklist

- [x] OWASP Top 10 reviewed
- [x] Authentication secure (JWT HS256, fail-closed, live-verified end-to-end)
- [x] Authorization enforced (role-gated tool registry, live and static verified)
- [x] Input validation complete (schema-validated tool args, bounded fields, TOCTOU-safe path guard)
- [x] Secrets management verified (no secrets in source/history; JWT secret precedence and file permissions confirmed live)
- [x] Dependencies audited (govulncheck + cargo audit run as blocking CI gates; gaps noted in F-C061-02)
- [x] Security headers configured (CSP, HSTS, X-Content-Type-Options, X-Frame-Options, X-XSS-Protection — verified in `securityHeadersMiddleware`)
- [x] Logging appropriate (gap noted in F-C061-01 for the dashboard auth path specifically)

## VERDICT: PASS

### Findings Summary
| Severity | Count | Resolved | Remaining |
|----------|-------|----------|-----------|
| CRITICAL | 0 | 0 | 0 |
| HIGH | 0 | 0 | 0 |
| MEDIUM | 2 | 0 | 2 (F-C061-01, F-C061-02 — remediation plans above) |
| LOW | 2 | 0 | 2 (F-C061-03, F-C061-04 — route to debt register) |

### Justification
No CRITICAL or HIGH finding was identified across the full v1.0 default-path surface,
verified both statically and live against a running stack (JWT auth, origin validation,
sidecar IPC authorization, container hardening, path-traversal defenses, dashboard
information-disclosure boundary all held under direct testing — see transcripts above).
The two MEDIUM findings (F-C061-01, F-C061-02) are real, live-verified gaps but are each
bounded in impact (information disclosure only, requiring an operator opt-in and a weak
token for F-C061-01; a detection-latency gap rather than an active leak for F-C061-02) and
each carry a concrete remediation plan per security-guidelines.md's requirement for
MEDIUM-severity findings. Per the security-guidelines.md gate rule, CRITICAL/HIGH block —
none were found, so the release gate is **PASS**, not conditional.

### OWASP Coverage
| Category | Reviewed | Findings |
|----------|----------|----------|
| A01: Broken Access Control | Yes | 0 |
| A02: Cryptographic Failures | Yes | 1 (LOW — F-C061-04) |
| A03: Injection | Yes | 0 |
| A04: Insecure Design | Yes | 1 (MEDIUM — F-C061-01) |
| A05: Security Misconfiguration | Yes | 0 |
| A06: Vulnerable Components | Yes | 1 (MEDIUM — F-C061-02) |
| A07: Auth Failures | Yes | 1 (MEDIUM — F-C061-01, shared) + 1 (LOW — F-C061-03) |
| A08: Data Integrity Failures | Yes | 0 |
| A09: Logging & Monitoring | Yes | 0 (folded into F-C061-01 as contributing factor, not counted separately) |
| A10: SSRF | Yes | 0 |
