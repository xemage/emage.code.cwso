# MCP Client Compatibility Matrix v1

**Task:** C033 (gates CG3 — "Protocol")
**Based on:** `docs/artifacts/mcp-gap-analysis-v1.md` (C030), `docs/decisions/ADR-013-mcp-protocol-path.md` (C031),
`orchestrator/internal/server/mcp_conformance_test.go` (C032), `orchestrator/internal/server/mcp_contract_snapshot_test.go` (C034)
**Date:** 2026-08-26
**Owner:** qa-engineer

## Scope and honesty statement

This matrix tests the **post-C032 protocol surface** against real, independently-implemented
MCP clients, over both transports the server exposes (stdio, Streamable HTTP). Per the task
brief, failures are recorded as valid, expected cell results — not hidden, not "fixed" by
patching server code mid-test. Two genuine, non-trivial, reproducible protocol-conformance
bugs were found this way (see [Cross-cutting findings](#cross-cutting-findings-not-tied-to-a-single-required-check)
below) and are reported here rather than worked around.

No server code was modified to produce any result in this document.

> **Correction (2026-08-27, pre-merge, applied in place, no version bump — see below):**
> independent Tech Lead review of this MR (!166) found two inaccuracies, both corrected in
> this document rather than left standing: (1) the acceptance-criteria table's "11 of 30"
> FAIL-or-N/A* count was arithmetic wrong (the reviewer's direct tally, independently
> reproduced by the orchestrator: 22 PASS / 6 FAIL / 2 N/A* = 8 of 30, not 11) — fixed in
> §6; (2) the justification for excluding MCP Inspector `2.4.0` in favor of `1.0.2` claimed
> a reproducible packaging bug that did not hold up under independent re-verification (by
> both the reviewer and, separately, the orchestrator) — corrected in §1. Neither correction
> changes the two reported protocol-conformance bugs (Findings A and B) or the underlying
> matrix results in §4, which the reviewer independently reproduced byte-for-byte before
> requesting these two narrow fixes. Edited in place under the existing `-v1` filename,
> not bumped to `-v2`: this document had not yet merged to `develop` at correction time (an
> open-MR revision, not a published-and-referenced-elsewhere rewrite), and the two tasks
> that already cite it by this filename (`docs/tasks/task-C036.md`, `task-C037.md`) depend
> only on Findings A/B, which are unchanged by this correction.

---

## 1. Client selection — investigation and rationale (judgment call, flagged)

The task brief's example client list (Claude Desktop/Claude Code, Cursor, VS Code MCP) was
**not** taken at face value. Each candidate was independently investigated in this sandbox
before being included or excluded:

| Candidate | Verdict | Evidence |
|---|---|---|
| **Claude Code CLI** (`claude`, v2.1.220) | **USED** | Installed, authenticated in-session (`claude -p "..."` returns real model output), native MCP client support via `claude mcp add` / `--mcp-config` / `--strict-mcp-config`, for both `stdio` and `http` transports. Genuine, independent Anthropic-maintained MCP client implementation. |
| **MCP Inspector** (`@modelcontextprotocol/inspector`, official MCP-project package) | **USED**, CLI mode | An initial `npx @modelcontextprotocol/inspector@latest` (2.4.0) attempt failed to start in this sandbox with an apparent module-resolution error (`Cannot find module '.../@modelcontextprotocol/core/dist/internal.mjs'`), which was characterized at the time as a reproducible `2.x`-release packaging bug. **Correction (post-review):** that characterization does not hold up. Independent re-verification — by the reviewing Tech Lead, and separately by the orchestrator, both launching `2.4.0` directly from an already-populated local package cache — shows `2.4.0` launches cleanly in this same sandbox, in both `--cli --help` mode and its full web-UI mode (`MCP Inspector Web is up and running at ... http://127.0.0.1:6274...`). The original failure was most likely a transient, one-off issue (e.g. a slow or incomplete first-time `npx` package resolution) rather than a genuine, environment-independent packaging defect, and calling it "not a sandbox artifact" was not warranted by the evidence available at the time. This correction does not affect the validity of this task's actual results: **v1.0.2** (used throughout §4.3–§4.4 below) is itself a genuine, official, functional release of the same package, tested unmodified, and those results stand unaffected — only the stated justification for not also testing `2.4.0` needed correcting, not the version actually used. Separately, Inspector's **web UI** could not be exercised regardless of version: the `mcp__playwright` MCP tool in this session is configured to launch the `chrome` channel binary, which is not installed at `/opt/google/chrome/chrome`, and installing it requires `sudo` (`npx playwright install chrome` fails: "a password is required" — no passwordless sudo in this sandbox). A bundled Chromium *is* present on disk (`~/.cache/ms-playwright/chromium-1228`), but the connected Playwright MCP server's launch args are fixed outside this task's file-ownership scope, so I did not modify shared config to force it. Used Inspector's own official **`--cli`** mode instead (same package, a first-class non-GUI mode, not a script I wrote) — this reaches the same proxy/client code, over both `stdio` and `http` (`--transport`, `--header`), with an actual `--method` dispatch (`tools/list`, `tools/call`, `prompts/list`, etc.). |
| **wong2/mcp-cli** (`@wong2/mcp-cli`, npm, v2.0.0) | **USED** | Third-party, published (not hand-rolled by me), built on the official `@modelcontextprotocol/sdk` (`^1.29.0`). Chosen as the third client after Cursor and VS Code (below) were ruled out. Supports both stdio (arbitrary command) and remote HTTP (`--url`, OAuth only). Not affiliated with Anthropic or the MCP project — an independent, real-world SDK consumer, which is exactly the kind of "different client, different quirks" coverage this task is after. Its use surfaced the two most serious findings in this report (see below), which is itself evidence it was exercised as a genuine independent implementation, not a rubber stamp. |
| **Cursor** | **Ruled out** (confirmed, not re-litigated) | `cursor` on `PATH` is an agent-wrapper CLI stub only ("No Cursor IDE installation found. Use 'cursor agent' or 'agent' to run the agent."). Not a real Cursor IDE/MCP-client install. Per the orchestrator's pre-flight check and my own confirmation of the same error, not pursued further. |
| **VS Code** (vanilla MCP, and its `anthropic.claude-code`/`saoudrizwan.claude-dev` (Cline) extensions) | **Investigated in depth, ruled out** | See [§2](#2-vs-code-investigation-in-detail) below. |
| **Codex CLI** (`codex`, `@openai/codex`) | Investigated, ruled out | Present on `PATH` (Windows npm global install reachable from WSL at `/mnt/c/...`), but genuinely broken: `Error: Missing optional dependency @openai/codex-linux-x64. Reinstall Codex: npm install -g @openai/codex@latest`. Not a real usable install in this sandbox. |
| Gemini CLI, OpenCode, Cline CLI, Pi | Investigated, ruled out (as *required*-slot candidates) | None of `gemini`, `opencode`, `cline`, `pi` exist as binaries on `PATH` in this sandbox (`command -v` fails for all four) despite this project's own multi-platform sync targeting some of these platforms' config folders. `continue` resolves via `command -v` but only because it is a **bash shell keyword**, not an installed tool — a false positive I verified and discarded. |
| **Cline CLI** (`cline` npm package, standalone — distinct from the VS Code extension) | Identified as a plausible 4th/bonus real client; **deliberately not pursued** | `npx cline --help` resolves to a real, actively-published, mainstream "autonomous coding agent CLI" with its own `mcp` subcommand and non-interactive prompt mode, structurally similar to `claude -p`. It requires authenticating a model provider (`cline auth`/`-P`/`-k`), and this sandbox has a live `ANTHROPIC_API_KEY` in its environment that would plausibly work. I chose **not** to spend it: three solid, independently-verified real clients (Claude Code, Inspector, wong2/mcp-cli) already satisfy the ≥3 requirement with full 5-step coverage across both transports, and using this session's live billed credential for a fourth, non-required row is not a cost I judged justified here. **Flagging this explicitly as a judgment call** — the orchestrator may reasonably disagree and ask for it as a 4th row. |

### 2. VS Code investigation, in detail

The orchestrator's brief noted VS Code Server is genuinely installed (`code --version` → real
`1.134.0`) but whether it's usable without a visual desktop was an open question. I investigated
rather than assumed either way:

- `code --status` proves this is **not a dead stub**: it reports a live, connected VS Code
  **Desktop** session running on the Windows host (`OS Version: Windows_NT`, real GPU/driver
  info), reached via Remote-WSL. This is a genuine, real, running GUI application — just not
  one whose screen I can see or click, because it renders on the Windows side, not through any
  X11/Wayland display in this Linux sandbox (consistent with the orchestrator's "no X server
  reachable" finding, which was correct for *this* sandbox's own display but is a different fact
  than "is VS Code Desktop running at all").
- `code --list-extensions --show-versions` shows this VS Code instance has **both**
  `anthropic.claude-code@2.1.246` (the Claude Code VS Code extension) and
  `saoudrizwan.claude-dev@4.1.16` (**Cline**, a genuine, well-known MCP-capable coding-agent
  extension) installed — real, MCP-relevant software, not absent.
- However, none of this is drivable from this session:
  - `code --help` (the remote-cli binary shipped in `.vscode-server`) exposes only file-open,
    diff/merge, and extension-management flags — no `chat`, `tunnel`, or any command that
    invokes MCP tool calls or dumps MCP connection state.
  - There is no accessibility bridge, VNC, or any other tool available to this session that can
    see or click a native Windows GUI window — `mcp__playwright` only automates browsers, not
    native desktop applications, and no such automation tool is present here.
  - The Claude Code VS Code extension is a UI wrapper around the same underlying engine as the
    `claude` CLI already used above — driving it through the GUI would not exercise a materially
    different MCP client implementation even if it were reachable.
  - Cline (`saoudrizwan.claude-dev`) *would* be a materially distinct client, but is
    webview/GUI-only in the VS Code extension form with no CLI/headless trigger for tool calls
    reachable from this session (see the *separate* standalone `cline` npm CLI discussion above,
    which is a different, non-VS-Code artifact).

**Conclusion: VS Code (in any of its installed forms here) cannot produce a verifiable
pass/fail result in this sandbox** — not because it's absent, but because there is no
scriptable or observable path to drive it. This is reported as the honest reason a 4th,
VS-Code-specific row is absent, rather than silently substituting something else in its place
or leaving the constraint unexplained.

---

## 3. Test environment

- Stack: `make up` (docker compose: `deploy/docker-compose.yml`) — orchestrator (`cwso/orchestrator:dev`),
  `git-shadow`, `merge-engine` sidecars. Confirmed healthy: `cwso-orchestrator` container status
  `Up ... (healthy)`, `GET /healthz` → `200`.
- **Streamable HTTP:** `http://127.0.0.1:8080/mcp`, JWT minted via `scripts/cwso-token.sh --ttl 21600`
  (role `orchestrator`, HS256, issuer `cwso`, audience `cwso-mcp`), header `Origin: http://localhost`
  (required by `CWSO_ALLOWED_ORIGINS`).
- **stdio:** `docker exec -i cwso-orchestrator /usr/local/bin/cwso-orchestrator` — this runs a
  *second* process inside the already-running orchestrator container, so it shares the exact
  same environment, filesystem, and sidecar Unix-domain-socket mounts (`git-shadow.sock`,
  `merge-engine.sock`, `sparse.sock`) as the HTTP instance, without needing a separate
  docker-compose topology just for stdio. `cfg.Transport` defaults to `stdio` when no `-transport`
  flag is passed (`cmd/cwso-orchestrator/main.go`), so no extra flags are required.
- Happy-path tool: `create_shadow_workspace` (no required arguments).
- Malformed-params tool: `query_ast` (requires `workspace_uuid`, `path`, `query_type`; allowed for
  both `orchestrator` and `worker` roles, so this exercises **parameter validation** distinctly
  from **role/permission** rejection — `commit_shadow`/`write_shadow_file` are `worker`-only, and
  `stdio` sessions are hardcoded to `Role: "orchestrator"` in `transport/stdio.go:22`, so those
  tools cannot be used for a clean malformed-params test over stdio; a role-permission rejection
  from `commit_shadow` is documented as a bonus finding instead, see below).
- Unknown-method / unknown-tool: either a literal unknown `tools/call` tool name, or the real
  spec method `prompts/list` (a documented **Missing** method per the gap analysis) where the
  client tooling allowed dispatching an arbitrary method name.
- No server code, config, or docker image was modified during testing. Test artifacts
  (ephemeral in-memory shadow workspaces created by the happy-path calls) require no manual
  cleanup — they exist only inside the disposable dev stack's process memory and are discarded
  when the stack is torn down after this task completes.

---

## 4. Matrix

Legend: **PASS** = behaved per gap-analysis-v1's definition of correct/expected (including
documented partial/deviation behavior); **FAIL** = did not — reproduction included;
**N/A*** = check could not be exercised as designed because of the *client's own* architecture,
not a pass or fail of the server — explained inline, never silently omitted.

