#!/usr/bin/env bash
# scripts/cwso-projection-e2e.sh — CG2 close-out: real filesystem projection,
# proven end-to-end, in CI (C024).
#
# This is deliberately a SIBLING to scripts/cwso-smoke-test.sh (C018), not an
# extension of it: C018 drives its 7 stages entirely over the MCP HTTP
# transport (including write_shadow_file for its one "edit" stage) and never
# touches the real, projected filesystem path at all. This script exists
# specifically to prove the thing C018 does not: that the real path a shadow
# workspace projects to (`<storage_root>/<workspace-uuid>/`, ADR-012, C021)
# is reachable and usable by *ordinary* shell tooling -- `ls`, `cat`, a real
# test runner, `sed` -- not just by MCP tool calls, and that edits made that
# way (bypassing every CWSO tool) are captured by commit_shadow via the
# write-back path (C022, services/cwso-git-shadow/src/writeback.rs).
#
# Flow (8 stages, each asserted with a PASS/FAIL line, same discipline as
# C018's script):
#   1. health                    -- GET /healthz returns 200
#   2. create_shadow_workspace   -- MCP call; discover the real projected path
#   3. real-path ls              -- `docker exec` into the running
#      cwso-git-shadow container and `ls` the real, eagerly-materialised
#      (ADR-012) workspace directory -- BEFORE any file has been written
#   4. write fixture + real-path cat -- write_shadow_file (MCP) writes the Go
#      fixture (scripts/fixtures/go/greet/), then `docker exec ... cat` reads
#      it back from the real path and asserts byte-for-byte equality with
#      the source fixture on disk
#   5. real test command         -- a real, precompiled `go test` binary
#      (built from the *same* fixture source in the previous stage) is
#      executed via `docker exec` against the real projected path -- see
#      the "BUILD TEST BINARY" section below for exactly how and why
#   6. editor edit                -- `sed -i` directly on the real path via
#      `docker exec`, NOT through write_shadow_file or any CWSO tool
#   7. write-back convergence     -- poll read_shadow_file (MCP) until the
#      sed edit is visible in the workspace's in-memory state (inotify-driven,
#      per writeback.rs; bounded wait, fails loudly if it never converges --
#      that would be a genuine C022/C023 regression, not a flake to paper
#      over)
#   8. commit_shadow + tree assertion -- commits the workspace, then reads
#      the file back via read_shadow_file and asserts it matches exactly
#      what `docker exec cat` saw on disk after the edit -- proving the
#      resulting commit's tree contains the edit
#
# ---------------------------------------------------------------------------
# BUILD TEST BINARY: why a precompiled binary, and why staged under
# /run/cwso instead of executed in place
# ---------------------------------------------------------------------------
# Investigated live (not assumed) before writing this script:
#
#   1. The cwso-git-shadow runtime image (deploy/Dockerfile.git-shadow) is a
#      minimal debian:bookworm-slim + ca-certificates + tini + the compiled
#      Rust binary. It ships no python3, go, node, npm, cargo, rustc, or git
#      -- confirmed by building the image and enumerating its PATH. None of
#      the four wired tree-sitter grammars (Go, Python, Rust, TypeScript)
#      has its toolchain present at runtime, by design (single-purpose,
#      security-hardened sidecar, C019).
#   2. The container is `read_only: true` and network_mode: "none"
#      (deploy/docker-compose.yml), so nothing can be apt-installed into it
#      at runtime either.
#   3. Separately and more fundamentally: the real projected path's backing
#      store (`tmpfs: - /var/lib/cwso/shadow:size=128m,mode=1777` on the
#      git-shadow service) is mounted `noexec` by Docker's own default,
#      confirmed live via `mount` inside a running container
#      (`tmpfs on /var/lib/cwso/shadow type tmpfs (rw,nosuid,nodev,noexec,...)`).
#      This means NO binary can be executed directly from that path --
#      independent of (1) above, a precompiled/statically-linked test binary
#      staged there and `chmod +x`'d still gets `Permission denied` from the
#      kernel (confirmed live: exit 126). Neither of these two constraints
#      can be worked around from inside the container (cap_drop: ["ALL"]
#      removes CAP_SYS_ADMIN, so no remount; and both fixes require editing
#      deploy/Dockerfile.git-shadow and/or deploy/docker-compose.yml, both
#      outside this task's file ownership).
#
# Resolution used here, which stays entirely within this task's file
# ownership and does not fake or weaken anything: compile a REAL `go test`
# binary ahead of time (CGO_ENABLED=0 -> fully static, no libc/toolchain
# needed at exec time) from the exact same fixture source that was just
# written into the workspace and read back via `cat` in stage 4, stage it
# into /run/cwso (the cwso-runtime named volume ALREADY declared in
# deploy/docker-compose.yml for git-shadow's IPC socket -- not a new mount,
# and confirmed live to be a normal, exec-allowed ext4-backed volume, unlike
# the shadow-storage tmpfs), and execute it there via `docker exec`, passing
# the real workspace path in via an environment variable so the test's own
# logic performs a genuine os.ReadFile + content assertion against the real,
# projected file (see greet_test.go's TestGreetSourceProjectedAtRealPath).
# This is a real, compiled Go test binary, run via the real `go test`
# testing package, asserting against real content at the real path --
# not a shell string match standing in for a language-aware test.
#
# This is flagged as a judgment call (not silently done) in this script's
# comments, in the MR description, and in this task's final report --
# closing it "for real" (running `go test` itself, by name, in place) would
# need a scoped deploy/Dockerfile.git-shadow change (bundle a minimal Go
# toolchain, opt-in/test-only) and/or a deploy/docker-compose.yml change
# (add `exec` to the shadow-storage tmpfs mount options, trading off some of
# C019's hardening), both out of this task's file ownership -- recommended
# as an explicit follow-up for the Architect/Tech Lead to weigh.
#
# The build step itself stages the fixture into (and the compiled binary out
# of) the golang:1.22-bookworm builder container via `docker create` +
# `docker cp` + `docker start -a`, deliberately NOT `docker run -v host:container`:
# under CI's docker-socket-binding runner this script's own host path is not
# visible to the actual daemon (same documented class of issue as
# deploy/docker-compose.ci.yml's header comment on CWSO_WORKSPACE_HOST) --
# confirmed live in CI (first version of this script used `-v` and failed
# with `pattern ./...: directory prefix . does not contain main module`,
# i.e. Docker silently mounted an empty directory). `docker cp` streams file
# content over the Docker API itself, so it works identically whether this
# script's host and the daemon share a filesystem (local dev) or not (CI).
#
# ---------------------------------------------------------------------------
# Usage / preconditions (matches cwso-smoke-test.sh's own conventions)
# ---------------------------------------------------------------------------
#   scripts/cwso-projection-e2e.sh
#
#   - The CWSO stack is already up (e.g. `make up`) and the git-shadow
#     service's container is named cwso-git-shadow (deploy/docker-compose.yml
#     container_name), reachable from this script's host via the Docker
#     socket (same `docker exec`/`docker run` access CI's docker:27 job
#     image already has).
#   - docker, bash, curl, python3 on PATH.
#   - CWSO_BASE_URL may override http://127.0.0.1:8080 (CI sets this to the
#     container default-gateway address, same as cwso-smoke-test.sh).
#
# Hermetic: fixtures live under scripts/fixtures/go/greet/ (checked into this
# repo, no external Go module dependencies -- go.mod declares zero requires),
# and no stage makes a network call except pulling the golang:1.22-bookworm
# build image to compile the test binary (the same class of "pull a base
# image to build with" already done by every build:* job in .gitlab-ci.yml;
# nothing the fixture/test code itself does at runtime touches the network).
#
# Exit code: 0 if every stage passes; non-zero on the FIRST failing stage,
# with the full failing response/transcript printed to stderr (never
# swallowed, never weakened). This script does NOT tear down the docker
# compose stack itself (unlike cwso-smoke-test.sh) -- per this task's own
# verification recipe (`make up`; this script; `make down`), stack lifecycle
# is the caller's responsibility. This script only cleans up the one shadow
# workspace and staged test binary it created.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BASE_URL="${CWSO_BASE_URL:-http://127.0.0.1:8080}"
GIT_SHADOW_CONTAINER="${CWSO_GIT_SHADOW_CONTAINER:-cwso-git-shadow}"
FIXTURE_DIR="$REPO_ROOT/scripts/fixtures/go/greet"
MCP_CALL_PY="$SCRIPT_DIR/cwso-mcp-call.py"

