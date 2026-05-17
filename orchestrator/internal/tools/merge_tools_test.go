package tools

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/mergeengine"
)

type mergeSidecarRequest struct {
	ID     string          `json:"id"`
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params"`
}

type mergeSidecarResponse struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  any             `json:"error,omitempty"`
}

func startMergeToolSidecar(t *testing.T, handler func(mergeSidecarRequest) mergeSidecarResponse) string {
	t.Helper()
	socket := t.TempDir() + "/merge.sock"
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
				body, err := readMergeWireFrame(c)
				if err != nil {
					return
				}
				var req mergeSidecarRequest
				if err := json.Unmarshal(body, &req); err != nil {
					return
				}
				resp := handler(req)
				wire, err := json.Marshal(resp)
				if err != nil {
					return
				}
				_ = writeMergeWireFrame(c, wire)
			}(conn)
		}
	}()

	return socket
}

func writeMergeWireFrame(c net.Conn, body []byte) error {
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(body)))
	if _, err := c.Write(hdr); err != nil {
		return err
	}
	_, err := c.Write(body)
	return err
}

func readMergeWireFrame(c net.Conn) ([]byte, error) {
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

func TestMergeConcurrentResultsSuccess(t *testing.T) {
	socket := startMergeToolSidecar(t, func(req mergeSidecarRequest) mergeSidecarResponse {
		if req.Op != "merge_three_way" {
			t.Fatalf("unexpected op: %q", req.Op)
		}
		return mergeSidecarResponse{ID: req.ID, OK: true, Result: mustJSON(map[string]any{
			"merged_b64": base64.StdEncoding.EncodeToString([]byte("package main\nfunc main() {}\n")),
		})}
	})

	tool := NewMergeConcurrentResults(mergeengine.NewClient(socket))
	res, err := tool.Execute(context.Background(), json.RawMessage(`{
		"source_workspace_uuids": ["11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"],
		"merge_inputs": [{
			"path": "main.go",
			"language": "go",
			"base_content": "package main\nfunc main() {\n}\n",
			"ours_content": "package main\nfunc main() {}\n",
			"theirs_content": "package main\nfunc main() {}\n"
		}]
	}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}

	out := decodeMergeResult(t, res)
	if out.Outcome != "success" {
		t.Fatalf("expected success outcome, got %+v", out)
	}
	if out.MergedCount != 1 || out.ConflictCount != 0 || out.FailureCount != 0 {
		t.Fatalf("unexpected counters: %+v", out)
	}
	if out.Results[0].Status != "merged" || out.Results[0].ReasonCode != "semantic_merge_success" {
		t.Fatalf("unexpected result item: %+v", out.Results[0])
	}
}

func TestMergeConcurrentResultsConflict(t *testing.T) {
	socket := startMergeToolSidecar(t, func(req mergeSidecarRequest) mergeSidecarResponse {
		return mergeSidecarResponse{ID: req.ID, OK: false, Error: map[string]any{
			"code":        "merge_conflict",
			"class":       "semantic_conflict",
			"reason_code": "ast_overlap_conflict",
			"message":     "AST semantic overlap conflict",
		}}
	})

	tool := NewMergeConcurrentResults(mergeengine.NewClient(socket))
	res, err := tool.Execute(context.Background(), json.RawMessage(`{
		"source_workspace_uuids": ["11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"],
		"merge_inputs": [{
			"path": "mod.rs",
			"language": "rust",
			"base_content": "fn value() -> i32 { 1 }",
			"ours_content": "fn value() -> i32 { 2 }",
			"theirs_content": "fn value() -> i32 { 3 }"
		}]
	}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}

	out := decodeMergeResult(t, res)
	if out.Outcome != "conflict" {
		t.Fatalf("expected conflict outcome, got %+v", out)
	}
	if out.MergedCount != 0 || out.ConflictCount != 1 || out.FailureCount != 0 {
		t.Fatalf("unexpected counters: %+v", out)
	}
	if out.Results[0].Status != "conflict" || out.Results[0].ReasonCode != "ast_overlap_conflict" {
		t.Fatalf("unexpected conflict item: %+v", out.Results[0])
	}
	if out.Results[0].EscalationClass != "semantic_conflict" || out.Results[0].EscalationAction != "manual_merge_review" {
		t.Fatalf("unexpected escalation mapping: %+v", out.Results[0])
	}
}

func TestMergeConcurrentResultsRuntimeFailure(t *testing.T) {
	tool := NewMergeConcurrentResults(mergeengine.NewClient(t.TempDir() + "/missing.sock"))
	res, err := tool.Execute(context.Background(), json.RawMessage(`{
		"source_workspace_uuids": ["11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"],
		"merge_inputs": [{
			"path": "main.py",
			"language": "python",
			"base_content": "def f():\n    return 1\n",
			"ours_content": "def f():\n    return 2\n",
			"theirs_content": "def f():\n    return 3\n"
		}]
	}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected top-level error: %+v", res)
	}

	out := decodeMergeResult(t, res)
	if out.Outcome != "error" {
		t.Fatalf("expected error outcome, got %+v", out)
	}
	if out.FailureCount != 1 {
		t.Fatalf("expected one failure, got %+v", out)
	}
	if out.Results[0].ReasonCode != "merge_engine_unavailable" {
		t.Fatalf("unexpected reason code: %+v", out.Results[0])
	}
	if out.Results[0].EscalationClass != "runtime_error" || out.Results[0].EscalationAction != "retry_or_investigate_runtime" {
		t.Fatalf("unexpected runtime escalation mapping: %+v", out.Results[0])
	}
}

func TestMapToolMergeErrorMatrix(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantStatus     string
		wantClass      string
		wantReasonCode string
		wantAction     string
	}{
		{
			name: "semantic conflict class from sidecar metadata",
			err: &mergeengine.SidecarError{
				Code:       "merge_conflict",
				Class:      "semantic_conflict",
				ReasonCode: "ast_overlap_conflict",
				Message:    "AST semantic overlap conflict",
			},
			wantStatus:     "conflict",
			wantClass:      "semantic_conflict",
			wantReasonCode: "ast_overlap_conflict",
			wantAction:     "manual_merge_review",
		},
		{
			name: "legacy semantic conflict fallback",
			err: &mergeengine.SidecarError{
				Code:    "unimplemented_conflict",
				Message: "legacy conflict",
			},
			wantStatus:     "conflict",
			wantClass:      "semantic_conflict",
			wantReasonCode: "semantic_conflict",
			wantAction:     "manual_merge_review",
		},
		{
			name: "policy conflict",
			err: &mergeengine.SidecarError{
				Code:       "invalid_input",
				Class:      "policy_conflict",
				ReasonCode: "invalid_payload_encoding",
				Message:    "bad payload",
			},
			wantStatus:     "error",
			wantClass:      "policy_conflict",
			wantReasonCode: "invalid_payload_encoding",
			wantAction:     "fix_input_and_retry",
		},
		{
			name: "runtime fallback",
			err: &mergeengine.SidecarError{
				Code:    "sidecar_timeout",
				Message: "timeout",
			},
			wantStatus:     "error",
			wantClass:      "runtime_error",
			wantReasonCode: "merge_engine_error",
			wantAction:     "retry_or_investigate_runtime",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := &mergeResultItem{}
			mapToolMergeError(item, tc.err)
			if item.Status != tc.wantStatus {
				t.Fatalf("status mismatch: got=%q want=%q", item.Status, tc.wantStatus)
			}
			if item.EscalationClass != tc.wantClass {
				t.Fatalf("class mismatch: got=%q want=%q", item.EscalationClass, tc.wantClass)
			}
			if item.ReasonCode != tc.wantReasonCode {
				t.Fatalf("reason mismatch: got=%q want=%q", item.ReasonCode, tc.wantReasonCode)
			}
			if item.EscalationAction != tc.wantAction {
				t.Fatalf("action mismatch: got=%q want=%q", item.EscalationAction, tc.wantAction)
			}
		})
	}
}

func TestMergeConcurrentResultsRequiresMergeInputs(t *testing.T) {
	tool := NewMergeConcurrentResults(mergeengine.NewClient(t.TempDir() + "/unused.sock"))
	res, err := tool.Execute(context.Background(), json.RawMessage(`{
		"source_workspace_uuids": ["11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"]
	}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].Text, "merge_inputs requires") {
		t.Fatalf("expected merge_inputs validation error, got %+v", res)
	}
}

func TestMergeConcurrentResultsSchemaRequiresMergeInputs(t *testing.T) {
	tool := NewMergeConcurrentResults(mergeengine.NewClient(t.TempDir() + "/unused.sock"))
	schema := tool.InputSchema()
	requiredRaw, ok := schema["required"]
	if !ok {
		t.Fatalf("schema missing required field list: %+v", schema)
	}

	required, ok := requiredRaw.([]string)
	if !ok {
		t.Fatalf("schema required field list has unexpected type: %T", requiredRaw)
	}

	hasWorkspace := false
	hasMergeInputs := false
	for _, field := range required {
		if field == "source_workspace_uuids" {
			hasWorkspace = true
		}
		if field == "merge_inputs" {
			hasMergeInputs = true
		}
	}

	if !hasWorkspace || !hasMergeInputs {
		t.Fatalf("schema required fields mismatch: %+v", required)
	}
}

func decodeMergeResult(t *testing.T, res *mcp.ToolCallResult) mergeConcurrentOutput {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("expected single content block, got %+v", res.Content)
	}
	var out mergeConcurrentOutput
	if err := json.Unmarshal([]byte(res.Content[0].Text), &out); err != nil {
		t.Fatalf("unmarshal merge result: %v; text=%s", err, res.Content[0].Text)
	}
	return out
}
