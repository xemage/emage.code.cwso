---
description: "Use when implementing authentication, authorization, input validation, cryptography, session management, or any security-sensitive code. Covers OWASP Top 10 prevention patterns, agent permission classification, and immutable security constraints."
applyTo: "**"
---

# Security Guidelines

## Input Validation
- Validate ALL user input on the server side (client-side is UX, not security)
- Use allowlists over denylists
- Validate type, length, format, and range
- Reject and log invalid input — don't silently fix it

## Authentication
- Hash passwords with bcrypt (cost >= 10) or argon2id
- Use cryptographically random session tokens (min 128 bits entropy)
- Implement account lockout after 5-10 failed attempts
- Enforce password policy: min 8 chars, complexity requirements
- Never store plaintext passwords

## Authorization
- Check authorization on EVERY request (not just the UI)
- Use principle of least privilege
- Validate resource ownership (`user.id === resource.ownerId`)
- Use role-based or attribute-based access control
- Log all authorization failures

## Session Management
- Set secure cookie flags: `HttpOnly`, `Secure`, `SameSite=Strict`
- Implement session expiration (idle + absolute timeout)
- Regenerate session ID after login
- Invalidate session on logout (server-side)

## API Security
- Use HTTPS everywhere (redirect HTTP → HTTPS)
- Implement rate limiting on all endpoints
- Set security headers:
  ```
  Strict-Transport-Security: max-age=31536000; includeSubDomains
  Content-Security-Policy: default-src 'self'
  X-Content-Type-Options: nosniff
  X-Frame-Options: DENY
  X-XSS-Protection: 0  (rely on CSP instead)
  ```
- Validate Content-Type on requests
- Use CORS allowlist (never `Access-Control-Allow-Origin: *` with credentials)

## Secrets Management
- NEVER commit secrets to source control
- Use environment variables or secret vaults
- Rotate secrets regularly
- Use different secrets per environment
- Audit secret access

## Database
- ALWAYS use parameterized queries / prepared statements
- Apply least-privilege database user permissions
- Encrypt sensitive data at rest
- Use TLS for database connections

## Logging
- DO log: authentication events, access control failures, input validation failures, admin actions
- DO NOT log: passwords, tokens, session IDs, PII, credit card numbers
- Include request ID for correlation
- Use structured logging format (JSON)

## Agent Permission Classification

Agents in the emage.code system operate under explicit permission boundaries. Each agent is classified by its allowed operations during specific phases.

### Write-Capable Agents (can modify code/files)
These agents may create, edit, and delete files within their assigned worktree scope:
- **Backend Engineer** — implementation phase
- **Frontend Engineer** — implementation phase
- **DevOps Engineer** — infrastructure/CI-CD files
- **Database Engineer** — schema and migration files
- **Technical Writer** — documentation files only

### Read-Only Agents (no code modification allowed)
These agents operate in read-only mode during their designated phases:
- **Tech Lead** — read-only during review phase (can only approve/reject, annotate, and request changes)
- **Security Engineer** — read-only during audit phase (can only flag issues, annotate, and create findings)
- **Product Owner** — read-only at all times (can only approve/reject requirements and priorities)

### Conditional Permissions
- **QA Engineer** — read-only during test execution; write access limited to test files only
- **Architect** — read-only for implementation code; write access limited to architecture docs and decision records

### Permission Enforcement Rules
1. Permission boundaries are enforced by the Orchestrator before task assignment
2. An agent cannot escalate its own permissions
3. Permission violations must be logged and the task must be rejected
4. The Orchestrator must verify agent permissions match the task type before dispatch

## Agent Safety Guards

Coding agents MUST treat the following as hard blocks unless the user explicitly
approves in the current session:

### Destructive operations (CC Safety Net)

Never run or suggest without explicit user approval:

| Category | Blocked patterns (examples) |
|----------|----------------------------|
| Git destructive | `git push --force`, `git push -f`, `git reset --hard`, `git clean -fd`, `git branch -D` on shared branches |
| Filesystem destructive | `rm -rf` on repo root or parent paths, `mv` overwriting without backup |
| History rewrite | `git rebase` on pushed branches, `git commit --amend` after push |

Safer alternatives: `git stash`, `git revert`, `git checkout -- <file>`, targeted `rm` on known paths.

### Secret and credential files (Envsitter-style)

- Do not read, cat, or copy: `.env`, `.env.*`, `credentials.json`, `*.pem`, `*.key`, `id_rsa`, `secrets.yaml`
- Do not paste secret values into chat, handoffs, checkpoints, or commits
- If env vars are needed, reference **names only** and instruct the user to set them locally

### Handoff and checkpoint content

- Handoff JSON (`docs/checkpoints/handoff-*.json`) must pass `runtime/handoff/validator.py`
- `forbiddenActions` in handoff constraints must include applicable destructive patterns
- Never serialize API keys or tokens in `payload` fields

### Violation response

1. Refuse the operation
2. Explain the guard that fired
3. Offer a safe alternative
4. Log as `SECURITY:MEDIUM` in the task or checkpoint if the user requested a blocked action

## Immutable Security Constraints

The following constraints are absolute and cannot be overridden by any agent, configuration, or runtime decision:

1. **No secrets in source control** — No API keys, passwords, tokens, certificates, or private keys may ever be committed. This is not negotiable regardless of PoC status, urgency, or convenience.
2. **No real PII in test data** — All test and demo data must use synthetic/anonymized data. Never use real personal information.
3. **No disabled security checks** — Security middleware, input validation, and authentication checks must never be bypassed, even in development or PoC mode.
4. **No unvalidated external input** — Every input from outside the system boundary (user input, API calls, file uploads, environment variables) must be validated before use.
5. **No privilege escalation paths** — No agent or service may grant itself elevated permissions. All permission changes require explicit approval from a higher-authority agent.
6. **No unencrypted sensitive data at rest** — Sensitive data must be encrypted. "We'll add encryption later" is not acceptable.

## OWASP Top 10 Checklist Reference

Use this checklist during security reviews and audits:

| # | Category | Key Prevention |
|---|----------|---------------|
| A01 | Broken Access Control | Deny by default, enforce server-side, validate ownership |
| A02 | Cryptographic Failures | Use strong algorithms, encrypt at rest + transit, no hardcoded keys |
| A03 | Injection | Parameterized queries, input validation, context-aware escaping |
| A04 | Insecure Design | Threat modeling, secure design patterns, abuse case testing |
| A05 | Security Misconfiguration | Hardened defaults, no unnecessary features, review all config |
| A06 | Vulnerable Components | Track dependencies, monitor CVEs, update promptly |
| A07 | Auth Failures | MFA, strong passwords, rate limiting, secure session management |
| A08 | Data Integrity Failures | Verify signatures, validate CI/CD pipeline integrity, use SRI |
| A09 | Logging & Monitoring Gaps | Log security events, alert on anomalies, retain audit trail |
| A10 | SSRF | Validate/sanitize URLs, use allowlists, block internal network access |

### Security Review Workflow
1. Run OWASP Top 10 checklist against every merge request touching security-sensitive code
2. Security Engineer flags findings as `SECURITY:CRITICAL`, `SECURITY:HIGH`, `SECURITY:MEDIUM`, or `SECURITY:LOW`
3. `CRITICAL` and `HIGH` findings block merge until resolved
4. `MEDIUM` findings must have a remediation plan before merge
5. `LOW` findings are tracked as technical debt
