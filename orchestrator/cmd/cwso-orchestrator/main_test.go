package main

import "testing"

// Synthetic test-only signing keys; never a real secret.
const (
	testSecret32 = "synthetic-test-key-0123456789abc"
	testSecret31 = "synthetic-test-key-0123456789ab"
)

func TestLoadConfigRejectsShortSecretWithTransportFlag(t *testing.T) {
	// CWSO_TRANSPORT says stdio, so Load alone skips the JWT checks; the
	// -transport http flag must not bypass them (SEC-T695-07).
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_JWT_SECRET", testSecret31)
	if _, err := loadConfig("", "http", ""); err == nil {
		t.Fatal("expected a short JWT secret to fail startup with -transport http")
	}
}

func TestLoadConfigAddrDefaultsToLoopback(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_JWT_SECRET", testSecret32)
	t.Setenv("CWSO_HTTP_ADDR", "")
	cfg, err := loadConfig("", "http", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("expected loopback default, got %q", cfg.HTTPAddr)
	}
}

func TestLoadConfigAddrOptIn(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_JWT_SECRET", testSecret32)
	t.Setenv("CWSO_HTTP_ADDR", ":9090")
	cfg, err := loadConfig("", "http", "")
	if err != nil || cfg.HTTPAddr != ":9090" {
		t.Fatalf("CWSO_HTTP_ADDR opt-in not honoured: %v", err)
	}
	cfg, err = loadConfig("", "http", ":8080")
	if err != nil || cfg.HTTPAddr != ":8080" {
		t.Fatalf("-addr flag must win over CWSO_HTTP_ADDR: %v", err)
	}
}
