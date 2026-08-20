package server

// mcp_contract_snapshot_test.go implements task C034: a contract snapshot
// test that fails CI on ANY drift in CWSO's live MCP protocol surface —
// method set, notification set, error-code behavior, and tool schema
// shapes — generated from the post-C032 surface (this repo's `develop` HEAD
// already includes C032's ADR-013 conformance execution: the
// `internal/mcp/protocol.go` POC-DEBT marker is gone, `ErrUnauthorized` was
// removed, the ErrInvalidRequest misuse fix and the
// notifications/resources/list_changed capability-mismatch fix both landed —
// see mcp_conformance_test.go, which asserts spec *behavior* row-by-row
// against docs/artifacts/mcp-gap-analysis-v1.md / docs/decisions/ADR-013-mcp-protocol-path.md).
//
// This file is deliberately NOT a re-implementation of mcp_conformance_test.go's
// per-row spec-conformance assertions. Its job is narrower and blunter: capture
// the *entire* live surface (tool list + schemas, dispatch-table method
// outcomes, notification never-emitted set, and every JSON-RPC error-code
// constant/trigger-scenario) into one golden JSON fixture
// (testdata/mcp_contract_snapshot_v1.json) and fail loudly if the live
// surface ever stops matching it byte-for-byte — including drift that no
// individual conformance assertion happens to cover. The two suites are
// complementary: mcp_conformance_test.go proves the surface is *correct*
// (spec-shaped); this file proves it hasn't *silently changed* since the
// fixture was last deliberately regenerated.
//
// Server configuration used for the snapshot ("core" config): ShadowSocket +
// MergeEngineSocket set (dummy, unconnected paths — tool registration is
// lazy and does not dial at construction time), everything else at zero
// value (no ASTSpikeResourcesEnabled, no SparseAgentsEnabled, no
// HHDHardwareAwareDispatch). This yields exactly 11 registered tools:
// commit_shadow, create_shadow_workspace, dispatch_concurrent_jobs,
// drop_shadow_workspace, list_dir, merge_concurrent_results, query_ast,
// read_file_sync, read_shadow_file, write_file_sync, write_shadow_file.
//
// This is a deliberate choice, not an oversight: it is exactly the tool set
// asserted by the sibling em-age/emage.code repo's own snapshot test
// (tests/functional/test_cwso_mcp_contract_snapshot.py, task T201, fixture
// tests/fixtures/cwso/tools_list_snapshot_v1.json) — see the C034 MR
// description for the full cross-repo alignment note, including a schemas/
// baseline-reliability finding this test deliberately routes around (see
// "Schema source of truth" below).
//
// Schema source of truth: the C034 task brief instructed sourcing tool
// schemas from schemas/*.json, contingent on task T198 (schemas/*.json vs.
// real Go InputSchema() drift fix) having landed on develop. It has not:
// docs/tasks/active-tasks.md lists T198 as `status: pending` as of this
// writing, and schemas/create_shadow_workspace.json / schemas/query_ast.json
// still contain the exact drift T198 was filed to fix (verified directly
// against shadow_tools.go's InputSchema() funcs below, and independently
// against docs/tasks/task-T198.md's own drift table). Trusting schemas/*.json
// as this snapshot's baseline would therefore have pinned a *known-wrong*
// tool contract into CI. This test instead sources tool schemas exclusively
// from the live tools/list response — i.e. the real, running
// tools.Tool.InputSchema() implementations — which is both the actual wire
// contract CWSO serves today and the only baseline consistent with ADR-013's
// own philosophy ("assert spec-shaped behavior for everything Implemented",
// not a stale doc). schemas/*.json is not read, modified, or otherwise
// referenced by this test (schemas/* is outside this task's file ownership
// per its brief either way). See the C034 MR description for the full
// finding.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/transport"
)

// snapshotFixturePath is the golden fixture this test compares the live
// surface against. Regenerate deliberately via `make mcp-contract-snapshot-update`
// (never automatically, and never in CI — see .gitlab-ci.yml's
// go:mcp-contract-snapshot job, which runs plain `go test`, no -update flag).
const snapshotFixturePath = "testdata/mcp_contract_snapshot_v1.json"

