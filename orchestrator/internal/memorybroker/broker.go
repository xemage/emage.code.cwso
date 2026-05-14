package memorybroker

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRetentionCapacity = 2048
	defaultIngressQueueSize  = 1024
)

// Record is an immutable, append-only event in the in-memory broker log.
type Record struct {
	Sequence uint64          `json:"sequence"`
	Topic    string          `json:"topic"`
	JobID    string          `json:"job_id,omitempty"`
	At       time.Time       `json:"at"`
	Payload  json.RawMessage `json:"payload"`
}

// QueryOptions controls filtered reads from the broker log.
type QueryOptions struct {
	Topics []string
	JobID  string
	Since  time.Time
	Window time.Duration
	Limit  int
}

type pendingEvent struct {
	topic   string
	payload any
	at      time.Time
}

// Broker is an append-only, bounded in-memory event log with non-blocking ingest.
type Broker struct {
	now func() time.Time

	ingestCh chan pendingEvent
	closed   chan struct{}
	wg       sync.WaitGroup

	mu       sync.RWMutex
	records  []Record
	head     int
	count    int
	capacity int
	nextSeq  uint64

	droppedIngress atomic.Uint64
}

// Option customizes Broker construction.
type Option func(*Broker)

// WithCapacity configures max retained records.
func WithCapacity(capacity int) Option {
	return func(b *Broker) {
		if capacity > 0 {
			b.capacity = capacity
		}
	}
}

// WithIngressQueueSize configures the non-blocking ingress queue depth.
func WithIngressQueueSize(size int) Option {
	return func(b *Broker) {
		if size > 0 {
			b.ingestCh = make(chan pendingEvent, size)
		}
	}
}

// WithNow injects a clock source for deterministic tests.
func WithNow(now func() time.Time) Option {
	return func(b *Broker) {
		if now != nil {
			b.now = now
		}
	}
}

// New constructs and starts a Broker.
func New(opts ...Option) *Broker {
	b := &Broker{
		now:      time.Now,
		ingestCh: make(chan pendingEvent, defaultIngressQueueSize),
		closed:   make(chan struct{}),
		capacity: defaultRetentionCapacity,
	}
	for _, opt := range opts {
		opt(b)
	}
	if b.capacity <= 0 {
		b.capacity = defaultRetentionCapacity
	}
	b.records = make([]Record, b.capacity)

	b.wg.Add(1)
	go b.run()
	return b
}

// Close stops ingestion workers and makes future ingest calls no-ops.
func (b *Broker) Close() {
	select {
	case <-b.closed:
		return
	default:
		close(b.closed)
	}
	b.wg.Wait()
}

// Ingest attempts to enqueue an event without blocking.
// It returns false when the broker is closed, topic is empty, or ingress queue is full.
func (b *Broker) Ingest(topic string, payload any) bool {
	if strings.TrimSpace(topic) == "" {
		return false
	}
	e := pendingEvent{
		topic:   topic,
		payload: payload,
		at:      b.now().UTC(),
	}

	select {
	case <-b.closed:
		return false
	default:
	}

	select {
	case b.ingestCh <- e:
		return true
	default:
		b.droppedIngress.Add(1)
		return false
	}
}

// DroppedIngress returns how many events could not be enqueued.
func (b *Broker) DroppedIngress() uint64 {
	return b.droppedIngress.Load()
}

// Len returns the current number of retained records.
func (b *Broker) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// Query returns retained records in ascending sequence order with optional filters.
func (b *Broker) Query(opts QueryOptions) []Record {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	topicSet := map[string]struct{}{}
	for _, topic := range opts.Topics {
		topic = strings.TrimSpace(topic)
		if topic != "" {
			topicSet[topic] = struct{}{}
		}
	}

	cutoff := time.Time{}
	if !opts.Since.IsZero() {
		cutoff = opts.Since.UTC()
	}
	if opts.Window > 0 {
		windowCutoff := b.now().UTC().Add(-opts.Window)
		if cutoff.IsZero() || windowCutoff.After(cutoff) {
			cutoff = windowCutoff
		}
	}

	out := make([]Record, 0, b.count)
	for i := 0; i < b.count; i++ {
		idx := (b.head + i) % b.capacity
		r := b.records[idx]

		if len(topicSet) > 0 {
			if _, ok := topicSet[r.Topic]; !ok {
				continue
			}
		}
		if opts.JobID != "" && r.JobID != opts.JobID {
			continue
		}
		if !cutoff.IsZero() && r.At.Before(cutoff) {
			continue
		}
		out = append(out, cloneRecord(r))
	}

	if opts.Limit <= 0 || len(out) <= opts.Limit {
		return out
	}

	start := len(out) - opts.Limit
	limited := make([]Record, 0, opts.Limit)
	for _, r := range out[start:] {
		limited = append(limited, cloneRecord(r))
	}
	return limited
}

func (b *Broker) run() {
	defer b.wg.Done()
	for {
		select {
		case <-b.closed:
			return
		case e := <-b.ingestCh:
			b.appendEvent(e)
		}
	}
}

func (b *Broker) appendEvent(e pendingEvent) {
	safe := sanitizePayload(e.payload)
	encoded, err := json.Marshal(safe)
	if err != nil {
		encoded = mustMarshalFallback(err)
	}

	b.mu.Lock()
	b.nextSeq++
	rec := Record{
		Sequence: b.nextSeq,
		Topic:    e.topic,
		JobID:    extractJobID(safe),
		At:       e.at,
		Payload:  encoded,
	}

	if b.count < b.capacity {
		idx := (b.head + b.count) % b.capacity
		b.records[idx] = rec
		b.count++
	} else {
		b.records[b.head] = rec
		b.head = (b.head + 1) % b.capacity
	}
	b.mu.Unlock()
}

func cloneRecord(r Record) Record {
	cloned := r
	if r.Payload != nil {
		cloned.Payload = append(json.RawMessage(nil), r.Payload...)
	}
	return cloned
}

func extractJobID(payload any) string {
	obj, ok := payload.(map[string]any)
	if !ok {
		return ""
	}
	v, ok := obj["job_id"]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func mustMarshalFallback(err error) json.RawMessage {
	fallback, marshalErr := json.Marshal(map[string]any{"error": fmt.Sprintf("payload marshal failed: %v", err)})
	if marshalErr != nil {
		return json.RawMessage(`{"error":"payload marshal failed"}`)
	}
	return fallback
}

func sanitizePayload(payload any) any {
	if payload == nil {
		return map[string]any{}
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return map[string]any{"value": fmt.Sprint(payload)}
	}

	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return map[string]any{"value": string(encoded)}
	}

	return sanitizeValue(decoded)
}

func sanitizeValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			if isSensitiveKey(k) {
				out[k] = "[redacted]"
				continue
			}
			out[k] = sanitizeValue(item)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = sanitizeValue(item)
		}
		return out
	default:
		return x
	}
}

func isSensitiveKey(key string) bool {
	norm := strings.ToLower(strings.TrimSpace(key))
	if norm == "" {
		return false
	}
	for _, marker := range []string{
		"token",
		"secret",
		"password",
		"authorization",
		"api_key",
		"apikey",
		"access_key",
		"refresh_token",
		"jwt",
		"cookie",
	} {
		if strings.Contains(norm, marker) {
			return true
		}
	}
	return false
}
