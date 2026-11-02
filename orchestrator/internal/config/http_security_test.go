package config

import (
	"reflect"
	"strings"
	"testing"
)

// Synthetic test-only signing keys; never a real secret.
const (
	testSecret32 = "synthetic-test-key-0123456789abc" // exactly 32 bytes
	testSecret31 = "synthetic-test-key-0123456789ab"  // 31 bytes
)

func setHTTPEnv(t *testing.T, secret, origins string) {
	t.Helper()
	t.Setenv("CWSO_TRANSPORT", "http")
	t.Setenv("CWSO_JWT_SECRET", secret)
	t.Setenv("CWSO_ALLOWED_ORIGINS", origins)
}

func TestLoadHTTPRejectsShortJWTSecret(t *testing.T) {
	if len(testSecret31) != MinJWTSecretBytes-1 || len(testSecret32) != MinJWTSecretBytes {
		t.Fatal("test fixtures have the wrong length")
	}
	setHTTPEnv(t, testSecret31, "")
	_, err := Load("")
	if err == nil {
		t.Fatal("expected a 31-byte JWT secret to be rejected")
	}
	if strings.Contains(err.Error(), testSecret31) {
		t.Fatal("error message must not contain the secret")
	}
	if !strings.Contains(err.Error(), "shorter than 32 bytes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadHTTPAcceptsMinimumLengthJWTSecret(t *testing.T) {
	setHTTPEnv(t, testSecret32, "")
	c, err := Load("")
	if err != nil {
		t.Fatalf("expected a 32-byte JWT secret to be accepted: %v", err)
	}
	if c.JWTSecret != testSecret32 {
		t.Fatal("JWT secret not loaded from CWSO_JWT_SECRET")
	}
}

func TestValidateHTTPSecurityChecksFlagSelectedTransport(t *testing.T) {
	// main sets Transport from -transport after Load; the re-check must apply.
	c := &Config{Transport: "http", JWTSecret: testSecret31, AllowedOrigins: []string{"http://localhost"}}
	if err := c.ValidateHTTPSecurity(); err == nil {
		t.Fatal("expected short secret to be rejected after a transport override")
	}
	c.Transport = "stdio"
	if err := c.ValidateHTTPSecurity(); err != nil {
		t.Fatalf("stdio does not use the JWT secret: %v", err)
	}
}

func TestLoadDefaultHTTPAddrIsLoopback(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_HTTP_ADDR", "")
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("expected loopback default, got %q", c.HTTPAddr)
	}
	t.Setenv("CWSO_HTTP_ADDR", ":8080")
	if c, err = Load(""); err != nil || c.HTTPAddr != ":8080" {
		t.Fatalf("CWSO_HTTP_ADDR opt-in not honoured: %v %q", err, c.HTTPAddr)
	}
}

func TestLoadDefaultAllowedOriginsPassValidation(t *testing.T) {
	setHTTPEnv(t, testSecret32, "")
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"http://localhost", "http://127.0.0.1"}
	if !reflect.DeepEqual(c.AllowedOrigins, want) {
		t.Fatalf("got %v, want %v", c.AllowedOrigins, want)
	}
}

func TestLoadNormalizesAllowedOrigins(t *testing.T) {
	setHTTPEnv(t, testSecret32, " HTTP://LocalHost:8080 ,https://cwso.example.com,http://localhost:8080,http://[::1]:8080")
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"http://localhost:8080", "https://cwso.example.com", "http://[::1]:8080"}
	if !reflect.DeepEqual(c.AllowedOrigins, want) {
		t.Fatalf("got %v, want %v", c.AllowedOrigins, want)
	}
}

func TestLoadRejectsInvalidAllowedOrigins(t *testing.T) {
	cases := map[string]string{
		"wildcard":        "http://localhost,*",
		"null":            "NULL",
		"no scheme":       "localhost:8080",
		"bare host":       "cwso.example.com",
		"empty host":      "http://",
		"port only":       "http://:8080",
		"other scheme":    "ftp://cwso.example.com",
		"path":            "https://cwso.example.com/mcp",
		"trailing slash":  "https://cwso.example.com/",
		"query":           "https://cwso.example.com?x=1",
		"fragment":        "https://cwso.example.com#x",
		"userinfo":        "https://user@cwso.example.com",
		"empty port":      "https://cwso.example.com:",
		"port too large":  "https://cwso.example.com:70000",
		"port zero":       "https://cwso.example.com:0",
		"no entries left": " , ,",
	}
	for name, origins := range cases {
		t.Run(name, func(t *testing.T) {
			setHTTPEnv(t, testSecret32, origins)
			if _, err := Load(""); err == nil {
				t.Fatalf("expected CWSO_ALLOWED_ORIGINS=%q to be rejected", origins)
			}
		})
	}
}

func TestLoadRejectsInvalidAllowedOriginsInStdioMode(t *testing.T) {
	t.Setenv("CWSO_TRANSPORT", "stdio")
	t.Setenv("CWSO_ALLOWED_ORIGINS", "*")
	if _, err := Load(""); err == nil {
		t.Fatal("expected wildcard origin to be rejected regardless of transport")
	}
}
