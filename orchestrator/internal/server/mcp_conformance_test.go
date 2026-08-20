package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/transport"
)

// This file is the C032 conformance suite executing ADR-013
// (docs/decisions/ADR-013-mcp-protocol-path.md): "keep the hand-rolled MCP
// kernel, prove it." It closes the 35-row surface inventoried by
// docs/artifacts/mcp-gap-analysis-v1.md (task C030) — 16 methods, 9
// notifications, 10 error-code constants (now 9; ErrUnauthorized removed,
// see internal/mcp/protocol.go) — asserting spec-shaped request/response
// behavior for everything the gap table marks Implemented/Partial, and a
// correct, spec-shaped "not supported" error for everything it marks
// Missing. Each test below cites the gap-table row(s) it closes.
//
// Two required fixes ADR-013 flagged as required (not documentation-only)
// are verified here as well: the ErrInvalidRequest misuse fix (§3a) and the
// notifications/resources/list_changed capability/behavior mismatch fix
// (§2). Both are implemented in internal/mcp/protocol.go and
// internal/server/server.go respectively.
//
// Non-spec notifications (notifications/log, notifications/job-state) are
// out of this suite's scope by definition (ADR-013: "should be smoke-tested
// for stability", not conformance-tested against a spec row) and already
// have solid coverage in internal/transport/{http_test.go,telemetry_test.go,
// http_sse_telemetry_test.go} — not duplicated here.

// errorCode extracts and returns a JSON-RPC error envelope's numeric code,
// failing the test if the envelope has no error object.
func errorCode(t *testing.T, env map[string]any) int {
	t.Helper()
	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected a JSON-RPC error envelope, got: %v", env)
	}
	code, ok := errObj["code"].(float64)
	if !ok {
		t.Fatalf("expected numeric error.code, got: %v", errObj["code"])
	}
	return int(code)
}

// callSess is call (see spike_resources_test.go) with an explicit session,
// for role-based conformance scenarios.
func callSess(t *testing.T, s *Server, sess *transport.Session, raw string) map[string]any {
	t.Helper()
	out, err := s.Handle(context.Background(), sess, []byte(raw))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("unmarshal response %s: %v", out, err)
	}
	return env
}

// --- Methods §1 — Implemented (4 rows) ---

// TestConformancePingDeviatesFromSpecEmptyResult closes gap table §1 row
// "ping". Implemented, with a named, tested deviation: spec's PingRequest
// expects an EmptyResult ({}); CWSO returns {"pong":true}. ADR-013 requires
// this deviation be an explicit assertion, not silently normalized away.
func TestConformancePingDeviatesFromSpecEmptyResult(t *testing.T) {
	s, _ := newTestServer(t)
	env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	if env["error"] != nil {
		t.Fatalf("ping should succeed, got: %v", env)
	}
	result, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected a result object, got: %v", env)
	}
	if pong, ok := result["pong"].(bool); !ok || !pong {
		t.Fatalf(`expected result {"pong":true} (documented spec deviation), got: %v`, result)
	}
	if len(result) != 1 {
		t.Fatalf("expected ping's result to be exactly {\"pong\":true}, deviating from spec's empty {}; got extra fields: %v", result)
	}
}

// TestConformanceNotificationsInitializedLenientLifecycle closes gap table
// §1/§2 "notifications/initialized". Implemented (lenient): accepted and
// discarded, and — the tested leniency — no session state machine enforces
// that it precedes other requests.
func TestConformanceNotificationsInitializedLenientLifecycle(t *testing.T) {
	s, _ := newTestServer(t)
	out, err := s.Handle(context.Background(), nil, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if out != nil {
		t.Fatalf("notifications/initialized must produce no response, got: %s", out)
	}
	env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if env["error"] != nil {
		t.Fatalf("tools/list must remain callable even though initialized was never sent (documented, tested leniency), got: %v", env)
	}
}

// TestConformanceToolsListSpecShape closes gap table §1 "tools/list".
// Implemented: asserts the ToolsListResult shape (result.tools[].{name,
// description, inputSchema}).
func TestConformanceToolsListSpecShape(t *testing.T) {
	s, _ := newTestServer(t)
	env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if env["error"] != nil {
		t.Fatalf("tools/list should succeed, got: %v", env)
	}
	result, _ := env["result"].(map[string]any)
	toolsRaw, ok := result["tools"].([]any)
	if !ok || len(toolsRaw) == 0 {
		t.Fatalf("expected a non-empty result.tools array, got: %v", result)
	}
	first, ok := toolsRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected tool entries to be objects, got: %v", toolsRaw[0])
	}
	for _, field := range []string{"name", "description", "inputSchema"} {
		if _, ok := first[field]; !ok {
			t.Fatalf("expected Tool field %q per spec schema, got: %v", field, first)
		}
	}
}