// updateSnapshot is the deliberate, explicit regeneration switch. It is never
// set by any CI job — only a human (or an agent under explicit instruction)
// running `go test ./internal/server/... -run TestMCPContractSnapshot -update-snapshot`
// (or `make mcp-contract-snapshot-update`) can regenerate the fixture. Drift
// must fail loudly, not self-heal.
var updateSnapshot = flag.Bool("update-snapshot", false, "regenerate testdata/mcp_contract_snapshot_v1.json instead of asserting against it (manual use only; never set in CI)")

// --- Snapshot data model ---
//
// Every field here is either (a) a name/identifier that only changes when
// the surface deliberately changes (tool names, method names, error-code
// constant names/values), or (b) a *structural shape* derived via shapeOf,
// which strips actual values (so timestamps, protocol/server version
// strings, and other volatile data never enter the fixture — only the set
// of result keys and their JSON types does). See shapeOf below.

type toolSnapshot struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type methodSnapshot struct {
	// Category is "success" (a JSON-RPC result was returned) or "error" (a
	// JSON-RPC error envelope was returned).
	Category string `json:"category"`
	// ResultShape is set only for Category=="success": the structural shape
	// (see shapeOf) of the result object.
	ResultShape any `json:"resultShape,omitempty"`
	// ErrorCode is set only for Category=="error".
	ErrorCode int `json:"errorCode,omitempty"`
}

type notificationSnapshot struct {
	// Recognized lists inbound notification method names the dispatch
	// switch explicitly handles (accepted, no response).
	Recognized []string `json:"recognized"`
	// NeverEmitted lists spec-defined server->client notification method
	// names confirmed, by a live traffic sweep, to never be published on
	// the event bus in this configuration.
	NeverEmitted []string `json:"neverEmitted"`
}

type errorCodeEntry struct {
	Name string `json:"name"`
	Code int    `json:"code"`
}

type contractSnapshot struct {
	// Tools is the full, sorted-by-name tool inventory: name, description,
	// and the exact live InputSchema() shape for every registered tool.
	Tools []toolSnapshot `json:"tools"`
	// Methods maps a probed JSON-RPC method name (or, for the two synthetic
	// keys noted inline, a labeled tools/call variant) to its outcome
	// category/shape/error-code.
	Methods       map[string]methodSnapshot `json:"methods"`
	Notifications notificationSnapshot      `json:"notifications"`
	// ErrorCodes is every error-code constant defined in internal/mcp/protocol.go,
	// by name and numeric value (catches renumbering/removal/addition).
	ErrorCodes []errorCodeEntry `json:"errorCodes"`
	// ToolCallErrors captures the live-observed error code for representative
	// tools/call failure scenarios not otherwise exercised by the Methods map.
	ToolCallErrors map[string]int `json:"toolCallErrors"`
	// ParseErrors captures the live-observed error code for each
	// mcp.ParseRequest failure branch (the ADR-013 required fix: only
	// malformed JSON should map to ErrParse; the other two are
	// ErrInvalidRequest).
	ParseErrors map[string]int `json:"parseErrors"`
}

// shapeOf reduces a decoded JSON value to its structural shape: object keys
// (recursively shaped) for objects, a single sampled+shaped element for
// non-empty arrays (result arrays here are homogeneous), and a fixed type
// tag for scalars. Actual values (version strings, timestamps, IDs, etc.)
// are deliberately discarded so the snapshot can never fail on volatile
// data — only on additions, removals, or type changes to the shape itself.
func shapeOf(v any) any {
	switch x := v.(type) {
	case map[string]any:
		shape := make(map[string]any, len(x))
		for k, vv := range x {
			shape[k] = shapeOf(vv)
		}
		return shape
	case []any:
		if len(x) == 0 {
			return []any{}
		}
		return []any{shapeOf(x[0])}
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", x)
	}
}