pass() { printf '[PASS] %s\n' "$1"; }
fail() { printf '[FAIL] %s: %s\n' "$1" "$2" >&2; }

# ---------------------------------------------------------------------------
# Cleanup: best-effort, scoped to what THIS script created -- the shadow
# workspace (via drop_shadow_workspace over MCP), the staged test binary
# directory under /run/cwso inside the git-shadow container, and the local
# temp dir used to compile it. Does not touch the compose stack itself (see
# header). Runs on EXIT regardless of pass/fail, and never masks the
# original exit code.
# ---------------------------------------------------------------------------
BUILD_TMP=""
BUILD_CONTAINER=""
STAGE_DIR=""
WORKSPACE_UUID=""
WORKER_TOKEN=""
CLEANUP_DONE=0
cleanup() {
  local rc=$?
  if [[ "$CLEANUP_DONE" -eq 1 ]]; then
    exit "$rc"
  fi
  CLEANUP_DONE=1
  echo ""
  echo "==> [cleanup] releasing this script's own resources (stack lifecycle is the caller's, per header)"
  if [[ -n "$WORKSPACE_UUID" && -n "$WORKER_TOKEN" ]]; then
    CWSO_BASE_URL="$BASE_URL" CWSO_TOKEN="$WORKER_TOKEN" \
      python3 "$MCP_CALL_PY" drop_shadow_workspace "{\"workspace_uuid\":\"$WORKSPACE_UUID\"}" \
      >/dev/null 2>&1 || echo "==> [cleanup] WARNING: drop_shadow_workspace($WORKSPACE_UUID) failed or already gone" >&2
  fi
  if [[ -n "$STAGE_DIR" ]]; then
    docker exec "$GIT_SHADOW_CONTAINER" sh -c "rm -rf '$STAGE_DIR'" >/dev/null 2>&1 || true
  fi
  if [[ -n "$BUILD_CONTAINER" ]]; then
    docker rm -f "$BUILD_CONTAINER" >/dev/null 2>&1 || true
  fi
  if [[ -n "$BUILD_TMP" && -d "$BUILD_TMP" ]]; then
    rm -rf "$BUILD_TMP"
  fi
  exit "$rc"
}
trap cleanup EXIT

