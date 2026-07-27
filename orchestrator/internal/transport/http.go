package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/eventbus"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

var heartbeatInterval = 15 * time.Second

type eventPublisher interface {
	Publish(topic string, payload any) error
}

// RecordFilter decides whether a broker/bus record should be delivered to a
// subscription-scoped SSE stream. Implemented by dispatch.SpikeSubscription.
type RecordFilter interface {
	Allow(topic string, payload []byte) bool
}

// SubscriptionResolver resolves a subscription id (from the ?subscription= query
// parameter) to its record filter. ok=false means the id is unknown.
type SubscriptionResolver func(id string) (RecordFilter, bool)

type httpOptions struct {
	resolveSub SubscriptionResolver
	rollout    http.Handler
}

// HTTPOption configures optional HTTP transport behaviour without growing the
// already-wide RunHTTP/newHTTPHandler positional signatures (see TD-02).
type HTTPOption func(*httpOptions)

// WithSubscriptionResolver enables subscription-scoped SSE: GET /mcp?subscription=<id>
// streams only the records the resolved filter allows.
func WithSubscriptionResolver(r SubscriptionResolver) HTTPOption {
	return func(o *httpOptions) { o.resolveSub = r }
}

// WithRolloutAPI mounts Polar REST routes (/rollout/*, /callbacks/*, /nodes/*) when set.
func WithRolloutAPI(h http.Handler) HTTPOption {
	return func(o *httpOptions) { o.rollout = h }
}

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
	bus *eventbus.Bus,
	broker *memorybroker.Broker,
	samplePublisher eventPublisher,
	h func(ctx context.Context, sess *Session, raw []byte) ([]byte, error),
	opts ...HTTPOption,
) error {
	if bus == nil {
		bus = eventbus.New()
	}
	if samplePublisher == nil {
		if broker != nil {
			samplePublisher = memorybroker.NewTeePublisher(bus, broker)
		} else {
			samplePublisher = bus
		}
	}

	handler := newHTTPHandler(ctx, cfg, log, bus, broker, samplePublisher, h, opts...)

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

func newHTTPHandler(ctx context.Context, cfg *config.Config, log *logging.Logger, bus *eventbus.Bus, broker *memorybroker.Broker,
	samplePublisher eventPublisher,
	h func(ctx context.Context, sess *Session, raw []byte) ([]byte, error),
	opts ...HTTPOption,
) http.Handler {
	var o httpOptions
	for _, opt := range opts {
		opt(&o)
	}
	mux := http.NewServeMux()

	originSet := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		originSet[strings.ToLower(strings.TrimSpace(o))] = struct{}{}
	}

	rateLimiter := newRateLimiterStore(ctx)

	mw := chain(
		recoverMiddleware(log),
		requestIDMiddleware(),
		originMiddleware(originSet, log),
		securityHeadersMiddleware(),
		rateLimitMiddleware(rateLimiter, log),
	)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	mux.Handle("/mcp", mw(authMiddleware(cfg, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlePOST(w, r, log, samplePublisher, h)
		case http.MethodGet:
			ip, _, _ := strings.Cut(r.RemoteAddr, ":")
			if !rateLimiter.sseConns.acquire(ip) {
				http.Error(w, "too many SSE connections", http.StatusTooManyRequests)
				return
			}
			defer rateLimiter.sseConns.release(ip)
			handleSSE(w, r, log, bus, broker, o.resolveSub)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	if o.rollout != nil {
		mux.Handle("/", mw(authMiddleware(cfg, log)(o.rollout)))
	}

	return mux
}

func handlePOST(w http.ResponseWriter, r *http.Request, log *logging.Logger, publisher eventPublisher,
	h func(ctx context.Context, sess *Session, raw []byte) ([]byte, error),
) {
	const maxBody = 8 << 20
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		log.Warn().Err(err).Str("handler", "POST /mcp").Msg("failed to read request body")
		http.Error(w, "bad request", http.StatusBadRequest)
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
		publishSampleEvents(publisher, log, reqMeta.Method, requestID, "failed", err.Error())
		log.Error().Err(err).Msg("handler error")
		http.Error(w, "handler error", http.StatusInternalServerError)
		return
	}
	if resp == nil {
		publishSampleEvents(publisher, log, reqMeta.Method, requestID, "accepted", "")
		// Notification — per MCP spec, return 202 Accepted with no body.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	publishSampleEvents(publisher, log, reqMeta.Method, requestID, "completed", "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp)
}

func handleSSE(w http.ResponseWriter, r *http.Request, log *logging.Logger, bus *eventbus.Bus, broker *memorybroker.Broker, resolve SubscriptionResolver) {
	// Resolve an optional subscription scope before writing any SSE headers so an unknown
	// id can still return a clean 404.
	var filter RecordFilter
	if id := r.URL.Query().Get("subscription"); id != "" {
		if resolve == nil {
			http.Error(w, "subscriptions not enabled", http.StatusNotFound)
			return
		}
		f, ok := resolve(id)
		if !ok {
			http.Error(w, "unknown subscription", http.StatusNotFound)
			return
		}
		filter = f
	}

	if broker != nil {
		handleBrokerSSE(w, r, log, broker, filter)
		return
	}
	if bus == nil {
		bus = eventbus.New()
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	// Disable the server-level WriteTimeout for this SSE connection.
	// WriteTimeout is measured from first byte written and fires at 30 s, which
	// would kill long-lived streams. SSE connections must run until the client
	// disconnects or the request context is cancelled.
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Msg("failed to clear SSE write deadline")
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
			if filter != nil && !filter.Allow(msg.Topic, msg.Payload) {
				continue
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

func handleBrokerSSE(w http.ResponseWriter, r *http.Request, log *logging.Logger, broker *memorybroker.Broker, filter RecordFilter) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	// Disable the server-level WriteTimeout for this SSE connection (same reason as handleSSE).
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Msg("failed to clear broker SSE write deadline")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	sub := broker.Subscribe()
	defer sub.Close()
	throttle := newTelemetryThrottle()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	_, _ = fmt.Fprintf(w, "event: ready\ndata: {\"protocolVersion\":\"2025-03-26\"}\n\n")
	flusher.Flush()
	log.Debug().Msg("SSE client connected")
	defer func() {
		fields := map[string]any{}
		for topic, counters := range throttle.Snapshot() {
			fields[topic] = map[string]int{
				"emitted":    counters.Emitted,
				"suppressed": counters.Suppressed,
			}
		}
		entry := log.Info().Any("telemetry_counts", fields)
		if dropped := sub.Dropped(); dropped > 0 {
			entry = log.Warn().Int("dropped_events", int(dropped)).Any("telemetry_counts", fields)
		}
		entry.Msg("SSE telemetry stream closed")
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case rec, ok := <-sub.Messages():
			if !ok {
				return
			}
			if filter != nil && !filter.Allow(rec.Topic, rec.Payload) {
				continue
			}
			if !throttle.Allow(rec) {
				continue
			}
			envelope, err := marshalJSONRPCNotification(rec.Topic, rec.Payload)
			if err != nil {
				log.Warn().Err(err).Str("topic", rec.Topic).Msg("failed to marshal notification envelope")
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

func publishSampleEvents(publisher eventPublisher, log *logging.Logger, method, requestID, state, errMsg string) {
	if publisher == nil {
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
	if err := publisher.Publish(eventbus.TopicNotificationsLog, logPayload); err != nil {
		log.Warn().Err(err).Msg("publish notifications/log failed")
	}
	if method != "tools/call" {
		return
	}
	jobPayload := map[string]any{
		"request_id": requestID,
		"state":      state,
	}
	if err := publisher.Publish(eventbus.TopicNotificationsJobState, jobPayload); err != nil {
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
func securityHeadersMiddleware() middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "0")
			if r.Method == http.MethodPost {
				w.Header().Set("Cache-Control", "no-store")
			}
			next.ServeHTTP(w, r)
		})
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

type sseConnectionStore struct {
	mu    sync.Mutex
	conns map[string]int
}

const maxSSEConnsPerIP = 10

func (s *sseConnectionStore) acquire(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conns[ip] >= maxSSEConnsPerIP {
		return false
	}
	s.conns[ip]++
	return true
}

func (s *sseConnectionStore) release(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conns[ip] > 0 {
		s.conns[ip]--
	}
}

type rateLimiterEntry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

type rateLimiterStore struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiterEntry
	sseConns *sseConnectionStore
}

func newRateLimiterStore(ctx context.Context) *rateLimiterStore {
	rls := &rateLimiterStore{
		limiters: make(map[string]*rateLimiterEntry),
		sseConns: &sseConnectionStore{conns: make(map[string]int)},
	}
	go rls.evictLoop(ctx)
	return rls
}

// isLocalhost checks if an IP is a loopback address (localhost in development)
func isLocalhost(ip string) bool {
	return ip == "127.0.0.1" || ip == "::1" || ip == "localhost"
}

func (rls *rateLimiterStore) evictLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-5 * time.Minute)
			rls.mu.Lock()
			for ip, entry := range rls.limiters {
				if entry.lastSeen.Before(cutoff) {
					delete(rls.limiters, ip)
				}
			}
			rls.mu.Unlock()
		}
	}
}

func (rls *rateLimiterStore) getLimiter(ip string) *rate.Limiter {
	rls.mu.Lock()
	defer rls.mu.Unlock()
	// No rate limiting for localhost (development convenience)
	// Remote clients get 60 req/min with burst capacity for connection handshakes
	if entry, ok := rls.limiters[ip]; ok {
		entry.lastSeen = time.Now()
		return entry.lim
	}
	// Create limiter: 60 requests per minute with burst of 10 for connection init
	lim := rate.NewLimiter(rate.Every(time.Minute/60), 10) // 60 req/min, burst=10
	rls.limiters[ip] = &rateLimiterEntry{lim: lim, lastSeen: time.Now()}
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

			// Exempt localhost from rate limiting (development convenience)
			if isLocalhost(ip) {
				next.ServeHTTP(w, r)
				return
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
// Supports HS256 selected by CWSO_JWT_ALG.
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
				http.Error(w, "forbidden: unrecognised role", http.StatusForbidden)
				return
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
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", cfg.JWTAlg)
	}

	parser := jwt.NewParser(jwt.WithLeeway(60*time.Second), jwt.WithExpirationRequired())
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

	if claims.Issuer != cfg.JWTIssuer {
		return nil, fmt.Errorf("invalid issuer: expected %q, got %q", cfg.JWTIssuer, claims.Issuer)
	}

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

	return claims, nil
}
