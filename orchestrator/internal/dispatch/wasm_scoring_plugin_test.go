package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
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

func TestNewWasmScoreAdjusterRejectsModuleOutsideTrustedDirectory(t *testing.T) {
	trustedDir := t.TempDir()
	moduleDir := t.TempDir()
	modulePath := filepath.Join(moduleDir, "plugin.wasm")
	if err := os.WriteFile(modulePath, []byte("not-a-real-wasm"), 0o600); err != nil {
		t.Fatalf("write module fixture: %v", err)
	}
	sum := sha256.Sum256([]byte("not-a-real-wasm"))

	_, err := NewWasmScoreAdjuster(context.Background(), WasmScoringConfig{
		Enabled:          true,
		ModulePath:       modulePath,
		ExpectedSHA256:   hex.EncodeToString(sum[:]),
		TrustedModuleDir: trustedDir,
	})
	if err == nil {
		t.Fatal("expected trusted directory enforcement error")
	}
	if !strings.Contains(err.Error(), "outside trusted directory") {
		t.Fatalf("expected trusted directory error, got %v", err)
	}
}

func TestNewWasmScoreAdjusterRejectsSHA256Mismatch(t *testing.T) {
	trustedDir := t.TempDir()
	modulePath := filepath.Join(trustedDir, "plugin.wasm")
	if err := os.WriteFile(modulePath, []byte("not-a-real-wasm"), 0o600); err != nil {
		t.Fatalf("write module fixture: %v", err)
	}

	_, err := NewWasmScoreAdjuster(context.Background(), WasmScoringConfig{
		Enabled:          true,
		ModulePath:       modulePath,
		ExpectedSHA256:   strings.Repeat("0", 64),
		TrustedModuleDir: trustedDir,
	})
	if err == nil {
		t.Fatal("expected module integrity verification failure")
	}
	if !strings.Contains(err.Error(), "integrity check failed") {
		t.Fatalf("expected integrity verification error, got %v", err)
	}
}
