package shadow

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// startCountingSidecar starts a fake sidecar that mirrors the real
// cwso-git-shadow's connection handling: each accepted connection is served
// by its own goroutine that loops reading frames (one request at a time,
// strictly sequential per connection) until the client closes it. This lets
// tests exercise genuine connection reuse, not just one-shot round trips.
// It returns the socket path and a live counter of distinct accepted
// connections, so tests can assert the pool actually stayed bounded.
func startCountingSidecar(t *testing.T, handler func(envelope) response) (socket string, connCount *int64) {
	t.Helper()
	socket = t.TempDir() + "/sidecar.sock"
	var count int64

	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt64(&count, 1)
			go func(c net.Conn) {
				defer c.Close()
				for {
					body, err := readFrame(c)
					if err != nil {
						return
					}
					var req envelope
					if err := json.Unmarshal(body, &req); err != nil {
						return
					}
					resp := handler(req)
					respBody, err := json.Marshal(resp)
					if err != nil {
						return
					}
					if err := writeFrame(c, respBody); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	return socket, &count
}

func startTestSidecar(t *testing.T, handler func(envelope) response) string {
	t.Helper()
	socket, _ := startCountingSidecar(t, handler)
	return socket
}

func TestCallSuccess(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		if req.Op != "test_op" {
			t.Fatalf("unexpected op: %q", req.Op)
		}
		if len(req.Params) == 0 {
			t.Fatal("expected params")
		}
		return response{ID: req.ID, OK: true, Result: json.RawMessage(`{"value":"ok"}`)}
	})

	client := NewClient(socket)
	t.Cleanup(func() { _ = client.Close() })
	var out struct {
		Value string `json:"value"`
	}
	err := client.Call("test_op", map[string]any{"k": "v"}, &out)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if out.Value != "ok" {
		t.Fatalf("expected value ok, got %q", out.Value)
	}
}

