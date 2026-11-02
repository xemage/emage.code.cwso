package transport

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/logging"
)

func TestIsLoopbackAddr(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8080":  true,
		"localhost:8080":  true,
		"[::1]:8080":      true,
		":8080":           false,
		"0.0.0.0:8080":    false,
		"[::]:8080":       false,
		"192.0.2.10:8080": false,
		"not-an-addr":     false,
	}
	for addr, want := range cases {
		if got := isLoopbackAddr(addr); got != want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", addr, got, want)
		}
	}
}

func TestLogListenerRecordsEffectiveOrigins(t *testing.T) {
	var buf bytes.Buffer
	log := logging.NewWithWriter("info", &buf)
	cfg := &config.Config{
		HTTPAddr:       "127.0.0.1:8080",
		AllowedOrigins: []string{"http://localhost", "https://cwso.example.com"},
	}
	logListener(log, cfg)

	var entry struct {
		Msg            string   `json:"msg"`
		Addr           string   `json:"addr"`
		LoopbackOnly   bool     `json:"loopback_only"`
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	if entry.Msg != "http transport listening" || entry.Addr != cfg.HTTPAddr || !entry.LoopbackOnly {
		t.Fatalf("unexpected log entry: %+v", entry)
	}
	if len(entry.AllowedOrigins) != 2 || entry.AllowedOrigins[1] != "https://cwso.example.com" {
		t.Fatalf("effective origin list not logged: %v", entry.AllowedOrigins)
	}
}
