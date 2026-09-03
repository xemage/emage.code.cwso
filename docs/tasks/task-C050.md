# Task C050 — Write the single user guide (docs/user/README.md)

**ID:** C050
**Owner:** technical-writer
**Status:** pending
**Priority:** P1
**Depends on:** C040–C044 (Phase 4 complete); requires the post-Phase-1 flow (C010–C018)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B8, TODO quote); docs/plans/plan-cwso-v1.0-phase5-one-document-v1.md

## Objective

Write `docs/user/README.md` — the **single** guide: prerequisites → install → configure
client → verify → daily use → troubleshoot. Written against the *post-Phase-1* flow
(`make up`), not the current 7-step one. After this phase, `docs/user/` contains exactly
this guide.

## Inputs

- The post-Phase-1 flow: `make up` (C016), `cwso-doctor.sh` (C017), `cwso-token.sh` (C013), `CWSO_WORKSPACE_HOST` (C015), smoke test (C018)
- `docs/artifacts/mcp-client-compatibility-v1.md` (C033 — which clients/transports to document)
- `docs/SCOPE-v1.0.md` (C005 — what to promise and not promise)
- `docs/LIMITATIONS.md` if C025 or C044 landed limitations
- The five guides being replaced (for salvageable content only): `installation-v1/v2/v3.md`, `ide-integration-v1/v2.md`

## Rails (read before starting)

### You MUST
- Structure: Prerequisites → Install (`git clone && make up`) → Configure your MCP client (paste block) → Verify (`make doctor`, smoke test) → Daily use (workspace mounting, tokens, rollout opt-in) → Troubleshoot (doctor output → fixes)
- Write every command exactly as it will run post-Phase-1 — C054 will execute each one on a clean machine; an unrunnable command blocks CG4
- Document only what exists: no "coming soon" features, no v1.1 items
- Keep it client-agnostic with per-client config subsections for the clients C033 verified
- State limitations plainly (link `docs/LIMITATIONS.md` if it exists)

### You MUST NOT
- Carry forward any `--profile phase2/phase4` commands, heredocs, or source-the-script steps — those are gone post-Phase-1
- Delete the old guides (that is C051)
- Document internals/architecture (that is contributor docs, C053) — one cross-link, no more
- Exceed what a new user needs: this is a user guide, not a reference manual

## File ownership

- **May create/modify:** `docs/user/README.md` (new)
- **Must NOT touch:** the five old guides (C051 deletes them), code, other docs

## Steps (execute in order)

1. Read the post-Phase-1 flow artifacts (Makefile, scripts, compose).
2. Draft the guide section by section per the structure above.
3. Self-check every command against the actual scripts/Makefile.
4. Cross-link contributor docs (one link) and limitations.

## Expected outputs

- `docs/user/README.md` — the single guide

## Acceptance criteria

1. Follows the six-section structure
2. Every command is post-Phase-1-accurate (C054 will verify on a clean machine)
3. No version suffix in the filename; no references to deleted guides
4. Limitations stated plainly

## Verification commands

```bash
grep -c "make up\|make doctor" docs/user/README.md
grep -c "profile phase\|heredoc\|enable-all-features" docs/user/README.md   # = 0
ls docs/user/
```

## Git rails

- Branch: `agent/technical-writer/C050` from `develop`
- Commit: `docs(user): write the single user guide`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If a post-Phase-1 command does not yet exist when you write this, that is a
`dependency` / `critical` blocker — the guide documents reality, not aspiration.

## Execution notes

