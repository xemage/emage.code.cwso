package transport

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func makeToken(secret string, claims map[string]any) string {
	hdr, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	body, _ := json.Marshal(claims)
	h := base64.RawURLEncoding.EncodeToString(hdr)
	b := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(h + "." + b))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return h + "." + b + "." + sig
}

func TestVerifyHS256_Success(t *testing.T) {
	secret := "test-secret-32-bytes-minimum-padding-x"
	tok := makeToken(secret, map[string]any{
		"sub":  "alice",
		"role": "worker",
		"exp":  float64(time.Now().Add(1 * time.Hour).Unix()),
	})
	claims, err := verifyHS256(tok, secret)
	if err != nil {
		t.Fatal(err)
	}
	if claims["sub"] != "alice" || claims["role"] != "worker" {
		t.Fatalf("unexpected claims: %v", claims)
	}
}

func TestVerifyHS256_Expired(t *testing.T) {
	secret := "test-secret-32-bytes-minimum-padding-x"
	tok := makeToken(secret, map[string]any{
		"exp": float64(time.Now().Add(-1 * time.Hour).Unix()),
	})
	if _, err := verifyHS256(tok, secret); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestVerifyHS256_BadSignature(t *testing.T) {
	tok := makeToken("secret-a", map[string]any{"sub": "x"})
	if _, err := verifyHS256(tok, "secret-b"); err == nil {
		t.Fatal("expected signature mismatch")
	}
}

func TestVerifyHS256_NoSecret(t *testing.T) {
	if _, err := verifyHS256("a.b.c", ""); err == nil {
		t.Fatal("expected error when secret unset")
	}
}

func TestOriginHostAllowed(t *testing.T) {
	allow := map[string]struct{}{
		"http://localhost": {},
		"http://127.0.0.1": {},
	}
	if !originHostAllowed(allow, "localhost") {
		t.Fatal("localhost should be allowed")
	}
	if originHostAllowed(allow, "evil.example.com") {
		t.Fatal("evil host should not be allowed")
	}
}