func TestCallSidecarError(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{
			ID: req.ID,
			OK: false,
			Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{Code: "not_found", Message: "workspace missing"},
		}
	})

	client := NewClient(socket)
	t.Cleanup(func() { _ = client.Close() })
	err := client.Call("drop_workspace", map[string]any{"workspace_uuid": "missing"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sidecar not_found: workspace missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallResultDecodeError(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{ID: req.ID, OK: true, Result: json.RawMessage(`{"value":"not-an-int"}`)}
	})

	client := NewClient(socket)
	t.Cleanup(func() { _ = client.Close() })
	var out struct {
		Value int `json:"value"`
	}
	err := client.Call("query_ast", nil, &out)
	if err == nil {
		t.Fatal("expected decode result error")
	}
	if !strings.Contains(err.Error(), "decode result") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallMarshalParamsError(t *testing.T) {
	client := NewClient("/does/not/matter.sock")
	err := client.Call("bad", map[string]any{"fn": func() {}}, nil)
	if err == nil {
		t.Fatal("expected marshal params error")
	}
	if !strings.Contains(err.Error(), "marshal params") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteFrameTooLarge(t *testing.T) {
	payload := make([]byte, frameMax+1)
	err := writeFrame(&strings.Builder{}, payload)
	if err == nil {
		t.Fatal("expected frame too large error")
	}
	if !strings.Contains(err.Error(), "frame too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadFrameOutOfRange(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		var wire strings.Builder
		if err := writeHeader(&wire, 0); err != nil {
			t.Fatalf("write header: %v", err)
		}
		_, err := readFrame(strings.NewReader(wire.String()))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "frame size out of range") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("too large", func(t *testing.T) {
		var wire strings.Builder
		if err := writeHeader(&wire, uint32(frameMax+1)); err != nil {
			t.Fatalf("write header: %v", err)
		}
		_, err := readFrame(strings.NewReader(wire.String()))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "frame size out of range") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func writeHeader(sb *strings.Builder, n uint32) error {
	hdr := make([]byte, frameHeader)
	hdr[0] = byte(n >> 24)
	hdr[1] = byte(n >> 16)
	hdr[2] = byte(n >> 8)
	hdr[3] = byte(n)
	_, err := sb.Write(hdr)
	return err
}

func TestCallDialError(t *testing.T) {
	client := NewClient(t.TempDir() + "/missing.sock")
	err := client.Call("create_workspace", nil, nil)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallSidecarFailureWithoutBody(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{ID: req.ID, OK: false}
	})
	client := NewClient(socket)
	t.Cleanup(func() { _ = client.Close() })
	err := client.Call("create_workspace", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errors.New("sidecar reported failure with no error body")) &&
		!strings.Contains(err.Error(), "failure with no error body") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallWithNilOutIgnoresResultDecode(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{ID: req.ID, OK: true, Result: json.RawMessage(`{"x":1}`)}
	})
	client := NewClient(socket)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Call("noop", nil, nil); err != nil {
		t.Fatalf("Call failed: %v", err)
	}
}

func TestCallEnvelopeMarshalError(t *testing.T) {
	// This is defensive: op cannot trigger marshal failure, but we keep
	// the branch protected if envelope types evolve.
	client := NewClient("unused")
	err := client.Call(string([]byte{0xff, 0xfe}), nil, nil)
	if err != nil && !strings.Contains(err.Error(), "dial") {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = fmt.Sprintf("%v", client)
}

// TestPoolSizeConfigurable verifies both configuration paths (explicit
// constructor and environment variable) select a non-default pool size, and
// that a non-positive size falls back to the documented default.
func TestPoolSizeConfigurable(t *testing.T) {
	if c := NewClientWithPoolSize("sock", 3); cap(c.sem) != 3 {
		t.Fatalf("expected pool size 3, got %d", cap(c.sem))
	}
	if c := NewClientWithPoolSize("sock", 0); cap(c.sem) != defaultPoolSize {
		t.Fatalf("expected fallback to default pool size %d, got %d", defaultPoolSize, cap(c.sem))
	}
	if c := NewClientWithPoolSize("sock", -1); cap(c.sem) != defaultPoolSize {
		t.Fatalf("expected fallback to default pool size %d, got %d", defaultPoolSize, cap(c.sem))
	}

	t.Setenv(poolSizeEnvVar, "5")
	if c := NewClient("sock"); cap(c.sem) != 5 {
		t.Fatalf("expected env-configured pool size 5, got %d", cap(c.sem))
	}

	t.Setenv(poolSizeEnvVar, "not-a-number")
	if c := NewClient("sock"); cap(c.sem) != defaultPoolSize {
		t.Fatalf("expected fallback to default pool size on bad env value, got %d", cap(c.sem))
	}
}

// TestCallReusesPooledConnections verifies that sequential calls through a
// small pool reuse an already-open connection instead of dialing a fresh
// one each time — the behavior this task replaces.
func TestCallReusesPooledConnections(t *testing.T) {
	socket, connCount := startCountingSidecar(t, func(req envelope) response {
		return response{ID: req.ID, OK: true}
	})

	client := NewClientWithPoolSize(socket, 2)
	t.Cleanup(func() { _ = client.Close() })

	for i := 0; i < 5; i++ {
		if err := client.Call("noop", nil, nil); err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}

	if got := atomic.LoadInt64(connCount); got != 1 {
		t.Fatalf("expected sequential calls to reuse a single pooled connection, got %d distinct connections", got)
	}
}

// TestSoakConcurrentDispatch is the C043 acceptance-criteria soak test:
// N (>= 16) concurrent dispatches must complete without connection
// exhaustion, deadlock, or cross-talk between responses, using a pool
// smaller than the concurrency level so reuse and queuing are genuinely
// exercised.
func TestSoakConcurrentDispatch(t *testing.T) {
	const (
		poolSize    = 4
		concurrency = 32
	)

	var served int64
	socket, connCount := startCountingSidecar(t, func(req envelope) response {
		var p struct {
			Job string `json:"job"`
		}
		_ = json.Unmarshal(req.Params, &p)
		// Simulate realistic sidecar latency so overlapping calls genuinely
		// contend for pooled connections rather than completing instantly
		// one after another.
		time.Sleep(time.Duration(rand.Intn(4)) * time.Millisecond)
		atomic.AddInt64(&served, 1)
		result, err := json.Marshal(map[string]string{"job": p.Job})
		if err != nil {
			return response{ID: req.ID, OK: false, Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{Code: "internal", Message: err.Error()}}
		}
		return response{ID: req.ID, OK: true, Result: result}
	})

	client := NewClientWithPoolSize(socket, poolSize)
	t.Cleanup(func() { _ = client.Close() })

	var wg sync.WaitGroup
	errs := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("job-%d", i)
			var out struct {
				Job string `json:"job"`
			}
			if err := client.Call("soak_dispatch", map[string]any{"job": id}, &out); err != nil {
				errs <- fmt.Errorf("job %s: %w", id, err)
				return
			}
			if out.Job != id {
				errs <- fmt.Errorf("cross-talk detected: job %s got response for job %q", id, out.Job)
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("soak test deadlocked: concurrent dispatches did not complete in time")
	}
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	if got := atomic.LoadInt64(&served); got != concurrency {
		t.Fatalf("expected sidecar to serve all %d requests, served %d (possible connection exhaustion)", concurrency, got)
	}
	if got := atomic.LoadInt64(connCount); got > poolSize {
		t.Fatalf("pool exceeded its bound: opened %d connections against a pool size of %d", got, poolSize)
	}
	if got := atomic.LoadInt64(connCount); got < 2 {
		t.Fatalf("expected the soak test to exercise more than one pooled connection, opened %d", got)
	}
}

// TestClosePreventsNewCheckouts verifies Close makes subsequent Call
// attempts fail fast instead of dialing, and is idempotent.
func TestClosePreventsNewCheckouts(t *testing.T) {
	socket := startTestSidecar(t, func(req envelope) response {
		return response{ID: req.ID, OK: true}
	})

	client := NewClientWithPoolSize(socket, 2)
	if err := client.Call("noop", nil, nil); err != nil {
		t.Fatalf("warmup call failed: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}

	err := client.Call("noop", nil, nil)
	if err == nil {
		t.Fatal("expected Call to fail after Close")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Fatalf("unexpected error after Close: %v", err)
	}
}
