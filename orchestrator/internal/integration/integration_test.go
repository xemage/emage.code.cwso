package integration_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/eventbus"
	"github.com/emage/cwso/orchestrator/internal/jobs"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
	"github.com/emage/cwso/orchestrator/internal/transport"
	"github.com/golang-jwt/jwt/v5"
)

// jwtClaims mirrors the transport package's internal jwtClaims struct.
type jwtClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// makeJWT creates a signed JWT token with HS256.
func makeJWT(secret string, claims *jwtClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secret))
	return tokenString
}

// readSSEFrame reads one complete SSE frame from a buffered reader.
// Returns the event type, data, and whether it was a heartbeat comment.
func readSSEFrame(t *testing.T, r *bufio.Reader, timeout time.Duration) (event string, data string, heartbeat bool) {
	t.Helper()
	type result struct {
		event     string
		data      string
		heartbeat bool
		err       error
	}
	ch := make(chan result, 1)
	go func() {
		res := result{}
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				res.err = err
				ch <- res
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				ch <- res
				return
			}
			if strings.HasPrefix(line, ":") {
				res.heartbeat = true
				continue
			}
			if strings.HasPrefix(line, "event: ") {
				res.event = strings.TrimPrefix(line, "event: ")
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				if res.data == "" {
					res.data = strings.TrimPrefix(line, "data: ")
				} else {
					res.data += "\n" + strings.TrimPrefix(line, "data: ")
				}
			}
		}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("read SSE frame: %v", res.err)
		}
		return res.event, res.data, res.heartbeat
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for SSE frame after %v", timeout)
		return "", "", false
	}
}

// openSSE opens an SSE connection to the given URL with a bearer token.
func openSSE(t *testing.T, baseURL, token string) (*http.Response, *bufio.Reader) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+"/mcp", nil)
	if err != nil {
		t.Fatalf("new SSE request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", "http://localhost")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open SSE: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("expected 200 from SSE endpoint, got %d body=%s", resp.StatusCode, string(body))
	}

	return resp, bufio.NewReader(resp.Body)
}

// testConfig creates a minimal HTTP config for testing.
func testConfig(addr string) *config.Config {
	return &config.Config{
		Transport:      "http",
		HTTPAddr:       addr,
		LogLevel:       "error",
		JWTSecret:      "test-secret-32-bytes-minimum-padding-x",
		JWTAlg:         "HS256",
		JWTIssuer:      "cwso",
		JWTAudience:    "cwso-mcp",
		AllowedOrigins: []string{"http://localhost"},
		JobWorkers:     2,
		JobQueueSize:   16,
	}
}

// testLogger creates a logger with error level to minimize noise.
func testLogger() *logging.Logger {
	return logging.New("error")
}

