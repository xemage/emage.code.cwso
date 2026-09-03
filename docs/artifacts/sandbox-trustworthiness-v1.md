# Artifact: sandbox-trustworthiness-v1.md

## Metadata
- **Producer agent**: backend-developer
- **Task**: C019
- **Created**: 2026-08-16 (initial draft); independently re-verified and revised 2026-08-16
- **Based on**: `orchestrator/internal/sandbox/{router.go,runner.go,runner_docker.go,runner_gvisor.go,runner_firecracker.go}` (T041-T044), `sandbox/README.md`, `sandbox/probe/host_probe.sh`, `deploy/docker-compose.yml` (pre-C019 state), `input/CWSO_ Agentic AI Orchestration Blueprint.md` §2.4, `docs/plans/plan-cwso-v1.0-roadmap.md` (Approval, decision 3), `orchestrator/internal/tools/dispatch_tools.go`, `orchestrator/internal/server/server.go`, `orchestrator/internal/config/config.go`, `orchestrator/internal/tools/fs_tools.go`, `services/cwso-git-shadow/{Cargo.toml,src/main.rs,src/repo.rs}`, `services/cwso-merge-engine/{Cargo.toml,src/ipc.rs}`, `deploy/Dockerfile.{git-shadow,merge-engine}`, `scripts/phase2-integration.py`, `Makefile` (`smoke-local` target)
- **Cited by**: `docs/tasks/task-C015.md` (read-write workspace mount default), `docs/tasks/task-C063.md` (LIMITATIONS.md — see "What the non-KVM tier does NOT guarantee")

> Note on the task brief's file paths: the brief's "Inputs" list refers to `sandbox/router.go`.
> The router and runner implementations actually live at
> `orchestrator/internal/sandbox/{router.go,runner_docker.go,runner_gvisor.go,runner_firecracker.go}`
> (this task's file ownership is read-only there). The top-level `sandbox/` directory
> contains only `README.md` and the host-capability probe (`sandbox/probe/`). This artifact
> audits the real implementation at the `orchestrator/internal/sandbox/` path and documents
> hardening changes made at the `deploy/docker-compose.yml` layer, per this task's actual
> file-ownership grant.

## 1. Scope: what "the non-KVM tier" is, precisely

CWSO's sandbox tiering (`ADR-003`, Blueprint §2.4) has three execution tiers:

| Tier | Backend | KVM required? | Intended use |
|---|---|---|---|
| `docker-trusted` | plain Docker (`runc`) | No | Internal orchestrator tooling only (non-escalation enforced) |
| `gvisor-fast-ephemeral` | Docker + `runsc` runtime | No | Benign, ephemeral sub-agent logic (**server-side default tier**) |
| `firecracker-secure-isolation` | Firecracker microVM | **Yes** | Untrusted/LLM-generated code, test execution |

`TierRouter.resolve()` (`orchestrator/internal/sandbox/router.go:125-150`) silently
demotes `firecracker-secure-isolation` requests to `gvisor-fast-ephemeral` whenever
`DegradedMode` is true (KVM/`vhost-net` absent) — reason code `DEGRADED_FALLBACK_GVISOR`
(`router.go:28-30,168-175`). Per this task's rails, this routing decision (Firecracker →
gVisor substitution on non-KVM hosts) is an already-made v1.0 scope call and is **not**
re-litigated here.

**"The non-KVM tier" = the `gvisor-fast-ephemeral` backend** (Docker + `runsc`, or plain
Docker/`runc` when an operator explicitly configures the single-tier `docker` runner
mode) is therefore the audit subject: it is both the default profile
(`ReasonPolicyDefault`, `router.go:126-129`) and the mandatory degraded-mode fallback for
every other tier. `docker-trusted` shares the identical hardening code path
(`dockerConfigFromGVisor()`, `runner_gvisor.go:120-139`) and is audited alongside it; it
is excluded from arbitrary dispatch by the non-escalation control
(`resolveDockerTrusted()`, `router.go:155-166`), verified below.

## 2. Property list (trustworthy = testable, non-negotiable minimums)

