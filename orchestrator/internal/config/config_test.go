package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Transport != "stdio" {
		t.Fatalf("expected stdio, got %s", c.Transport)
	}
}

func TestLoadHTTPRequiresJWTSecret(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "http")
	t.Setenv("CWSO_JWT_SECRET", "")
	if _, err := Load(""); err == nil {
		t.Fatal("expected error when http transport has no JWT secret")
	}
}

func TestLoadInvalidTransport(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "carrier-pigeon")
	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid transport error")
	}
}