// TestConformanceToolsCallSpecShape closes gap table §1 "tools/call".
// Implemented: asserts the CallToolResult shape on both the success path
// (result.content[] text blocks) and the tool-level-error path (isError:true
// inside CallToolResult, not a protocol-level JSON-RPC error — matching
// spec's CallToolResult guidance).
func TestConformanceToolsCallSpecShape(t *testing.T) {
	s, _ := newTestServer(t)

	env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file_sync","arguments":{"path":"hello.txt"}}}`)
	if env["error"] != nil {
		t.Fatalf("expected success, got: %v", env)
	}
	result, _ := env["result"].(map[string]any)
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected result.content array per CallToolResult, got: %v", result)
	}
	block, _ := content[0].(map[string]any)
	if block["type"] != "text" {
		t.Fatalf("expected a text content block, got: %v", block)
	}
	if isErr, present := result["isError"]; present && isErr == true {
		t.Fatalf("success result must not set isError:true, got: %v", result)
	}

	env = call(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"read_file_sync","arguments":{"path":"does-not-exist.txt"}}}`)
	if env["error"] != nil {
		t.Fatalf("tool-level failures must be reported via CallToolResult.isError, not a protocol error, got: %v", env)
	}
	result, _ = env["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError:true for a tool-level failure, got: %v", result)
	}
}

// --- Methods §1 — Partial (6 rows) ---

// TestConformanceInitializeSpecShapeAndVersionEchoGap closes gap table §1
// "initialize". Partial: the working path (InitializeResult shape) plus the
// documented, tested deviation — the server unconditionally echoes its own
// protocol version regardless of what the client requested (no
// negotiation/rejection path).
func TestConformanceInitializeSpecShapeAndVersionEchoGap(t *testing.T) {
	s, _ := newTestServer(t)

	env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"test","version":"0"}}}`)
	if env["error"] != nil {
		t.Fatalf("initialize should succeed, got: %v", env)
	}
	result, _ := env["result"].(map[string]any)
	for _, field := range []string{"protocolVersion", "capabilities", "serverInfo"} {
		if _, ok := result[field]; !ok {
			t.Fatalf("expected InitializeResult field %q, got: %v", field, result)
		}
	}

	env2 := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2099-01-01","clientInfo":{"name":"test","version":"0"}}}`)
	if env2["error"] != nil {
		t.Fatalf("initialize with an unsupported/future protocolVersion should still succeed (documented gap: no negotiation/rejection path), got: %v", env2)
	}
	result2, _ := env2["result"].(map[string]any)
	if result2["protocolVersion"] != mcp.SupportedProtocolVersion {
		t.Fatalf("expected the server to always echo its own supported version %q regardless of client request, got: %v", mcp.SupportedProtocolVersion, result2["protocolVersion"])
	}
}

