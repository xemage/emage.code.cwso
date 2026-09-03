// Package shadow is the orchestrator-side client for the cwso-git-shadow
// Rust sidecar. The protocol is framed JSON (4-byte big-endian length +
// JSON body) over a Unix Domain Socket.
//
// The client maintains a bounded pool of persistent connections to the
// sidecar (see cwso-git-shadow's handle_client, which loops reading frames
// off a single accepted connection until EOF). Each Call checks out one
// connection from the pool for the duration of its round trip and returns
// it afterward; synchronization is per-connection, not global, so up to
// poolSize RPCs can be in flight against the sidecar concurrently.
package shadow

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	frameHeader = 4
	frameMax    = 8 * 1024 * 1024 // 8 MiB
	dialTimeout = 5 * time.Second
	ioTimeout   = 30 * time.Second

	// defaultPoolSize bounds how many concurrent UDS connections a Client
	// will hold open to the sidecar when no explicit size is given. It can
	// be overridden per-process via CWSO_SHADOW_POOL_SIZE, or per-Client via
	// NewClientWithPoolSize.
	defaultPoolSize = 8

	// poolSizeEnvVar names the environment variable NewClient consults for
	// the default pool size.
	poolSizeEnvVar = "CWSO_SHADOW_POOL_SIZE"
)

// Client is a goroutine-safe, connection-pooled client to a single sidecar
// socket.
type Client struct {
	socket string

	// sem bounds the number of live connections at the configured pool
	// size. A slot is held for the lifetime of a connection (from dial to
	// close), not just for a single Call.
	sem chan struct{}
	// idle holds open connections that are not currently checked out.
	idle chan net.Conn

	closeOnce sync.Once
	closed    chan struct{}
}

// NewClient constructs a client for the given socket path, using a bounded
// connection pool sized by the CWSO_SHADOW_POOL_SIZE environment variable
// (if set to a positive integer) or defaultPoolSize otherwise.
func NewClient(socket string) *Client {
	return NewClientWithPoolSize(socket, poolSizeFromEnv())
}

// NewClientWithPoolSize constructs a client with an explicit bounded
// connection-pool size. size <= 0 falls back to defaultPoolSize.
func NewClientWithPoolSize(socket string, size int) *Client {
	if size <= 0 {
		size = defaultPoolSize
	}
	return &Client{
		socket: socket,
		sem:    make(chan struct{}, size),
		idle:   make(chan net.Conn, size),
		closed: make(chan struct{}),
	}
}

func poolSizeFromEnv() int {
	v := strings.TrimSpace(os.Getenv(poolSizeEnvVar))
	if v == "" {
		return defaultPoolSize
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultPoolSize
	}
	return n
}

// Close closes all idle pooled connections and prevents the client from
// checking out new ones. Calls already in flight are not interrupted and
// will finish normally; their connections are discarded on release rather
// than returned to the pool. Close is safe to call more than once and is
// safe to call concurrently with Call.
func (c *Client) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	for {
		select {
		case conn := <-c.idle:
			_ = conn.Close()
			<-c.sem
		default:
			return nil
		}
	}
}

// acquire checks out a connection from the pool: it reuses an idle
// connection if one is available, dials a new one if the pool has spare
// capacity, or blocks until either becomes available. It never opens more
// than the configured pool size of concurrent connections.
func (c *Client) acquire() (net.Conn, error) {
	// Check closed first (and non-blocking) so a Close that happens-before
	// this call deterministically rejects it, rather than racing against
	// the idle/sem cases below in Go's pseudo-random select.
	select {
	case <-c.closed:
		return nil, errors.New("shadow client closed")
	default:
	}

	select {
	case conn := <-c.idle:
		return conn, nil
	default:
	}

	select {
	case conn := <-c.idle:
		return conn, nil
	case c.sem <- struct{}{}:
		conn, err := net.DialTimeout("unix", c.socket, dialTimeout)
		if err != nil {
			<-c.sem
			return nil, fmt.Errorf("dial %s: %w", c.socket, err)
		}
		return conn, nil
	case <-c.closed:
		return nil, errors.New("shadow client closed")
	}
}

// release returns a connection to the pool for reuse, or — if it is no
// longer usable, or the pool has been closed — closes it and frees its
// pool slot.
func (c *Client) release(conn net.Conn, healthy bool) {
	if !healthy {
		_ = conn.Close()
		<-c.sem
		return
	}
	select {
	case <-c.closed:
		_ = conn.Close()
		<-c.sem
		return
	default:
	}
	select {
	case c.idle <- conn:
	default:
		// idle is at capacity (cap(idle) == cap(sem), so this should not
		// happen in practice); close defensively rather than leak the slot.
		_ = conn.Close()
		<-c.sem
	}
}

// envelope is the wire envelope shared with the Rust sidecar.
type envelope struct {
	ID     string          `json:"id"`
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params,omitempty"`
}

// response is the wire response from the sidecar.
type response struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Call sends one request and waits for the matching reply. It is safe to
// call concurrently: each call checks out its own pooled connection and
// never shares it with another in-flight call, so concurrent Calls cannot
// interleave frames on the wire.
func (c *Client) Call(op string, params any, out any) error {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		raw = b
	}
	env := envelope{ID: newID(), Op: op, Params: raw}
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	conn, err := c.acquire()
	if err != nil {
		return err
	}
	healthy := false
	defer func() { c.release(conn, healthy) }()

	_ = conn.SetDeadline(time.Now().Add(ioTimeout))

	if err := writeFrame(conn, body); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	respBody, err := readFrame(conn)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	// The frame round trip completed cleanly, so the connection's byte
	// stream is in a known-good state and safe to hand back to the pool
	// regardless of what the decoded payload turns out to be.
	healthy = true

	var resp response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if resp.ID != env.ID {
		return fmt.Errorf("cross-talk detected: sent request %s, got response %s", env.ID, resp.ID)
	}
	if !resp.OK {
		if resp.Error != nil {
			return fmt.Errorf("sidecar %s: %s", resp.Error.Code, resp.Error.Message)
		}
		return errors.New("sidecar reported failure with no error body")
	}
	if out != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return fmt.Errorf("decode result: %w", err)
		}
	}
	return nil
}

func writeFrame(w io.Writer, body []byte) error {
	if len(body) > frameMax {
		return fmt.Errorf("frame too large: %d", len(body))
	}
	hdr := make([]byte, frameHeader)
	binary.BigEndian.PutUint32(hdr, uint32(len(body)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func readFrame(r io.Reader) ([]byte, error) {
	hdr := make([]byte, frameHeader)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr)
	if n == 0 || n > frameMax {
		return nil, fmt.Errorf("frame size out of range: %d", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

var idCounter uint64
var idMu sync.Mutex

func newID() string {
	idMu.Lock()
	idCounter++
	id := idCounter
	idMu.Unlock()
	return fmt.Sprintf("orch-%d-%d", time.Now().UnixNano(), id)
}