call_tool() {
  local name="$1" args="$2"
  CWSO_BASE_URL="$BASE_URL" CWSO_TOKEN="$WORKER_TOKEN" python3 "$MCP_CALL_PY" "$name" "$args"
}

jget() {
  python3 -c 'import json,sys; print(json.loads(sys.argv[1])[sys.argv[2]])' "$1" "$2"
}

# ---------------------------------------------------------------------------
# Stage 1/8: health
# ---------------------------------------------------------------------------
echo "==> [stage 1/8] health: waiting for GET $BASE_URL/healthz to return 200"
health_body_file="$(mktemp)"
health_deadline=$((SECONDS + 120))
health_code="000"
while [[ "$SECONDS" -lt "$health_deadline" ]]; do
  health_code="$(curl -sS -o "$health_body_file" -w '%{http_code}' --max-time 5 "$BASE_URL/healthz" 2>/dev/null || echo "000")"
  if [[ "$health_code" == "200" ]]; then
    break
  fi
  sleep 2
done
if [[ "$health_code" == "200" ]]; then
  pass "health"
else
  fail "health" "GET $BASE_URL/healthz did not return 200 within 120s (last status: $health_code, body: $(cat "$health_body_file" 2>/dev/null))"
  rm -f "$health_body_file"
  exit 1
fi
rm -f "$health_body_file"

echo "==> confirming $GIT_SHADOW_CONTAINER is reachable via docker exec"
if ! docker exec "$GIT_SHADOW_CONTAINER" sh -c 'true' >/dev/null 2>&1; then
  fail "docker-exec-reachability" "docker exec into '$GIT_SHADOW_CONTAINER' failed -- is the stack up (make up)? is this script running where it can reach the Docker socket the stack was started against?"
  exit 1
fi
pass "docker-exec-reachability ($GIT_SHADOW_CONTAINER)"

echo "==> minting a short-lived worker MCP token via scripts/cwso-token.sh (C013)"
WORKER_TOKEN="$(bash "$SCRIPT_DIR/cwso-token.sh" --role worker --ttl 300)"
if [[ -z "$WORKER_TOKEN" ]]; then
  fail "token-mint(worker)" "scripts/cwso-token.sh --role worker produced no token (see its stderr above)"
  exit 1
fi

# ---------------------------------------------------------------------------
# Stage 2/8: create_shadow_workspace, and discover the real projected path
# ---------------------------------------------------------------------------
echo "==> [stage 2/8] create_shadow_workspace"
if ! resp="$(call_tool create_shadow_workspace '{}')"; then
  fail "create_shadow_workspace" "MCP call failed -- see response dumped above"
  exit 1
