# CWSO — User Guide

> Moved from (deleted, C051): `installation-v1.md`, `installation-v2.md`, `installation-v3.md`, `ide-integration-v1.md`, `ide-integration-v2.md` — superseded by this guide; see git history for their content.

This is the single guide for getting CWSO running locally and connecting an MCP
client to it. It documents the **current, post-Phase-1** flow only: one command
(`make up`) to stand up the whole stack, one config paste to connect a client.
There is no separate "enable Phase N" or "enable all features" step — that flow
was removed.

If a command below doesn't work as written on a clean checkout, that's a bug in
this guide (or in CWSO) — please file an issue.

## Prerequisites

You need, on the host machine (everything else runs inside Docker):

| Tool | Why | Check |
|---|---|---|
| Docker Engine + Compose v2 plugin | Runs the whole stack; `docker compose` (v2), not the standalone `docker-compose` binary | `docker --version && docker compose version` |
| bash | `make up`, `make doctor`, and `make smoke` shell out to bash scripts (`#!/usr/bin/env bash`, using bash-only syntax) — a native Windows shell won't run them; use WSL2 on Windows | `bash --version` |
| curl | Used directly on the host by `make up` (health-wait step) and `scripts/cwso-doctor.sh`/`scripts/cwso-smoke-test.sh` | `curl --version` |
| python3 | `scripts/cwso-token.sh` shells out to `python3` to sign JWTs — needed even though no Go/Rust toolchain is required | `python3 --version` |
| git | To clone the repository | `git --version` |

Local Go/Rust toolchains are **not** required — everything builds and runs in
containers.

## Install

```bash
git clone https://gitlab.com/em-age/emage.code.cwso.git
cd emage.code.cwso
make up
```

`make up` is one command that: bootstraps a dev-only JWT signing secret if
none exists (`scripts/cwso-bootstrap-secrets.sh`), builds the images, starts
the stack (`docker compose up -d`), waits up to 120s for
`http://127.0.0.1:8080/healthz` to return `200`, then mints a short-lived MCP
token and prints a ready-to-paste client config block. The first run takes a
few minutes (image builds); later runs are fast.