// makeTestJWT creates a JWT token for testing.
func makeTestJWT(secret string) string {
	claims := &jwtClaims{
		Role: "worker",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "integration-test",
			Issuer:    "cwso",
			Audience:  jwt.ClaimStrings{"cwso-mcp"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	return makeJWT(secret, claims)
}

// getRandomAddr returns a listen address on localhost with a random port.
func getRandomAddr() string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

// waitForServer blocks until the HTTP server is ready or timeout.
func waitForServer(t *testing.T, baseURL string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server not ready after %v", timeout)
}

// TestIntegrationMultiSubscriberSSEFanout verifies that multiple SSE subscribers
// receive published events without dropping.
func TestIntegrationMultiSubscriberSSEFanout(t *testing.T) {
	addr := getRandomAddr()
	cfg := testConfig(addr)
	log := testLogger()
	bus := eventbus.New()

	// Handler that does nothing
	handler := func(ctx context.Context, sess *transport.Session, raw []byte) ([]byte, error) {
		return []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`), nil
	}

	// Start RunHTTP in a goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.RunHTTP(ctx, cfg, log, bus, nil, nil, handler)
	}()

	// Wait for the server to be ready
	baseURL := "http://" + addr
	token := makeTestJWT(cfg.JWTSecret)

	// Wait for server to be ready
	waitForServer(t, baseURL, 3*time.Second)

	// Open two SSE connections
	resp1, reader1 := openSSE(t, baseURL, token)
	defer resp1.Body.Close()

	resp2, reader2 := openSSE(t, baseURL, token)
	defer resp2.Body.Close()

	// Read ready frames
	event1, _, _ := readSSEFrame(t, reader1, 1*time.Second)
	if event1 != "ready" {
		t.Fatalf("expected 'ready' event from subscriber 1, got %q", event1)
	}

	event2, _, _ := readSSEFrame(t, reader2, 1*time.Second)
	if event2 != "ready" {
		t.Fatalf("expected 'ready' event from subscriber 2, got %q", event2)
	}

	// Give subscribers time to start receiving
	time.Sleep(50 * time.Millisecond)

	// Publish one event
	payload := json.RawMessage(`{"msg":"hello"}`)
	err := bus.Publish(eventbus.TopicNotificationsLog, payload)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Both subscribers should receive the event
	event1, data1, hb1 := readSSEFrame(t, reader1, 1*time.Second)
	if hb1 || data1 == "" {
		t.Fatalf("subscriber 1: expected non-heartbeat data frame, got event=%q heartbeat=%v data=%q", event1, hb1, data1)
	}

	// Verify the data contains the expected method
	var n1 map[string]interface{}
	err = json.Unmarshal([]byte(data1), &n1)
	if err != nil {
		t.Fatalf("subscriber 1: unmarshal JSON-RPC notification: %v", err)
	}
	if method, ok := n1["method"]; !ok || method != eventbus.TopicNotificationsLog {
		t.Fatalf("subscriber 1: expected method %q, got %v", eventbus.TopicNotificationsLog, method)
	}

	event2, data2, hb2 := readSSEFrame(t, reader2, 1*time.Second)
	if hb2 || data2 == "" {
		t.Fatalf("subscriber 2: expected non-heartbeat data frame, got event=%q heartbeat=%v data=%q", event2, hb2, data2)
	}

	var n2 map[string]interface{}
	err = json.Unmarshal([]byte(data2), &n2)
	if err != nil {
		t.Fatalf("subscriber 2: unmarshal JSON-RPC notification: %v", err)
	}
	if method, ok := n2["method"]; !ok || method != eventbus.TopicNotificationsLog {
		t.Fatalf("subscriber 2: expected method %q, got %v", eventbus.TopicNotificationsLog, method)
	}
}

// TestIntegrationBrokerBackedSSEThrottling verifies that the broker SSE path
// applies telemetry throttling correctly.
func TestIntegrationBrokerBackedSSEThrottling(t *testing.T) {
	addr := getRandomAddr()
	cfg := testConfig(addr)
	log := testLogger()
	bus := eventbus.New()
	broker := memorybroker.New(
		memorybroker.WithCapacity(256),
		memorybroker.WithIngressQueueSize(256),
	)
	defer broker.Close()
	publisher := memorybroker.NewTeePublisher(bus, broker)

	handler := func(ctx context.Context, sess *transport.Session, raw []byte) ([]byte, error) {
		return []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.RunHTTP(ctx, cfg, log, bus, broker, publisher, handler)
	}()

	baseURL := "http://" + addr
	waitForServer(t, baseURL, 3*time.Second)

	token := makeTestJWT(cfg.JWTSecret)

	// Open SSE connection (will use broker path since broker != nil)
	resp, reader := openSSE(t, baseURL, token)
	defer resp.Body.Close()

	// Read ready frame
	event, _, _ := readSSEFrame(t, reader, 1*time.Second)
	if event != "ready" {
		t.Fatalf("expected 'ready' event, got %q", event)
	}

	// Give subscriber time to start receiving
	time.Sleep(50 * time.Millisecond)

	// Publish 3 log events in rapid succession (within 10ms window)
	// The throttle window for log is 250ms, so only the first should emit
	payload := json.RawMessage(`{"msg":"event1"}`)
	_ = publisher.Publish(eventbus.TopicNotificationsLog, payload)
	time.Sleep(2 * time.Millisecond)

	payload = json.RawMessage(`{"msg":"event2"}`)
	_ = publisher.Publish(eventbus.TopicNotificationsLog, payload)
	time.Sleep(2 * time.Millisecond)

	payload = json.RawMessage(`{"msg":"event3"}`)
	_ = publisher.Publish(eventbus.TopicNotificationsLog, payload)

	// Then publish a terminal job-state event (should bypass throttle)
	statePayload := json.RawMessage(`{"job_id":"j1","state":"completed"}`)
	_ = publisher.Publish(eventbus.TopicNotificationsJobState, statePayload)

	// First frame should be the log event (not suppressed by throttle)
	event1, data1, _ := readSSEFrame(t, reader, 1*time.Second)
	if data1 == "" {
		t.Fatalf("expected data frame 1, got event=%q data=%q", event1, data1)
	}

	var n1 map[string]interface{}
	err := json.Unmarshal([]byte(data1), &n1)
	if err != nil {
		t.Fatalf("unmarshal notification 1: %v", err)
	}
	if method, ok := n1["method"]; !ok || method != eventbus.TopicNotificationsLog {
		t.Fatalf("expected method %q, got %v", eventbus.TopicNotificationsLog, method)
	}

	// Second frame should be the job-state event (terminal event bypasses throttle)
	event2, data2, _ := readSSEFrame(t, reader, 1*time.Second)
	if data2 == "" {
		t.Fatalf("expected data frame 2, got event=%q data=%q", event2, data2)
	}

	var n2 map[string]interface{}
	err = json.Unmarshal([]byte(data2), &n2)
	if err != nil {
		t.Fatalf("unmarshal notification 2: %v", err)
	}
	if method, ok := n2["method"]; !ok || method != eventbus.TopicNotificationsJobState {
		t.Fatalf("expected method %q, got %v", eventbus.TopicNotificationsJobState, method)
	}

	t.Logf("✓ Throttle test passed: first log event emitted, 2nd/3rd suppressed, terminal state emitted")
}

// TestIntegrationJobDispatchToSSENotification verifies that job state changes
// are published to SSE subscribers.
func TestIntegrationJobDispatchToSSENotification(t *testing.T) {
	addr := getRandomAddr()
	cfg := testConfig(addr)
	log := testLogger()
	bus := eventbus.New()
	broker := memorybroker.New(
		memorybroker.WithCapacity(256),
		memorybroker.WithIngressQueueSize(256),
	)
	defer broker.Close()
	publisher := memorybroker.NewTeePublisher(bus, broker)

	// Create job manager
	jobMgr, err := jobs.NewManager(jobs.Config{
		Workers:   2,
		QueueSize: 16,
	}, publisher)
	if err != nil {
		t.Fatalf("create job manager: %v", err)
	}
	defer jobMgr.Close()

	handler := func(ctx context.Context, sess *transport.Session, raw []byte) ([]byte, error) {
		return []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.RunHTTP(ctx, cfg, log, bus, broker, publisher, handler)
	}()

	baseURL := "http://" + addr
	waitForServer(t, baseURL, 3*time.Second)

	token := makeTestJWT(cfg.JWTSecret)

	// Open SSE connection
	resp, reader := openSSE(t, baseURL, token)
	defer resp.Body.Close()

	// Read ready frame
	event, _, _ := readSSEFrame(t, reader, 1*time.Second)
	if event != "ready" {
		t.Fatalf("expected 'ready' event, got %q", event)
	}

	// Give subscriber time to start receiving
	time.Sleep(50 * time.Millisecond)

	// Enqueue a job
	jobReq := jobs.Request{
		Name:    "test-job",
		Timeout: 1 * time.Second,
		Run: func(ctx context.Context) error {
			// Job completes immediately
			return nil
		},
	}
	_, err = jobMgr.Enqueue(jobReq)
	if err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	// Drain SSE frames for up to 3 seconds looking for job-state notifications
	deadline := time.Now().Add(3 * time.Second)
	receivedJobStateFrame := false

	for time.Now().Before(deadline) {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			break
		}
		if timeout > 1*time.Second {
			timeout = 1 * time.Second
		}
		_, data, hb := readSSEFrame(t, reader, timeout)

		if hb {
			// Heartbeat, skip
			continue
		}

		if data == "" {
			// No data, skip
			continue
		}

		var n map[string]interface{}
		err := json.Unmarshal([]byte(data), &n)
		if err != nil {
			t.Logf("failed to unmarshal frame: %v", err)
			continue
		}

		method, ok := n["method"]
		if !ok {
			continue
		}

		if method == eventbus.TopicNotificationsJobState {
			receivedJobStateFrame = true
			t.Logf("✓ Received job-state notification: %s", data)
			break
		}
	}

	if !receivedJobStateFrame {
		t.Fatal("did not receive job-state notification within 3 seconds")
	}
}

// TestIntegrationEndToEndSignalPath verifies the full signal path from job
// dispatch through broker to SSE subscribers.
func TestIntegrationEndToEndSignalPath(t *testing.T) {
	addr := getRandomAddr()
	cfg := testConfig(addr)
	log := testLogger()
	bus := eventbus.New()
	broker := memorybroker.New(
		memorybroker.WithCapacity(512),
		memorybroker.WithIngressQueueSize(512),
	)
	defer broker.Close()
	publisher := memorybroker.NewTeePublisher(bus, broker)

	// Create job manager
	jobMgr, err := jobs.NewManager(jobs.Config{
		Workers:   2,
		QueueSize: 16,
	}, publisher)
	if err != nil {
		t.Fatalf("create job manager: %v", err)
	}
	defer jobMgr.Close()

	handler := func(ctx context.Context, sess *transport.Session, raw []byte) ([]byte, error) {
		return []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.RunHTTP(ctx, cfg, log, bus, broker, publisher, handler)
	}()

	baseURL := "http://" + addr
	waitForServer(t, baseURL, 3*time.Second)

	token := makeTestJWT(cfg.JWTSecret)

	// Open 2 SSE subscribers
	resp1, reader1 := openSSE(t, baseURL, token)
	defer resp1.Body.Close()

	resp2, reader2 := openSSE(t, baseURL, token)
	defer resp2.Body.Close()

	// Read ready frames
	event1, _, _ := readSSEFrame(t, reader1, 1*time.Second)
	if event1 != "ready" {
		t.Fatalf("subscriber 1: expected 'ready', got %q", event1)
	}

	event2, _, _ := readSSEFrame(t, reader2, 1*time.Second)
	if event2 != "ready" {
		t.Fatalf("subscriber 2: expected 'ready', got %q", event2)
	}

	// Give subscribers time to start receiving
	time.Sleep(50 * time.Millisecond)

	// Dispatch 3 concurrent jobs
	jobIDs := []string{}
	for i := 0; i < 3; i++ {
		jobReq := jobs.Request{
			Name:    fmt.Sprintf("job-%d", i),
			Timeout: 2 * time.Second,
			Run: func(ctx context.Context) error {
				// Jobs complete immediately
				return nil
			},
		}
		job, err := jobMgr.Enqueue(jobReq)
		if err != nil {
			t.Fatalf("enqueue job %d: %v", i, err)
		}
		jobIDs = append(jobIDs, job.ID)
		t.Logf("enqueued job %s", job.ID)
	}

	// Wait for all 3 jobs to reach terminal state and collect frames from both subscribers
	deadline := time.Now().Add(5 * time.Second)
	frameCount1 := 0
	frameCount2 := 0

	readyCount1 := 0
	readyCount2 := 0

	for time.Now().Before(deadline) {
		// Try to read from subscriber 1 with short timeout to not block
		event1, data1, hb1 := readSSEFrame(t, reader1, 100*time.Millisecond)
		if !hb1 && data1 != "" {
			frameCount1++
			t.Logf("subscriber 1: frame %d received", frameCount1)
		}
		if event1 == "ready" && readyCount1 == 0 {
			readyCount1++
		}

		// Try to read from subscriber 2 with short timeout
		event2, data2, hb2 := readSSEFrame(t, reader2, 100*time.Millisecond)
		if !hb2 && data2 != "" {
			frameCount2++
			t.Logf("subscriber 2: frame %d received", frameCount2)
		}
		if event2 == "ready" && readyCount2 == 0 {
			readyCount2++
		}

		if frameCount1 >= 3 && frameCount2 >= 3 {
			t.Logf("✓ Both subscribers received ≥3 frames")
			break
		}
	}

	if frameCount1 < 3 {
		t.Fatalf("subscriber 1: expected ≥3 frames, got %d", frameCount1)
	}
	if frameCount2 < 3 {
		t.Fatalf("subscriber 2: expected ≥3 frames, got %d", frameCount2)
	}

	// Verify broker accumulated telemetry
	if brokerLen := broker.Len(); brokerLen == 0 {
		t.Fatal("expected broker to have accumulated records, got 0")
	}
	t.Logf("✓ Broker accumulated %d records", broker.Len())

	// Verify broker Query returns records for job-state topic
	records := broker.Query(memorybroker.QueryOptions{
		Topics: []string{eventbus.TopicNotificationsJobState},
	})
	if len(records) == 0 {
		t.Fatal("expected broker.Query to return job-state records, got none")
	}
	t.Logf("✓ Broker Query returned %d job-state records", len(records))
}
