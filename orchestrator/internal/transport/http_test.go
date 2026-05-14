package transport

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/eventbus"
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

func readSSEFrame(t *testing.T, r *bufio.Reader, timeout time.Duration) (event string, data string, heartbeat bool) {
	t.Helper()
	type result struct {
		event     string
		data      string
		heartbeat bool
		err       error
	}
	ch := make(chan result, 1)
	go func() {
		res := result{}
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				res.err = err
				ch <- res
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				ch <- res
				return
			}
			if strings.HasPrefix(line, ":") {
				res.heartbeat = true
				continue
			}
			if strings.HasPrefix(line, "event: ") {
				res.event = strings.TrimPrefix(line, "event: ")
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				if res.data == "" {
					res.data = strings.TrimPrefix(line, "data: ")
				} else {
					res.data += "\n" + strings.TrimPrefix(line, "data: ")
				}
			}
		}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("read SSE frame: %v", res.err)
		}
		return res.event, res.data, res.heartbeat
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for SSE frame")
		return "", "", false
	}
}

func newSSETestServer(t *testing.T, bus *eventbus.Bus,
	h func(ctx context.Context, sess *Session, raw []byte) ([]byte, error),
) (*httptest.Server, string) {
	t.Helper()
	secret := "test-secret-32-bytes-minimum-padding-x"
	cfg := &config.Config{
		JWTSecret:      secret,
		JWTAlg:         "HS256",
		JWTIssuer:      "cwso",
		JWTAudience:    "cwso-mcp",
		AllowedOrigins: []string{"http://localhost"},
	}
	log := logging.New("error")

	handler := newHTTPHandler(cfg, log, bus, bus, h)
	srv := httptest.NewServer(handler)

	claims := &jwtClaims{
		Role: "worker",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "sse-test",
			Issuer:    "cwso",
			Audience:  jwt.ClaimStrings{"cwso-mcp"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tok := makeJWT(secret, claims)
	return srv, tok
}

func openSSE(t *testing.T, baseURL, token string) (*http.Response, *bufio.Reader) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+"/mcp", nil)
	if err != nil {
		t.Fatalf("new sse request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", "http://localhost")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open sse: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("expected 200 from SSE endpoint, got %d body=%s", resp.StatusCode, string(body))
	}

	return resp, bufio.NewReader(resp.Body)
}

func TestSSEReadyAndHeartbeat(t *testing.T) {
	oldHeartbeat := heartbeatInterval
	heartbeatInterval = 20 * time.Millisecond
	defer func() { heartbeatInterval = oldHeartbeat }()

	bus := eventbus.New()
	srv, token := newSSETestServer(t, bus, func(ctx context.Context, sess *Session, raw []byte) ([]byte, error) {
		return []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`), nil
	})
	defer srv.Close()

	resp, reader := openSSE(t, srv.URL, token)
	defer resp.Body.Close()

	event, data, heartbeat := readSSEFrame(t, reader, 200*time.Millisecond)
	if heartbeat {
		t.Fatal("expected ready frame first, got heartbeat")
	}
	if event != "ready" {
		t.Fatalf("expected ready event, got %q", event)
	}
	if !strings.Contains(data, `"protocolVersion":"2025-03-26"`) {
		t.Fatalf("unexpected ready payload: %s", data)
	}

	_, _, hb := readSSEFrame(t, reader, 200*time.Millisecond)
	if !hb {
		t.Fatal("expected heartbeat frame")
	}
}

func TestSSEBroadcastToTwoSubscribers(t *testing.T) {
	oldHeartbeat := heartbeatInterval
	heartbeatInterval = 500 * time.Millisecond
	defer func() { heartbeatInterval = oldHeartbeat }()

	bus := eventbus.New()
	srv, token := newSSETestServer(t, bus, func(ctx context.Context, sess *Session, raw []byte) ([]byte, error) {
		return []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`), nil
	})
	defer srv.Close()

	resp1, r1 := openSSE(t, srv.URL, token)
	defer resp1.Body.Close()
	resp2, r2 := openSSE(t, srv.URL, token)
	defer resp2.Body.Close()

	_, _, _ = readSSEFrame(t, r1, 200*time.Millisecond)
	_, _, _ = readSSEFrame(t, r2, 200*time.Millisecond)

	start := time.Now()
	if err := bus.Publish(eventbus.TopicNotificationsJobState, map[string]any{"state": "running"}); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	_, data1, _ := readSSEFrame(t, r1, 200*time.Millisecond)
	latency1 := time.Since(start)
	_, data2, _ := readSSEFrame(t, r2, 200*time.Millisecond)

	if latency1 > 100*time.Millisecond {
		t.Fatalf("notification latency exceeded target: %v", latency1)
	}

	var env1 struct {
		JSONRPC string         `json:"jsonrpc"`
		Method  string         `json:"method"`
		Params  map[string]any `json:"params"`
	}
	if err := json.Unmarshal([]byte(data1), &env1); err != nil {
		t.Fatalf("unmarshal sse envelope #1: %v", err)
	}
	if env1.JSONRPC != "2.0" || env1.Method != eventbus.TopicNotificationsJobState {
		t.Fatalf("unexpected sse envelope #1: %s", data1)
	}
	if env1.Params["state"] != "running" {
		t.Fatalf("unexpected params in envelope #1: %v", env1.Params)
	}

	var env2 struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal([]byte(data2), &env2); err != nil {
		t.Fatalf("unmarshal sse envelope #2: %v", err)
	}
	if env2.Method != eventbus.TopicNotificationsJobState {
		t.Fatalf("unexpected sse envelope #2: %s", data2)
	}
}

func TestSSEWithPOSTNoRegressionAndSampleNotifications(t *testing.T) {
	oldHeartbeat := heartbeatInterval
	heartbeatInterval = 500 * time.Millisecond
	defer func() { heartbeatInterval = oldHeartbeat }()

	bus := eventbus.New()
	srv, token := newSSETestServer(t, bus, func(ctx context.Context, sess *Session, raw []byte) ([]byte, error) {
		return []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`), nil
	})
	defer srv.Close()

	resp, reader := openSSE(t, srv.URL, token)
	defer resp.Body.Close()
	_, _, _ = readSSEFrame(t, reader, 200*time.Millisecond) // ready

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ping","arguments":{}}}`)
	postReq, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new post request: %v", err)
	}
	postReq.Header.Set("Authorization", "Bearer "+token)
	postReq.Header.Set("Origin", "http://localhost")
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatalf("post /mcp: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(postResp.Body)
		t.Fatalf("expected 200 from POST /mcp, got %d body=%s", postResp.StatusCode, string(b))
	}
	b, _ := io.ReadAll(postResp.Body)
	if !strings.Contains(string(b), `"result":{"ok":true}`) {
		t.Fatalf("unexpected post response body: %s", string(b))
	}

	seen := map[string]bool{}
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) && !(seen[eventbus.TopicNotificationsLog] && seen[eventbus.TopicNotificationsJobState]) {
		_, data, heartbeat := readSSEFrame(t, reader, 200*time.Millisecond)
		if heartbeat {
			continue
		}
		var env struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal([]byte(data), &env); err != nil {
			t.Fatalf("unmarshal notification envelope: %v", err)
		}
		seen[env.Method] = true
	}

	if !seen[eventbus.TopicNotificationsLog] || !seen[eventbus.TopicNotificationsJobState] {
		t.Fatalf("expected both sample notification topics, got seen=%v", seen)
	}
}
