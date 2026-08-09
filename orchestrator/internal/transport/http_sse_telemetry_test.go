package transport

import (
	"bytes"
	"context"
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

func TestBrokerSSETelemetryLogOnClose(t *testing.T) {
	t.Run("clean_close_logs_info", func(t *testing.T) {
		oldHB := heartbeatInterval
		heartbeatInterval = 500 * time.Millisecond
		defer func() { heartbeatInterval = oldHB }()

		var logBuf bytes.Buffer
		bus := eventbus.New()
		broker := memorybroker.New(
			memorybroker.WithCapacity(128),
			memorybroker.WithIngressQueueSize(128),
		)
		t.Cleanup(broker.Close)
		publisher := memorybroker.NewTeePublisher(bus, broker)

		log := logging.NewWithWriter("debug", &logBuf)
		handler := newHTTPHandler(context.Background(), &config.Config{
			JWTSecret:      "test-secret-32-bytes-minimum-padding-x",
			JWTAlg:         "HS256",
			JWTIssuer:      "cwso",
			JWTAudience:    "cwso-mcp",
			AllowedOrigins: []string{"http://localhost"},
		}, HTTPHandlerConfig{
			Log:             log,
			Bus:             bus,
			Broker:          broker,
			SamplePublisher: publisher,
			Handler: func(_ context.Context, _ *Session, _ []byte) ([]byte, error) {
				return []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`), nil
			},
		})
		srv := httptest.NewServer(handler)
		// srv.Close() is called explicitly before reading the buffer so the handler
		// goroutine (which writes the deferred telemetry log) is guaranteed to have
		// finished before we inspect logBuf — no t.Cleanup needed.

		token := makeJWT("test-secret-32-bytes-minimum-padding-x", &jwtClaims{
			Role: "worker",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "sse-test",
				Issuer:    "cwso",
				Audience:  jwt.ClaimStrings{"cwso-mcp"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			},
		})

		resp, reader := openSSE(t, srv.URL, token)

		// Assert ready event observed before connection close.
		event, data, heartbeat := readSSEFrame(t, reader, 200*time.Millisecond)
		if heartbeat {
			t.Fatal("expected ready frame first, got heartbeat")
		}
		if event != "ready" {
			t.Fatalf("expected ready event, got %q", event)
		}
		if !strings.Contains(data, `"protocolVersion":"2025-03-26"`) {
			t.Fatalf("unexpected ready payload: %s", data)
		}

		// Publish one event so the throttle records at least one counter.
		if err := publisher.Publish(eventbus.TopicNotificationsLog, map[string]any{"idx": 1}); err != nil {
			t.Fatalf("publish: %v", err)
		}
		time.Sleep(50 * time.Millisecond)

		// Close the connection — signals r.Context().Done() server-side.
		resp.Body.Close()
		// Shutdown blocks until the handler goroutine exits and the deferred
		// brokerSSETelemetryDefer closure has written to logBuf.
		srv.Close()

		output := logBuf.String()
		if !strings.Contains(output, "SSE telemetry stream closed") {
			t.Fatalf("expected 'SSE telemetry stream closed' in log, got:\n%s", output)
		}
		if strings.Contains(output, "dropped_events") {
			t.Fatalf("expected no dropped_events in clean close, got:\n%s", output)
		}
		if !strings.Contains(output, `"level":"info"`) {
			t.Fatalf("expected info-level log in clean close, got:\n%s", output)
		}
		if !strings.Contains(output, "telemetry_counts") {
			t.Fatalf("expected telemetry_counts in log, got:\n%s", output)
		}
	})

	// The drop-path test calls brokerSSETelemetryDefer directly: the subscriber is
	// never drained by an SSE handler loop, so events fill the depth-1 channel and
	// subsequent ones are dropped at the subscriber level — guaranteed and deterministic.
	t.Run("dropped_events_logs_warn", func(t *testing.T) {
		var logBuf bytes.Buffer
		log := logging.NewWithWriter("debug", &logBuf)

		broker := memorybroker.New(
			memorybroker.WithCapacity(128),
			memorybroker.WithIngressQueueSize(128),
			memorybroker.WithSubscriberQueueDepth(1),
		)
		defer broker.Close()

		sub := broker.Subscribe()
		defer sub.Close()
		throttle := newTelemetryThrottle()

		// Ingest 10 events without any reader on sub.Messages():
		// after the first event fills the depth-1 channel, events 2-10 are dropped.
		for i := 0; i < 10; i++ {
			broker.Ingest(eventbus.TopicNotificationsLog, map[string]any{"idx": i})
		}
		time.Sleep(50 * time.Millisecond) // let broker goroutine process all ingests

		brokerSSETelemetryDefer(log, sub, throttle)()

		output := logBuf.String()
		if !strings.Contains(output, "SSE telemetry stream closed") {
			t.Fatalf("expected 'SSE telemetry stream closed' in log, got:\n%s", output)
		}
		if !strings.Contains(output, "dropped_events") {
			t.Fatalf("expected 'dropped_events' in warn log, got:\n%s", output)
		}
		if !strings.Contains(output, `"level":"warn"`) {
			t.Fatalf("expected warn-level log in drop path, got:\n%s", output)
		}
	})
}
