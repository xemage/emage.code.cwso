package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/shadow"
)

type recordingSink struct {
	mu     sync.Mutex
	events []dispatch.WriteEvent
}

func (r *recordingSink) ObserveWrite(ev dispatch.WriteEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
	return nil
}

func (r *recordingSink) snapshot() []dispatch.WriteEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]dispatch.WriteEvent, len(r.events))
	copy(out, r.events)
	return out
}

func TestWriteShadowFileFeedsWriteEvent(t *testing.T) {
	socket := startToolSidecar(t, func(req sidecarRequest) sidecarResponse {
		return sidecarResponse{ID: req.ID, OK: true, Result: mustJSON(map[string]any{"blob_oid": "b10b", "size": 12})}
	})
	sink := &recordingSink{}
	tool := NewWriteShadowFileWithObserver(shadow.NewClient(socket), sink)

	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"workspace_uuid":"ws-1","path":"pkg/main.go","content":"package main"}`)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	events := sink.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected 1 write event, got %d", len(events))
	}
	ev := events[0]
	if ev.Workspace != "ws-1" || ev.Path != "pkg/main.go" {
		t.Fatalf("unexpected workspace/path: %+v", ev)
	}
	if ev.Language != "go" {
		t.Fatalf("expected language go, got %q", ev.Language)
	}
	if ev.Symbol != "pkg/main.go" {
		t.Fatalf("expected symbol to default to path, got %q", ev.Symbol)
	}
	want := sha256.Sum256([]byte("package main"))
	if ev.SignatureHash != hex.EncodeToString(want[:]) {
		t.Fatalf("unexpected signature hash %q", ev.SignatureHash)
	}
	if ev.At.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
}

func TestWriteShadowFileFailedWriteDoesNotFeed(t *testing.T) {
	socket := startToolSidecar(t, func(req sidecarRequest) sidecarResponse {
		return sidecarResponse{ID: req.ID, OK: false, Error: map[string]any{"code": "boom", "message": "nope"}}
	})
	sink := &recordingSink{}
	tool := NewWriteShadowFileWithObserver(shadow.NewClient(socket), sink)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"workspace_uuid":"ws-1","path":"a.go","content":"x"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result from failed write")
	}
	if len(sink.snapshot()) != 0 {
		t.Fatal("a failed write must not emit a write event")
	}
}

func TestWriteShadowFileNoObserverIsNoop(t *testing.T) {
	socket := startToolSidecar(t, func(req sidecarRequest) sidecarResponse {
		return sidecarResponse{ID: req.ID, OK: true, Result: mustJSON(map[string]any{"blob_oid": "b10b", "size": 1})}
	})
	tool := NewWriteShadowFile(shadow.NewClient(socket)) // no observer
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"workspace_uuid":"ws-1","path":"a.go","content":"x"}`)); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestLanguageFromPath(t *testing.T) {
	cases := map[string]string{
		"a.go":          "go",
		"dir/b.py":      "python",
		"c.rs":          "rust",
		"d.ts":          "typescript",
		"e.TSX":         "typescript",
		"f.js":          "javascript",
		"g.jsx":         "javascript",
		"README.md":     "",
		"noext":         "",
		"weird.UNKNOWN": "",
	}
	for in, want := range cases {
		if got := languageFromPath(in); got != want {
			t.Errorf("languageFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}