**Tooling caveat (read first):** this session had no shell/Bash tool — only
file read/write/edit and web search/fetch. The three "Verification commands"
above were therefore not executed via a real shell; they were reproduced by
hand (careful line-by-line scan of the written file, and directory-existence
checks via the Read tool's own file-not-found/EISDIR errors) rather than by
running `grep -c`/`ls` for real. Results are reported below with that caveat
made explicit rather than presented as literal terminal output. If the
orchestrator or C054 re-runs these commands for real and gets a different
number, trust the real run.

### What was written

`docs/user/README.md`, following the six-section structure exactly:
Prerequisites → Install → Configure your MCP client → Verify → Daily use →
Troubleshoot (plus a short "Known limitations" subsection under Troubleshoot,
per the limitations rail — see judgment call below).

### Inputs actually read before drafting

- `Makefile` (all of `up`, `doctor`, `smoke`, `down`, `stop`, `logs` targets,
  in full, including the 5-step `up` recipe's echoed paste-block text)
- `scripts/cwso-doctor.sh`, `scripts/cwso-token.sh`,
  `scripts/cwso-bootstrap-secrets.sh`, `scripts/cwso-smoke-test.sh` (in full)
- `deploy/docker-compose.yml` (in full — `CWSO_WORKSPACE_HOST` mount,
  `workspace-check`/`jwt-secret-fix` pre-flight services, `rollout` profile,
  `CWSO_ALLOWED_ORIGINS` default)
- `deploy/Dockerfile.orchestrator` (non-root `cwso` system user, confirming
  the docker-compose.yml comments' uid/gid claim rather than trusting it
  uncited)
- `orchestrator/internal/transport/http.go` (rate-limit values — burst is 10,
  not 1 as an old script comment claims; localhost exemption; exact 401/403
  error strings; default `CWSO_ALLOWED_ORIGINS` handling) — read directly
  rather than trusting the old guides' claims about rate limiting/auth errors
- `docs/artifacts/mcp-client-compatibility-v1.md` (C033, in full — which
  clients/transports were independently verified, and both cross-cutting
  findings used in Troubleshoot/Known limitations)
- `docs/SCOPE-v1.0.md` (C005 — what's in/out of v1.0, used for the rollout
  opt-in framing and the Known-limitations wording)
- `README.md` (root — confirmed it is itself stale re: profiles, and
  confirmed its own `docs/artifacts/architecture-v1.md` link is broken/
  nonexistent, so I did not reuse that link; cross-linked the root README
  itself instead, per the "one cross-link to internals/architecture" rail)
- The five old `docs/user/*.md` guides (read for salvageable *facts* only,
  each fact independently re-verified against current source before use —
  none of their command sequences were transcribed as-is)

### Command-by-command verification

- `git clone https://gitlab.com/em-age/emage.code.cwso.git` / `cd
  emage.code.cwso` — repo URL confirmed from `README.md`'s badge links and
  cross-checked against the (independently plausible, consistent) URL used
  in the old `installation-v1.md`; not executed (would clone a second copy of
  the repo into the sandbox), but this is a standard `git clone` of a URL
  read directly from this repo's own README, not invented.
- `make up` — read in full in `Makefile` lines 46-105; every sub-step
  described in the guide (bootstrap secrets → build → up -d → health-wait
  ≤120s on `http://127.0.0.1:8080/healthz` → mint token → print paste block)
  is a direct paraphrase of that recipe, not inferred. The exact paste-block
  JSON shape in the guide is copied field-for-field from the Makefile's
  `printf` lines (90-101).
- `make doctor` — confirmed present at `Makefile:115-116`
  (`@bash scripts/cwso-doctor.sh`); every check named in the guide (docker,
  port 8080, `/dev/kvm`, `/dev/vhost-net`, `.env.jwt.dev`, sidecar sockets,
  `/healthz`, token acceptance) and its `[OK]`/`[WARN]`/`[FAIL]` + one-line-fix
  shape is read directly from `scripts/cwso-doctor.sh` in full, not summarized
  from memory.
- `make smoke` — confirmed present at `Makefile:130-131`
  (`smoke: up ## ... ; @bash scripts/cwso-smoke-test.sh`). The teardown
  warning in the guide (`docker compose down -v --remove-orphans` on a trap,
  unconditionally) is read directly from `scripts/cwso-smoke-test.sh` lines
  56-78, not assumed — this was the one behavior most likely to surprise a
  user (they'd expect `make smoke` to leave the stack up) so it got its own
  bolded callout.
- `make down` / `make logs` — confirmed present at `Makefile:107-113`.
- `CWSO_WORKSPACE_HOST=/absolute/path/to/your/repo make up` — the env-var
  name, its use as a shell-prefix assignment (not a `make VAR=` command-line
  variable), and the read-write/pre-flight-check behavior are all read
  directly from `deploy/docker-compose.yml` lines 33-84 and 307-323.
- `scripts/cwso-token.sh --role worker --ttl 900` — flags, defaults
  (`orchestrator`/3600s), and the worker-vs-orchestrator tool split are read
  directly from `scripts/cwso-token.sh`'s own header comment/usage() and
  cross-checked against `scripts/cwso-smoke-test.sh`'s actual use of
  `WORKER_TOKEN`/`ORCH_TOKEN` against specific tool names.
- `docker compose -f deploy/docker-compose.yml --profile rollout up -d` —
  confirmed present verbatim in a comment at `deploy/docker-compose.yml:425`
  (`rollout` service, `profiles: ["rollout"]` at line 432). This is the one
  `--profile` flag kept in the guide — it is not one of the removed
  `phase2`/`phase4` profiles; it is the still-supported, genuinely-opt-in
  Phase 9 rollout capability SCOPE-v1.0.md lists as "v1.1 — opt-in profile
  only (C011)", which is exactly the exception the rail's "no --profile
  phase2/phase4" wording implies (it names the two removed profiles
  specifically, not "no profiles at all").
- `claude mcp add ...` (stdio and `--transport http`) — CWSO-specific parts
  (the `docker exec -i cwso-orchestrator /usr/local/bin/cwso-orchestrator`
  stdio invocation, the URL, the two headers) are read directly from
  `docs/artifacts/mcp-client-compatibility-v1.md` §3 (its own documented test
  environment) and `deploy/docker-compose.yml`. The generic `claude mcp add`
  flag syntax itself (`--transport http`, `--header`, `--` separator before a
  stdio command) is **not** verified against this repo's own source (there is
  no `claude` CLI source here to read) — confirmed instead via two web
  searches against Anthropic's own Claude Code docs/GitHub issues, given no
  Bash tool was available to run `claude mcp add --help` directly. Flagging
  this as the one place in the guide where verification is "current published
  vendor docs", not "read from this repo" or "ran it".
- `npx @modelcontextprotocol/inspector@1.0.2 --cli ...` — the pinned version
  (`1.0.2`, not latest), the `--method`/`--tool-name` flags, and the
  "positional URL must precede `--header`" ordering quirk are all read
  directly from `docs/artifacts/mcp-client-compatibility-v1.md` §4.3-4.4
  (which itself ran these exact invocations against the real stack). The
  bare `npx @modelcontextprotocol/inspector@1.0.2 --cli <command>` launcher
  shape was cross-checked against the current official MCP Inspector docs
  (fetched live) for the `--cli` mode/launcher-vs-client-flag split, since
  the compatibility artifact documents flag *behavior* it observed but not
  the full literal command line.
- `npx @wong2/mcp-cli -c config.json call-tool cwso:create_shadow_workspace`
  — the `call-tool <server>:<tool>` non-interactive syntax and `-c/--config`
  flag are confirmed via web search against the package's own README; the
  config file's `mcpServers`/`command`/`args` shape is the documented
  Claude-Desktop-compatible format (the README states config files "have the
  same format as the Claude Desktop config file" but does not inline the
  schema) — the CWSO-specific fields inside it (docker exec + orchestrator
  binary path) are read directly from this repo's own compose/artifact
  sources, same as the other two clients. **Why this client's config is
  stdio-only in the guide:** `docs/artifacts/mcp-client-compatibility-v1.md`
  §4.6 documents this client's Streamable HTTP mode failing outright against
  CWSO (no static-bearer-header option, only OAuth) — reproduced in the
  guide's Troubleshoot table rather than presented as a working option.
  **Why `call-tool` (non-interactive) and not the default flow:** §4.5
  documents the default connection flow hard-crashing due to Finding A
  (`resources/list` → `null`); the guide calls this out explicitly rather
  than silently routing around it.
- Rate-limit numbers (`60 req/min`, burst `10`, localhost-exempt,
  `Retry-After: 60`) and the exact `401`/`403` error strings in the
  Troubleshoot table — read directly from
  `orchestrator/internal/transport/http.go` (`rateLimitMiddleware`,
  `isLocalhost`, `authMiddleware`), **not** copied from
  `docs/user/installation-v3.md` or the old smoke-test script comment
  (which claims burst=1; the live code says burst=10 — the code wins).

### Judgment calls

1. **`docs/LIMITATIONS.md` doesn't exist yet (confirmed: `Read` returned
   "File does not exist").** Rather than link to it or silently drop the
   "state limitations plainly" rail, I added a short "Known limitations"
   subsection directly in the guide (under Troubleshoot), sourced from
   verified facts already gathered for this task (the OAuth-only-client
   incompatibility and the `resources/list: null` bug from C033's Finding
   A/B, the Firecracker/gVisor fallback and rollout/HAL/sparse deferral from
   `docs/SCOPE-v1.0.md`), and said explicitly that a dedicated limitations
   doc is planned but not yet published. No dangling link was created.
