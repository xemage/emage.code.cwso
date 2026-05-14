package eventbus

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
)

const (
	TopicNotificationsLog      = "notifications/log"
	TopicNotificationsJobState = "notifications/job-state"

	defaultQueueDepth = 256
	defaultQueueBytes = 1 << 20
)

// Message is a transport-agnostic event payload routed to subscribers.
type Message struct {
	Topic   string
	Payload json.RawMessage
}

type queuedMessage struct {
	msg  Message
	size int
}

type subscriber struct {
	id uint64

	mu          sync.Mutex
	ch          chan queuedMessage
	queuedBytes int
	maxBytes    int
	closed      bool
	dropped     uint64
}

func (s *subscriber) offer(msg Message, size int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	if s.queuedBytes+size > s.maxBytes || len(s.ch) >= cap(s.ch) {
		atomic.AddUint64(&s.dropped, 1)
		return
	}

	select {
	case s.ch <- queuedMessage{msg: msg, size: size}:
		s.queuedBytes += size
	default:
		atomic.AddUint64(&s.dropped, 1)
	}
}

func (s *subscriber) onDequeued(size int) {
	s.mu.Lock()
	s.queuedBytes -= size
	if s.queuedBytes < 0 {
		s.queuedBytes = 0
	}
	s.mu.Unlock()
}

func (s *subscriber) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.ch)
}

// Bus is an in-memory, process-local pub/sub broker.
type Bus struct {
	mu         sync.RWMutex
	nextID     uint64
	subs       map[uint64]*subscriber
	queueDepth int
	queueBytes int
}

// Option customizes a Bus.
type Option func(*Bus)

// WithQueueDepth sets bounded per-subscriber queue depth.
func WithQueueDepth(depth int) Option {
	return func(b *Bus) {
		if depth > 0 {
			b.queueDepth = depth
		}
	}
}

// WithSubscriberMaxBytes sets bounded per-subscriber buffered bytes.
func WithSubscriberMaxBytes(maxBytes int) Option {
	return func(b *Bus) {
		if maxBytes > 0 {
			b.queueBytes = maxBytes
		}
	}
}

// New constructs a Bus with safe defaults.
func New(opts ...Option) *Bus {
	b := &Bus{
		subs:       make(map[uint64]*subscriber),
		queueDepth: defaultQueueDepth,
		queueBytes: defaultQueueBytes,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Publish fans out an event to all current subscribers.
func (b *Bus) Publish(topic string, payload any) error {
	if topic == "" {
		return errors.New("topic is required")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := Message{Topic: topic, Payload: encoded}
	size := len(topic) + len(encoded)

	b.mu.RLock()
	subs := make([]*subscriber, 0, len(b.subs))
	for _, s := range b.subs {
		subs = append(subs, s)
	}
	b.mu.RUnlock()

	for _, s := range subs {
		s.offer(msg, size)
	}

	return nil
}

// Subscription is a per-session stream from the event bus.
type Subscription struct {
	bus  *Bus
	sub  *subscriber
	out  chan Message
	once sync.Once
}

// Subscribe registers a new subscriber with bounded buffering.
func (b *Bus) Subscribe() *Subscription {
	id := atomic.AddUint64(&b.nextID, 1)
	sub := &subscriber{
		id:       id,
		ch:       make(chan queuedMessage, b.queueDepth),
		maxBytes: b.queueBytes,
	}

	b.mu.Lock()
	b.subs[id] = sub
	b.mu.Unlock()

	s := &Subscription{
		bus: b,
		sub: sub,
		out: make(chan Message),
	}
	go s.pump()
	return s
}

func (s *Subscription) pump() {
	defer close(s.out)
	for qm := range s.sub.ch {
		s.out <- qm.msg
		s.sub.onDequeued(qm.size)
	}
}

// Messages returns the subscriber stream.
func (s *Subscription) Messages() <-chan Message {
	return s.out
}

// Dropped returns the number of dropped messages for this subscriber.
func (s *Subscription) Dropped() uint64 {
	return atomic.LoadUint64(&s.sub.dropped)
}

// Close removes this subscription and closes its stream.
func (s *Subscription) Close() {
	s.once.Do(func() {
		s.bus.mu.Lock()
		delete(s.bus.subs, s.sub.id)
		s.bus.mu.Unlock()
		s.sub.close()
	})
}