| # | Property | Definition |
|---|---|---|
| P1 | **Filesystem confinement** | A sandboxed process can read/write only its declared workspace mount (respecting the requested `ro`/`rw` mode) plus ephemeral scratch space; it cannot reach any other path on the container host or orchestrator host filesystem. |
| P2 | **Process isolation** | A sandboxed process runs in its own PID namespace, holds zero Linux capabilities, cannot acquire new privileges, and cannot escalate (`su`, `setuid`, direct device/mount manipulation). |
| P3 | **Resource limits** | CPU, memory, and process/thread count (pids) are capped by the container runtime (cgroups) at operator-configured, finite values; a runaway or hostile workload cannot exhaust host resources. |
| P4 | **Network policy** | A sandboxed process has no network egress by default (`NetworkMode=none`); `host` networking is rejected outright at the config-validation layer, not just by convention. |

## 3. Audit environment (why it is representative)

All live evidence below was captured on this task's execution host:

```
$ docker info --format '{{json .Runtimes}}'
{"io.containerd.runc.v2":..., "nvidia":..., "runc":...}   # no "runsc" entry

$ ls -la /dev/kvm
ls: cannot access '/dev/kvm': No such file or directory

$ uname -r
6.18.33.2-microsoft-standard-WSL2
```

No `/dev/kvm`, no `runsc` registered with the Docker daemon. This is a faithful stand-in
for "most v1.0 users [who] will run without KVM" per this task's brief — including the
fact that `runsc` is **not** present out of the box (`sandbox/README.md` documents it as a
manual host prerequisite: *"configure Docker runtime `runsc` ... and restart Docker"*).

## 4. Audit results

### 4.0 Fail-closed check: what happens when `runsc` is absent (this host's actual state)

```
$ docker run --rm --runtime=runsc alpine:3.20 true
docker: Error response from daemon: unknown or invalid runtime name: runsc
exit_code=125
```

In code, `GVisorRunner.ensureRuntime()` (`runner_gvisor.go:164-176`) performs the
equivalent check against `docker info` **before** any container is created, and returns
`missingRuntimeError(...)` — Execute() never reaches container creation. Confirmed by the
existing unit test suite, run live for this audit:

```
$ docker run --rm -v "$PWD/orchestrator:/src:ro" -w /src golang:1.25-alpine \
    go test ./internal/sandbox/... -v
...
--- PASS: TestGVisorRunnerMissingRuntimeFailsWithActionableError (0.00s)
--- PASS: TestFirecrackerDegradedModeFallsBackToGVisor (0.00s)
--- PASS: TestDockerTrustedWithoutAuthorizationOverridesToGVisor (0.00s)
--- PASS: TestDockerRunnerRejectsHostNetwork (0.00s)
--- PASS: TestDockerRunnerAppliesSecurityAndResourceDefaults (0.00s)
... (33 tests total)
PASS
ok  	github.com/emage/cwso/orchestrator/internal/sandbox	0.202s
```

**Verdict: no property is at risk here** — on a host lacking `runsc`, the gVisor tier
refuses to execute anything (fail-closed), rather than silently falling back to a weaker
or unsandboxed runtime. See §5 for what this means operationally.

The remainder of this audit tests the container-hardening layer shared by
`docker-trusted` and `gvisor-fast-ephemeral` (`DockerRunnerConfig` /
`dockerHostConfig`, `runner_docker.go:33-115,678-690`) — the actual isolation mechanism
that both tiers rely on, reproduced with live `docker run` invocations using the *exact*
flags the Go runner constructs by default. Two code paths combine to produce these
flags, not one: config-level defaulting in `withDefaults()` (`runner_docker.go:52-93`)
sets `NetworkMode="none"` (line 59-61), `CpuQuota=100000` (defaultCPUQuotaMicros,
line 62-64), `Memory=268435456` (defaultMemoryBytes, line 65-67), `PidsLimit=128`
(defaultPIDsLimit, line 68-70), and `ReadonlyRootfs=true` (line 89-91); the remaining
flags are hardcoded — not merely defaulted, and not overridable by any config or
request field — directly in `buildCreateRequest()` (`runner_docker.go:250-261`):
`Privileged=false`, `CapDrop=["ALL"]`, `SecurityOpt=["no-new-privileges:true"]`,
`CpuPeriod=100000` (the constant `defaultCPUPeriodMicros`, always applied regardless
of config).

