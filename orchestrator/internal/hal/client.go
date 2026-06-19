// Package hal is the orchestrator-side client for the cwso-hal sidecar (the Hardware
// Abstraction Layer). The protocol is framed JSON (4-byte big-endian length + JSON body)
// over a Unix Domain Socket, identical to the other CWSO sidecars.
package hal

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
	frameMax    = 8 * 1024 * 1024 // 8 MiB
	dialTimeout = 5 * time.Second
	ioTimeout   = 60 * time.Second
)

// SidecarError is a structured error returned by cwso-hal.
type SidecarError struct {
	Code       string
	Class      string
	ReasonCode string
	Message    string
}

func (e *SidecarError) Error() string {
	return fmt.Sprintf("hal %s: %s", e.Code, e.Message)
}

// InferenceRequest mirrors the cwso-hal `InferenceRequest` wire shape.
type InferenceRequest struct {
	RequestID       string   `json:"request_id,omitempty"`
	WorkloadTags    []string `json:"workload_tags,omitempty"`
	Prompt          string   `json:"prompt"`
	ContextTokens   int      `json:"context_tokens"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	LatencyClass    string   `json:"latency_class,omitempty"`
}

// Completion mirrors the cwso-hal `Completion` wire shape.
type Completion struct {
	ProviderID    string `json:"provider_id"`
	Output        string `json:"output"`
	TokensIn      int    `json:"tokens_in"`
	TokensOut     int    `json:"tokens_out"`
	LatencyMS     int64  `json:"latency_ms"`
	Deterministic bool   `json:"deterministic"`
}

// Attempt records one provider attempt within a dispatch.
type Attempt struct {
	ProviderID string `json:"provider_id"`
	Outcome    string `json:"outcome"`
}

// InferResult is the cwso-hal `infer` result envelope.
type InferResult struct {
	ServedBy          string     `json:"served_by"`
	RequestedProvider string     `json:"requested_provider"`
	FallbackCount     int        `json:"fallback_count"`
	Completion        Completion `json:"completion"`
	Attempts          []Attempt  `json:"attempts"`
}

type inferParams struct {
	SelectedProvider string           `json:"selected_provider"`
	FallbackChain    []string         `json:"fallback_chain"`
	Request          InferenceRequest `json:"request"`
}

// Client is a goroutine-safe client to a single cwso-hal socket.
type Client struct {
	socket string
	mu     sync.Mutex
}

// NewClient constructs a client for the given socket path.
func NewClient(socket string) *Client {
	return &Client{socket: socket}
}

// Infer dispatches one inference request to the selected provider, passing the ranked
// fallback chain so the HAL can fall back deterministically (terminating at cpu-baseline).
// The context bounds the call: if it is cancelled or its deadline passes, the in-flight
// request is aborted and the context error is returned.
func (c *Client) Infer(ctx context.Context, providerID string, fallbackChain []string, req InferenceRequest) (*InferResult, error) {
	var out InferResult
	if err := c.Call(ctx, "infer", inferParams{
		SelectedProvider: providerID,
		FallbackChain:    fallbackChain,
		Request:          req,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stat returns the sidecar service banner (used as a connectivity probe).
func (c *Client) Stat() (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.Call(context.Background(), "stat", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Capability mirrors the cwso-hal `ProviderCapability` wire shape (field-aligned with the
// Go dispatch.ProviderCapability, snake_case on the wire).
type Capability struct {
	ProviderID            string   `json:"provider_id"`
	ContractVersion       string   `json:"contract_version"`
	HealthState           string   `json:"health_state"`
	LatencyClass          string   `json:"latency_class"`
	CostClass             string   `json:"cost_class"`
	QueueDepth            int      `json:"queue_depth"`
	SupportedWorkloadTags []string `json:"supported_workload_tags"`
	ReliabilityClass      string   `json:"reliability_class"`
	FeatureFlags          []string `json:"feature_flags"`
}

// Capabilities returns the live capability records advertised by the HAL's registered
// backends (used to keep the control-plane capability registry in sync).
func (c *Client) Capabilities() ([]Capability, error) {
	var out struct {
		Providers []Capability `json:"providers"`
	}
	if err := c.Call(context.Background(), "capabilities", nil, &out); err != nil {
		return nil, err
	}
	return out.Providers, nil
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

// Call sends one request and waits for the matching reply, bounded by ctx. Cancelling ctx
// (or hitting its deadline) closes the connection to unblock any in-flight I/O and returns
// the context error.
func (c *Client) Call(ctx context.Context, op string, params any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
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

	// Bound I/O by the smaller of the fixed ceiling and the context deadline.
	deadline := time.Now().Add(ioTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	// Close the connection on cancellation so a blocked read/write returns promptly.
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
		return errors.New("hal reported failure with no error body")
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
	return fmt.Sprintf("hal-orch-%d-%d", time.Now().UnixNano(), id)
}
