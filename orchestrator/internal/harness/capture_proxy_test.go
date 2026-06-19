package harness

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type capturedCall struct {
	Path string
	Body []byte
}

type captureProxy struct {
	server   *httptest.Server
	upstream *httptest.Server
	mu       sync.Mutex
	calls    []capturedCall
}

func newCaptureProxy(t *testing.T) *captureProxy {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"mock","choices":[{"message":{"content":"ok"},`+
			`"finish_reason":"stop","logprobs":{"content":[{"token_id":42,"logprob":-0.02}]}}],`+
			`"usage":{"prompt_tokens":2}}`)
	}))
	t.Cleanup(upstream.Close)

	proxy := &captureProxy{upstream: upstream}
	proxy.server = httptest.NewServer(http.HandlerFunc(proxy.handle))
	t.Cleanup(proxy.server.Close)
	return proxy
}

func (p *captureProxy) URL() string {
	return strings.TrimRight(p.server.URL, "/")
}

func (p *captureProxy) Calls() []capturedCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]capturedCall, len(p.calls))
	copy(out, p.calls)
	return out
}

func (p *captureProxy) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	p.mu.Lock()
	p.calls = append(p.calls, capturedCall{Path: r.URL.Path, Body: body})
	p.mu.Unlock()

	upstreamURL := p.upstream.URL + "/v1/chat/completions"
	resp, err := http.Post(upstreamURL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, "upstream failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "upstream read", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (p *captureProxy) hasChatCompletion() bool {
	for _, call := range p.Calls() {
		if call.Path == "/v1/chat/completions" {
			var doc map[string]any
			if err := json.Unmarshal(call.Body, &doc); err == nil {
				return true
			}
		}
	}
	return false
}
