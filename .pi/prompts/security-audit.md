---
description: "Request a security audit of the project or specific components, checking for OWASP Top 10 vulnerabilities."
argument-hint: "Specify scope: full project, specific feature, or files..."
---

Please perform a security audit on:

{{input}}

## Audit Scope

1. OWASP Top 10 compliance check
2. Authentication and authorization review
3. Input validation and injection prevention
4. Cryptographic implementation review
5. Secrets management verification
6. Dependency vulnerability scan
7. Security headers and configuration

## OWASP Coverage Matrix

8. **Produce an OWASP Top 10 coverage matrix**:

| # | OWASP Category | Status | Findings | Severity |
|---|---------------|--------|----------|----------|
| A01 | Broken Access Control | ✅/⚠️/❌ | ... | ... |
| A02 | Cryptographic Failures | ✅/⚠️/❌ | ... | ... |
| A03 | Injection | ✅/⚠️/❌ | ... | ... |
| A04 | Insecure Design | ✅/⚠️/❌ | ... | ... |
| A05 | Security Misconfiguration | ✅/⚠️/❌ | ... | ... |
| A06 | Vulnerable Components | ✅/⚠️/❌ | ... | ... |
| A07 | Auth Failures | ✅/⚠️/❌ | ... | ... |
| A08 | Data Integrity Failures | ✅/⚠️/❌ | ... | ... |
| A09 | Logging & Monitoring | ✅/⚠️/❌ | ... | ... |
| A10 | SSRF | ✅/⚠️/❌ | ... | ... |

## Severity Classification

9. Classify each finding with severity:
   - **CRITICAL**: Actively exploitable, immediate remediation required
   - **HIGH**: Exploitable with moderate effort, fix within current sprint
   - **MEDIUM**: Potential risk, schedule for next sprint
   - **LOW**: Hardening recommendation, add to backlog

## Verdict Output

10. **Produce a structured VERDICT** at the end of the audit:

```
## VERDICT

- **Status**: PASS | CONDITIONAL_PASS | FAIL
- **CRITICAL findings**: <count>
- **HIGH findings**: <count>
- **MEDIUM findings**: <count>
- **LOW findings**: <count>
- **OWASP coverage**: <n>/10 categories assessed
- **Blocker IDs**: [if FAIL — list blocking findings]
- **Auditor**: security-engineer
- **Timestamp**: <ISO-8601>
```

Provide a structured security report with findings, severity levels, and remediation steps.
If any CRITICAL findings exist, the verdict MUST be FAIL.
