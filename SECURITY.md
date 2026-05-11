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
