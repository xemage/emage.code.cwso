// Package mergeengine is the orchestrator-side client for the cwso-merge-engine
// sidecar. The protocol is framed JSON (4-byte big-endian length + JSON body)
// over a Unix Domain Socket.
package mergeengine

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	frameHeader = 4
	frameMax    = 8 * 1024 * 1024 // 8 MiB
	dialTimeout = 5 * time.Second
	ioTimeout   = 30 * time.Second
)

// SidecarError is a structured error returned by cwso-merge-engine.
type SidecarError struct {
	Code       string
	Class      string
	ReasonCode string
	Message    string
}

func (e *SidecarError) Error() string {
	return fmt.Sprintf("sidecar %s: %s", e.Code, e.Message)
}

// Client is a goroutine-safe client to a single merge-engine socket.
type Client struct {
	socket string
	mu     sync.Mutex
}

// NewClient constructs a client for the given socket path.
func NewClient(socket string) *Client {
	return &Client{socket: socket}
}

type envelope struct {
	ID     string          `json:"id"`
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params,omitempty"`
}

type response struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code       string `json:"code"`
		Class      string `json:"class,omitempty"`
		ReasonCode string `json:"reason_code,omitempty"`
		Message    string `json:"message"`
	} `json:"error,omitempty"`
}

// Call sends one request and waits for the matching reply.
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

	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := net.DialTimeout("unix", c.socket, dialTimeout)
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.socket, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(ioTimeout))

	if err := writeFrame(conn, body); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	respBody, err := readFrame(conn)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	var resp response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !resp.OK {
		if resp.Error != nil {
			return &SidecarError{
				Code:       resp.Error.Code,
				Class:      resp.Error.Class,
				ReasonCode: resp.Error.ReasonCode,
				Message:    resp.Error.Message,
			}
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
	return fmt.Sprintf("merge-orch-%d-%d", time.Now().UnixNano(), id)
}