### 4.1 Claude Code CLI (v2.1.220, agent-sdk 0.3.245) — stdio

| # | Check | Result | Detail |
|---|---|---|---|
| 1 | `initialize` handshake | PASS | Debug log: `Successfully connected (transport: stdio) in 114ms`; capabilities `{"hasTools":true,"hasResources":true,"hasResourceSubscribe":true}` — matches gap-analysis's documented resources-capability gating (spike/sparse tools enabled in this deployment). |
| 2 | `tools/list` | PASS | All 14 registered tools listed, incl. `create_shadow_workspace`, via `ToolSearchTool: keyword search for "mcp__cwso", found 14 matches`. |
| 3 | `tools/call` happy path (`create_shadow_workspace`) | PASS | `{"workspace_uuid":"598db64f-...","base_tree_oid":null}` — spec-shaped `CallToolResult` with a text content block, no `isError`. |
| 4 | Error path: unknown tool | **N/A*** | Claude Code validates tool names against its own already-fetched `tools/list` cache *before* dispatch. Attempting `mcp__cwso__does_not_exist_probe` produced `<tool_use_error>Error: No such tool available: mcp__cwso__does_not_exist_probe</tool_use_error>` — a client-side message, generated without a wire round-trip to the server (confirmed: this string is not our server's JSON-RPC error shape from `protocol.go`). This is genuinely untestable through normal use of this client — recorded as a client-architecture fact, not a server defect. |
| 5 | Error path: malformed params (`query_ast`, empty args) | PASS | `{"content":"workspace_uuid, path, query_type are required","is_error":true}` — real round trip, tool-level `isError` result, matches server's documented validation message. |

### 4.2 Claude Code CLI — Streamable HTTP

| # | Check | Result | Detail |
|---|---|---|---|
| 1 | `initialize` handshake | PASS | Debug log: `Successfully connected (transport: http) in 22ms`; same capabilities as stdio. Auth via `Authorization: Bearer <JWT>` + `Origin: http://localhost` headers, no OAuth fallback triggered (unlike the `installation-v3.md`-documented VS Code quirk — Claude Code sends the bearer header on the very first request). |
| 2 | `tools/list` | PASS | All 14 tools listed (as `mcp__cwso__<name>`). |
| 3 | `tools/call` happy path | PASS | `{"workspace_uuid":"ebf1c2db-...","base_tree_oid":null}`. |
| 4 | Error path: unknown tool | **N/A*** | Same client-side short-circuit as stdio; this time the client didn't even attempt a `tool_use` call after its internal `ToolSearch` found nothing. |
| 5 | Error path: malformed params (`query_ast`, empty args) | PASS | `{"content":"workspace_uuid, path, query_type are required","is_error":true}`. |

### 4.3 MCP Inspector v1.0.2 (`--cli` mode) — stdio

| # | Check | Result | Detail |
|---|---|---|---|
| 1 | `initialize` handshake | PASS | Implicit in successful connect (the CLI's `connect()` performs `initialize` before any method call; a failure here would abort before any output). |
| 2 | `tools/list` | PASS | `--method tools/list` → full 14-tool JSON array with `inputSchema`s, incl. `create_shadow_workspace`. |
| 3 | `tools/call` happy path | PASS | `--method tools/call --tool-name create_shadow_workspace` → `{"content":[{"type":"text","text":"{\"workspace_uuid\":\"d63fe2de-...\",\"base_tree_oid\":null}"}]}`. |
| 4 | Error path: unknown method (`prompts/list`) | PASS | `Failed to list prompts: MCP error -32601: method not found: prompts/list` — a genuine wire-level round trip (Inspector CLI's `--method` is validated client-side against a small known-method allowlist, but `prompts/list` **is** on that allowlist and is a real spec method our server marks Missing — so this reaches the actual server dispatch and gets the real `ErrMethodNotFound`, exit code 1). |
| 5 | Error path: malformed params (`query_ast`, empty args) | PASS | `{"content":[{"type":"text","text":"workspace_uuid, path, query_type are required"}],"isError":true}`. |

### 4.4 MCP Inspector v1.0.2 (`--cli` mode) — Streamable HTTP

| # | Check | Result | Detail |
|---|---|---|---|
| 1 | `initialize` handshake | PASS | `--transport http <url> --header "Authorization: Bearer <JWT>" --header "Origin: http://localhost"` connects successfully. (Note: positional target URL must precede the variadic `--header` flags on the command line — Commander.js's variadic option parsing otherwise swallows the URL as an extra header value. A CLI ergonomics quirk of this client, not a server issue — recorded for reproducibility.) |
| 2 | `tools/list` | PASS | Full 14-tool list returned. |
| 3 | `tools/call` happy path | PASS | `{"content":[{"type":"text","text":"{\"workspace_uuid\":\"3a13254e-...\",\"base_tree_oid\":null}"}]}`. |
| 4 | Error path: unknown method (`prompts/list`) | PASS | `Failed to list prompts: MCP error -32601: method not found: prompts/list`. |
| 5 | Error path: malformed params (`query_ast`, empty args) | PASS | `{"content":[{"type":"text","text":"workspace_uuid, path, query_type are required"}],"isError":true}`. |

### 4.5 wong2/mcp-cli v2.0.0 — stdio

| # | Check | Result | Detail |
|---|---|---|---|
| 1 | `initialize` handshake | PASS | `client.connect(transport)` completes successfully in both the default interactive flow and the config-driven non-interactive flow (server logs show `stdio transport ready` and a real request/response round trip in both cases). |
| 2 | `tools/list` | **FAIL** (severe — uncaught client crash, root-caused to a real server defect) | The client's **default, normal-use** connection flow (`npx @wong2/mcp-cli -- docker exec -i cwso-orchestrator /usr/local/bin/cwso-orchestrator`) calls `listPrimitives()`, which fires `client.listTools()` **and** `client.listResources()` concurrently via `Promise.all` (because the server's `initialize` response advertises the `resources` capability). `resources/list` returns `{"resources": null}` when there are zero active AST-spike/sparse-agent subscriptions (confirmed independently via a raw `docker exec` JSON-RPC call — see [Cross-cutting findings](#cross-cutting-findings-not-tied-to-a-single-required-check)). The MCP SDK's own Zod-based response schema requires `resources` to be an array; `null` fails validation and throws **synchronously inside the SDK's inbound-message handler**, outside any `try/catch` in `mcp-cli`'s own code. Result: an **uncaught exception and hard process crash** (`$ZodError: ... expected array, received null`, non-zero exit) before the tool ever prints a tools list, a prompt, or any usable output. `tools/list` itself likely succeeds over the wire (its own promise in the `Promise.all` isn't inherently broken), but the crash makes it entirely unreachable through this client's normal, documented usage. See full repro below. |
| 3 | `tools/call` happy path | PASS | Using the client's non-interactive `call-tool` mode (`--config <cfg> call-tool cwso:create_shadow_workspace`), which does **not** call `listPrimitives()`/`resources/list` and so does not hit the crash above: `{"content":[{"type":"text","text":"{\"workspace_uuid\":\"1510ad04-...\",\"base_tree_oid\":null}"}]}`. |
| 4 | Error path: unknown tool (`does_not_exist_probe`) | PASS | `call-tool cwso:does_not_exist_probe` → `{"error":"MCP error -32010: tool not found: does_not_exist_probe"}` — a genuine wire-level round trip (this client's non-interactive mode does not pre-validate tool names against a cached list, unlike Claude Code), correctly surfacing the server's `ErrToolNotFound`. |
| 5 | Error path: malformed params (`query_ast`, empty args) | PASS | `call-tool cwso:query_ast` → `{"content":[{"type":"text","text":"workspace_uuid, path, query_type are required"}],"isError":true}`. |

### 4.6 wong2/mcp-cli v2.0.0 — Streamable HTTP

| # | Check | Result | Detail |
|---|---|---|---|
| 1 | `initialize` handshake | **FAIL** | This client has **no option to set a static `Authorization` header** for its `--url` (Streamable HTTP) mode — its only remote-auth mechanism is a full OAuth 2.1 client-registration flow (confirmed by reading `src/mcp.js`: `StreamableHTTPClientTransport(new URL(uri), { authProvider })`, no header-injection path anywhere in the CLI or its config-file schema). Pointed at CWSO's bearer-JWT-protected `/mcp` endpoint with no credentials, it receives `HTTP 401 missing bearer token` (plain-text body, confirmed via raw `curl`), and instead of failing cleanly, its OAuth fallback path tries to parse that plain-text 401 body as an OAuth JSON error document and **throws an uncaught exception**: `ServerError: HTTP 401: Invalid OAuth error response: SyntaxError: Unexpected token 'm', "missing be"... is not valid JSON. Raw body: missing bearer token`. This closely mirrors the exact "Dynamic Client Registration not supported" / OAuth-fallback failure mode already documented for VS Code in `docs/user/installation-v3.md` §7 — this is a second, independent real client hitting the same class of problem, this time with a worse (crashing, not just confusing) failure mode. |
| 2 | `tools/list` | **FAIL** (blocked) | Never reached — connection never completes. |
| 3 | `tools/call` happy path | **FAIL** (blocked) | Never reached. |
| 4 | Error path: unknown tool | **FAIL** (blocked) | Never reached. |
| 5 | Error path: malformed params | **FAIL** (blocked) | Never reached. |

This is reported as a genuine client/server incompatibility, not "fixed" by e.g. standing up a
header-injecting proxy in front of the server for this client only — that would test the proxy,
not CWSO's real HTTP endpoint as a real user of this real client would actually experience it.

---

## 5. Cross-cutting findings (not tied to a single required check)

### Finding A — `resources/list` returns `{"resources": null}` instead of `{"resources": []}` when empty

**Reproduction (raw JSON-RPC, no client involved, both transports affected identically since
it's server-side and transport-independent):**

```
$ printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"resources/list"}' \
    | docker exec -i cwso-orchestrator /usr/local/bin/cwso-orchestrator
{"jsonrpc":"2.0","id":1,"result":{"resources":null}}
```

**Expected** (per MCP schema.ts `ListResourcesResult` and `mcp-gap-analysis-v1.md` §1
`resources/list` row, which documents the *happy-path* shape as `result.resources[]` but did not
call out the *empty*-array case): `{"resources":[]}` when there are zero active AST-spike/
sparse-agent subscriptions. A Go `nil` slice marshals to JSON `null`, not `[]`; this is a classic
Go/JSON-RPC empty-collection footgun, not a resources-gating issue (the capability itself is
correctly advertised and reachable — this is purely the shape of the *empty* result).

**Impact, by client, observed directly in this task:**
- **wong2/mcp-cli** (default flow): hard crash, uncaught `$ZodError`, non-zero exit, no usable
  output at all (§4.5).
- **Claude Code** (both transports): does not crash, but logs a real `[ERROR] Failed to fetch
  resources` after 3 retries with backoff (250ms/500ms/1000ms) client-side; `tools/list` and
  `tools/call` are unaffected because Claude Code's resources fetch and tools fetch are
  independent, not bundled in a single failing `Promise.all` the way `wong2/mcp-cli`'s is.
- **MCP Inspector CLI**: unaffected in the tests run here, because `--method tools/list` /
  `tools/call` do not implicitly also call `resources/list` (unlike the two clients above, which
  proactively fetch resources right after connecting because the capability is advertised).

This is a real, currently-shipping protocol-conformance defect surfaced only by testing against
independent real clients — a hand-rolled test would very plausibly not have caught it, since it
requires a client that (a) trusts the advertised `resources` capability enough to eagerly call
`resources/list`, and (b) validates the response shape strictly enough to reject `null`. This is
exactly the class of bug this task exists to find.

### Finding B — HTTP bearer-auth failure mode is not visible-client-specific

`docs/user/installation-v3.md` §7 already documents this exact class of problem for VS Code
("Dynamic Client Registration not supported" when a bearer token is missing/misconfigured). This
task's testing shows it is not a VS-Code-specific quirk: **any** MCP client whose only supported
remote-auth mechanism is OAuth (not static bearer headers) will hit the same wall against CWSO's
JWT-only HTTP endpoint, and at least one of them (`wong2/mcp-cli`) fails *worse* than VS Code does
— with an uncaught exception rather than a recoverable "ask for OAuth client ID" UX state. This
strengthens (does not newly discover) `mcp-gap-analysis-v1.md` Ambiguity #5's note that CWSO's
custom HS256 JWT bearer scheme, while spec-legal (`basic/authorization` is optional), is a real
interoperability cost against the broader real-world MCP client ecosystem.

---

## 6. Acceptance criteria check

| Criterion | Status |
|---|---|
| ≥6 cells (≥3 real clients × 2 transports), 5-step procedure per cell | **Met** — 3 clients × 2 transports = 6 cells, each with the 5 required checks attempted (30 checks total; 2 marked N/A* with a client-architecture explanation, never silently dropped) |
| Every failure recorded with reproduction detail | **Met** — §4.5, §4.6, and both cross-cutting findings include exact commands/output |
| Matrix published, failures included honestly | **Met** — this document; 22 PASS / 6 FAIL / 2 N/A* of 30 checks (8 of 30 are FAIL-or-N/A*), not all-green |

---

## 7. Verification

```
$ grep -c "stdio\|Streamable HTTP" docs/artifacts/mcp-client-compatibility-v1.md
21
$ grep -c "FAIL\|fail" docs/artifacts/mcp-client-compatibility-v1.md
21
$ git add docs/artifacts/mcp-client-compatibility-v1.md
$ git diff --stat --cached
 docs/artifacts/mcp-client-compatibility-v1.md | 20 ++++++++++++++++++--
 1 file changed, 18 insertions(+), 2 deletions(-)
```

(Counts above are post-correction — see the corrigendum note earlier in this document. No
result cell changed; the rise from the original 19 is caused only by lines that discuss the
count itself, which this sentence deliberately avoids re-explaining at length, to keep this
number from drifting on every future edit to this paragraph. If this exact figure and the
one immediately above ever disagree with a fresh run of the command, trust the fresh run.)

(`grep -c` counts matching *lines*, not occurrences — both terms appear multiple times on some
lines, e.g. table header rows. `git diff --stat` alone shows no output for this file because it
is new/untracked until staged; `--cached` against the last commit is the correct invocation for
a brand-new file and is what's reproduced above and in the MR description.)