### 4.1 P1 — Filesystem confinement: **MET**

Setup: hardened container launched with the runner's exact defaults, workspace bind-mounted read-only per `WorkspaceWritable=false` (the code default — see below):

```
$ docker run -d --name c019-hardened-test --privileged=false --cap-drop=ALL \
    --security-opt no-new-privileges:true --read-only --network=none \
    --cpu-period=100000 --cpu-quota=100000 --memory=268435456 --pids-limit=128 \
    --tmpfs /tmp:size=32m -v "$PWD/workspace-dir:/workspace:ro" alpine:3.20 sleep 60
```

Evidence:

```
$ docker exec c019-hardened-test sh -c 'echo x > /workspace/hello.txt'
sh: can't create /workspace/hello.txt: Read-only file system   # exit_code=1

$ docker exec c019-hardened-test sh -c 'echo x > /etc/pwned'
sh: can't create /etc/pwned: Read-only file system              # exit_code=1

$ docker exec c019-hardened-test sh -c 'mount | grep -v "^overlay\|^proc\|^tmpfs\|^sysfs\|^devpts\|^mqueue\|^shm\|^cgroup"'
/dev/sdf on /workspace type ext4 (ro,relatime,discard,errors=remount-ro,data=ordered)
/dev/sdd on /etc/resolv.conf type ext4 (ro,relatime)
/dev/sdd on /etc/hostname type ext4 (ro,relatime)
/dev/sdd on /etc/hosts type ext4 (ro,relatime)
```

No mount other than the declared `/workspace` and Docker's own bookkeeping files is
present; a sibling host directory placed next to the workspace fixture
(`host-outside-workspace/secret.txt`) is invisible from inside the container:

```
$ docker run --rm -v "$PWD/workspace-dir:/workspace:rw" --cap-drop=ALL \
    --security-opt no-new-privileges:true --read-only --tmpfs /tmp:size=32m \
    alpine:3.20 sh -c 'find / -maxdepth 1 -iname "*outside*" 2>/dev/null | wc -l'
0
```

When the workspace mode is explicitly `rw` (writable-mode override), writes land only
inside the declared mount and nowhere else:

```
$ docker run --rm -v "$PWD/workspace-dir:/workspace:rw" --cap-drop=ALL \
    --security-opt no-new-privileges:true --read-only --tmpfs /tmp:size=32m \
    alpine:3.20 sh -c 'touch /workspace/test2 && echo "rw-mount write OK"'
rw-mount write OK
```

Code-level corroboration:
- `runner_docker.go:246-249,263-268` — `RootFSWritable` requires
  `r.cfg.AllowWritableMount`, which is never set `true` anywhere in
  `orchestrator/internal/server/server.go`'s runner wiring (checked: `grep -n
  AllowWritableMount orchestrator/internal/server/server.go` → no matches). Root
  filesystem write override is unreachable in the current deployment.
- **`orchestrator/internal/tools/dispatch_tools.go:109-129`** — the dispatch tool's
  `RunRequest` construction never sets `WorkspaceWritable`. It defaults to Go's zero
  value (`false`), and `dispatchJobSpec` (`dispatch_tools.go:207-212` and the
  `InputSchema()` at `dispatch_tools.go:165-197`) exposes no `workspace_writable` field
  to MCP callers. **Every sub-agent job dispatched through `dispatch_concurrent_jobs`
  gets the workspace mounted read-only, unconditionally, regardless of caller input.**
- `dispatch_tools.go:116` — `CWSO_TARGET_WORKSPACE` (`spec.TargetWorkspaceUUID`) reaches
  the container only as an **environment variable value**, never as a bind-mount source
  path. The bind-mount source (`req.WorkspaceDir`) is fixed at server construction time
  from the single operator-configured `s.cfg.Workspace` value
  (`server.go:591-602`) — no per-request path is caller-controlled.

### 4.2 P2 — Process isolation: **MET**

```
$ docker exec c019-hardened-test sh -c 'cat /proc/self/status | grep -i cap'
CapInh:	0000000000000000
CapPrm:	0000000000000000
CapEff:	0000000000000000
CapBnd:	0000000000000000
CapAmb:	0000000000000000

