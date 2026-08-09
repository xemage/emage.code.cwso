package memorybroker

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func waitForLen(t *testing.T, b *Broker, want int) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if b.Len() >= want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for len >= %d (got=%d)", want, b.Len())
}

// waitForMaxSeq polls until the highest stored sequence reaches want.
// Use this instead of waitForLen when the broker may overwrite older records
// before the caller inspects them (ring-buffer eviction race).
func waitForMaxSeq(t *testing.T, b *Broker, want uint64) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if recs := b.Query(QueryOptions{}); len(recs) > 0 && recs[len(recs)-1].Sequence >= want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for max sequence >= %d", want)
}

func TestSequenceMonotonicity(t *testing.T) {
	b := New(WithCapacity(16), WithIngressQueueSize(64))
	defer b.Close()

	for i := 0; i < 6; i++ {
		if ok := b.Ingest("notifications/job-state", map[string]any{"job_id": "job-a", "idx": i}); !ok {
			t.Fatalf("ingest %d failed", i)
		}
	}

	waitForLen(t, b, 6)
	records := b.Query(QueryOptions{})
	if len(records) != 6 {
		t.Fatalf("expected 6 records, got %d", len(records))
	}

	var prev uint64
	for i, rec := range records {
		if i > 0 && rec.Sequence <= prev {
			t.Fatalf("sequence must increase strictly, prev=%d current=%d", prev, rec.Sequence)
		}
		prev = rec.Sequence
	}
}

func TestRetentionEvictionOldestFirst(t *testing.T) {
	b := New(WithCapacity(3), WithIngressQueueSize(32))
	defer b.Close()

	for i := 0; i < 5; i++ {
		if ok := b.Ingest("notifications/log", map[string]any{"job_id": "job-evict", "idx": i}); !ok {
			t.Fatalf("ingest %d failed", i)
		}
	}

	// Wait until all 5 events have been processed and the ring settled at capacity.
	// waitForLen(b,3) is insufficient: Len hits 3 after the 3rd event, before events
	// 4 and 5 overwrite the oldest slots in the ring buffer.
	waitForMaxSeq(t, b, 5)
	records := b.Query(QueryOptions{})
	if len(records) != 3 {
		t.Fatalf("expected retained size 3, got %d", len(records))
	}

	if records[0].Sequence != 3 || records[1].Sequence != 4 || records[2].Sequence != 5 {
		t.Fatalf("expected oldest-first eviction with sequences 3,4,5; got %d,%d,%d", records[0].Sequence, records[1].Sequence, records[2].Sequence)
	}

	var payload map[string]any
	if err := json.Unmarshal(records[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["idx"].(float64) != 2 {
		t.Fatalf("expected first retained payload idx=2, got %v", payload["idx"])
	}
}

func TestConcurrentIngestAndReadRaceSafe(t *testing.T) {
	b := New(WithCapacity(256), WithIngressQueueSize(1024))
	defer b.Close()

	const writers = 8
	const perWriter = 120

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				_ = b.Ingest("notifications/job-state", map[string]any{
					"job_id": "job-concurrent",
					"worker": worker,
					"idx":    i,
				})
			}
		}(w)
	}

	stopReaders := make(chan struct{})
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopReaders:
					return
				default:
					_ = b.Query(QueryOptions{Topics: []string{"notifications/job-state"}, JobID: "job-concurrent", Limit: 50})
				}
			}
		}()
	}

	writersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(writersDone)
	}()

	time.Sleep(150 * time.Millisecond)
	close(stopReaders)
	select {
	case <-writersDone:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent ingest/read did not complete")
	}

	records := b.Query(QueryOptions{})
	if len(records) == 0 {
		t.Fatal("expected retained records after concurrent ingest")
	}
	if len(records) > 256 {
		t.Fatalf("expected retention bound 256, got %d", len(records))
	}

	for i := 1; i < len(records); i++ {
		if records[i].Sequence <= records[i-1].Sequence {
			t.Fatalf("non-monotonic sequence at i=%d (%d <= %d)", i, records[i].Sequence, records[i-1].Sequence)
		}
	}
}