// TestConformanceResourcesGatedMethodsReturnMethodNotFoundWhenDisabled closes
// gap table §1 "resources/list", "resources/templates/list",
// "resources/read", "resources/subscribe", "resources/unsubscribe": Partial
// (feature-flagged). When neither spikeSubs nor sparseAgents is configured,
// every one of these methods must return a spec-shaped ErrMethodNotFound
// (-32601), not a malformed response.
func TestConformanceResourcesGatedMethodsReturnMethodNotFoundWhenDisabled(t *testing.T) {
	s := newSpikeServer(t, false)
	methods := []string{"resources/list", "resources/templates/list", "resources/read", "resources/subscribe", "resources/unsubscribe"}
	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			params := ""
			if m == "resources/read" || m == "resources/subscribe" || m == "resources/unsubscribe" {
				params = `,"params":{"uri":"cwso://spikes/anything"}`
			}
			env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"`+m+`"`+params+`}`)
			if code := errorCode(t, env); code != mcp.ErrMethodNotFound {
				t.Fatalf("expected ErrMethodNotFound (%d) when resources disabled, got %d: %v", mcp.ErrMethodNotFound, code, env)
			}
		})
	}
}

// TestConformanceResourcesEnabledSpecShape closes the "happy path" half of
// gap table §1 resources/* rows when spikeSubs is configured: asserts
// ResourcesListResult, ResourceTemplatesListResult, and ResourceReadResult
// shapes.
func TestConformanceResourcesEnabledSpecShape(t *testing.T) {
	s := newSpikeServer(t, true)
	id := subscribe(t, s, `{"path":"pkg/*.go","semantic_threshold":"any"}`)

	list := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`)
	result, _ := list["result"].(map[string]any)
	resources, ok := result["resources"].([]any)
	if !ok || len(resources) == 0 {
		t.Fatalf("expected result.resources array, got: %v", result)
	}
	entry, _ := resources[0].(map[string]any)
	for _, field := range []string{"uri", "name"} {
		if _, ok := entry[field]; !ok {
			t.Fatalf("expected Resource field %q, got: %v", field, entry)
		}
	}

	tmpl := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"resources/templates/list"}`)
	tresult, _ := tmpl["result"].(map[string]any)
	templates, ok := tresult["resourceTemplates"].([]any)
	if !ok || len(templates) == 0 {
		t.Fatalf("expected result.resourceTemplates array, got: %v", tresult)
	}
	tentry, _ := templates[0].(map[string]any)
	for _, field := range []string{"uriTemplate", "name"} {
		if _, ok := tentry[field]; !ok {
			t.Fatalf("expected ResourceTemplate field %q, got: %v", field, tentry)
		}
	}

	uri := dispatch.SpikeResourcePrefix + id
	read := call(t, s, `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"`+uri+`"}}`)
	rresult, _ := read["result"].(map[string]any)
	contents, ok := rresult["contents"].([]any)
	if !ok || len(contents) == 0 {
		t.Fatalf("expected result.contents array per ResourceReadResult, got: %v", rresult)
	}
	centry, _ := contents[0].(map[string]any)
	for _, field := range []string{"uri", "text"} {
		if _, ok := centry[field]; !ok {
			t.Fatalf("expected ResourceContents field %q, got: %v", field, centry)
		}
	}
}

// TestConformanceResourcesSubscribeAcceptsButNeverPublishesUpdatedNotification
// closes gap table §1 "resources/subscribe" (Partial) and §2
// "notifications/resources/updated" (Missing). This is a documented,
// tested gap, not a required fix (unlike notifications/resources/list_changed
// below): the subscription is accepted, but notifications/resources/updated
// — the entire spec-defined point of subscribing — is never sent.
func TestConformanceResourcesSubscribeAcceptsButNeverPublishesUpdatedNotification(t *testing.T) {
	s := newSpikeServer(t, true)
	id := subscribe(t, s, `{"path":"pkg/*.go","semantic_threshold":"any"}`)
	uri := dispatch.SpikeResourcePrefix + id

	busSub := s.bus.Subscribe()
	defer busSub.Close()

	env := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"resources/subscribe","params":{"uri":"`+uri+`"}}`)
	if env["error"] != nil {
		t.Fatalf("resources/subscribe should be accepted (only the push mechanism is the documented gap), got: %v", env)
	}

	if err := s.publisher.Publish(dispatch.TopicASTSemanticSpike,
		dispatch.SemanticSpikeEvent{Workspace: "ws1", Path: "pkg/a.go", SpikeKind: string(dispatch.SpikeKindSignatureChange)}); err != nil {
		t.Fatal(err)
	}
	waitForBroker(t, s, dispatch.TopicASTSemanticSpike, 1)

	deadline := time.After(150 * time.Millisecond)
	for {
		select {
		case msg, ok := <-busSub.Messages():
			if !ok {
				return
			}
			if msg.Topic == "notifications/resources/updated" {
				t.Fatalf("expected notifications/resources/updated to never be published (documented gap), but it was")
			}
		case <-deadline:
			return
		}
	}
}