$ docker exec c019-hardened-test sh -c 'grep NoNewPrivs /proc/self/status'
NoNewPrivs:	1

$ docker exec c019-hardened-test sh -c 'ps aux'
PID   USER     TIME  COMMAND
    1 root      0:00 sleep 60
   49 root      0:00 ps aux
$ ps aux | wc -l    # host process count, for contrast
134

$ docker exec c019-hardened-test sh -c 'su - root -c id'
su: can't set groups: Operation not permitted    # exit_code=1

$ docker exec c019-hardened-test sh -c 'mount -t tmpfs tmpfs /mnt'
mount: permission denied (are you root?)
```

All five capability sets are zero, `NoNewPrivs=1`, the PID namespace shows only the
container's own 2 processes against 134 on the host, and both a `su`-based
group-membership escalation and a raw `mount(2)` call (needs `CAP_SYS_ADMIN`, dropped)
fail. Code reference: `runner_docker.go:250-261` (`CapDrop: ["ALL"]`,
`SecurityOpt: ["no-new-privileges:true"]`, `Privileged: false` — all
non-configurable, i.e. not overridable by any `RunRequest` field).

### 4.3 P3 — Resource limits: **MET** (code-level was already met; **compose-level had a gap, closed — see §6**)

```
$ docker exec c019-hardened-test sh -c 'cat /sys/fs/cgroup/memory.max'
268435456
$ docker exec c019-hardened-test sh -c 'cat /sys/fs/cgroup/pids.max'
128
$ docker exec c019-hardened-test sh -c 'cat /sys/fs/cgroup/cpu.max'
100000 100000
```

cgroup values match `DockerRunnerConfig`'s defaults exactly
(`runner_docker.go:20-30,62-92`: `defaultCPUQuotaMicros=100000`,
`defaultMemoryBytes=256MiB`, `defaultPIDsLimit=128`).

Enforcement under load — 200 spawn attempts against a 128-pid cap:

```
$ docker exec c019-hardened-test2 sh -c '
i=0; ok=0; fail=0
while [ $i -lt 200 ]; do sleep 30 & rc=$?; [ $rc -eq 0 ] && ok=$((ok+1)) || fail=$((fail+1)); i=$((i+1)); done
echo "spawn_attempts=200 spawned_ok=$ok spawn_refused=$fail"'
sh: can't fork: Resource temporarily unavailable
```
(The shell itself hit the pids cap mid-loop — direct proof the limit is enforced by the
kernel cgroup controller, not merely configured.)

**Compose-level gap found and closed** (§6): `deploy/docker-compose.yml`'s
`orchestrator`, `git-shadow`, `merge-engine`, and `rollout` services (the containers
*hosting* the orchestrator/sidecars themselves, not the dynamically-created sub-agent
containers) had `read_only`, `cap_drop`, and `security_opt` but **no `pids_limit`** —
verified live before the fix:

```
$ docker inspect cwso-orchestrator --format '{{.HostConfig.PidsLimit}}'
<nil>
```

### 4.4 P4 — Network policy: **MET**

```
$ docker exec c019-hardened-test2 sh -c 'ip addr show'
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 ...
    inet 127.0.0.1/8 scope host lo
    inet6 ::1/128 scope host

$ docker exec c019-hardened-test2 sh -c 'ping -c1 -W2 1.1.1.1; wget -T3 -O- http://1.1.1.1'
ping: sendto: Network unreachable            # ping_exit=1
wget: can't connect to remote host (1.1.1.1): Network unreachable   # wget_exit=1
```

Only loopback exists; there is no route to any external or container-to-container
address. `host` networking is rejected at config-validation time, not merely never
requested — confirmed both by live evidence and by existing unit tests:

```
$ docker run --rm --network=host --cap-drop=ALL --security-opt no-new-privileges:true \
    --read-only alpine:3.20 sh -c 'ip addr show | head -3'