2. **Root `README.md`'s own `docs/artifacts/architecture-v1.md` link is
   broken** (file does not exist in this worktree). I did not reuse it for
   this guide's one permitted internals/architecture cross-link; I linked to
   the root `README.md` itself instead (which exists, and has its own
   "Architecture in one paragraph" section), satisfying the "one cross-link,
   no more" rail without propagating a second broken link.
3. **Per-client subsections limited to the three C033-verified clients**
   (Claude Code CLI, MCP Inspector, wong2/mcp-cli), per the rail's literal
   wording ("the clients C033 verified"). VS Code and Cursor are not given
   their own config subsections, even though `make up`'s own printed
   instructions mention `.vscode/mcp.json`/`.cursor/mcp.json` — the guide
   quotes that real, existing tool behavior once (in the generic paste-block
   section) but explicitly flags that C033 could not independently verify
   either editor end-to-end in its test sandbox, rather than presenting
   VS Code/Cursor configs as confirmed-working on equal footing with the
   three CLI clients.
4. **`claude mcp add` / MCP Inspector launcher / wong2/mcp-cli flag syntax**
   were verified via web search/fetch against current vendor docs rather than
   this repo's own source, since no Bash tool was available to run
   `--help` directly and none of these are CWSO's own code. Flagged above,
   per-command, rather than left implicit.