// newCoreContractServer builds the "core" config server described in this
// file's header comment: shadow + merge tools enabled (dummy, unconnected
// sockets — safe, since tool construction is lazy), everything else at zero
// value.
func newCoreContractServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/hello.txt", []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Transport:         "stdio",
		LogLevel:          "error",
		Workspace:         dir,
		AllowedOrigins:    []string{"http://localhost"},
		ShadowSocket:      dir + "/shadow.sock",
		MergeEngineSocket: dir + "/merge-engine.sock",
	}
	s, err := New(cfg, logging.New("error"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// buildContractSnapshot exercises the live server and assembles the full
// contractSnapshot. This is the single source of truth both the assertion
// path and the -update-snapshot regeneration path call — there is no second,
// hand-maintained copy of "what the surface should look like".
func buildContractSnapshot(t *testing.T, s *Server) contractSnapshot {
	t.Helper()

	snap := contractSnapshot{
		Methods:        map[string]methodSnapshot{},
		ToolCallErrors: map[string]int{},
		ParseErrors:    map[string]int{},
	}

	// --- Tools (full literal fidelity: name, description, inputSchema) ---
	listEnv := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if listEnv["error"] != nil {
		t.Fatalf("tools/list failed: %v", listEnv)
	}
	listResult, _ := listEnv["result"].(map[string]any)
	toolsRaw, _ := listResult["tools"].([]any)
	for _, tr := range toolsRaw {
		tm, ok := tr.(map[string]any)
		if !ok {
			t.Fatalf("unexpected tool entry shape: %v", tr)
		}
		name, _ := tm["name"].(string)
		desc, _ := tm["description"].(string)
		schema, _ := tm["inputSchema"].(map[string]any)
		if name == "" {
			t.Fatalf("tool entry missing name: %v", tm)
		}
		snap.Tools = append(snap.Tools, toolSnapshot{Name: name, Description: desc, InputSchema: schema})
	}
	sort.Slice(snap.Tools, func(i, j int) bool { return snap.Tools[i].Name < snap.Tools[j].Name })

	// Cross-repo alignment sanity: tools/list must be identical regardless
	// of caller role (em-age/emage.code's T201 snapshot test asserts this
	// same tool list for both "orchestrator" and "worker" roles — role
	// gating happens at tools/call time, not tools/list time; see
	// tools.Registry.List(), which does not take a role parameter).
	for _, role := range []string{"orchestrator", "worker"} {
		roleEnv := callSess(t, s, &transport.Session{Role: role}, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		roleResult, _ := roleEnv["result"].(map[string]any)
		roleTools, _ := roleResult["tools"].([]any)
		if len(roleTools) != len(snap.Tools) {
			t.Fatalf("tools/list for role %q returned %d tools, expected %d (tools/list must not vary by role)", role, len(roleTools), len(snap.Tools))
		}
	}

	// --- Methods: dispatch-table probes (excludes tools/list, handled above,
	// and tools/call, handled below with dedicated success/tool-error cases) ---
	successProbes := []struct {
		key string
		raw string
	}{
		{"ping", `{"jsonrpc":"2.0","id":1,"method":"ping"}`},
		{"initialize", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"contract-snapshot","version":"0"}}}`},
	}
	for _, p := range successProbes {
		env := call(t, s, p.raw)
		if env["error"] != nil {
			t.Fatalf("expected %q to succeed, got: %v", p.key, env)
		}
		result, _ := env["result"].(map[string]any)
		snap.Methods[p.key] = methodSnapshot{Category: "success", ResultShape: shapeOf(result)}
	}

	// Missing methods (gap table §1: 6 rows) + feature-flagged resources/*
	// methods, which are also ErrMethodNotFound in this core config (no
	// spikeSubs/sparseAgents configured — see resourcesEnabled()) + one
	// non-spec canary method name to pin the default-branch behavior itself.
	errorProbes := map[string]string{
		"prompts/list":              `{"jsonrpc":"2.0","id":1,"method":"prompts/list"}`,
		"prompts/get":               `{"jsonrpc":"2.0","id":1,"method":"prompts/get"}`,
		"logging/setLevel":          `{"jsonrpc":"2.0","id":1,"method":"logging/setLevel"}`,
		"completion/complete":       `{"jsonrpc":"2.0","id":1,"method":"completion/complete"}`,
		"sampling/createMessage":    `{"jsonrpc":"2.0","id":1,"method":"sampling/createMessage"}`,
		"roots/list":                `{"jsonrpc":"2.0","id":1,"method":"roots/list"}`,
		"resources/list":            `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`,
		"resources/templates/list":  `{"jsonrpc":"2.0","id":1,"method":"resources/templates/list"}`,
		"resources/read":            `{"jsonrpc":"2.0","id":1,"method":"resources/read"}`,
		"resources/subscribe":       `{"jsonrpc":"2.0","id":1,"method":"resources/subscribe"}`,
		"resources/unsubscribe":     `{"jsonrpc":"2.0","id":1,"method":"resources/unsubscribe"}`,
		"x-snapshot-canary/unknown": `{"jsonrpc":"2.0","id":1,"method":"x-snapshot-canary/unknown"}`,
	}
	for key, raw := range errorProbes {
		env := call(t, s, raw)
		snap.Methods[key] = methodSnapshot{Category: "error", ErrorCode: errorCode(t, env)}
	}

	// tools/call: success path + tool-level (isError:true) path. Both are
	// JSON-RPC-level successes per spec's CallToolResult guidance, so both
	// are recorded under Category "success" with distinct labeled keys.
	okEnv := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file_sync","arguments":{"path":"hello.txt"}}}`)
	if okEnv["error"] != nil {
		t.Fatalf("expected tools/call success, got: %v", okEnv)
	}
	okResult, _ := okEnv["result"].(map[string]any)
	snap.Methods["tools/call"] = methodSnapshot{Category: "success", ResultShape: shapeOf(okResult)}

	toolErrEnv := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file_sync","arguments":{"path":"does-not-exist.txt"}}}`)
	if toolErrEnv["error"] != nil {
		t.Fatalf("expected tool-level failure via CallToolResult.isError, not a protocol error, got: %v", toolErrEnv)
	}
	toolErrResult, _ := toolErrEnv["result"].(map[string]any)
	if toolErrResult["isError"] != true {
		t.Fatalf("expected isError:true, got: %v", toolErrResult)
	}
	snap.Methods["tools/call#tool-level-error"] = methodSnapshot{Category: "success", ResultShape: shapeOf(toolErrResult)}

	// --- Notifications ---
	notifEnv, err := s.Handle(t.Context(), nil, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if err != nil {
		t.Fatalf("handle notifications/initialized: %v", err)
	}
	if notifEnv != nil {
		t.Fatalf("notifications/initialized must produce no response, got: %s", notifEnv)
	}
	snap.Notifications.Recognized = []string{"notifications/initialized"}

	forbidden := []string{
		"notifications/cancelled",
		"notifications/progress",
		"notifications/roots/list_changed",
		"notifications/message",
		"notifications/resources/updated",
		"notifications/resources/list_changed",
		"notifications/tools/list_changed",
		"notifications/prompts/list_changed",
	}
	sort.Strings(forbidden)
	snap.Notifications.NeverEmitted = forbidden
	assertNoneEmitted(t, s, forbidden)

	// --- Error codes: every constant defined in internal/mcp/protocol.go ---
	codes := []errorCodeEntry{
		{"ErrParse", mcp.ErrParse},
		{"ErrInvalidRequest", mcp.ErrInvalidRequest},
		{"ErrMethodNotFound", mcp.ErrMethodNotFound},
		{"ErrInvalidParams", mcp.ErrInvalidParams},
		{"ErrInternal", mcp.ErrInternal},
		{"ErrPermissionDenied", mcp.ErrPermissionDenied},
		{"ErrToolNotFound", mcp.ErrToolNotFound},
		{"ErrToolExecution", mcp.ErrToolExecution},
		{"ErrResourceNotFound", mcp.ErrResourceNotFound},
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i].Name < codes[j].Name })
	snap.ErrorCodes = codes

	// --- Representative tools/call error-trigger scenarios ---
	unknownToolEnv := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"does_not_exist","arguments":{}}}`)
	snap.ToolCallErrors["unknown_tool"] = errorCode(t, unknownToolEnv)

	missingNameEnv := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"arguments":{}}}`)
	snap.ToolCallErrors["missing_name"] = errorCode(t, missingNameEnv)

	roleForbiddenEnv := callSess(t, s, &transport.Session{Role: "orchestrator"},
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_file_sync","arguments":{"path":"x.txt","content":"oops"}}}`)
	snap.ToolCallErrors["role_forbidden"] = errorCode(t, roleForbiddenEnv)

	// --- ParseRequest failure-branch mapping (ADR-013 required fix) ---
	malformedEnv := call(t, s, `{`)
	snap.ParseErrors["malformed_json"] = errorCode(t, malformedEnv)

	wrongVersionEnv := call(t, s, `{"jsonrpc":"1.0","id":1,"method":"ping"}`)
	snap.ParseErrors["wrong_jsonrpc_version"] = errorCode(t, wrongVersionEnv)

	missingMethodEnv := call(t, s, `{"jsonrpc":"2.0","id":1}`)
	snap.ParseErrors["missing_method"] = errorCode(t, missingMethodEnv)

	return snap
}

