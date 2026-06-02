---
name: "testing-strategy"
description: "Define test strategies, create test plans, select testing frameworks, design test architectures, and establish coverage targets. Use when planning testing approach for a project, choosing test frameworks, or creating comprehensive test plans."
---

# Testing Strategy

Skill for creating comprehensive testing strategies and test plans for software projects.

## When to Use
- Planning testing approach for a new project
- Choosing and configuring test frameworks
- Creating test plans for features or releases
- Defining coverage targets and quality gates
- Setting up test infrastructure

## Procedure

### 1. Assess Testing Needs

Determine testing scope based on:
- Application type (API, web app, mobile, library)
- Risk profile (financial, healthcare = higher coverage)
- Team size and testing expertise
- CI/CD pipeline capabilities

### 2. Testing Pyramid

```
         ╱  E2E Tests   ╲        Few (5-10%)
        ╱  Smoke + Critical ╲     Slow, brittle, high confidence
       ╱                     ╲
      ╱  Integration Tests    ╲   Moderate (20-30%)
     ╱  API, DB, Components    ╲  Medium speed, good confidence
    ╱                           ╲
   ╱     Unit Tests              ╲  Many (60-70%)
  ╱  Functions, Classes, Utils    ╲ Fast, isolated, foundational
 ╱─────────────────────────────────╲
```

### 3. Framework Selection

#### JavaScript/TypeScript
| Type | Framework | Use Case |
|------|-----------|----------|
| Unit | Vitest / Jest | Functions, services, utilities |
| Component | Testing Library | React/Vue/Angular components |
| Integration | Supertest | API endpoint testing |
| E2E | Playwright | Browser automation |
| API | Playwright API / Hurl | HTTP API testing |

#### Python
| Type | Framework | Use Case |
|------|-----------|----------|
| Unit | pytest | Functions, classes |
| Integration | pytest + httpx | API testing |
| E2E | Playwright | Browser automation |
| Load | Locust | Performance testing |

#### .NET
| Type | Framework | Use Case |
|------|-----------|----------|
| Unit | xUnit / NUnit | Functions, services |
| Integration | WebApplicationFactory | API testing |
| E2E | Playwright | Browser automation |

### 4. Test Plan Template

```markdown
# Test Plan: [Feature/Release Name]

## Scope
- **In Scope**: [What will be tested]
- **Out of Scope**: [What will NOT be tested]

## Approach
| Test Type | Coverage Target | Framework | Responsible |
|-----------|----------------|-----------|-------------|
| Unit | 80% | [framework] | Developers |
| Integration | Key APIs | [framework] | QA + Dev |
| E2E | Critical paths | [framework] | QA |
| Performance | Response < 200ms | [framework] | QA |
| Security | OWASP Top 10 | Manual + SAST | Security |

## Test Cases
### [Feature Area 1]
| TC-ID | Description | Priority | Type | Status |
|-------|-------------|----------|------|--------|
| TC-001 | [Test description] | High | Functional | Not Run |
| TC-002 | [Test description] | Medium | Edge Case | Not Run |

## Entry Criteria
- [ ] Feature code complete
- [ ] Code review approved
- [ ] Test environment available

## Exit Criteria
- [ ] All high-priority test cases passed
- [ ] No critical or high bugs open
- [ ] Coverage targets met
- [ ] Performance targets met

## Risks
| Risk | Mitigation |
|------|-----------|
| [Risk] | [Mitigation] |
```

### 5. Test Data Strategy
- Use factories/builders for test data creation
- Separate test data from production data
- Reset database state between test runs
- Use realistic but anonymized data

### 6. Quality Gates (CI/CD)
```yaml
# Minimum thresholds to pass pipeline
quality_gates:
  unit_coverage: 80%
  integration_tests: all_pass
  security_scan: no_critical
  lint: zero_errors
  e2e_critical_paths: all_pass
```

## Output
- Testing strategy document
- Framework recommendations with setup instructions
- Test plan template populated for the project
- Coverage targets and quality gates
- CI/CD test stage configuration

## Multi-Agent Workflow Validation Protocol

