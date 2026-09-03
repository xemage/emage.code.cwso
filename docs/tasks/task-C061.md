# Task C061 — Security pass: close T010

**ID:** C061
**Owner:** security-engineer
**Status:** pending
**Priority:** P0
**Depends on:** C050–C054 (gate CG4). May start earlier at orchestrator discretion — it needs no Phase-5 output.
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C061 row); docs/plans/plan-cwso-v1.0-phase6-release-v1.md; T010 (open since 2026-08-06)

## Objective

Complete the security audit that T010 opened on 2026-08-06 (auth, secret leakage) and
close it. Run the OWASP Top 10 checklist against the v1.0 surface. Per
security-guidelines.md: CRITICAL and HIGH findings block the release; MEDIUM needs a
remediation plan; LOW becomes tracked debt.

## Inputs

- T010's original scope (no `docs/tasks/task-T010.md` brief exists — it predates this
  project's brief-file convention; reconstruct scope from
  `docs/plans/feature-operator-dashboard.md` — "T010 | SE: Security audit (auth on
  dashboard, no secret leakage in JSON)" — and `docs/decisions/ADR-011-operator-dashboard.md`,
  which names the specific dashboard security gate T010 was meant to enforce)
- `SECURITY.md`, `docs/artifacts/security-baseline-v2.md`
- The full v1.0 surface: `orchestrator/`, `services/`, `deploy/`, `scripts/` (including the new C012/C013/C016/C017 scripts)
- OWASP Top 10 checklist (security-guidelines.md)

## Rails (read before starting)

### You MUST
- Audit: JWT auth (secret bootstrap C012, token script C013), secret handling (no secrets in code/logs/git), the new read-write workspace mount (C015), socket permissions (C044 outcome), container hardening posture, input validation at the MCP boundary
- Classify every finding `SECURITY:CRITICAL` / `HIGH` / `MEDIUM` / `LOW` per the workflow
- Produce `docs/artifacts/security-v1.0-audit-v1.md`: findings table + verdict
- Close T010 per the task-management archive procedure (orchestrator executes the ledger move; you supply the verdict)
- You are read-only for code: flag, annotate, create findings — do not fix

### You MUST NOT
- Modify any code — Security Engineer is read-only during audit
- Downgrade a finding to make the release date — CRITICAL/HIGH block; that is the rule, not a suggestion
- Audit v1.1-deferred surface (HAL, sparse, rollout internals beyond their compose reachability) — scope is the v1.0 default path
- Paste any discovered secret value into the findings report (describe location + type only)

## File ownership

- **May create/modify:** `docs/artifacts/security-v1.0-audit-v1.md` (new)
- **Must NOT touch:** all code, `SECURITY.md` (findings may *recommend* changes)

## Steps (execute in order)

1. Read T010's scope and the security baseline.
2. Run the OWASP checklist over the v1.0 surface.
3. Classify and write findings.
4. Deliver the verdict; hand T010 closure to the orchestrator.

## Expected outputs

- `docs/artifacts/security-v1.0-audit-v1.md`
- T010 closure verdict

## Acceptance criteria

1. OWASP checklist run over the v1.0 surface, findings classified
2. No unresolved CRITICAL/HIGH (or the release is correctly blocked)
3. T010 closed with the audit artifact as its outcome

## Verification commands

```bash
grep -c "SECURITY:CRITICAL\|SECURITY:HIGH\|SECURITY:MEDIUM\|SECURITY:LOW" docs/artifacts/security-v1.0-audit-v1.md
grep -rn "password\|secret\|token" orchestrator/ services/ --include="*.go" --include="*.rs" -i | grep -v "_test" | grep -iv "jwt_secret\|CWSO_JWT" | head   # leakage sweep
```

## Git rails

- Branch: `agent/security-engineer/C061` from `develop`
- Commit: `docs(security): v1.0.0 security audit closing T010`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
A CRITICAL/HIGH finding is not a blocker on *you* — it is a release blocker. Report it
plainly; do not soften it.

