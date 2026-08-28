# Contributing to CWSO

This is the contributor-process reference for CWSO: how to build and test the
stack from source, how branching and merge requests work, how the task
system tracks work, where outstanding proof-of-concept debt is tracked, and
where contributor material lives versus user-facing material.

If you only want to *run* CWSO rather than contribute to it, you don't need
this file — see "Docs vs. code layout" below for where to go instead.

## Build & test

Everything builds and runs in Docker; no local Go or Rust toolchain is
required.

```bash
make build   # build all service images (orchestrator, git-shadow, merge-engine)
make test    # run the Go + Rust test suites in containers
make lint    # run golangci-lint in a container
make fmt     # go fmt in a container
```

Run `make help` (or bare `make`) for the full target list, including `make
up` (one-command local stack), `make doctor` (pre-flight diagnostics), and
`make smoke` (the v1.0 definition-of-done end-to-end check). See the
[Makefile](Makefile) for exact target definitions.

## Branching & merge requests

CWSO follows GitFlow. Full branch-naming rules, the worktree lifecycle for
agent contributions, protected-branch handling, and the Conventional Commits
format are documented in [docs/branching.md](docs/branching.md) and
[.github/instructions/git-workflow.instructions.md](.github/instructions/git-workflow.instructions.md).

In short:

1. Branch off `develop`: `feature/<id>-<slug>`, `bugfix/<id>-<slug>`, or
   `agent/<role>/<task-id>` for agent-worktree contributions.
2. Commit using [Conventional Commits](https://www.conventionalcommits.org/)
   (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, …).
3. Open a merge request targeting `develop`. CI must pass and at least one
   review approval is required before merge.
4. Squash-and-merge `feature/*`/`bugfix/*` branches; `release/*` and
   `hotfix/*` preserve history. Delete the source branch after merge.

`main` and `develop` are protected — there are no direct pushes, including
for docs-only changes. See issue templates under
[.gitlab/issue_templates](.gitlab/issue_templates/) for how to file bugs and
feature requests.

## Task process

Work is tracked through a lightweight task ledger, not just merge requests:

- [docs/tasks/active-tasks.md](docs/tasks/active-tasks.md) — the live task
  board (`ID | Title | Owner | Status | Priority | Depends on | Last
  update`).
- [docs/tasks/completed-tasks.md](docs/tasks/completed-tasks.md) — append-only
  log of finished tasks and their outcome artifacts.
- `docs/tasks/task-<ID>.md` — one brief per task: objective, inputs, file
  ownership, rails, acceptance criteria, and (filled in after execution)
  execution notes.

Task lifecycle, ID conventions, and the full delegation/checkpoint protocol
this project runs on are defined in [AGENTS.md](AGENTS.md).

## Debt register

Proof-of-concept shortcuts and known limitations are tracked centrally in
[docs/DEBT-REGISTER.md](docs/DEBT-REGISTER.md), not scattered across ad hoc
notes. Every row carries a disposition (`v1.0-blocker`, `v1.1`, `wontfix`,
`fixed`, `unclear`). If you introduce a shortcut, tag it inline with a
`POC-DEBT` marker adjacent to the code and add a corresponding row to the
register — see the register's own "Purpose" section for the full format and
category conventions.

## Docs vs. code layout

Contributor-facing documentation lives at the repository root and under
`docs/` (this file, [docs/branching.md](docs/branching.md),
[docs/DEBT-REGISTER.md](docs/DEBT-REGISTER.md),
[docs/tasks/](docs/tasks/), [docs/decisions/](docs/decisions/) ADRs,
[docs/artifacts/](docs/artifacts/) requirements/architecture/security
baselines, and [docs/checkpoints/](docs/checkpoints/) phase reviews). It does
not live in a separate `docs/dev/` tree.

User-facing documentation — installing and running CWSO, connecting an MCP
client, troubleshooting — lives entirely under [docs/user/](docs/user/), in
the single guide to [using CWSO](docs/user/README.md). Keep the two
separate: link between them, don't duplicate content across them.

Application source lives under `orchestrator/` (Go MCP kernel),
`services/` (Rust sidecars), `schemas/` (shared MCP tool schemas),
`deploy/` (Dockerfiles, compose, CI), and `scripts/` (dev/integration
helpers). See the root [README.md](README.md) "Layout" section for the full
tree.

## Security

Read [SECURITY.md](SECURITY.md) before touching anything security-sensitive
(auth, IPC, sandboxing, secrets). Coding standards, including the OWASP-Top-10
and input-validation expectations that apply to every contribution, are in
[.github/instructions/coding-standards.instructions.md](.github/instructions/coding-standards.instructions.md).