// assertNoneEmitted drives a representative sweep of every implemented
// request path across a fresh bus subscription and fails if any forbidden
// notification method name is published within a short deadline. Mirrors
// mcp_conformance_test.go's TestConformancePlainMissingNotificationsNeverEmitted
// sweep, adapted to the core (no spikeSubs/sparseAgents) config this file uses.
func assertNoneEmitted(t *testing.T, s *Server, forbidden []string) {
	t.Helper()
	forbiddenSet := make(map[string]bool, len(forbidden))
	for _, f := range forbidden {
		forbiddenSet[f] = true
	}

	busSub := s.bus.Subscribe()
	defer busSub.Close()

	call(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)
	call(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	call(t, s, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file_sync","arguments":{"path":"hello.txt"}}}`)
	call(t, s, `{"jsonrpc":"2.0","id":4,"method":"resources/list"}`)
	if _, err := s.Handle(t.Context(), nil, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(150 * time.Millisecond)
	for {
		select {
		case msg, ok := <-busSub.Messages():
			if !ok {
				return
			}
			if forbiddenSet[msg.Topic] {
				t.Fatalf("spec notification %q must never be emitted, but it was published", msg.Topic)
			}
		case <-deadline:
			return
		}
	}
}

// TestMCPContractSnapshot is the C034 gate: build the live surface and
// compare it against testdata/mcp_contract_snapshot_v1.json. Run with
// -update-snapshot to deliberately regenerate the fixture (never in CI).
func TestMCPContractSnapshot(t *testing.T) {
	s := newCoreContractServer(t)
	live := buildContractSnapshot(t, s)

	liveJSON, err := json.MarshalIndent(live, "", "  ")
	if err != nil {
		t.Fatalf("marshal live snapshot: %v", err)
	}
	liveJSON = append(liveJSON, '\n')

	if *updateSnapshot {
		if err := os.WriteFile(snapshotFixturePath, liveJSON, 0o644); err != nil {
			t.Fatalf("write %s: %v", snapshotFixturePath, err)
		}
		t.Logf("regenerated %s (deliberate -update-snapshot run)", snapshotFixturePath)
		return
	}

	golden, err := os.ReadFile(snapshotFixturePath)
	if err != nil {
		t.Fatalf("read %s: %v (run `make mcp-contract-snapshot-update` to generate it)", snapshotFixturePath, err)
	}

	// Compare structurally (not by raw bytes) so key ordering differences
	// introduced by hand-editing the fixture don't cause spurious failures,
	// while still catching any real content drift.
	var goldenDecoded, liveDecoded any
	if err := json.Unmarshal(golden, &goldenDecoded); err != nil {
		t.Fatalf("unmarshal golden fixture %s: %v", snapshotFixturePath, err)
	}
	if err := json.Unmarshal(liveJSON, &liveDecoded); err != nil {
		t.Fatalf("unmarshal live snapshot: %v", err)
	}

	goldenCanon, _ := json.MarshalIndent(goldenDecoded, "", "  ")
	liveCanon, _ := json.MarshalIndent(liveDecoded, "", "  ")
	if string(goldenCanon) != string(liveCanon) {
		t.Fatalf(
			"MCP protocol surface has drifted from %s.\n"+
				"This means the live method/notification/error-code/tool-schema surface no\n"+
				"longer matches the committed contract snapshot. If this drift is INTENTIONAL,\n"+
				"regenerate the fixture deliberately with `make mcp-contract-snapshot-update`\n"+
				"(never automatically) and include the regenerated fixture in your MR for\n"+
				"review. If it is NOT intentional, this is a real protocol regression.\n\n"+
				"--- golden (%s) ---\n%s\n\n--- live (current code) ---\n%s\n",
			snapshotFixturePath, snapshotFixturePath, goldenCanon, liveCanon,
		)
	}
}