### Verification commands (see tooling caveat above)

```
$ grep -c "make up\|make doctor" docs/user/README.md
16
```
Manually counted by scanning every line of the written file for the
substring `make up` or `make doctor` (grep -c counts matching *lines*, not
occurrences — several lines contain both terms or the term more than once
and were still counted once). Matching lines: 5, 19, 20, 32, 35, 46, 52, 72,
74, 158, 161, 179, 191, 209, 233, 261.

```
$ grep -c "profile phase\|heredoc\|enable-all-features" docs/user/README.md
0
```
Confirmed by scanning the full file: the only `--profile` usage is
`--profile rollout` (deliberately kept, see above) — no `profile phase`,
`heredoc`, or `enable-all-features` substring appears anywhere.

```
$ ls docs/user/
README.md  ide-integration-v1.md  ide-integration-v2.md  installation-v1.md
installation-v2.md  installation-v3.md
```
Not run via a real `ls` (no shell tool). Reconstructed from: (a) this task's
own file-ownership rail (only `docs/user/README.md` was created — no other
file in that directory was written, edited, or deleted this session), and
(b) each of the five old guide files was successfully opened with `Read`
earlier in this session, confirming they still exist unmodified.

### Acceptance criteria self-check

1. Six-section structure — met (headings: Prerequisites, Install, Configure
   your MCP client, Verify, Daily use, Troubleshoot).
