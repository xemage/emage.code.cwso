package transport

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/eventbus"
	"github.com/emage/cwso/orchestrator/internal/logging"
)

type capturingPublisher struct {
	topics []string
	calls  int
	errAt  int
}

func (p *capturingPublisher) Publish(topic string, _ any) error {
	p.calls++
	p.topics = append(p.topics, topic)
	if p.errAt > 0 && p.calls == p.errAt {
		return errors.New("publish failed")
	}
	return nil
}

func TestMarshalJSONRPCNotification(t *testing.T) {
	env, err := marshalJSONRPCNotification(eventbus.TopicNotificationsLog, nil)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}
	var got struct {
		JSONRPC string         `json:"jsonrpc"`
		Method  string         `json:"method"`
		Params  map[string]any `json:"params"`
	}
	if err := json.Unmarshal(env, &got); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if got.JSONRPC != "2.0" || got.Method != eventbus.TopicNotificationsLog {
		t.Fatalf("unexpected envelope: %+v", got)
	}
	if len(got.Params) != 0 {
		t.Fatalf("expected empty params object for nil payload, got %+v", got.Params)
	}
}

func TestPublishSampleEventsByMethod(t *testing.T) {
	pub := &capturingPublisher{}
	log := logging.New("error")

	publishSampleEvents(pub, log, sampleEventParams{method: "tools/call", requestID: "rid-1", state: "completed"})
	if len(pub.topics) != 2 {
		t.Fatalf("expected two publishes for tools/call, got %v", pub.topics)
	}
	if pub.topics[0] != eventbus.TopicNotificationsLog || pub.topics[1] != eventbus.TopicNotificationsJobState {
		t.Fatalf("unexpected topic sequence: %v", pub.topics)
	}

	pub.topics = nil
	pub.calls = 0
	publishSampleEvents(pub, log, sampleEventParams{method: "ping", requestID: "rid-2", state: "completed"})
	if len(pub.topics) != 1 || pub.topics[0] != eventbus.TopicNotificationsLog {
		t.Fatalf("expected log-only publish for non tools/call, got %v", pub.topics)
	}
}

func TestPublishSampleEventsHandlesPublisherErrors(t *testing.T) {
	pub := &capturingPublisher{errAt: 1}
	log := logging.New("error")

	publishSampleEvents(pub, log, sampleEventParams{method: "tools/call", requestID: "rid-3", state: "failed", errMsg: "boom"})
	if pub.calls != 2 {
		t.Fatalf("expected both publish attempts despite first failure, got %d", pub.calls)
	}

	publishSampleEvents(nil, log, sampleEventParams{method: "tools/call", requestID: "rid-4", state: "completed"})
}

func TestSSEConnectionStoreAcquireRelease(t *testing.T) {
	store := &sseConnectionStore{conns: map[string]int{}}
	ip := "127.0.0.1"

	for i := 0; i < maxSSEConnsPerIP; i++ {
		if !store.acquire(ip) {
			t.Fatalf("acquire %d should succeed", i)
		}
	}
	if store.acquire(ip) {
		t.Fatal("acquire beyond max should fail")
	}

	store.release(ip)
	if !store.acquire(ip) {
		t.Fatal("acquire should succeed after release")
	}

	other := "127.0.0.2"
	if !store.acquire(other) {
		t.Fatal("different IP should have independent counter")
	}
}

func TestOriginHostAllowedSkipsMalformedOrigins(t *testing.T) {
	allowed := map[string]struct{}{
		"localhost":        {},
		"http://127.0.0.1": {},
	}
	if !originHostAllowed(allowed, "127.0.0.1") {
		t.Fatal("expected host to match well-formed allowed origin")
	}
	if originHostAllowed(allowed, "localhost") {
		t.Fatal("expected malformed origin entry without scheme to be ignored")
	}
}