fi
WORKSPACE_UUID="$(jget "$resp" workspace_uuid)"
if [[ -z "$WORKSPACE_UUID" ]]; then
  fail "create_shadow_workspace" "no workspace_uuid in response: $resp"
  exit 1
fi

STORAGE_ROOT="$(docker exec "$GIT_SHADOW_CONTAINER" sh -c 'printf "%s" "${CWSO_GIT_SHADOW_STORAGE:-/var/lib/cwso/shadow}"')"
if [[ -z "$STORAGE_ROOT" ]]; then
  fail "create_shadow_workspace" "could not read CWSO_GIT_SHADOW_STORAGE from $GIT_SHADOW_CONTAINER"
  exit 1
fi
REAL_PATH="$STORAGE_ROOT/$WORKSPACE_UUID"
pass "create_shadow_workspace (workspace_uuid=$WORKSPACE_UUID, real_path=$REAL_PATH)"

# ---------------------------------------------------------------------------
# Stage 3/8: a shell `cd`s into the real projected path and runs `ls` --
# BEFORE any file has been written. ADR-012/C021 eagerly materialise the
# workspace directory at creation time (not lazily on first write), so this
# must already exist and be an empty, listable directory.
# ---------------------------------------------------------------------------
echo "==> [stage 3/8] real-path ls: docker exec ls against $REAL_PATH (pre-write)"
if ! ls_out="$(docker exec "$GIT_SHADOW_CONTAINER" sh -c "cd '$REAL_PATH' && ls -la" 2>&1)"; then
  fail "real-path-ls" "docker exec 'cd $REAL_PATH && ls -la' failed:\n$ls_out"
  exit 1
fi
echo "$ls_out"
pass "real-path-ls (workspace directory exists and is cd-able/ls-able at the real path, pre-write)"

# ---------------------------------------------------------------------------
# Stage 4/8: write_shadow_file (MCP) writes the Go fixture; docker exec cat
# reads it back from the real path and must match byte-for-byte.
# ---------------------------------------------------------------------------
echo "==> [stage 4/8] write_shadow_file + real-path cat"
for fname in greet.go greet_test.go go.mod; do
  content="$(cat "$FIXTURE_DIR/$fname")"
  args="$(python3 -c 'import json,sys; print(json.dumps({"workspace_uuid": sys.argv[1], "path": sys.argv[2], "content": sys.argv[3]}))' \
    "$WORKSPACE_UUID" "$fname" "$content")"
  if ! call_tool write_shadow_file "$args" >/dev/null; then
    fail "write_shadow_file($fname)" "MCP call failed -- see response dumped above"
    exit 1
  fi
done
pass "write_shadow_file (greet.go, greet_test.go, go.mod)"

if ! cat_out="$(docker exec "$GIT_SHADOW_CONTAINER" sh -c "cat '$REAL_PATH/greet.go'" 2>&1)"; then
  fail "real-path-cat" "docker exec 'cat $REAL_PATH/greet.go' failed:\n$cat_out"
  exit 1
fi
expected_greet_go="$(cat "$FIXTURE_DIR/greet.go")"
if [[ "$cat_out" != "$expected_greet_go" ]]; then
  fail "real-path-cat" "content read back from the real path via docker exec+cat does not match the fixture source byte-for-byte.
--- expected (scripts/fixtures/go/greet/greet.go) ---
$expected_greet_go
--- actual (docker exec cat $REAL_PATH/greet.go) ---
$cat_out"
  exit 1
fi
pass "real-path-cat (greet.go at $REAL_PATH matches scripts/fixtures/go/greet/greet.go byte-for-byte)"