1: lo: <LOOPBACK,UP,LOWER_UP> ...    # the Docker CLI *would* allow this
```
i.e. nothing at the Docker Engine level blocks `--network=host` — the rejection is
enforced by CWSO's own code: `runner_docker.go:96-98` /
`runner_gvisor.go:66-67` (`strings.EqualFold(c.NetworkMode, "host") → error`), proven
live:

```
$ go test ./internal/sandbox/... -run TestDockerRunnerRejectsHostNetwork -v
--- PASS: TestDockerRunnerRejectsHostNetwork (0.00s)
```

## 5. Non-escalation control (context, not one of the four required properties)

`TierRouter.resolveDockerTrusted()` (`router.go:152-166`) — dispatch callers requesting
`docker-trusted` are unconditionally overridden to `gvisor-fast-ephemeral`
(`ReasonOverrideNonEscalation`) unless `AllowDockerTrusted=true`, which is reserved for
"fully-internal orchestrator workloads" and defaults `false`. Confirmed live:
`TestDockerTrustedWithoutAuthorizationOverridesToGVisor` — PASS. This means an arbitrary
dispatched sub-agent job cannot self-select the lower-friction `docker-trusted` path to
avoid `runsc` requirements; it either gets `gvisor-fast-ephemeral` (which fails closed
without `runsc`, §4.0) or is rejected.

**Important operational caveat**: this non-escalation check only applies when
`CWSO_SANDBOX_RUNNER=router`. The single-tier `CWSO_SANDBOX_RUNNER=docker` and
`CWSO_SANDBOX_RUNNER=gvisor` modes (`server.go:70-138`) each wire exactly one runner
directly — `TierRouter` and its non-escalation logic are not in the call path at all in
those modes, by design (there is only one tier to select). This is a config-mode
distinction, not a defect in either mode; the four required properties (§4) hold
identically in all three modes since they all funnel through the same
`dockerHostConfig` construction.

## 6. Hardening changes made

All changes are additive `deploy/docker-compose.yml` keys with inline justifying
comments; no existing hardening key was weakened, removed, or relaxed.

| Service | Change | Rationale |
|---|---|---|
| `orchestrator` | `pids_limit: 512` | No compose-level pids cap existed (`PidsLimit=<nil>` confirmed live pre-change). 512 gives headroom for the Go runtime's OS-thread usage while still bounding a runaway/compromised process. |
| `git-shadow` | `network_mode: "none"`, `pids_limit: 128` | Communicates with `merge-engine`/orchestrator exclusively via UDS sockets on the shared `cwso-runtime` volume; publishes no ports; no compose service resolves it by DNS name. Full network denial is safe and closes off any TCP/UDP egress/lateral-movement path. |
| `merge-engine` | `network_mode: "none"`, `pids_limit: 128` | Same rationale as `git-shadow`. |
| `rollout` | `pids_limit: 256` | Cannot go `network_mode: none` (publishes 8787, proxies to an upstream URL) — pids cap only. |

Verified functionally after the change (stack rebuilt and brought up). This required
working around one unrelated pre-existing local-environment issue first: the compose
`secrets:` block bind-mounts `../.env.jwt.dev` directly into
`/run/secrets/jwt_secret` (not a Swarm-managed secret), so the file's host permissions
are what the container sees; `scripts/cwso-bootstrap-secrets.sh` (C012) `chmod 600`s
that file to the *host* user, but the orchestrator container runs as a non-root,
different-UID `cwso` user (`deploy/Dockerfile.orchestrator:17-18,21`), so it cannot
read the mode-600 file and `orchestrator/internal/config/config.go:127-131,242-244`
correctly (fail-closed) refuses to start without a JWT secret. This is a pre-existing
secrets-provisioning gap unrelated to sandbox tiering, out of this task's file
ownership (`deploy/docker-compose.yml`'s `secrets:` block and
`scripts/cwso-bootstrap-secrets.sh` are C012/C013-owned), and **not modified by this
task** — permissions were changed locally only, for the duration of this verification,
and reverted immediately after:

```
$ chmod 644 .env.jwt.dev   # local-only workaround for this verification run; reverted after
$ docker compose -f deploy/docker-compose.yml up -d --build orchestrator git-shadow merge-engine
 Container cwso-git-shadow Started
 Container cwso-merge-engine Started
 Container cwso-orchestrator Started