## Execution notes

**Executed:** 2026-08-28 · security-engineer · branch `agent/security-engineer/C061`

### Methodology

1. Read this brief in full, plus `SECURITY.md`, `docs/artifacts/security-baseline-v2.md`,
   `docs/plans/feature-operator-dashboard.md` and `docs/decisions/ADR-011-operator-dashboard.md`
   (T010's original scope reference), and `docs/DEBT-REGISTER.md` before touching any code.
2. Static review of the full v1.0 surface: JWT auth chain (`scripts/cwso-bootstrap-secrets.sh`,
   `scripts/cwso-token.sh`, `orchestrator/internal/transport/http.go`), config fail-closed
   validation (`orchestrator/internal/config/config.go`), the C015 read-write workspace mount's
   TOCTOU-safe path guard (`orchestrator/internal/tools/fs_tools.go`), MCP tool registry role
   enforcement (`orchestrator/internal/tools/registry.go`, `orchestrator/internal/server/server.go`),
   tool input schemas (`dispatch_tools.go`, `merge_tools.go`, `shadow_tools.go`), sidecar IPC
   authorization (`services/cwso-git-shadow/src/main.rs`, `services/cwso-merge-engine/src/ipc.rs`),
   the operator dashboard (`orchestrator/internal/dashboard/dashboard.go`), and
   `deploy/docker-compose.yml`'s full hardening posture.
3. **Live verification against a real running stack** (not source inspection alone):
   bootstrapped a JWT secret (`scripts/cwso-bootstrap-secrets.sh`) and brought up
   `deploy/docker-compose.yml` with `docker compose up -d`. Verified live: `/healthz` (200),
   `/mcp` JWT auth (no token → 401, bad origin → 403, valid minted token → 200), sidecar
   socket permissions (`stat -c '%a'` → `660` on both `git-shadow.sock`/`merge-engine.sock`),
   `scripts/check-ipc-gid-drift.sh` (clean, live uid=100/gid=101 matches the allowlist),
   `docker inspect` on orchestrator/git-shadow/merge-engine (confirmed `read_only: true`,
   `cap_drop: [ALL]`, `no-new-privileges:true`, non-root `USER cwso`, `network_mode: none`
   on the two sidecars), and the `rollout` opt-in service's SEC-C044-001 fix (started it
   with `--profile rollout`, confirmed `/run/cwso` is empty inside the container — no
   filesystem path to either sidecar socket). Full transcripts are in
   `docs/artifacts/security-v1.0-audit-v1.md`, "Live verification transcripts".
4. Manual secret-leakage sweep: `grep -rni "password|secret|token"` across `orchestrator/`
   and `services/` (excluding tests and known-safe JWT identifiers), plus
   `git log --all --diff-filter=A --name-only` for ever-added `.env*`/`.pem`/`.key`/
   `id_rsa`-pattern files. Zero live secrets found; only `.env.example` and
   `scripts/cwso-enable-all-features.env.example` templates exist, both non-live.
5. Live-exercised a specific hypothesis about the dashboard's rate-limiting posture (see
   Findings below) with 150 unauthenticated `GET /dashboard/status` requests against the
   running stack, contrasted directly against the same limiter's behavior on `POST /mcp`
   in the same process.
6. Ran the OWASP Top 10 checklist against the full in-scope surface — see the checklist
   table in `docs/artifacts/security-v1.0-audit-v1.md`.
7. Cleaned up fully after live testing: `docker compose down -v` (including the opt-in
   `rollout` profile), deleted the locally-generated `.env.jwt.dev`, confirmed
   `git status --short` clean before and after (no code, config, or non-artifact file was
   touched — read-only compliance maintained throughout).

### Areas audited

- JWT auth: secret bootstrap, token minting, HTTP verification middleware — PASS, live-verified.
- Secret handling (code, logs, git history) — PASS, zero secrets found.
- C015 read-write workspace mount + path-traversal defense (symlink-race-safe via
  `openat`/`O_NOFOLLOW` per-component walk) — PASS, reviewed in depth.
