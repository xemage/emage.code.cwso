package eventbus

import (
	"encoding/json"
	"testing"
	"time"
)

func mustReadMessage(t *testing.T, ch <-chan Message) Message {
	t.Helper()
	select {
	case msg, ok := <-ch:
		if !ok {
			t.Fatal("subscription closed unexpectedly")
		}
		return msg
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for message")
		return Message{}
	}
}

func TestPublishFanoutToMultipleSubscribers(t *testing.T) {
	bus := New()
	sub1 := bus.Subscribe()
	sub2 := bus.Subscribe()
	defer sub1.Close()
	defer sub2.Close()

	err := bus.Publish(TopicNotificationsJobState, map[string]any{"state": "running"})
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	msg1 := mustReadMessage(t, sub1.Messages())
	msg2 := mustReadMessage(t, sub2.Messages())

	if msg1.Topic != TopicNotificationsJobState || msg2.Topic != TopicNotificationsJobState {
		t.Fatalf("unexpected topics: %q %q", msg1.Topic, msg2.Topic)
	}

	var payload map[string]string
	if err := json.Unmarshal(msg1.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["state"] != "running" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}

func TestSlowSubscriberDropsWithoutAffectingFastSubscriber(t *testing.T) {
	bus := New(
		WithQueueDepth(4),
		WithSubscriberMaxBytes(256),
	)
	slow := bus.Subscribe()
	fast := bus.Subscribe()
	defer slow.Close()
	defer fast.Close()

	const publishCount = 30
	gotFast := make(chan int, 1)
	go func() {
		count := 0
		deadline := time.After(500 * time.Millisecond)
		for count < publishCount {
			select {
			case <-fast.Messages():
				count++
			case <-deadline:
				gotFast <- count
				return
			}
		}
		gotFast <- count
	}()

	for i := 0; i < publishCount; i++ {
		payload := map[string]any{
			"idx":   i,
			"state": "running",
			"blob":  "123456789012345678901234567890",
		}
		if err := bus.Publish(TopicNotificationsLog, payload); err != nil {
			t.Fatalf("publish %d failed: %v", i, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	fastCount := <-gotFast
	if fastCount != publishCount {
		t.Fatalf("fast subscriber should receive all events: got %d want %d", fastCount, publishCount)
	}

	if fast.Dropped() != 0 {
		t.Fatalf("fast subscriber should not drop events, got %d", fast.Dropped())
	}
	if slow.Dropped() == 0 {
		t.Fatal("slow subscriber should drop events")
	}
}