// TestConformanceResourcesUnsubscribeSpecShapeAndUnknownID closes gap table
// §1 "resources/unsubscribe" (Partial). resources/unsubscribe was previously
// only exercised incidentally, as a side effect inside
// TestConformancePlainMissingNotificationsNeverEmitted, which discards the
// call's own response and asserts nothing about resources/unsubscribe's
// response shape. This test asserts that shape directly: a successful
// unsubscribe of a real, active subscription returns a spec-shaped empty
// result, and — the actual gap — unsubscribing an id that was never
// subscribed (or was already removed) returns a correct, spec-shaped
// ErrResourceNotFound JSON-RPC error rather than a false success or a
// malformed error.
func TestConformanceResourcesUnsubscribeSpecShapeAndUnknownID(t *testing.T) {
	s := newSpikeServer(t, true)
	id := subscribe(t, s, `{"path":"pkg/*.go","semantic_threshold":"any"}`)
	uri := dispatch.SpikeResourcePrefix + id

	env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"resources/unsubscribe","params":{"uri":"`+uri+`"}}`)
	if env["error"] != nil {
		t.Fatalf("unsubscribing an active subscription should succeed, got: %v", env)
	}
	if _, ok := env["result"].(map[string]any); !ok {
		t.Fatalf("expected a result object per the empty-object ResourceURIParams response, got: %v", env)
	}

	unknownURI := dispatch.SpikeResourcePrefix + "never-subscribed-id"
	env = call(t, s, `{"jsonrpc":"2.0","id":2,"method":"resources/unsubscribe","params":{"uri":"`+unknownURI+`"}}`)
	if env["result"] != nil {
		t.Fatalf("unsubscribing an unknown id must not report success, got: %v", env)
	}
	if code := errorCode(t, env); code != mcp.ErrResourceNotFound {
		t.Fatalf("expected ErrResourceNotFound (%d) for unknown subscription id, got %d: %v", mcp.ErrResourceNotFound, code, env)
	}

	// Bonus edge case: unsubscribing the same, now-already-removed id a
	// second time must hit the same unknown-id error path, not silently
	// re-succeed.
	env = call(t, s, `{"jsonrpc":"2.0","id":3,"method":"resources/unsubscribe","params":{"uri":"`+uri+`"}}`)
	if env["result"] != nil {
		t.Fatalf("double-unsubscribe must not report success, got: %v", env)
	}
	if code := errorCode(t, env); code != mcp.ErrResourceNotFound {
		t.Fatalf("expected ErrResourceNotFound (%d) on double-unsubscribe, got %d: %v", mcp.ErrResourceNotFound, code, env)
	}
}

// --- Methods §1 — Missing (6 rows) ---

// TestConformanceMissingMethodsReturnMethodNotFound closes gap table §1
// "prompts/list", "prompts/get", "logging/setLevel", "completion/complete",
// "sampling/createMessage", "roots/list". Missing: every one must return a
// correct, spec-shaped ErrMethodNotFound (-32601) JSON-RPC error — never a
// malformed response.
func TestConformanceMissingMethodsReturnMethodNotFound(t *testing.T) {
	s, _ := newTestServer(t)
	missing := []string{
		"prompts/list", "prompts/get", "logging/setLevel",
		"completion/complete", "sampling/createMessage", "roots/list",
	}
	for _, m := range missing {
		t.Run(m, func(t *testing.T) {
			env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"`+m+`"}`)
			if env["jsonrpc"] != "2.0" {
				t.Fatalf("expected a jsonrpc:2.0 envelope, got: %v", env)
			}
			if code := errorCode(t, env); code != mcp.ErrMethodNotFound {
				t.Fatalf("expected ErrMethodNotFound (%d) for missing method %q, got %d: %v", mcp.ErrMethodNotFound, m, code, env)
			}
			errObj, _ := env["error"].(map[string]any)
			if msg, _ := errObj["message"].(string); msg == "" {
				t.Fatalf("expected a non-empty spec-shaped error message, got: %v", errObj)
			}
			if env["result"] != nil {
				t.Fatalf("an error response must not also carry a result, got: %v", env)
			}
		})
	}
}

// TestConformanceUnimplementedCapabilitiesNeverAdvertised closes gap table §1
// "sampling/createMessage"/"roots/list" ("correctly absent") together with
// the never-implemented "prompts"/"logging"/"completions" capability sets:
// across every capability-affecting config combination in this codebase, the
// server must never claim a capability it cannot deliver (Ambiguity #4: no
// server-initiated request/response correlation plumbing exists at all).
func TestConformanceUnimplementedCapabilitiesNeverAdvertised(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s := newSpikeServer(t, enabled)
		env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
		result, _ := env["result"].(map[string]any)
		caps, _ := result["capabilities"].(map[string]any)
		for _, forbidden := range []string{"sampling", "roots", "prompts", "logging", "completions"} {
			if _, ok := caps[forbidden]; ok {
				t.Fatalf("server must never advertise %q capability (unimplemented), got: %v", forbidden, caps)
			}
		}
	}
}

// --- Notifications §2 ---