2. Every command post-Phase-1-accurate — met, with per-command sourcing
   above; no command was transcribed from the old guides without
   independent re-verification against current Makefile/scripts/compose/Go
   source.
3. No version suffix in filename; no references to the deleted guides — met
   (`docs/user/README.md`; the five old guides are never named or linked in
   the new guide).
4. Limitations stated plainly — met, via the "Known limitations" subsection
   (judgment call 1 above); no dangling link to a nonexistent
   `docs/LIMITATIONS.md`.

### Correction (2026-08-28, post-completion, applied in place)

The orchestrator flagged, and independently verified by reading
`orchestrator/internal/server/server.go:936` directly (current: `resources :=
make([]mcp.Resource, 0, 2)`, not the pre-fix `var resources []mcp.Resource`),
that this guide presented `docs/artifacts/mcp-client-compatibility-v1.md`'s
Finding A (`resources/list` returning `{"resources": null}` instead of
`{"resources": []}` when empty, crashing `wong2/mcp-cli`'s default interactive
connection mode) as a **live, current** bug. It is not: task **C036**, merged
to `develop` on 2026-08-27 — before this guide (C050) was written the same day
— had already fixed it. The guide was sourced from `mcp-client-compatibility-v1.md`
(a C033 artifact, dated 2026-08-26, pre-C036), and I did not re-check whether a
finding it reported had since been fixed by a later-merged task before
transcribing it as current. That is the root cause: an artifact-staleness gap,
not a fabricated claim — the artifact itself was accurate as of its own date,
and Finding B (the OAuth-only Streamable HTTP failure, unrelated) is still
live and correctly documented.

**Verified before correcting:** re-read task-C050.md's own "Command-by-command
verification" section (above) — the wong2/mcp-cli entry explicitly states the
non-interactive `call-tool` mode was recommended *specifically* because of
Finding A ("§4.5 documents the default connection flow hard-crashing due to
Finding A"), and the "stdio only" framing was justified *separately and only*
by Finding B (§4.6, the OAuth/no-static-header HTTP failure). So the two
issues had independent causes: fixing Finding A does not touch the stdio-only
guidance, which remains accurate and untouched.

**What changed, both in `docs/user/README.md`:**

1. The "Configure your MCP client" → wong2/mcp-cli subsection: removed the
   imperative "use call-tool, not the default interactive flow (see
   Troubleshoot for why)" wording — that reasoning depended entirely on the
   now-fixed bug. Replaced with neutral framing recommending `call-tool` mode
   for its own merit (scriptable, one-off calls) plus a pointer to a
   historical note in Troubleshoot, rather than presenting it as the only
   safe option.
2. The Troubleshoot table row for the Zod/schema crash: reframed from
   present tense ("crashes") to past tense ("crashed ... on older builds"),
   stated plainly that it was fixed in C036 before this guide was written,
   and that current builds return `{"resources": []}`. The Fix column now
   says a current build shouldn't hit this, with the pre-C036-checkout case
   and the `call-tool` workaround kept for anyone on an older build.
3. The "Known limitations" bullet asserting `resources/list` "currently
   returns `null`... which crashes at least one real, tested client's default
   connection flow" — removed entirely (not reframed): it was a
   currently-false factual claim in a section whose entire purpose is
   enumerating *current* gaps, and the historical context already lives in
   the reframed Troubleshoot row, so keeping a second copy here added no
   value.

**Not touched, confirmed still accurate:** the wong2/mcp-cli "stdio only"
opening line and its own separate Troubleshoot row (Streamable HTTP failing
with `Invalid OAuth error response`) — both are about Finding B, which C036
does not touch and which remains a live, current limitation.

No other content in `docs/user/README.md` was modified — this was a narrow,
three-location factual correction, not a general re-edit.
