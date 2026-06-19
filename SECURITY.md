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