// TestConformanceUnknownNotificationSilentlyAccepted closes gap table §2
// "notifications/cancelled", representative of every unrecognized
// notification: no dispatch case exists, and per Handle()'s default branch,
// an unrecognized message with no id is silently discarded (nil, nil)
// rather than erroring.
func TestConformanceUnknownNotificationSilentlyAccepted(t *testing.T) {
	s, _ := newTestServer(t)
	out, err := s.Handle(context.Background(), nil, []byte(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}`))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if out != nil {
		t.Fatalf("an unrecognized notification should produce no response, got: %s", out)
	}
}

// TestConformancePlainMissingNotificationsNeverEmitted closes gap table §2's
// 7 plain-Missing rows (notifications/cancelled, notifications/progress,
// notifications/roots/list_changed, notifications/message,
// notifications/resources/updated, notifications/tools/list_changed,
// notifications/prompts/list_changed): across a representative sweep of
// every implemented request path, none of these spec notification method
// names is ever published on the event bus.
func TestConformancePlainMissingNotificationsNeverEmitted(t *testing.T) {
	s := newSpikeServer(t, true)
	id := subscribe(t, s, `{"path":"pkg/*.go","semantic_threshold":"any"}`)
	uri := dispatch.SpikeResourcePrefix + id

	busSub := s.bus.Subscribe()
	defer busSub.Close()

	call(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)
	call(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	call(t, s, `{"jsonrpc":"2.0","id":3,"method":"resources/list"}`)
	call(t, s, `{"jsonrpc":"2.0","id":4,"method":"resources/templates/list"}`)
	call(t, s, `{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"`+uri+`"}}`)
	call(t, s, `{"jsonrpc":"2.0","id":6,"method":"resources/subscribe","params":{"uri":"`+uri+`"}}`)
	call(t, s, `{"jsonrpc":"2.0","id":7,"method":"resources/unsubscribe","params":{"uri":"`+uri+`"}}`)
	if _, err := s.Handle(context.Background(), nil, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); err != nil {
		t.Fatal(err)
	}

	forbidden := map[string]bool{
		"notifications/cancelled":            true,
		"notifications/progress":             true,
		"notifications/roots/list_changed":   true,
		"notifications/message":              true,
		"notifications/resources/updated":    true,
		"notifications/tools/list_changed":   true,
		"notifications/prompts/list_changed": true,
	}
	deadline := time.After(150 * time.Millisecond)
	for {
		select {
		case msg, ok := <-busSub.Messages():
			if !ok {
				return
			}
			if forbidden[msg.Topic] {
				t.Fatalf("spec notification %q must never be emitted (unimplemented / consistent with its advertised capability), but it was published", msg.Topic)
			}
		case <-deadline:
			return
		}
	}
}

// TestConformanceResourcesListChangedCapabilityMismatchFixed closes gap
// table §2 "notifications/resources/list_changed" — the one row C030 flagged
// as a genuine spec-conformance defect (capability advertised, notification
// never published), not a plain not-built-yet gap, and which ADR-013
// requires be actually fixed by C032, not merely documented. Fixed by
// truthfully advertising capabilities.resources.listChanged:false (the
// publish path itself is not implemented; see server.go handleInitialize).
func TestConformanceResourcesListChangedCapabilityMismatchFixed(t *testing.T) {
	s := newSpikeServer(t, true)
	env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	result, _ := env["result"].(map[string]any)
	caps, _ := result["capabilities"].(map[string]any)
	resources, ok := caps["resources"].(map[string]any)
	if !ok {
		t.Fatalf("expected resources capability to be advertised, got: %v", caps)
	}
	if resources["listChanged"] != false {
		t.Fatalf("expected capabilities.resources.listChanged:false (truthful: the notification is never published), got: %v", resources["listChanged"])
	}
	if resources["subscribe"] != true {
		t.Fatalf("expected capabilities.resources.subscribe to remain true (resources/subscribe acceptance is implemented; unaffected by this fix), got: %v", resources["subscribe"])
	}
}

// --- Error codes §3 ---

// TestConformanceParseVsInvalidRequestErrorCodes closes gap table §3a's
// "misuse finding" end to end through Server.Handle (the mcp-package-level
// assertion lives in internal/mcp/protocol_test.go): malformed JSON must map
// to Parse error (-32700); a syntactically valid envelope with the wrong
// jsonrpc version or a missing method must map to Invalid Request (-32600),
// not be collapsed into -32700. Required fix per ADR-013.
func TestConformanceParseVsInvalidRequestErrorCodes(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantCode int
	}{
		{"malformed json", `{`, mcp.ErrParse},
		{"wrong jsonrpc version", `{"jsonrpc":"1.0","id":1,"method":"ping"}`, mcp.ErrInvalidRequest},
		{"missing method", `{"jsonrpc":"2.0","id":1}`, mcp.ErrInvalidRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newTestServer(t)
			env := call(t, s, tc.raw)
			if code := errorCode(t, env); code != tc.wantCode {
				t.Fatalf("expected code %d, got %d: %v", tc.wantCode, code, env)
			}
		})
	}
}

// TestConformanceErrorCodeTriggerScenarios closes the remainder of gap table
// §3a/§3b: each used error-code constant is asserted in its documented
// trigger scenario.
func TestConformanceErrorCodeTriggerScenarios(t *testing.T) {
	t.Run("ErrMethodNotFound on unknown method", func(t *testing.T) {
		s, _ := newTestServer(t)
		env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"does/not/exist"}`)
		if code := errorCode(t, env); code != mcp.ErrMethodNotFound {
			t.Fatalf("expected %d, got %d", mcp.ErrMethodNotFound, code)
		}
	})

	t.Run("ErrInvalidParams on malformed tools/call params", func(t *testing.T) {
		s, _ := newTestServer(t)
		env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"not-an-object"}`)
		if code := errorCode(t, env); code != mcp.ErrInvalidParams {
			t.Fatalf("expected %d, got %d", mcp.ErrInvalidParams, code)
		}
	})

	t.Run("ErrInvalidParams on missing tool name", func(t *testing.T) {
		s, _ := newTestServer(t)
		env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"arguments":{}}}`)
		if code := errorCode(t, env); code != mcp.ErrInvalidParams {
			t.Fatalf("expected %d, got %d", mcp.ErrInvalidParams, code)
		}
	})

	t.Run("ErrToolNotFound on unknown tool", func(t *testing.T) {
		s, _ := newTestServer(t)
		env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"does_not_exist","arguments":{}}}`)
		if code := errorCode(t, env); code != mcp.ErrToolNotFound {
			t.Fatalf("expected %d, got %d", mcp.ErrToolNotFound, code)
		}
	})

	t.Run("ErrPermissionDenied on role-forbidden tool", func(t *testing.T) {
		s, _ := newTestServer(t)
		env := callSess(t, s, &transport.Session{Role: "orchestrator"},
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_file_sync","arguments":{"path":"x.txt","content":"oops"}}}`)
		if code := errorCode(t, env); code != mcp.ErrPermissionDenied {
			t.Fatalf("expected %d, got %d", mcp.ErrPermissionDenied, code)
		}
	})

	t.Run("ErrToolExecution on tool Execute error", func(t *testing.T) {
		s, _ := newTestServer(t)
		if err := s.Registry().Register(&errTool{}); err != nil {
			t.Fatal(err)
		}
		env := callSess(t, s, &transport.Session{Role: "worker"},
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"err_tool","arguments":{}}}`)
		if code := errorCode(t, env); code != mcp.ErrToolExecution {
			t.Fatalf("expected %d, got %d", mcp.ErrToolExecution, code)
		}
	})

	t.Run("ErrInternal on tool returning a nil result", func(t *testing.T) {
		s, _ := newTestServer(t)
		if err := s.Registry().Register(&nilTool{}); err != nil {
			t.Fatal(err)
		}
		env := callSess(t, s, &transport.Session{Role: "orchestrator"},
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nil_tool","arguments":{}}}`)
		if code := errorCode(t, env); code != mcp.ErrInternal {
			t.Fatalf("expected %d, got %d", mcp.ErrInternal, code)
		}
	})

	t.Run("ErrResourceNotFound on unknown resource uri", func(t *testing.T) {
		s := newSpikeServer(t, true)
		env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"cwso://other/x"}}`)
		if code := errorCode(t, env); code != mcp.ErrResourceNotFound {
			t.Fatalf("expected %d, got %d", mcp.ErrResourceNotFound, code)
		}
	})

	// All CWSO-specific reserved-range codes exercised above must stay
	// within the JSON-RPC-reserved implementation-defined server-error
	// range (-32000..-32099) and must not collide with a spec-defined code.
	for _, code := range []int{mcp.ErrPermissionDenied, mcp.ErrToolNotFound, mcp.ErrToolExecution, mcp.ErrResourceNotFound} {
		if code < -32099 || code > -32000 {
			t.Fatalf("reserved-range code %d falls outside -32000..-32099", code)
		}
	}
}
