<!--
Use this template for issues that come from a CWSO planning task (T-prefix).
Match the brief in docs/tasks/task-<ID>.md.
-->

## Objective
<!-- One paragraph: what this task delivers and why it matters now. -->

## Inputs
<!-- Required upstream artifacts with explicit versions, e.g. requirements-v1.md. -->
- 

## Expected Outputs
<!-- Concrete files / endpoints / behaviours produced by this task. -->
- 

## Acceptance Criteria
<!-- Specific, testable conditions. Each line becomes a checkbox. -->
- [ ] 
- [ ] 
- [ ] 

## Dependencies
<!-- GitLab issue links (#NN) or task IDs blocking this work. -->

## Constraints
- Phase budget:
- File ownership boundaries:
- Technology constraints:

## Definition of Done
- [ ] Implementation complete and self-reviewed
- [ ] Unit tests added/updated; `go test ./... -race` and/or `cargo test --release` pass
- [ ] Integration test still green (where applicable)
- [ ] Documentation updated (`docs/`, ADRs if architectural)
- [ ] PoC-DEBT entries registered for any shortcuts
- [ ] Conventional Commits used throughout
- [ ] MR opened against `develop` and approved

/label ~"status::pending"