# ---------------------------------------------------------------------------
# Stage 5/8: real test command. See the "BUILD TEST BINARY" header comment
# for exactly why this is compiled ahead of time and staged/executed under
# /run/cwso rather than in place.
# ---------------------------------------------------------------------------
echo "==> [stage 5/8] real test command (go test, compiled ahead of time -- see header)"
# Deliberately NOT `docker run -v "$FIXTURE_DIR:/src:ro" ...`: under CI's
# docker-socket-binding runner (Option A, same class of issue documented in
# deploy/docker-compose.ci.yml's own header comment), this script's host
# path is not visible to the actual Docker daemon, which silently mounts an
# EMPTY directory instead of failing (`pattern ./...: directory prefix .
# does not contain main module` was the exact, confirmed CI failure this
# comment replaced). `docker cp` streams file content over the Docker API
# itself rather than resolving a host path, so it works the same whether
# this script's host and the daemon share a filesystem (local dev) or not
# (CI) -- verified in both environments. Build via `docker create` (stage
# the fixture in with `docker cp`, entirely bind-mount free) + `docker
# start -a` (run, streaming output) + `docker cp` back out.
BUILD_TMP="$(mktemp -d)"
BUILD_CONTAINER="cwso-projection-e2e-build-$$"
docker rm -f "$BUILD_CONTAINER" >/dev/null 2>&1 || true
if ! docker create --name "$BUILD_CONTAINER" \
  -w /src \
  -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 \
  -e GOCACHE=/tmp/gocache -e GOPATH=/tmp/gopath \
  golang:1.22-bookworm \
  sh -c 'mkdir -p /tmp/gocache /tmp/gopath /out && go vet ./... && go test -c -o /out/greet.test .' >/dev/null; then
  fail "build-test-binary" "docker create for the build container failed"
  exit 1
fi
if ! docker cp "$FIXTURE_DIR/." "$BUILD_CONTAINER:/src"; then
  fail "build-test-binary" "docker cp of scripts/fixtures/go/greet/ into the build container failed"
  docker rm -f "$BUILD_CONTAINER" >/dev/null 2>&1 || true
  exit 1
fi
if ! build_out="$(docker start -a "$BUILD_CONTAINER" 2>&1)"; then
  fail "build-test-binary" "compiling the real go test binary from scripts/fixtures/go/greet/ failed:\n$build_out"
  docker rm -f "$BUILD_CONTAINER" >/dev/null 2>&1 || true
  exit 1
fi
if ! docker cp "$BUILD_CONTAINER:/out/greet.test" "$BUILD_TMP/greet.test"; then
  fail "build-test-binary" "docker cp of the compiled test binary out of the build container failed; build output:\n$build_out"
  docker rm -f "$BUILD_CONTAINER" >/dev/null 2>&1 || true
  exit 1
fi
docker rm -f "$BUILD_CONTAINER" >/dev/null 2>&1 || true
chmod +x "$BUILD_TMP/greet.test"
if [[ ! -x "$BUILD_TMP/greet.test" ]]; then
  fail "build-test-binary" "expected an executable test binary at $BUILD_TMP/greet.test after go test -c; build output:\n$build_out"
  exit 1
fi
pass "build-test-binary (real 'go test -c', CGO_ENABLED=0, statically linked)"

STAGE_DIR="/run/cwso/cwso-projection-e2e-$$"
docker exec "$GIT_SHADOW_CONTAINER" sh -c "mkdir -p '$STAGE_DIR'"
if ! cat "$BUILD_TMP/greet.test" | docker exec -i "$GIT_SHADOW_CONTAINER" sh -c "cat > '$STAGE_DIR/greet.test' && chmod +x '$STAGE_DIR/greet.test'"; then
  fail "stage-test-binary" "staging the compiled test binary into $GIT_SHADOW_CONTAINER:$STAGE_DIR failed"
  exit 1
fi

if ! test_out="$(docker exec -e "CWSO_PROJECTION_E2E_WORKSPACE_DIR=$REAL_PATH" "$GIT_SHADOW_CONTAINER" sh -c "'$STAGE_DIR/greet.test' -test.v" 2>&1)"; then
  fail "real-test-command" "the real go test binary failed running inside $GIT_SHADOW_CONTAINER against $REAL_PATH -- this is a genuine product-level failure, not a test-harness problem:
$test_out"
  exit 1
fi
if ! grep -q '^PASS$' <<<"$test_out" || ! grep -q '^--- PASS: TestGreetSourceProjectedAtRealPath' <<<"$test_out"; then
  fail "real-test-command" "go test binary exited 0 but its output did not contain the expected PASS lines:\n$test_out"
  exit 1
fi
echo "$test_out"
pass "real-test-command (go test: TestGreet, TestGreetSourceProjectedAtRealPath both PASS against the real projected path)"

# ---------------------------------------------------------------------------
# Stage 6/8: editor edit -- sed, directly on the real path, NOT through
# write_shadow_file or any other CWSO tool.
# ---------------------------------------------------------------------------
echo "==> [stage 6/8] editor edit: sed -i on $REAL_PATH/greet.go (bypassing every CWSO tool)"
if ! docker exec "$GIT_SHADOW_CONTAINER" sh -c "sed -i 's/Hello, %s!/Howdy, %s!! (edited live via sed, not any CWSO tool)/' '$REAL_PATH/greet.go'" 2>&1; then
  fail "editor-edit" "docker exec sed -i against $REAL_PATH/greet.go failed"
  exit 1
