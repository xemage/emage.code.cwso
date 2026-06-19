// Package sparse is the orchestrator-side client for the cwso-sparse sidecar (sparse Wasm
// micro-agent tier). Wire format matches cwso-hal: framed JSON over a Unix Domain Socket.
package sparse

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

// SidecarError is a structured error returned by cwso-sparse.
type SidecarError struct {
	Code       string
	ReasonCode string
	Message    string
}

func (e *SidecarError) Error() string {
	if e.ReasonCode != "" {
		return fmt.Sprintf("sparse %s (%s): %s", e.Code, e.ReasonCode, e.Message)
	}
	return fmt.Sprintf("sparse %s: %s", e.Code, e.Message)
}

// CreateAgentParams is the wire shape for the create_agent IPC op.
type CreateAgentParams struct {
	SkillDomain   string  `json:"skill_domain"`
	Quantization  string  `json:"quantization,omitempty"`
	MaxRAMMB      uint32  `json:"max_ram_mb"`
	TargetASTNode *string `json:"target_ast_node,omitempty"`
}

// CreateAgentResult is returned when an ephemeral sparse agent is created.
type CreateAgentResult struct {
	WasmAgentID   string  `json:"wasm_agent_id"`
	SkillDomain   string  `json:"skill_domain"`
	SliceSHA256   string  `json:"slice_sha256"`
	State         string  `json:"state"`
	ColdStartMS   float64 `json:"cold_start_ms"`
	ResidentRAMMB float64 `json:"resident_ram_mb"`
	TokensPerSec  float64 `json:"tokens_per_sec"`
	TargetASTNode *string `json:"target_ast_node,omitempty"`
}

// AgentStatResult is the live telemetry snapshot for one agent.
type AgentStatResult struct {
	WasmAgentID   string  `json:"wasm_agent_id"`
	SkillDomain   string  `json:"skill_domain"`
	SliceSHA256   string  `json:"slice_sha256"`
	State         string  `json:"state"`
	ColdStartMS   float64 `json:"cold_start_ms"`
	ResidentRAMMB float64 `json:"resident_ram_mb"`
	TokensPerSec  float64 `json:"tokens_per_sec"`
	TargetASTNode *string `json:"target_ast_node,omitempty"`
}

// Client is a goroutine-safe client to a single cwso-sparse socket.
type Client struct {
	socket string
	mu     sync.Mutex
}

// NewClient constructs a client for the given socket path.
func NewClient(socket string) *Client {
	return &Client{socket: socket}
}

// CreateAgent resolves a skill slice, instantiates the wasmtime sandbox, and returns metrics.
func (c *Client) CreateAgent(ctx context.Context, p CreateAgentParams) (*CreateAgentResult, error) {
	var out CreateAgentResult
	if err := c.Call(ctx, "create_agent", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DropAgent removes a tracked agent from the sidecar.
func (c *Client) DropAgent(ctx context.Context, agentID string) error {
	return c.Call(ctx, "drop_agent", map[string]string{"agent_id": agentID}, nil)
}

// AgentStat returns the current sidecar snapshot for an agent.
func (c *Client) AgentStat(ctx context.Context, agentID string) (*AgentStatResult, error) {
	var out AgentStatResult
	if err := c.Call(ctx, "agent_stat", map[string]string{"agent_id": agentID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stat returns the sidecar service banner (connectivity probe).
func (c *Client) Stat() (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.Call(context.Background(), "stat", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
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
		ReasonCode string `json:"reason_code,omitempty"`
		Message    string `json:"message"`
	} `json:"error,omitempty"`
}

// Call sends one request and waits for the matching reply, bounded by ctx.
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

	var resp response
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
		return errors.New("sparse reported failure with no error body")
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
	return fmt.Sprintf("sparse-orch-%d-%d", time.Now().UnixNano(), id)
}