func TestFilteredQueryByTopicJobIDWindowAndLimit(t *testing.T) {
	now := time.Now
	b := New(WithCapacity(32), WithIngressQueueSize(64), WithNow(now))
	defer b.Close()

	_ = b.Ingest("notifications/log", map[string]any{"job_id": "job-1", "state": "queued", "token": "secret-token"})
	_ = b.Ingest("notifications/job-state", map[string]any{"job_id": "job-1", "state": "running"})
	time.Sleep(25 * time.Millisecond)
	_ = b.Ingest("notifications/job-state", map[string]any{"job_id": "job-2", "state": "running"})
	_ = b.Ingest("notifications/job-state", map[string]any{"job_id": "job-1", "state": "completed"})
	_ = b.Ingest("notifications/log", map[string]any{"job_id": "job-3", "state": "completed"})

	waitForLen(t, b, 5)

	filtered := b.Query(QueryOptions{
		Topics: []string{"notifications/job-state"},
		JobID:  "job-1",
	})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 records for topic/job filter, got %d", len(filtered))
	}
	if filtered[0].Topic != "notifications/job-state" || filtered[1].Topic != "notifications/job-state" {
		t.Fatalf("unexpected topics: %q %q", filtered[0].Topic, filtered[1].Topic)
	}
	if filtered[0].JobID != "job-1" || filtered[1].JobID != "job-1" {
		t.Fatalf("unexpected job ids: %q %q", filtered[0].JobID, filtered[1].JobID)
	}

	recent := b.Query(QueryOptions{Window: 20 * time.Millisecond, Limit: 2})
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent records with limit, got %d", len(recent))
	}
	if recent[0].Sequence >= recent[1].Sequence {
		t.Fatalf("expected ascending sequence order in limited view, got %d then %d", recent[0].Sequence, recent[1].Sequence)
	}

	all := b.Query(QueryOptions{Topics: []string{"notifications/log"}})
	if len(all) != 2 {
		t.Fatalf("expected 2 log topic records, got %d", len(all))
	}

	var payload map[string]any
	if err := json.Unmarshal(all[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal sanitized payload: %v", err)
	}
	if payload["token"] != "[redacted]" {
		t.Fatalf("expected token redaction, got %v", payload["token"])
	}
}

func TestIngestQueueFullIsNonBlocking(t *testing.T) {
	var tick atomic.Int64
	b := New(
		WithCapacity(4),
		WithIngressQueueSize(1),
		WithNow(func() time.Time {
			n := tick.Add(1)
			return time.UnixMilli(n)
		}),
	)
	defer b.Close()

	first := b.Ingest("notifications/log", map[string]any{"job_id": "j1"})
	second := b.Ingest("notifications/log", map[string]any{"job_id": "j1"})
	if !first {
		t.Fatal("expected first ingest to succeed")
	}
	if first && second {
		return
	}

	if b.DroppedIngress() == 0 {
		t.Fatal("expected dropped ingress count to increment when queue is full")
	}
}

func TestLiveSubscriptionReceivesSanitizedRecordsInOrder(t *testing.T) {
	b := New(WithCapacity(16), WithIngressQueueSize(32))
	defer b.Close()

	sub := b.Subscribe()
	defer sub.Close()

	if ok := b.Ingest("notifications/log", map[string]any{"job_id": "job-1", "token": "secret", "idx": 1}); !ok {
		t.Fatal("expected ingest to succeed")
	}
	if ok := b.Ingest("notifications/log", map[string]any{"job_id": "job-1", "idx": 2}); !ok {
		t.Fatal("expected second ingest to succeed")
	}

	first := waitForLiveRecord(t, sub, 200*time.Millisecond)
	second := waitForLiveRecord(t, sub, 200*time.Millisecond)

	if first.Sequence >= second.Sequence {
		t.Fatalf("expected strictly increasing live sequence, got %d then %d", first.Sequence, second.Sequence)
	}

	var payload map[string]any
	if err := json.Unmarshal(first.Payload, &payload); err != nil {
		t.Fatalf("unmarshal first payload: %v", err)
	}
	if payload["token"] != "[redacted]" {
		t.Fatalf("expected live payload redaction, got %v", payload["token"])
	}
	if payload["idx"].(float64) != 1 {
		t.Fatalf("expected first live payload idx=1, got %v", payload["idx"])
	}
	if second.Topic != "notifications/log" {
		t.Fatalf("unexpected second topic: %q", second.Topic)
	}
}

func TestLiveSubscriptionDropsWhenSubscriberBackpressures(t *testing.T) {
	b := New(
		WithCapacity(8),
		WithIngressQueueSize(32),
		WithSubscriberQueueDepth(1),
		WithSubscriberMaxBytes(128),
	)
	defer b.Close()

	sub := b.Subscribe()
	defer sub.Close()

	for i := 0; i < 12; i++ {
		_ = b.Ingest("notifications/log", map[string]any{"idx": i, "message": "burst payload to overflow subscriber queue"})
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if sub.Dropped() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("expected live subscriber to drop records under backpressure")
}

func waitForLiveRecord(t *testing.T, sub *Subscription, timeout time.Duration) Record {
	t.Helper()
	select {
	case rec, ok := <-sub.Messages():
		if !ok {
			t.Fatal("subscription closed before record arrived")
		}
		return rec
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for live record after %v", timeout)
	}
	return Record{}
}