By default, with no other configuration, the orchestrator mounts the bundled
`sample-workspace/` directory (nothing of yours) — see
[Daily use](#daily-use) below for pointing CWSO at your own repository.

If `make up` fails, it tells you which of its 5 steps failed and why (bootstrap,
build, start, health-wait, or token-mint) and exits non-zero. `make down` stops
the stack; `make logs` tails it.

## Configure your MCP client

When `make up` finishes, it prints a block that looks like this (the token
value is real and freshly minted — this is not a placeholder to fill in):

```text
===== PASTE INTO YOUR MCP CLIENT =====
{
  "servers": {
    "cwso": {
      "type": "http",
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "Authorization": "Bearer <token>",
        "Origin": "http://localhost"
      }
    }
  }
}
===== END =====
```

This is the generic `"servers"`-shaped MCP config block; `make up` itself
suggests pasting it into `.vscode/mcp.json` or `.cursor/mcp.json`. The token
expires per `scripts/cwso-token.sh`'s default TTL (1 hour) — re-run `make up`
or `scripts/cwso-token.sh` (see [Daily use](#daily-use)) to mint a new one.

**A note on client verification:** this repository's own MCP client
compatibility testing (`docs/artifacts/mcp-client-compatibility-v1.md`)
independently verified three real clients end-to-end, over both transports
CWSO exposes (stdio, Streamable HTTP): **Claude Code CLI**, **MCP Inspector**,
and **wong2/mcp-cli**. VS Code and Cursor were investigated but could not be
driven to a verifiable pass/fail result in that testing sandbox (no scriptable
path to their GUI) — so the paste-block instructions above reflect what the
tool prints, not an independently confirmed end-to-end result for those two
editors specifically. The subsections below are the clients this repo has
actually confirmed working.

### Claude Code CLI

stdio (no port, talks straight to the running container):

```bash
claude mcp add cwso -- docker exec -i cwso-orchestrator /usr/local/bin/cwso-orchestrator
```

Streamable HTTP:

```bash
TOKEN=$(scripts/cwso-token.sh)
claude mcp add --transport http cwso http://127.0.0.1:8080/mcp \
  --header "Authorization: Bearer $TOKEN" \
  --header "Origin: http://localhost"
```

### MCP Inspector (`--cli` mode)

Verified against `@modelcontextprotocol/inspector@1.0.2` specifically — pin
this version, since it's what CWSO's compatibility testing exercised.

stdio:

```bash
npx @modelcontextprotocol/inspector@1.0.2 --cli \
  docker exec -i cwso-orchestrator /usr/local/bin/cwso-orchestrator \
  --method tools/list
```

Streamable HTTP (the URL must come immediately after `--transport http`,
*before* any `--header` flags, or the CLI's argument parser swallows the URL
as a header value):

```bash
TOKEN=$(scripts/cwso-token.sh)
npx @modelcontextprotocol/inspector@1.0.2 --cli \
  --transport http http://127.0.0.1:8080/mcp \
  --header "Authorization: Bearer $TOKEN" \
  --header "Origin: http://localhost" \
  --method tools/list
```

### wong2/mcp-cli

**stdio only** — see [Troubleshoot](#troubleshoot) for why Streamable HTTP
does not work with this client against CWSO. Create a config file (same
format as the Claude Desktop config file):

```json
{
  "mcpServers": {
    "cwso": {
      "command": "docker",
      "args": ["exec", "-i", "cwso-orchestrator", "/usr/local/bin/cwso-orchestrator"]
    }
  }
}
```

The example below uses the **non-interactive `call-tool` mode**, which is
convenient for scripted one-off calls (see Troubleshoot for a historical note
on this client's default interactive mode):

```bash
npx @wong2/mcp-cli -c config.json call-tool cwso:create_shadow_workspace
```

## Verify

```bash
make doctor
```

Safe to run any time, before or after `make up`. Prints one `[OK]`/`[WARN]`/
`[FAIL]` line per check (Docker/Compose availability, port 8080, `/dev/kvm`,
`/dev/vhost-net`, the JWT secret file, and — only if the stack is already
running — sidecar sockets, `/healthz`, and that a freshly minted token is
accepted) with a one-line fix suggestion after every `[WARN]`/`[FAIL]`. It
never modifies anything and exits non-zero only if a `[FAIL]` was printed;
`[WARN]` alone is not a failure.

For a real, no-mocks, end-to-end proof that the whole flow works — create a
shadow workspace, write a file into it, query its AST, commit it, merge it,
tear it down — run:

```bash
make smoke
```

**This tears down the stack when it finishes**, on both success and failure
(`docker compose down -v --remove-orphans`, via a trap) — it's a
definition-of-done check, not a daily-use command. Run `make up` again
afterward before configuring a client or doing further work.

## Daily use

### Point CWSO at your own repository

By default the orchestrator mounts the bundled `sample-workspace/`. To use
your own repository instead, set `CWSO_WORKSPACE_HOST` to its **absolute
host path** before bringing the stack up:

```bash
CWSO_WORKSPACE_HOST=/absolute/path/to/your/repo make up
```

This mount is **read-write**. That doesn't mean an agent's edits land in your
working tree the instant a tool is called — your mounted repository is the
source of truth agents branch *from*; actual concurrent edits happen in
isolated, ephemeral **shadow workspaces**, which are created from (and merged
back into) this mount. A pre-flight check rejects an empty/missing
`CWSO_WORKSPACE_HOST` before the orchestrator starts (Docker would otherwise
silently create an empty directory for a typo'd path instead of failing).

The orchestrator container runs as a non-root Alpine system user — for the
mount to genuinely be writable, that user needs host-level write permission on
your repository directory (standard POSIX permissions still apply on top of
Docker's `:rw` mount flag).

### Re-minting tokens

`make up` mints a 1-hour `orchestrator`-role token automatically. Mint one
directly when you need a different role or lifetime:

```bash
scripts/cwso-token.sh --role worker --ttl 900
```

Use `worker`-role tokens for the shadow-workspace tools (`create_shadow_workspace`,
`write_shadow_file`, `query_ast`, `commit_shadow`, `drop_shadow_workspace`).
Use `orchestrator`-role tokens for `dispatch_concurrent_jobs` and
`merge_concurrent_results`. See `scripts/cwso-token.sh --help` for full usage.

### Rollout/Polar capture (opt-in)

The rollout/Polar trajectory-capture sidecar is real, working code, but
explicitly out of v1.0's default path — start it explicitly, only if you need
it:

```bash
docker compose -f deploy/docker-compose.yml --profile rollout up -d
```

## Deployment guides

The [Install](#install) section above covers the default, current local flow (`make up`
via Docker Compose). Additional deployment guides — for other environments, and for
wiring CWSO into an emage.code orchestrator — were received from emage.code (task T403)
and live under [`docs/user/deployment/`](deployment/README.md):

- [Local Docker Desktop guide](deployment/local-docker-desktop-guide.md) — an older,
  more manual Docker Compose flow (`deploy/docker-compose-t226.yml`) than the `make up`
  flow above; see the provenance index for a note on this overlap
- [GCP Cloud Run guide](deployment/gcp-cloud-run-guide.md) — not yet validated end-to-end
- [Proxmox LXC guide](deployment/proxmox-lxc-guide.md) — not yet validated end-to-end
- [CWSO overview and emage.code agent integration guide](deployment/cwso-overview-and-agent-integration-guide.md)
- [Connect CWSO to the emage.code orchestrator](deployment/cwso-emage-orchestrator-connection-guide.md) — validated
- [Deployment troubleshooting guide](deployment/troubleshooting-guide.md)

See [`deployment/README.md`](deployment/README.md) for provenance and validation status
of each.

## Troubleshoot

Start with `make doctor` — every `[WARN]`/`[FAIL]` line it prints comes with
its own one-line fix. A few things it won't catch, plus common `/mcp` errors:

| Symptom | Cause | Fix |
|---|---|---|
| `401 missing bearer token` | No `Authorization: Bearer <token>` header sent | Use the paste-block header, or mint one with `scripts/cwso-token.sh` |
| `401 invalid token` | Expired, or `iss`/`aud` claim mismatch | Mint a fresh token; claims must be `iss=cwso`, `aud=cwso-mcp` |
| `403 forbidden origin` | Missing/disallowed `Origin` header | Send `Origin: http://localhost` (matches the default `CWSO_ALLOWED_ORIGINS`) |
| `403 forbidden: unrecognised role` | Token's `role` claim isn't `worker` or `orchestrator` | Re-mint with `--role worker` or `--role orchestrator` |
| `429 too many requests` | Remote (non-localhost) IPs are limited to 60 req/min, burst 10 | Connecting from `127.0.0.1`/`::1` is exempt; otherwise wait for the `Retry-After` header |
| A client prompts for an OAuth client ID / "Dynamic Client Registration not supported" | The client didn't send a valid bearer header and fell back to its OAuth discovery flow — CWSO is bearer-JWT-only by design, it does not implement OAuth 2.1 | Fix the client's static-header config instead of registering an OAuth client; this is a genuine interoperability limitation, not a bug you can work around server-side |
| `wong2/mcp-cli`'s default (interactive) connection mode crashed with a Zod/schema error on older builds | Historical, already-fixed server bug: `resources/list` used to return `{"resources": null}` instead of `{"resources": []}` when empty, and this client's SDK rejects `null` where it expects an array. Fixed in task C036, before this guide was written; current builds return `{"resources": []}`. | If you're on a current build this shouldn't occur; the `call-tool` example above works either way. If you still see this crash, you're on a pre-C036 checkout — update, or use the non-interactive `call-tool` mode shown above as a workaround |
| `wong2/mcp-cli` over Streamable HTTP fails outright (`Invalid OAuth error response`) | This client has no static-bearer-header option for HTTP — only OAuth, which CWSO doesn't implement | Use stdio with this client instead (see above) |
| `CWSO workspace path is missing or empty` at startup | `CWSO_WORKSPACE_HOST` doesn't resolve to an existing, non-empty directory | Point it at a real, existing directory (an absolute path) and retry |

### Known limitations

- CWSO's `/mcp` HTTP endpoint authenticates with a bearer-only HS256 JWT and
  does not implement the MCP spec's (optional) OAuth 2.1 flow. Any client
  whose only remote-auth mechanism is OAuth will not work over HTTP against
  CWSO — this is a deliberate, spec-legal design choice, not a bug, but it is
  a real interoperability cost (see `docs/artifacts/mcp-client-compatibility-v1.md`).
- The Firecracker microVM sandbox tier requires `/dev/kvm` and `/dev/vhost-net`
  on the host. Without them, sandboxed execution degrades to a documented
  gVisor-only fallback — this is a supported, non-broken state, not a missing
  feature; `make doctor` only warns about it.
- Hardware-aware dispatch, sparse micro-agents, and the rollout/Polar capture
  sidecar are real, working code shipped in this build, but are explicitly
  out of v1.0's supported default path (deferred to v1.1) — the rollout
  opt-in profile above is the one exception exposed here.
- A dedicated, standalone limitations reference is planned but not published
  yet; this section is the current source of truth for known gaps.

For CWSO's internals and architecture, start at the project
[`README.md`](../../README.md). Looking to work on CWSO itself rather than
just run it — build/test, branching, the task process, or the debt
register? See [contributing](../../CONTRIBUTING.md).
