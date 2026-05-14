package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/eventbus"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

var heartbeatInterval = 15 * time.Second

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
func RunHTTP(ctx context.Context, cfg *config.Config, log *logging.Logger,
	h func(ctx context.Context, sess *Session, raw []byte) ([]byte, error),
) error {
	bus := eventbus.New()

	handler := newHTTPHandler(cfg, log, bus, h)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

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

func newHTTPHandler(cfg *config.Config, log *logging.Logger, bus *eventbus.Bus,
	h func(ctx context.Context, sess *Session, raw []byte) ([]byte, error),
) http.Handler {
	mux := http.NewServeMux()

	originSet := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		originSet[strings.ToLower(strings.TrimSpace(o))] = struct{}{}
	}

	rateLimiter := &rateLimiterStore{
		limiters: make(map[string]*rate.Limiter),
	}

	mw := chain(
		recoverMiddleware(log),
		requestIDMiddleware(),
		originMiddleware(originSet, log),
		rateLimitMiddleware(rateLimiter, log),
	)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	mux.Handle("/mcp", mw(authMiddleware(cfg, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlePOST(w, r, log, bus, h)
		case http.MethodGet:
			handleSSE(w, r, log, bus)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	return mux
}

func handlePOST(w http.ResponseWriter, r *http.Request, log *logging.Logger, bus *eventbus.Bus,
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

	var reqMeta struct {
		Method string `json:"method"`
	}
	_ = json.Unmarshal(body, &reqMeta)
	requestID, _ := r.Context().Value(requestIDCtxKey{}).(string)

	sess, _ := r.Context().Value(sessionCtxKey{}).(*Session)
	resp, err := h(r.Context(), sess, body)
	if err != nil {
		publishSampleEvents(bus, log, reqMeta.Method, requestID, "failed", err.Error())
		log.Error().Err(err).Msg("handler error")
		http.Error(w, "handler error", http.StatusInternalServerError)
		return
	}
	if resp == nil {
		publishSampleEvents(bus, log, reqMeta.Method, requestID, "accepted", "")
		// Notification — per MCP spec, return 202 Accepted with no body.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	publishSampleEvents(bus, log, reqMeta.Method, requestID, "completed", "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp)
}

func handleSSE(w http.ResponseWriter, r *http.Request, log *logging.Logger, bus *eventbus.Bus) {
	if bus == nil {
		bus = eventbus.New()
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	sub := bus.Subscribe()
	defer sub.Close()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	_, _ = fmt.Fprintf(w, "event: ready\ndata: {\"protocolVersion\":\"2025-03-26\"}\n\n")
	flusher.Flush()
	log.Debug().Msg("SSE client connected")
	defer func() {
		dropped := sub.Dropped()
		if dropped > 0 {
			log.Warn().Int("dropped_events", int(dropped)).Msg("SSE subscriber dropped events due to backpressure")
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case msg, ok := <-sub.Messages():
			if !ok {
				return
			}
			envelope, err := marshalJSONRPCNotification(msg.Topic, msg.Payload)
			if err != nil {
				log.Warn().Err(err).Str("topic", msg.Topic).Msg("failed to marshal notification envelope")
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", envelope); err != nil {
				log.Debug().Err(err).Msg("SSE write failed")
				return
			}
			flusher.Flush()
		}
	}
}

func marshalJSONRPCNotification(topic string, payload json.RawMessage) ([]byte, error) {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	env := struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}{
		JSONRPC: "2.0",
		Method:  topic,
		Params:  payload,
	}
	return json.Marshal(env)
}

func publishSampleEvents(bus *eventbus.Bus, log *logging.Logger, method, requestID, state, errMsg string) {
	if bus == nil {
		return
	}
	logPayload := map[string]any{
		"request_id": requestID,
		"method":     method,
		"state":      state,
	}
	if errMsg != "" {
		logPayload["error"] = errMsg
	}
	if err := bus.Publish(eventbus.TopicNotificationsLog, logPayload); err != nil {
		log.Warn().Err(err).Msg("publish notifications/log failed")
	}
	if method != "tools/call" {
		return
	}
	jobPayload := map[string]any{
		"request_id": requestID,
		"state":      state,
	}
	if err := bus.Publish(eventbus.TopicNotificationsJobState, jobPayload); err != nil {
		log.Warn().Err(err).Msg("publish notifications/job-state failed")
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

// --- Rate limiting middleware (T029 remediation #7) ---
//
// Token-bucket rate limiter per IP address. Default: 60 requests per minute.
// Rejects excess requests with HTTP 429 Too Many Requests.

type rateLimiterStore struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

func (rls *rateLimiterStore) getLimiter(ip string) *rate.Limiter {
	rls.mu.Lock()
	defer rls.mu.Unlock()
	if lim, ok := rls.limiters[ip]; ok {
		return lim
	}
	lim := rate.NewLimiter(rate.Every(time.Minute/60), 1) // 60 req/min, burst=1
	rls.limiters[ip] = lim
	return lim
}

func rateLimitMiddleware(store *rateLimiterStore, log *logging.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only rate-limit /mcp POST (not GET SSE)
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr // fallback if SplitHostPort fails
			}

			lim := store.getLimiter(ip)
			if !lim.Allow() {
				log.Warn().Str("ip", ip).Msg("rate limit exceeded")
				w.Header().Set("Retry-After", "60")
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- JWT verification using github.com/golang-jwt/jwt/v5 (T029 remediation) ---
//
// Supports both HS256 (dev) and RS256 (prod) selected by CWSO_JWT_ALG.
// Validates iss, aud, nbf, exp with 60s leeway.

type jwtClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func authMiddleware(cfg *config.Config, log *logging.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(authz, prefix) {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(authz, prefix)
			claims, err := verifyJWT(token, cfg, log)
			if err != nil {
				log.Warn().Err(err).Msg("jwt rejected")
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			role := claims.Role
			if role != "orchestrator" && role != "worker" {
				role = "orchestrator"
			}
			sess := &Session{Role: role, Subject: claims.Subject}
			ctx := context.WithValue(r.Context(), sessionCtxKey{}, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func verifyJWT(tokenString string, cfg *config.Config, log *logging.Logger) (*jwtClaims, error) {
	if cfg.JWTSecret == "" {
		return nil, errors.New("jwt secret not configured")
	}

	claims := &jwtClaims{}

	var keyFunc jwt.Keyfunc
	switch cfg.JWTAlg {
	case "HS256":
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != "HS256" {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(cfg.JWTSecret), nil
		}
	case "RS256":
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != "RS256" {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// For RS256, would load public key from JWKSPath or JWKS endpoint
			// POC-DEBT: RS256 key loading deferred to T029 Phase 2
			return nil, errors.New("RS256 not yet implemented")
		}
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", cfg.JWTAlg)
	}

	parser := jwt.NewParser(jwt.WithLeeway(60 * time.Second))
	_, err := parser.ParseWithClaims(tokenString, claims, keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token expired")
		}
		if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, errors.New("token used before valid (nbf)")
		}
		return nil, fmt.Errorf("parse token: %w", err)
	}

	if cfg.JWTIssuer != "" && claims.Issuer != cfg.JWTIssuer {
		return nil, fmt.Errorf("invalid issuer: expected %q, got %q", cfg.JWTIssuer, claims.Issuer)
	}

	if cfg.JWTAudience != "" {
		audienceFound := false
		for _, aud := range claims.Audience {
			if aud == cfg.JWTAudience {
				audienceFound = true
				break
			}
		}
		if !audienceFound {
			return nil, fmt.Errorf("invalid audience: expected %q", cfg.JWTAudience)
		}
	}

	return claims, nil
}
