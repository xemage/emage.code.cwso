package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/eventbus"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
	"github.com/golang-jwt/jwt/v5"
)

// wsFilter only allows records on one topic whose payload workspace matches.
type wsFilter struct {
	topic     string
	workspace string
}

func (f wsFilter) Allow(topic string, payload []byte) bool {
	if topic != f.topic {
		return false
	}
	var env struct {
		Workspace string `json:"workspace"`
	}
	_ = json.Unmarshal(payload, &env)
	return env.Workspace == f.workspace
}

func newScopedSSEServer(t *testing.T, resolver SubscriptionResolver) (*httptest.Server, string, *memorybroker.Broker) {
	t.Helper()
	secret := "test-secret-32-bytes-minimum-padding-x"
	cfg := &config.Config{
		JWTSecret:      secret,
		JWTAlg:         "HS256",
		JWTIssuer:      "cwso",
		JWTAudience:    "cwso-mcp",
		AllowedOrigins: []string{"http://localhost"},
	}
	log := logging.New("error")
	bus := eventbus.New()
	broker := memorybroker.New(memorybroker.WithCapacity(128), memorybroker.WithIngressQueueSize(128))
	t.Cleanup(broker.Close)
	publisher := memorybroker.NewTeePublisher(bus, broker)

	var opts []HTTPOption
	if resolver != nil {
		opts = append(opts, WithSubscriptionResolver(resolver))
	}
	handler := newHTTPHandler(context.Background(), cfg, log, bus, broker, publisher,
		func(context.Context, *Session, []byte) ([]byte, error) { return nil, nil }, opts...)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	tok := makeJWT(secret, &jwtClaims{
		Role: "worker",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "scoped-sse",
			Issuer:    "cwso",
			Audience:  jwt.ClaimStrings{"cwso-mcp"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	return srv, tok, broker
}

func openScopedSSE(t *testing.T, baseURL, token, query string) (*http.Response, *bufio.Reader) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+"/mcp"+query, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", "http://localhost")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open sse: %v", err)
	}
	return resp, bufio.NewReader(resp.Body)
}

// nextDataFrame reads SSE frames skipping heartbeats and the initial ready event.
func nextDataFrame(t *testing.T, reader *bufio.Reader, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		event, data, hb := readSSEFrame(t, reader, timeout)
		if hb || event == "ready" {
			continue
		}
		if data != "" {
			return data
		}
	}
	t.Fatal("no data frame received")
	return ""
}

func TestScopedSSEFiltersBySubscription(t *testing.T) {
	oldHB := heartbeatInterval
	heartbeatInterval = 500 * time.Millisecond
	defer func() { heartbeatInterval = oldHB }()

	resolver := func(id string) (RecordFilter, bool) {
		if id == "sub-1" {
			return wsFilter{topic: "ast/semantic-spike", workspace: "ws-keep"}, true
		}
		return nil, false
	}
	srv, tok, broker := newScopedSSEServer(t, resolver)

	resp, reader := openScopedSSE(t, srv.URL, tok, "?subscription=sub-1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Consume the ready frame.
	if event, _, _ := readSSEFrame(t, reader, time.Second); event != "ready" {
		t.Fatalf("expected ready first, got %q", event)
	}

	// Wait for the live subscriber to attach before publishing.
	time.Sleep(20 * time.Millisecond)
	// Filtered out: wrong workspace.
	_ = broker.Ingest("ast/semantic-spike", map[string]any{"workspace": "ws-other", "spike_kind": "signature_change"})
	// Filtered out: unrelated topic.
	_ = broker.Ingest("dispatch/decision", map[string]any{"workspace": "ws-keep"})
	// Delivered: matching workspace + topic.
	_ = broker.Ingest("ast/semantic-spike", map[string]any{"workspace": "ws-keep", "spike_kind": "signature_change"})

	data := nextDataFrame(t, reader, 2*time.Second)
	if !strings.Contains(data, "ws-keep") {
		t.Fatalf("expected ws-keep event, got: %s", data)
	}
	if strings.Contains(data, "ws-other") || strings.Contains(data, "dispatch/decision") {
		t.Fatalf("filtered events leaked through: %s", data)
	}
}

func TestScopedSSEUnknownSubscriptionIs404(t *testing.T) {
	resolver := func(id string) (RecordFilter, bool) { return nil, false }
	srv, tok, _ := newScopedSSEServer(t, resolver)

	resp, _ := openScopedSSE(t, srv.URL, tok, "?subscription=nope")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown subscription, got %d", resp.StatusCode)
	}
}

func TestScopedSSEDisabledWhenNoResolver(t *testing.T) {
	srv, tok, _ := newScopedSSEServer(t, nil)
	resp, _ := openScopedSSE(t, srv.URL, tok, "?subscription=anything")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when subscriptions not enabled, got %d", resp.StatusCode)
	}
}
