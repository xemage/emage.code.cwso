package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
	"github.com/emage/cwso/orchestrator/internal/transport"
)

// startFakeShadow serves a minimal cwso-git-shadow sidecar that ACKs write_file ops so the
// orchestrator's write_shadow_file tool succeeds (and thus feeds the AST spike monitor).
func startFakeShadow(t *testing.T) string {
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
				for {
					hdr := make([]byte, 4)
					if _, err := c.Read(hdr); err != nil {
						return
					}
					n := binary.BigEndian.Uint32(hdr)
					body := make([]byte, n)
					read := 0
					for read < int(n) {
						nr, err := c.Read(body[read:])
						if err != nil {
							return
						}
						read += nr
					}
					var req struct {
						ID string `json:"id"`
					}
					_ = json.Unmarshal(body, &req)
					resp, _ := json.Marshal(map[string]any{
						"id": req.ID, "ok": true,
						"result": map[string]any{"blob_oid": "b10b", "size": 4},
					})
					out := make([]byte, 4)
					binary.BigEndian.PutUint32(out, uint32(len(resp)))
					if _, err := c.Write(out); err != nil {
						return
					}
					if _, err := c.Write(resp); err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return socket
}

func TestWriteShadowFileFeedsSpikeMonitorEndToEnd(t *testing.T) {
	cfg := &config.Config{
		Transport:                 "stdio",
		LogLevel:                  "error",
		Workspace:                 t.TempDir(),
		AllowedOrigins:            []string{"http://localhost"},
		ShadowSocket:              startFakeShadow(t),
		ASTSpikeMonitorEnabled:    true,
		ASTSpikeWindowMS:          60000,
		ASTSpikeThreshold:         3,
		ASTSpikeDebounceMS:        1,
		ASTSpikeMaxHotPaths:       5,
		ASTSpikeSemanticThreshold: "any",
		ASTSpikeConflictWindowMS:  2000,
		ASTSpikeSignatureTTLMS:    30000,
		ASTSpikeMaxConflictPeers:  8,
	}
	s, err := New(cfg, logging.New("error"))
	if err != nil {
		t.Fatal(err)
	}
	if s.astSink == nil {
		t.Fatal("expected AST write sink to be constructed")
	}

	worker := &transport.Session{Role: "worker"}
	for i := 0; i < 3; i++ {
		raw := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"write_shadow_file","arguments":{"workspace_uuid":"ws-1","path":"pkg/f%d.go","content":"package main // %d"}}}`, i, i, i)
		out, err := s.Handle(context.Background(), worker, []byte(raw))
		if err != nil {
			t.Fatalf("handle write %d: %v", i, err)
		}
		var env map[string]any
		_ = json.Unmarshal(out, &env)
		if env["error"] != nil {
			t.Fatalf("write %d returned error: %s", i, out)
		}
	}

	// The third write crosses the volume threshold → ast/spike on the broker.
	deadline := time.Now().Add(time.Second)
	for {
		if len(s.memory.Query(memorybroker.QueryOptions{Topics: []string{dispatch.TopicASTSpike}})) >= 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for ast/spike from write_shadow_file feeder")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