### Scenario Suites
- Run three standard scenarios:
  - `small`: TODO-style app with basic CRUD and auth.
  - `medium`: commerce-style app with API + UI + background jobs.
  - `complex`: distributed or integration-heavy system with security-critical flows.

### Required Validation Layers
- Role behavior checks:
  - Product Owner quality of acceptance criteria and scope boundaries.
  - Architect decision consistency and artifact lineage.
  - Tech Lead merge-gate quality and waiver handling.
- Handoff integration checks:
  - API contract parity (backend outputs vs frontend expectations).
  - Defect routing closure (QA findings mapped to owners and resolved state).
  - Security-to-release blocking logic consistency.
- Command regression checks:
  - `new-project`, `new-poc`, `evaluate-poc`, `code-review`, `prepare-release`, `team-status`.

### Scorecard Template
Use this scorecard for each scenario:

```markdown
# Workflow Validation Scorecard: [scenario]

## Completion Metrics
- Time to first architecture checkpoint:
- Time to integration checkpoint:
- Blocker escalation correctness:

## Quality Metrics
- Artifact lineage complete: yes/no
- Gate decisions coherent: yes/no
- Critical defects escaped: count

## Handoff Metrics
- API contract mismatches: count
- Unresolved blocker carryover: count
- Command regression failures: count

## Verdict
- pass | conditional_pass | fail
- Required remediation backlog:
```

### Acceptance Thresholds
- `pass`: zero critical contract or gate failures, no unresolved escalated blockers.
- `conditional_pass`: only medium/low workflow gaps with explicit owners and remediation ETA.
- `fail`: any critical workflow contract violation or release-blocking inconsistency.

---

## Protocol-Aware Enhancements

### Validation Gate Awareness

Test results are a critical input to the **integration validation gate**. The testing strategy must account for how test outcomes feed into gate decisions:

**Gate integration flow:**
```
Test Execution → Results Collection → Coverage Analysis → Gate Verdict
```

The QA validation gate consumes test results and produces a verdict:
```
[VERDICT] gate=qa-validation | result=PASS|CONDITIONAL_PASS|FAIL | coverage={N}% | failed_tests={count} | critical_failures={count} | artifact_ref=test-plan-v{N} | date={YYYY-MM-DD}
```

### VERDICT Format for QA Gate

The QA gate uses the same VERDICT protocol as code review, ensuring consistent gate behavior across the workflow:

| Verdict | Criteria | Pipeline Effect |
|---------|----------|-----------------|
| `PASS` | All tests pass, coverage thresholds met, no critical defects. | Pipeline proceeds to next stage. |
| `CONDITIONAL_PASS` | Minor test failures (non-critical paths), coverage within 5% of threshold. | Pipeline proceeds; failures logged as tasks in `docs/tasks/active-tasks.md`. |
| `FAIL` | Critical test failures, coverage below threshold by >5%, or security test failures. | Pipeline halts. Defects must be fixed and tests re-run. |

### Coverage Thresholds That Determine Gate Outcome

The following coverage thresholds directly determine the QA gate verdict:

| Metric | PASS Threshold | CONDITIONAL_PASS Range | FAIL Threshold |
|--------|---------------|----------------------|----------------|
| **Unit test coverage** | ≥ 80% | 75–79% | < 75% |
| **Integration test pass rate** | 100% | ≥ 95% (no critical failures) | < 95% or any critical failure |
| **E2E critical path pass rate** | 100% | N/A (critical paths must pass) | < 100% |
| **Security scan** | No critical/high findings | No critical; high findings have mitigations | Any critical finding |
| **Performance** | All targets met | ≤ 10% deviation on non-critical endpoints | > 10% deviation on critical endpoints |

**Important:** These thresholds are defaults. Project-specific overrides can be documented in the test plan artifact (`test-plan-vN.md`), but overrides require Tech Lead approval recorded as a decision artifact.

### Test Result Reporting in Checkpoints

Every checkpoint that follows a test execution phase MUST include a test summary:

```
[TEST-SUMMARY] suite={unit|integration|e2e} | total={N} | passed={N} | failed={N} | skipped={N} | coverage={N}% | duration={N}s
```

Multiple test summaries can appear in a single checkpoint (one per suite).
