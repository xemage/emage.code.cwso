package dispatch

import (
	"context"
	"strings"
	"testing"
)

func TestNewWasmScoreAdjusterDisabledReturnsNil(t *testing.T) {
	adjuster, err := NewWasmScoreAdjuster(context.Background(), WasmScoringConfig{Enabled: false})
	if err != nil {
		t.Fatalf("expected nil error for disabled wasm scorer, got %v", err)
	}
	if adjuster != nil {
		t.Fatalf("expected nil adjuster when disabled, got %#v", adjuster)
	}
}

func TestNewWasmScoreAdjusterRejectsUnknownHostCall(t *testing.T) {
	_, err := NewWasmScoreAdjuster(context.Background(), WasmScoringConfig{
		Enabled:          true,
		ModulePath:       "does-not-exist.wasm",
		AllowedHostCalls: []string{"fs.read_file"},
	})
	if err == nil {
		t.Fatal("expected error for unknown host call")
	}
	if !strings.Contains(err.Error(), "host call") {
		t.Fatalf("expected host-call validation error, got %v", err)
	}
}
