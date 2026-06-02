# Task T093 — Enforce/document TLS for non-loopback HAL accelerator endpoints

- **Status:** in_review
- **Owner:** devops-engineer
- **Priority:** P1
- **Depends on:** T089 (done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A) — follow-up
- **Based on:** `docs/artifacts/gate-phase6-feature-a-2026-06-02.md`, `SECURITY.md`

## Objective
Prevent the accelerator bearer API key (and prompt/response payload) from being sent over
plaintext `http://` to a remote host, where it could be intercepted.

## Design
New `services/cwso-hal/src/security.rs` with `validate_endpoint(base_url, allow_insecure)`:

- **`https://`** → allowed.
- **`http://` to a loopback host** (`localhost`, `*.localhost`, `127.0.0.0/8`, `[::1]`) →
  allowed (co-located sidecar; traffic never leaves the host).
- **`http://` to a non-loopback host** → **rejected** (`InsecureNonLoopback`) unless
  `CWSO_HAL_ALLOW_INSECURE_ENDPOINTS=true`, in which case it is allowed with a startup warning.
- Unsupported scheme / unparseable URL → rejected.

Wired into `main.rs::register_openai_from_env`: a rejected endpoint is **not registered**
(error logged), so a misconfigured remote `http` endpoint never ships a secret in cleartext;
the deployment simply falls back to the CPU baseline. The URL parser strips userinfo, port,
path, query, and fragment, and unwraps IPv6 literals.

Documented in `SECURITY.md` under "HAL accelerator endpoints (TLS)".

## Acceptance Criteria
- [x] Plaintext `http` to a non-loopback host is refused by default.
- [x] `https` and loopback `http` remain allowed.
- [x] Explicit `CWSO_HAL_ALLOW_INSECURE_ENDPOINTS=true` override (with warning) for trusted nets.
- [x] Behavior documented in `SECURITY.md`.
- [x] `cargo fmt --check` clean; `cargo test -p cwso-hal` green.

## Tests
- `security`: https-allowed, loopback-allowed (incl. IPv4/IPv6/`*.localhost`), non-loopback
  rejected, override-allowed, userinfo/port stripped, unsupported scheme, unparseable, IPv6
  non-loopback rejected.

## Notes / Follow-ups
- The HAL ↔ orchestrator channel remains a UDS with SO_PEERCRED authz (already in place); this
  task covers only the outbound HAL → accelerator HTTP leg.
