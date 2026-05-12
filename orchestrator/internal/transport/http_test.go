package transport

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

func makeJWT(secret string, claims *jwtClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secret))
	return tokenString
}

func TestVerifyJWT_ValidHS256(t *testing.T) {
	secret := "test-secret-32-bytes-minimum-padding-x"
	cfg := &config.Config{
		JWTSecret:   secret,
		JWTAlg:      "HS256",
		JWTIssuer:   "cwso",
		JWTAudience: "cwso-mcp",
	}
	log := logging.New("debug")

	claims := &jwtClaims{
		Role: "worker",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "alice",
			Issuer:    "cwso",
			Audience:  jwt.ClaimStrings{"cwso-mcp"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tok := makeJWT(secret, claims)

	verified, err := verifyJWT(tok, cfg, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verified.Subject != "alice" || verified.Role != "worker" {
		t.Fatalf("unexpected claims: %+v", verified)
	}
}

func TestVerifyJWT_Expired(t *testing.T) {
	secret := "test-secret-32-bytes-minimum-padding-x"
	cfg := &config.Config{
		JWTSecret:   secret,
		JWTAlg:      "HS256",
		JWTIssuer:   "cwso",
		JWTAudience: "cwso-mcp",
	}
	log := logging.New("debug")

	claims := &jwtClaims{
		Role: "worker",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	tok := makeJWT(secret, claims)

	_, err := verifyJWT(tok, cfg, log)
	if err == nil || err.Error() != "token expired" {
		t.Fatalf("expected 'token expired' error, got: %v", err)
	}
}

func TestVerifyJWT_WrongIssuer(t *testing.T) {
	secret := "test-secret-32-bytes-minimum-padding-x"
	cfg := &config.Config{
		JWTSecret:   secret,
		JWTAlg:      "HS256",
		JWTIssuer:   "cwso",
		JWTAudience: "cwso-mcp",
	}
	log := logging.New("debug")

	claims := &jwtClaims{
		Role: "worker",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "wrong-issuer",
			Audience:  jwt.ClaimStrings{"cwso-mcp"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tok := makeJWT(secret, claims)

	_, err := verifyJWT(tok, cfg, log)
	if err == nil || err.Error() != "invalid issuer: expected \"cwso\", got \"wrong-issuer\"" {
		t.Fatalf("expected issuer validation error, got: %v", err)
	}
}

func TestVerifyJWT_WrongAudience(t *testing.T) {
	secret := "test-secret-32-bytes-minimum-padding-x"
	cfg := &config.Config{
		JWTSecret:   secret,
		JWTAlg:      "HS256",
		JWTIssuer:   "cwso",
		JWTAudience: "cwso-mcp",
	}
	log := logging.New("debug")

	claims := &jwtClaims{
		Role: "worker",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cwso",
			Audience:  jwt.ClaimStrings{"wrong-audience"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tok := makeJWT(secret, claims)

	_, err := verifyJWT(tok, cfg, log)
	if err == nil || err.Error() != "invalid audience: expected \"cwso-mcp\"" {
		t.Fatalf("expected audience validation error, got: %v", err)
	}
}

func TestVerifyJWT_WrongAlgorithm(t *testing.T) {
	secret := "test-secret-32-bytes-minimum-padding-x"
	cfg := &config.Config{
		JWTSecret:   secret,
		JWTAlg:      "HS256",
		JWTIssuer:   "cwso",
		JWTAudience: "cwso-mcp",
	}
	log := logging.New("debug")

	// Create token with RS256 (wrong algorithm)
	hdr, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	body, _ := json.Marshal(map[string]any{"sub": "alice", "role": "worker"})
	h := base64.RawURLEncoding.EncodeToString(hdr)
	b := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(h + "." + b))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	tok := h + "." + b + "." + sig

	_, err := verifyJWT(tok, cfg, log)
	if err == nil {
		t.Fatalf("expected algorithm mismatch error, got nil")
	}
}

func TestVerifyJWT_NotBeforeClaimInFuture(t *testing.T) {
	secret := "test-secret-32-bytes-minimum-padding-x"
	cfg := &config.Config{
		JWTSecret:   secret,
		JWTAlg:      "HS256",
		JWTIssuer:   "cwso",
		JWTAudience: "cwso-mcp",
	}
	log := logging.New("debug")

	claims := &jwtClaims{
		Role: "worker",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cwso",
			Audience:  jwt.ClaimStrings{"cwso-mcp"},
			NotBefore: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(3 * time.Hour)),
		},
	}
	tok := makeJWT(secret, claims)

	_, err := verifyJWT(tok, cfg, log)
	if err == nil || err.Error() != "token used before valid (nbf)" {
		t.Fatalf("expected 'token used before valid' error, got: %v", err)
	}
}

func TestVerifyJWT_Leeway(t *testing.T) {
	// Test that 60s leeway allows slightly expired tokens
	secret := "test-secret-32-bytes-minimum-padding-x"
	cfg := &config.Config{
		JWTSecret:   secret,
		JWTAlg:      "HS256",
		JWTIssuer:   "cwso",
		JWTAudience: "cwso-mcp",
	}
	log := logging.New("debug")

	// Token expired 30s ago (within 60s leeway)
	claims := &jwtClaims{
		Role: "worker",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cwso",
			Audience:  jwt.ClaimStrings{"cwso-mcp"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-30 * time.Second)),
		},
	}
	tok := makeJWT(secret, claims)

	// Should succeed due to 60s leeway
	verified, err := verifyJWT(tok, cfg, log)
	if err != nil {
		t.Fatalf("should accept token within leeway, got error: %v", err)
	}
	if verified == nil {
		t.Fatal("expected verified claims")
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

func TestRateLimitMiddleware_AllowsFirstRequest(t *testing.T) {
	store := &rateLimiterStore{
		limiters: make(map[string]*rate.Limiter),
	}
	log := logging.New("debug")
	limiter := rateLimitMiddleware(store, log)

	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.RemoteAddr = "127.0.0.1:8000"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_EnforcesLimit(t *testing.T) {
	store := &rateLimiterStore{
		limiters: make(map[string]*rate.Limiter),
	}
	log := logging.New("debug")
	limiter := rateLimitMiddleware(store, log)

	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should succeed
	req1 := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req1.RemoteAddr = "127.0.0.1:8000"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected first request to succeed with 200, got %d", w1.Code)
	}

	// Second request should be rate limited (burst=1)
	req2 := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req2.RemoteAddr = "127.0.0.1:8000"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be rate limited with 429, got %d", w2.Code)
	}
	if w2.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected Retry-After header, got: %s", w2.Header().Get("Retry-After"))
	}
}

func TestRateLimitMiddleware_PerIP(t *testing.T) {
	store := &rateLimiterStore{
		limiters: make(map[string]*rate.Limiter),
	}
	log := logging.New("debug")
	limiter := rateLimitMiddleware(store, log)

	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request from IP1 succeeds
	req1 := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req1.RemoteAddr = "192.168.1.1:8000"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected IP1 request to succeed with 200, got %d", w1.Code)
	}

	// Request from IP2 should also succeed (different bucket)
	req2 := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req2.RemoteAddr = "192.168.1.2:8000"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected IP2 request to succeed with 200, got %d", w2.Code)
	}
}

func TestRateLimitMiddleware_SkipsGET(t *testing.T) {
	store := &rateLimiterStore{
		limiters: make(map[string]*rate.Limiter),
	}
	log := logging.New("debug")
	limiter := rateLimitMiddleware(store, log)

	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// GET requests should not be rate limited
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.RemoteAddr = "127.0.0.1:8000"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected GET to skip rate limiting, got %d", w.Code)
	}
}