$ docker compose -f deploy/docker-compose.yml ps
NAME                STATUS
cwso-git-shadow     Up 9 seconds
cwso-merge-engine   Up 9 seconds
cwso-orchestrator   Up 9 seconds (health: starting)

$ curl -sf http://localhost:8080/healthz
ok

$ docker logs cwso-orchestrator 2>&1 | grep -i "shadow tools enabled\|merge tools enabled"
{"level":"info","msg":"shadow tools enabled","socket":"/run/cwso/git-shadow.sock","time":"2026-08-16T18:19:24.066699569Z"}
{"level":"info","msg":"merge tools enabled","socket":"/run/cwso/merge-engine.sock","time":"2026-08-16T18:19:24.06671427Z"}

$ docker inspect cwso-git-shadow --format '{{.HostConfig.NetworkMode}} {{.HostConfig.PidsLimit}}'
none 128
$ docker inspect cwso-merge-engine --format '{{.HostConfig.NetworkMode}} {{.HostConfig.PidsLimit}}'
none 128
$ docker inspect cwso-orchestrator --format '{{.HostConfig.PidsLimit}}'
512

$ docker logs cwso-git-shadow 2>&1 | tail -1
{"timestamp":"2026-08-16T18:19:24.023685Z","level":"INFO","fields":{"message":"cwso-git-shadow ready", ...}}
$ docker logs cwso-merge-engine 2>&1 | tail -1
{"timestamp":"2026-08-16T18:19:24.032229Z","level":"INFO","fields":{"message":"cwso-merge-engine ready", ...}}

