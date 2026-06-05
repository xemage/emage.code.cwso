// Client talks to the cwso-rollout sidecar over framed JSON UDS (T132).
package rollout

import (
	"context"
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
	frameMax    = 8 * 1024 * 1024
	dialTimeout = 5 * time.Second
	ioTimeout   = 60 * time.Second
)

// SidecarError is a structured error returned by cwso-rollout.
type SidecarError struct {
	Code       string
	ReasonCode string
	Message    string
}

func (e *SidecarError) Error() string {
	if e.ReasonCode != "" {
		return fmt.Sprintf("rollout %s (%s): %s", e.Code, e.ReasonCode, e.Message)
	}
	return fmt.Sprintf("rollout %s: %s", e.Code, e.Message)
}

// CaptureStats is the capture queue snapshot from the sidecar.
type CaptureStats struct {
	Pending int    `json:"pending"`
	Dropped uint64 `json:"dropped"`
}

// Client is a goroutine-safe client to a single cwso-rollout socket.
type Client struct {
	socket string
	mu     sync.Mutex
}

// NewClient constructs a client for the given socket path.
func NewClient(socket string) *Client {
	return &Client{socket: socket}
}

// Stat returns the sidecar service banner (connectivity probe).
func (c *Client) Stat(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.call(ctx, "stat", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CaptureStats returns pending and dropped capture counters.
func (c *Client) CaptureStats(ctx context.Context) (*CaptureStats, error) {
	var out CaptureStats
	if err := c.call(ctx, "capture_stats", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DrainCapture pulls up to limit completion records from the sidecar queue.
func (c *Client) DrainCapture(ctx context.Context, limit uint32) ([]CompletionRecord, error) {
	if limit == 0 {
		limit = 1
	}
	var out struct {
		Records []CompletionRecord `json:"records"`
	}
	params := map[string]uint32{"limit": limit}
	if err := c.call(ctx, "drain_capture", params, &out); err != nil {
		return nil, err
	}
	return out.Records, nil
}

// BuildFromDrain drains capture records and assembles a trajectory group for sessionID.
func (c *Client) BuildFromDrain(ctx context.Context, sessionID string, limit uint32) (TrajectoryGroup, error) {
	records, err := c.DrainCapture(ctx, limit)
	if err != nil {
		return TrajectoryGroup{}, err
	}
	group := BuildTrajectoryGroup(sessionID, records)
	if err := ValidateTrajectoryGroup(group); err != nil {
		return TrajectoryGroup{}, fmt.Errorf("validate trajectory: %w", err)
	}
	return group, nil
}

type requestEnvelope struct {
	ID    string `json:"id"`
	Op    string `json:"op"`
	Limit uint32 `json:"limit,omitempty"`
}

type responseEnvelope struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code       string `json:"code"`
		ReasonCode string `json:"reason_code,omitempty"`
		Message    string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) call(ctx context.Context, op string, params any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	var body []byte
	var err error
	switch op {
	case "drain_capture":
		limit, _ := params.(map[string]uint32)["limit"]
		body, err = json.Marshal(requestEnvelope{ID: newID(), Op: op, Limit: limit})
	default:
		body, err = json.Marshal(requestEnvelope{ID: newID(), Op: op})
	}
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

	deadline := time.Now().Add(ioTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()

	if err := writeFrame(conn, body); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("write: %w", err)
	}
	respBody, err := readFrame(conn)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("read: %w", err)
	}

	var resp responseEnvelope
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !resp.OK {
		if resp.Error != nil {
			return &SidecarError{
				Code:       resp.Error.Code,
				ReasonCode: resp.Error.ReasonCode,
				Message:    resp.Error.Message,
			}
		}
		return errors.New("rollout reported failure with no error body")
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

var (
	idCounter uint64
	idMu      sync.Mutex
)

func newID() string {
	idMu.Lock()
	idCounter++
	id := idCounter
	idMu.Unlock()
	return fmt.Sprintf("rollout-orch-%d-%d", time.Now().UnixNano(), id)
}