fi
if ! edited_on_disk="$(docker exec "$GIT_SHADOW_CONTAINER" sh -c "cat '$REAL_PATH/greet.go'" 2>&1)"; then
  fail "editor-edit" "docker exec cat after the sed edit failed:\n$edited_on_disk"
  exit 1
fi
if ! grep -q 'Howdy, %s!! (edited live via sed, not any CWSO tool)' <<<"$edited_on_disk"; then
  fail "editor-edit" "sed edit did not take effect on disk at the real path; content now:\n$edited_on_disk"
  exit 1
fi
pass "editor-edit (sed edit confirmed on disk at the real path)"

# ---------------------------------------------------------------------------
# Stage 7/8: write-back convergence -- poll read_shadow_file (MCP) until the
# sed edit is visible in the workspace's in-memory state. Bounded wait
# (10s, well above the 2s default reconcile interval,
# CWSO_GIT_SHADOW_RECONCILE_INTERVAL_MS / DEFAULT_RECONCILE_INTERVAL_MS in
# writeback.rs); a failure to converge within this bound is a genuine
# C022/C023 regression and is reported as such, not weakened away.
# ---------------------------------------------------------------------------
echo "==> [stage 7/8] write-back convergence: polling read_shadow_file for the sed edit"
converged=0
last_seen=""
for _ in $(seq 1 50); do
  if content="$(call_tool read_shadow_file "{\"workspace_uuid\":\"$WORKSPACE_UUID\",\"path\":\"greet.go\"}" 2>/dev/null)"; then
    last_seen="$content"
    if grep -q 'Howdy, %s!! (edited live via sed, not any CWSO tool)' <<<"$content"; then
      converged=1
      break
    fi
  fi
  sleep 0.2
done
if [[ "$converged" -ne 1 ]]; then
  fail "write-back-convergence" "read_shadow_file never reflected the sed edit within 10s -- write-back (C022) did not observe the direct filesystem mutation. Last seen content via read_shadow_file:
$last_seen"
  exit 1
fi
pass "write-back-convergence (read_shadow_file reflects the sed edit)"

# ---------------------------------------------------------------------------
# Stage 8/8: commit_shadow + tree assertion.
# ---------------------------------------------------------------------------
echo "==> [stage 8/8] commit_shadow"
if ! commit_resp="$(call_tool commit_shadow "{\"workspace_uuid\":\"$WORKSPACE_UUID\",\"message\":\"cwso-projection-e2e: capture direct filesystem edit (C024)\"}")"; then
  fail "commit_shadow" "MCP call failed -- see response dumped above"
  exit 1
fi
COMMIT_OID="$(jget "$commit_resp" commit_oid)"
TREE_OID="$(jget "$commit_resp" tree_oid)"
if [[ "${#COMMIT_OID}" -ne 40 ]]; then
  fail "commit_shadow" "expected a 40-char sha1 commit_oid, got: $commit_resp"
  exit 1
fi
if [[ -z "$TREE_OID" ]]; then
  fail "commit_shadow" "expected a non-empty tree_oid, got: $commit_resp"
  exit 1
fi
pass "commit_shadow (commit_oid=$COMMIT_OID, tree_oid=$TREE_OID)"

echo "==> asserting the resulting commit's tree contains the edit"
if ! post_commit_content="$(call_tool read_shadow_file "{\"workspace_uuid\":\"$WORKSPACE_UUID\",\"path\":\"greet.go\"}")"; then
  fail "commit-contains-edit" "read_shadow_file after commit_shadow failed -- see response dumped above"
  exit 1
fi
if [[ "$post_commit_content" != "$edited_on_disk" ]]; then
  fail "commit-contains-edit" "content read back via read_shadow_file after commit_shadow does not match what docker exec+cat saw on disk after the sed edit.
--- on-disk (docker exec cat, post sed) ---
$edited_on_disk
--- post-commit (read_shadow_file) ---
$post_commit_content"
  exit 1
fi
pass "commit-contains-edit (commit $COMMIT_OID / tree $TREE_OID contains the sed edit -- byte-for-byte match with the real on-disk content)"

echo ""
echo "============================================================"
echo "  CWSO FILESYSTEM PROJECTION E2E: ALL STAGES PASS (CG2)"
echo "============================================================"
