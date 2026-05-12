package transport

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/logging"
)

// RunHTTP starts the Streamable HTTP transport per MCP spec 2025-03-26.
//
// Endpoints:
//
//	POST /mcp     — JSON-RPC request, immediate response (or 202 + UUID in Phase 3)
//	GET  /mcp     — opens a Server-Sent Events stream for server→client notifications (Phase 3)
//	GET  /healthz — liveness
//
// Security (FR-7.1, FR-7.2):
//
//   - Origin allow-list validated on every request (DNS-rebinding protection)
//   - JWT (HS256) Bearer token required on /mcp endpoints
//
// POC-DEBT: SSE GET endpoint is registered but only emits a heartbeat in
// Phase 1. Real notifications land in Phase 3 (T030). Tracked in
// POC-DEBT-SCORECARD-phase1.md.
func RunHTTP(ctx context.Context, cfg *config.Config, log *logging.Logger,
	h func(ctx context.Context, sess *Session, raw []byte) ([]byte, error),
) error {

	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	originSet := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		originSet[strings.ToLower(strings.TrimSpace(o))] = struct{}{}
	}

	mw := chain(
		recoverMiddleware(log),
		requestIDMiddleware(),
		originMiddleware(originSet, log),
	)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	mux.Handle("/mcp", mw(authMiddleware(cfg.JWTSecret, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlePOST(w, r, log, h)
		case http.MethodGet:
			handleSSE(w, r, log)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	errCh := make(chan error, 1)
	go func() {
		log.Info().Str("addr", cfg.HTTPAddr).Msg("http transport listening")
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func handlePOST(w http.ResponseWriter, r *http.Request, log *logging.Logger,
	h func(ctx context.Context, sess *Session, raw []byte) ([]byte, error),
) {
	const maxBody = 8 << 20
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(body) > maxBody {
		http.Error(w, "request body exceeds 8MiB limit", http.StatusRequestEntityTooLarge)
		return
	}

	sess, _ := r.Context().Value(sessionCtxKey{}).(*Session)
	resp, err := h(r.Context(), sess, body)
	if err != nil {
		log.Error().Err(err).Msg("handler error")
		http.Error(w, "handler error", http.StatusInternalServerError)
		return
	}
	if resp == nil {
		// Notification — per MCP spec, return 202 Accepted with no body.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp)
}

func handleSSE(w http.ResponseWriter, r *http.Request, log *logging.Logger) {
	// POC-DEBT: SSE only emits heartbeats in Phase 1. Phase 3 (T030) wires
	// the EventBus → SSE bridge for real telemetry.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	_, _ = fmt.Fprintf(w, "event: ready\ndata: {\"protocolVersion\":\"2025-03-26\"}\n\n")
	flusher.Flush()
	log.Debug().Msg("SSE client connected")

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// --- middleware ---

type sessionCtxKey struct{}
type requestIDCtxKey struct{}

type middleware func(http.Handler) http.Handler

func chain(mws ...middleware) middleware {
	return func(next http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func recoverMiddleware(log *logging.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error().Any("panic", rec).Msg("panic recovered")
					http.Error(w, "internal error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

var ridCounter struct {
	mu sync.Mutex
	n  uint64
}

func nextRequestID() string {
	ridCounter.mu.Lock()
	defer ridCounter.mu.Unlock()
	ridCounter.n++
	return fmt.Sprintf("rid-%d-%d", time.Now().UnixNano(), ridCounter.n)
}

func requestIDMiddleware() middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get("X-Request-ID")
			if rid == "" {
				rid = nextRequestID()
			}
			w.Header().Set("X-Request-ID", rid)
			ctx := context.WithValue(r.Context(), requestIDCtxKey{}, rid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func originMiddleware(allowed map[string]struct{}, log *logging.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.ToLower(r.Header.Get("Origin"))
			if origin == "" {
				// MCP local clients (curl, mcp-inspector) may omit Origin.
				// We still require a Host header match against allow-list
				// to prevent DNS rebinding.
				host := strings.ToLower(strings.SplitN(r.Host, ":", 2)[0])
				if !originHostAllowed(allowed, host) {
					log.Warn().Str("host", host).Msg("origin/host not allowed")
					http.Error(w, "forbidden origin", http.StatusForbidden)
					return
				}
			} else if _, ok := allowed[origin]; !ok {
				log.Warn().Str("origin", origin).Msg("origin not allowed")
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func originHostAllowed(allowed map[string]struct{}, host string) bool {
	for o := range allowed {
		// allowed entries look like "http://localhost" or "https://example.com"
		i := strings.Index(o, "://")
		if i < 0 {
			continue
		}
		h := o[i+3:]
		h = strings.SplitN(h, ":", 2)[0]
		if h == host {
			return true
		}
	}
	return false
}

// --- minimal HS256 JWT verification (Phase 1 PoC) ---
//
// POC-DEBT: Hand-rolled HS256 verifier; production must adopt
// github.com/golang-jwt/jwt/v5 with RS256, key rotation, and proper claims
// validation (iss, aud, exp leeway). Tracked in POC-DEBT-SCORECARD-phase1.md.

func authMiddleware(secret string, log *logging.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(authz, prefix) {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(authz, prefix)
			claims, err := verifyHS256(token, secret)
			if err != nil {
				log.Warn().Err(err).Msg("jwt rejected")
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			role, _ := claims["role"].(string)
			if role != "orchestrator" && role != "worker" {
				role = "orchestrator"
			}
			sub, _ := claims["sub"].(string)
			sess := &Session{Role: role, Subject: sub}
			ctx := context.WithValue(r.Context(), sessionCtxKey{}, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func verifyHS256(token, secret string) (map[string]any, error) {
	if secret == "" {
		return nil, errors.New("jwt secret not configured")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token")
	}
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errors.New("signature mismatch")
	}

	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	var hdr struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(hdrBytes, &hdr); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if hdr.Alg != "HS256" {
		return nil, fmt.Errorf("unsupported alg %q", hdr.Alg)
	}

	claimBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(claimBytes, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, errors.New("token expired")
		}
	}
	return claims, nil
}