- C044 sidecar socket permissions and uid/gid allowlist — PASS, **re-verified live**
  (not taken on trust from `completed-tasks.md`), matches `SECURITY.md`'s documented claims
  exactly.
- Container hardening posture (`deploy/docker-compose.yml`) — PASS, live-verified via
  `docker inspect` against all four default-stack + opt-in-profile services.
- MCP boundary input validation (`orchestrator/internal/server/`, `orchestrator/internal/tools/`)
  — PASS, bounded/typed/schema-validated throughout.
- Operator dashboard (T010's original target: auth + no secret leakage in JSON) — auth is
  correctly implemented (SHA-256 hash + constant-time compare) and the JSON response is
  confirmed secret-free both statically and via a live response capture, but the auth
  endpoint itself has no rate limiting — see Findings.
- CI security-tooling posture against `security-baseline-v2.md` §5's required-checks list
  — 2 of 6 required tools present and gating (`govulncheck`, `cargo audit`); 4 absent.

### Findings

2 MEDIUM, 2 LOW. Zero CRITICAL/HIGH. Full findings table, evidence, and remediation in
`docs/artifacts/security-v1.0-audit-v1.md`:
- **F-C061-01** (MEDIUM, A04/A07/A09): dashboard auth endpoints (`/dashboard`,
  `/dashboard/status`) are not rate-limited — live-verified with 150 unthrottled GET
  attempts, contrasted against `/mcp` POST hitting 429 at request #11 under the same
  limiter.
- **F-C061-02** (MEDIUM, A06): 4 of 6 security-baseline-v2.md-required CI security tools
  (`gosec`, `cargo-deny`, Trivy image scan, `gitleaks`/`trufflehog`) are absent from
  `.gitlab-ci.yml`.
- **F-C061-03** (LOW, A07): `scripts/cwso-token.sh --ttl` has no upper bound.
- **F-C061-04** (LOW, A02): no TLS/reverse-proxy guidance for self-hosted, non-loopback
  deployments (`docs/user/deployment/proxmox-lxc-guide.md`).

### Blocker encountered (resolved)

`docker compose up -d --build` failed in this sandbox due to a WSL/vsock-level Docker
registry-credential issue unrelated to CWSO's own code (`error getting credentials`,
`UtilAcceptVsock:273: accept4 failed 110`). Mitigated: cached images already present in
the shared Docker image store (built 2026-08-28 from the same `develop` tip by a prior
agent session) allowed `docker compose up -d` (without `--build`) to succeed, producing a
fully live stack against which every live-verification claim above was captured. Logged as
type=external, severity=minor, 1 retry attempt, not escalated — did not block any
acceptance criterion.

### Verdict

**PASS.** Zero unresolved CRITICAL/HIGH findings. Two MEDIUM findings (F-C061-01,
F-C061-02) each carry a concrete remediation plan per security-guidelines.md; two LOW
findings (F-C061-03, F-C061-04) are routed to the debt register for C060 tracking. Full
verdict, OWASP coverage table, and findings-summary table in
`docs/artifacts/security-v1.0-audit-v1.md`'s `## VERDICT: PASS` section.

**T010 closure**: this audit supersedes and closes T010 (opened 2026-08-06, never run —
confirmed by the orchestrator's 2026-08-28 investigation). T010's original narrow scope
("auth on dashboard, no secret leakage in JSON") is fully covered above and in the linked
artifact — the dashboard's auth mechanism is sound and its JSON response is confirmed
secret-free live, with one live-verified gap (F-C061-01, rate limiting) that T010's
original scope would very plausibly have caught had it actually run in 2026-08-06.
Ledger archival of T010 (and this task, C061) is the orchestrator's to execute per the
task-management protocol; this brief supplies the verdict and artifact as the closing
evidence for both.
