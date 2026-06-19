package tools

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/shadow"
)

type sidecarRequest struct {
	ID     string          `json:"id"`
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params"`
}

type sidecarResponse struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  any             `json:"error,omitempty"`
}

func startToolSidecar(t *testing.T, handler func(sidecarRequest) sidecarResponse) string {
	t.Helper()
	socket := t.TempDir() + "/shadow.sock"
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
			go func(c net.Conn) {
				defer c.Close()
				body, err := readWireFrame(c)
				if err != nil {
					return
				}
				var req sidecarRequest
				if err := json.Unmarshal(body, &req); err != nil {
					return
				}
				resp := handler(req)
				wire, err := json.Marshal(resp)
				if err != nil {
					return
				}
				_ = writeWireFrame(c, wire)
			}(conn)
		}
	}()

	return socket
}

func writeWireFrame(c net.Conn, body []byte) error {
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(body)))
	if _, err := c.Write(hdr); err != nil {
		return err
	}
	_, err := c.Write(body)
	return err
}

func readWireFrame(c net.Conn) ([]byte, error) {
	hdr := make([]byte, 4)
	if _, err := c.Read(hdr); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr)
	body := make([]byte, n)
	read := 0
	for read < int(n) {
		nr, err := c.Read(body[read:])
		if err != nil {
			return nil, err
		}
		read += nr
	}
	return body, nil
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestCreateShadowWorkspaceExecute(t *testing.T) {
	socket := startToolSidecar(t, func(req sidecarRequest) sidecarResponse {
		if req.Op != "create_workspace" {
			t.Fatalf("unexpected op: %q", req.Op)
		}
		var p map[string]any
		if err := json.Unmarshal(req.Params, &p); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if p["base_commit_sha"] != "abc123" {
			t.Fatalf("unexpected base_commit_sha: %#v", p["base_commit_sha"])
		}
		return sidecarResponse{ID: req.ID, OK: true, Result: mustJSON(map[string]any{
			"workspace_uuid": "ws-1",
			"base_tree_oid":  "deadbeef",
		})}
	})

	tool := NewCreateShadowWorkspace(shadow.NewClient(socket))
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"base_commit_sha":"abc123"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %+v", res)
	}
	if len(res.Content) != 1 || !strings.Contains(res.Content[0].Text, `"workspace_uuid":"ws-1"`) {
		t.Fatalf("unexpected content: %+v", res.Content)
	}
}

func TestReadShadowFileExecute(t *testing.T) {
	socket := startToolSidecar(t, func(req sidecarRequest) sidecarResponse {
		if req.Op != "read_file" {
			t.Fatalf("unexpected op: %q", req.Op)
		}
		return sidecarResponse{ID: req.ID, OK: true, Result: mustJSON(map[string]any{
			"content_b64": base64.StdEncoding.EncodeToString([]byte("hello from sidecar")),
			"size":        18,
		})}
	})

	tool := NewReadShadowFile(shadow.NewClient(socket))
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"workspace_uuid":"ws-1","path":"main.go"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
	if got := res.Content[0].Text; got != "hello from sidecar" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestReadShadowFileInvalidArgs(t *testing.T) {
	tool := NewReadShadowFile(shadow.NewClient("/tmp/unused.sock"))
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"workspace_uuid":"","path":""}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "workspace_uuid and path are required") {
		t.Fatalf("expected argument validation error, got: %+v", res)
	}
}

func TestWriteShadowFileAllowedRoles(t *testing.T) {
	tool := NewWriteShadowFile(shadow.NewClient("/tmp/unused.sock"))
	roles := tool.AllowedRoles()
	if len(roles) != 1 || roles[0] != RoleWorker {
		t.Fatalf("expected worker-only role, got: %+v", roles)
	}
}

func TestWriteShadowFileExecute(t *testing.T) {
	socket := startToolSidecar(t, func(req sidecarRequest) sidecarResponse {
		if req.Op != "write_file" {
			t.Fatalf("unexpected op: %q", req.Op)
		}
		var p struct {
			ContentB64 string `json:"content_b64"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		decoded, err := base64.StdEncoding.DecodeString(p.ContentB64)
		if err != nil {
			t.Fatalf("decode content_b64: %v", err)
		}
		if string(decoded) != "package main" {
			t.Fatalf("unexpected payload: %q", decoded)
		}
		return sidecarResponse{ID: req.ID, OK: true, Result: mustJSON(map[string]any{
			"blob_oid": "b10b",
			"size":     len(decoded),
		})}
	})

	tool := NewWriteShadowFile(shadow.NewClient(socket))
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"workspace_uuid":"ws-1","path":"main.go","content":"package main"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "wrote 12 bytes") {
		t.Fatalf("unexpected text: %q", res.Content[0].Text)
	}
}

func TestCommitShadowRequiresMessage(t *testing.T) {
	tool := NewCommitShadow(shadow.NewClient("/tmp/unused.sock"))
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"workspace_uuid":"ws-1"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "workspace_uuid and message are required") {
		t.Fatalf("expected validation error, got: %+v", res)
	}
}

func TestQueryASTExecute(t *testing.T) {
	socket := startToolSidecar(t, func(req sidecarRequest) sidecarResponse {
		if req.Op != "query_ast" {
			t.Fatalf("unexpected op: %q", req.Op)
		}
		return sidecarResponse{ID: req.ID, OK: true, Result: json.RawMessage(`[{"kind":"definition","name":"main","line":1}]`)}
	})

	tool := NewQueryAST(shadow.NewClient(socket))
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"workspace_uuid":"ws-1","path":"main.go","query_type":"find_definition","target_symbol":"main"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %+v", res)
	}
	if got := res.Content[0].Text; !strings.Contains(got, `"kind":"definition"`) {
		t.Fatalf("unexpected query result: %q", got)
	}
}