$ docker compose -f deploy/docker-compose.yml down -v
$ chmod 600 .env.jwt.dev   # workaround reverted
```

The orchestrator successfully dialed both `git-shadow.sock` and `merge-engine.sock` with
`network_mode: none` on those two services (UDS over the shared volume is unaffected by
container network namespace), and both sidecars logged their own "ready" status,
confirming the hardening did not break IPC or startup.

`docker compose -f deploy/docker-compose.yml config` and
`docker compose -f deploy/docker-compose.yml --profile rollout config` both parse
cleanly after the edits (exit 0), confirming compose-spec validity for all four
services including the profile-gated `rollout` service.

No changes were made to `orchestrator/**` code, `services/**` code, the C014
environment blocks, or the C011 rollout service's own environment/volumes — only the
listed hardening keys above, each commented `# C019: ...` in place.

### 6.1 Runtime-model check for the `pids_limit` values (git-shadow / merge-engine)

Both `cwso-git-shadow` and `cwso-merge-engine` use a thread-per-connection model —
`thread::spawn` per accepted UDS connection (`services/cwso-git-shadow/src/main.rs:76`,
`services/cwso-merge-engine/src/ipc.rs:41`) — and the cgroup `pids` controller counts
threads as well as processes, so `pids_limit: 128` is a real cap on concurrent
in-flight IPC connections per sidecar, not just process count. `128` gives roughly 2x
headroom over `defaultDispatchMaxBatch = 64`
(`orchestrator/internal/tools/dispatch_tools.go:20`, the orchestrator's own cap on a
single dispatched job batch), which is a plausible upper bound on concurrent
shadow/merge IPC calls in the v1.0 dispatch model. Neither service execs subprocesses
(`grep -rn "Command::new\|std::process::Command" services/cwso-git-shadow/src
services/cwso-merge-engine/src` → no matches), so there is no additional
per-request-forked-process pressure on the cap beyond the connection-handling threads
themselves.

### 6.2 Network-dependency check for `network_mode: "none"` (git-shadow / merge-engine)

Independently confirmed, not merely inherited from the prior draft: neither crate
declares a networking dependency (`services/cwso-git-shadow/Cargo.toml`,
`services/cwso-merge-engine/Cargo.toml` — no `reqwest`, `hyper`, `tokio` with a `net`
feature, `ureq`, or similar), and neither service's source imports `std::net::*`
(`grep -rn "std::net\|TcpStream\|TcpListener\|UdpSocket" services/cwso-git-shadow/src
services/cwso-merge-engine/src` → no matches; both only use
`std::os::unix::net::{UnixListener, UnixStream}`). `cwso-git-shadow` links `git2`
(vendored libgit2) but only for local, in-memory bare-repo blob storage — `repo.rs`
never calls `fetch`/`push`/`Remote`/`RemoteCallbacks`/`Cred` (grep confirms zero
matches), i.e. no remote Git operations are performed, so no DNS resolution or
outbound connection is ever attempted at runtime. The only network activity either
image's *build* performs is `apt-get`/`cargo build` inside the build stage
(`deploy/Dockerfile.git-shadow`, `deploy/Dockerfile.merge-engine`), which happens
before the image exists and is unaffected by the *running* container's
`network_mode`. `network_mode: "none"` is therefore correct and safe for both
services.

### 6.3 Note on this task's verification-command evidence: `scripts/cwso-smoke-test.sh`

This task's brief specifies `bash scripts/cwso-smoke-test.sh` as the required
smoke-test verification command. **That script does not exist in this repository**
(confirmed: `find . -iname "*smoke*"` returns no such file). The closest equivalent is
`make smoke-local`, which runs `scripts/phase2-integration.py` — a full end-to-end
integration test that builds all three images, brings up the compose stack, and drives
the orchestrator's MCP HTTP transport with a signed JWT.

Run against this task's changes (after the `.env.jwt.dev` permission workaround in
§6): the stack builds and starts cleanly, `/healthz` and the `git-shadow.sock` UDS
socket both become reachable — i.e. the same positive result independently captured
live in §6 above. The script then fails at its first authenticated MCP call
(`tools/list`) with `401 invalid token`. This failure is **unrelated to this task**:
it reproduces identically with this task's `deploy/docker-compose.yml` diff stashed
out (i.e. against the pre-C019 baseline), confirming it is a pre-existing JWT
signing/verification mismatch in the test harness or secrets flow (`.env.jwt.dev`'s
`KEY=VALUE` line format is passed through as a single opaque secret string by both the
orchestrator, `orchestrator/internal/config/config.go:127-128`, and the test client,
`scripts/phase2-integration.py:64-77`, without parsing — the two *should* end up with
an identical value, but evidently do not, for a reason this audit did not chase down
further since it sits outside `sandbox/**` / `deploy/docker-compose.yml`'s sandbox keys
and would require touching `orchestrator/*` and/or `scripts/*` this task does not own).
This is a smoke-test-harness/secrets-provisioning defect, not a sandbox-tiering
property (P1-P4) failure, and is reported here rather than silently worked around:

```
$ git stash push -- deploy/docker-compose.yml   # isolate: is this task's diff the cause?
$ make smoke-local
...
--- waiting for orchestrator /healthz ---
  OK  /healthz reachable
--- waiting for git-shadow socket ---
  OK  /run/cwso/git-shadow.sock present
--- 1. tools/list shows shadow tools ---
  unexpected response: {'_http_status': 401, '_body': 'invalid token\n'}
--- tearing down ---
make: *** [Makefile:55: smoke-local] Error 1
$ git stash pop   # restore this task's diff — identical failure, confirms no regression
```

**This gap is flagged for whichever task owns `scripts/phase2-integration.py` /
`scripts/cwso-bootstrap-secrets.sh` (C012/C013 lineage) to fix; it is not a hard-stop
for this task** because none of the four required trustworthiness properties (P1-P4,
§4) depend on this test script succeeding — P1-P4 were verified directly with live
`docker run`/`docker exec` evidence (§4) and the Go unit suite (§4.0), independent of
`phase2-integration.py`'s JWT flow.

## 7. What the non-KVM tier does NOT guarantee

Stated plainly, for `docs/user/LIMITATIONS.md` (C063) to cite directly:

1. **`runsc` (gVisor) is not installed or configured by default.** On a fresh v1.0
   install without KVM, `CWSO_SANDBOX_RUNNER=gvisor` (or `router` in degraded mode)
   requests **fail outright** with an actionable error
   (`missingRuntimeError`, `runner_gvisor.go:178-188`) until an operator manually
   registers `runsc` under Docker's `daemon.json` and restarts Docker
   (`sandbox/README.md`). This is fail-closed (no code executes, no property is
   violated) but it means **most v1.0 users will not get working sandboxed sub-agent
   execution out of the box** without that extra step.
2. **The default one-command compose stack does not wire sandbox execution at all.**
   `deploy/docker-compose.yml`'s `orchestrator` service does not set
   `CWSO_SANDBOX_RUNNER` (config default: `"none"`, `orchestrator/internal/config/config.go:152`),
   and does not mount the Docker socket. With the compose defaults as shipped,
   `dispatch_concurrent_jobs` runs `defaultRunFunc` (`dispatch_tools.go:145-154`) — an
   inert stub that returns immediately without executing any code, sandboxed or
   otherwise. Wiring `CWSO_SANDBOX_RUNNER` and Docker-socket access into the default
   compose stack is a separate, not-yet-scoped change outside this task's file
   ownership (`orchestrator/*` environment/volumes) and outside its "MUST NOT" list
   (no MCP tool surface / dispatch semantics changes).
3. **gVisor's syscall-interception layer (its differentiator from plain `runc`) could
   not be exercised with live evidence in this audit**, because this reference
   environment — deliberately chosen to match "most v1.0 users without KVM" — also
   lacks `runsc`. Everything verified in §4 is enforced by the OCI/`runc` layer
   (namespaces, cgroups, capabilities, seccomp, AppArmor — all present per `docker info`)
   that both `docker-trusted` and `gvisor-fast-ephemeral` share; gVisor's additional
   user-space kernel is extra defense-in-depth against kernel-level exploits
   specifically, and its absence should be read as *reduced* defense-in-depth against
   that one exploit class, not as an unmet P1-P4 property.
4. **This tier does not provide hardware-virtualization-level isolation.** Per the
   Blueprint (§2.4) and ADR-003, that guarantee is reserved for the Firecracker tier,
   which requires KVM and is out of scope for v1.0's shipped fallback path — this is an
   already-made, documented scope decision, not re-litigated here.
5. **Container image supply-chain integrity is out of scope for this audit.** P1-P4
   assume a non-malicious base image; image provenance/signing is not evaluated here.
6. **This audit does not cover the MCP tool surface** (`read_file_sync`,
   `write_file_sync`, `list_dir`) that the orchestrator's own trusted process (not a
   sandboxed sub-agent) uses to read/write the mounted workspace directly, confined by
   `pathGuard()` (`orchestrator/internal/tools/fs_tools.go:22-48`). That trust boundary
   is separate from sandbox tiering and is out of this task's scope
   (`orchestrator/*`, MCP tool surface — explicitly off-limits per the rails).

## 8. Overall verdict

| Property | Verdict |
|---|---|
| P1 — Filesystem confinement | **MET** |
| P2 — Process isolation | **MET** |
| P3 — Resource limits | **MET** (compose-level gap found and closed in this task — §6) |
| P4 — Network policy | **MET** |

No required property was found to be unmeetable on the non-KVM path within v1.0 scope.
The hard-stop condition in this task's rails was not triggered.

**For C015**: the container-level isolation guarantees (P1-P4) that back the
`gvisor-fast-ephemeral`/`docker-trusted` tiers are solid and evidenced above. The
read-write workspace mount itself is an **orchestrator-container-to-host** mount, not a
sub-agent-sandbox mount — dispatched sub-agent jobs get the workspace read-only,
unconditionally, by current code (§4.1). C015 should still read §7 items 1-2 carefully:
the sandbox tiers are not wired into the default compose stack yet, so today, no
sandboxed *or* unsandboxed sub-agent process actually touches the mounted repository at
all via `dispatch_concurrent_jobs` — the read-write exposure surface in the shipped
default stack is the orchestrator's own trusted Go binary and its `pathGuard`-confined
file tools, not arbitrary/LLM-generated code execution.

## Consumed by
- C015 (devops-engineer) — cites this artifact for the read-write workspace mount default
- C063 (LIMITATIONS.md author) — §7 "What the non-KVM tier does NOT guarantee"
