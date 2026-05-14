package memorybroker

import "testing"

type stubPublisher struct {
	topic   string
	payload any
	err     error
}

func (s *stubPublisher) Publish(topic string, payload any) error {
	s.topic = topic
	s.payload = payload
	return s.err
}

type stubIngestor struct {
	topic   string
	payload any
	called  bool
}

func (s *stubIngestor) Ingest(topic string, payload any) bool {
	s.called = true
	s.topic = topic
	s.payload = payload
	return true
}

func TestTeePublisherForwardsAndMirrors(t *testing.T) {
	upstream := &stubPublisher{}
	ingestor := &stubIngestor{}
	p := NewTeePublisher(upstream, ingestor)

	payload := map[string]any{"job_id": "job-1"}
	if err := p.Publish("notifications/job-state", payload); err != nil {
		t.Fatalf("publish returned error: %v", err)
	}

	if upstream.topic != "notifications/job-state" {
		t.Fatalf("unexpected upstream topic: %q", upstream.topic)
	}
	if !ingestor.called {
		t.Fatal("expected ingestor to be called")
	}
}

func TestTeePublisherReturnsUpstreamError(t *testing.T) {
	upstream := &stubPublisher{err: errStub}
	ingestor := &stubIngestor{}
	p := NewTeePublisher(upstream, ingestor)

	err := p.Publish("notifications/log", map[string]any{"state": "failed"})
	if err == nil {
		t.Fatal("expected upstream error")
	}
	if !ingestor.called {
		t.Fatal("expected ingestor to still be called")
	}
}

var errStub = &stubError{s: "upstream failed"}

type stubError struct{ s string }

func (e *stubError) Error() string { return e.s }
