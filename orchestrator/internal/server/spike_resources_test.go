package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
)

func newSpikeServer(t *testing.T, enabled bool) *Server {
	t.Helper()
	cfg := &config.Config{
		Transport:                "stdio",
		LogLevel:                 "error",
		Workspace:                t.TempDir(),
		AllowedOrigins:           []string{"http://localhost"},
		ASTSpikeResourcesEnabled: enabled,
	}
	s, err := New(cfg, logging.New("error"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func call(t *testing.T, s *Server, raw string) map[string]any {
	t.Helper()
	out, err := s.Handle(context.Background(), nil, []byte(raw))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("unmarshal response %s: %v", out, err)
	}
	return env
}

// subscribe creates a subscription via the tool and returns its id.
func subscribe(t *testing.T, s *Server, args string) string {
	t.Helper()
	env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"subscribe_ast_spikes","arguments":`+args+`}}`)
	result, _ := env["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("no content in subscribe result: %v", env)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	var payload struct {
		SubscriptionID string `json:"subscription_id"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("parse subscribe payload %q: %v", text, err)
	}
	if payload.SubscriptionID == "" {
		t.Fatalf("empty subscription id: %s", text)
	}
	return payload.SubscriptionID
}

func waitForBroker(t *testing.T, s *Server, topic string, n int) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if len(s.memory.Query(memorybroker.QueryOptions{Topics: []string{topic}})) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d records on %s", n, topic)
}

func TestResourcesCapabilityAdvertised(t *testing.T) {
	s := newSpikeServer(t, true)
	env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	result, _ := env["result"].(map[string]any)
	caps, _ := result["capabilities"].(map[string]any)
	if _, ok := caps["resources"]; !ok {
		t.Fatalf("expected resources capability, got: %v", caps)
	}
}

func TestResourcesDisabledIsMethodNotFound(t *testing.T) {
	s := newSpikeServer(t, false)
	env := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`)
	if env["error"] == nil {
		t.Fatalf("expected method-not-found error when disabled, got: %v", env)
	}
	// Capability must NOT be advertised when disabled.
	init := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"initialize"}`)
	caps, _ := init["result"].(map[string]any)["capabilities"].(map[string]any)
	if _, ok := caps["resources"]; ok {
		t.Fatal("resources capability must not be advertised when disabled")
	}
}

func TestResourcesListAndTemplates(t *testing.T) {
	s := newSpikeServer(t, true)
	id := subscribe(t, s, `{"path":"pkg/*.go","semantic_threshold":"any"}`)

	list := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"resources/list"}`)
	b, _ := json.Marshal(list)
	if !strings.Contains(string(b), dispatch.SpikeResourcePrefix+id) {
		t.Fatalf("expected resource uri in list: %s", b)
	}

	tmpl := call(t, s, `{"jsonrpc":"2.0","id":3,"method":"resources/templates/list"}`)
	tb, _ := json.Marshal(tmpl)
	if !strings.Contains(string(tb), "{subscription_id}") {
		t.Fatalf("expected uri template: %s", tb)
	}
}

func TestResourcesReadSnapshotFiltersByThreshold(t *testing.T) {
	s := newSpikeServer(t, true)
	id := subscribe(t, s, `{"semantic_threshold":"signature_change"}`)

	// A signature_change event matches; a symbol_added event is below threshold.
	if err := s.publisher.Publish(dispatch.TopicASTSemanticSpike,
		dispatch.SemanticSpikeEvent{Workspace: "ws1", Path: "pkg/a.go", SpikeKind: string(dispatch.SpikeKindSignatureChange)}); err != nil {
		t.Fatal(err)
	}
	if err := s.publisher.Publish(dispatch.TopicASTSemanticSpike,
		dispatch.SemanticSpikeEvent{Workspace: "ws1", Path: "pkg/b.go", SpikeKind: string(dispatch.SpikeKindSymbolAdded)}); err != nil {
		t.Fatal(err)
	}
	waitForBroker(t, s, dispatch.TopicASTSemanticSpike, 2)

	env := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"`+dispatch.SpikeResourcePrefix+id+`"}}`)
	result, _ := env["result"].(map[string]any)
	contents, _ := result["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("expected one content block, got: %v", env)
	}
	text, _ := contents[0].(map[string]any)["text"].(string)
	var snap struct {
		Events []struct {
			Event struct {
				SpikeKind string `json:"spike_kind"`
			} `json:"event"`
		} `json:"events"`
	}
	if err := json.Unmarshal([]byte(text), &snap); err != nil {
		t.Fatalf("parse snapshot %q: %v", text, err)
	}
	if len(snap.Events) != 1 {
		t.Fatalf("expected exactly 1 matching event (threshold-gated), got %d: %s", len(snap.Events), text)
	}
	if snap.Events[0].Event.SpikeKind != string(dispatch.SpikeKindSignatureChange) {
		t.Fatalf("unexpected event kind: %s", text)
	}
}

func TestResourcesSubscribeUnsubscribeLifecycle(t *testing.T) {
	s := newSpikeServer(t, true)
	id := subscribe(t, s, `{"semantic_threshold":"any"}`)
	uri := dispatch.SpikeResourcePrefix + id

	if env := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"resources/subscribe","params":{"uri":"`+uri+`"}}`); env["error"] != nil {
		t.Fatalf("subscribe failed: %v", env)
	}
	if env := call(t, s, `{"jsonrpc":"2.0","id":3,"method":"resources/unsubscribe","params":{"uri":"`+uri+`"}}`); env["error"] != nil {
		t.Fatalf("unsubscribe failed: %v", env)
	}
	if _, ok := s.spikeSubs.Get(id); ok {
		t.Fatal("subscription should be removed after unsubscribe")
	}
	// Reading an unknown uri now errors.
	if env := call(t, s, `{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"`+uri+`"}}`); env["error"] == nil {
		t.Fatal("expected resource-not-found error for removed subscription")
	}
}

func TestResourcesReadRejectsBadURI(t *testing.T) {
	s := newSpikeServer(t, true)
	if env := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"cwso://other/x"}}`); env["error"] == nil {
		t.Fatal("expected error for non-spike uri")
	}
}
