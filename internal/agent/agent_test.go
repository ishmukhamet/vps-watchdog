package agent

import (
	"context"
	"testing"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/config"
	"github.com/ishmukhamet/vps-watchdog/internal/event"
)

type captureNotifier struct{ messages []string }

func (c *captureNotifier) Send(_ context.Context, message string) error {
	c.messages = append(c.messages, message)
	return nil
}

func TestEvaluateThresholdAlertsAfterDuration(t *testing.T) {
	n := &captureNotifier{}
	a := New(config.Default(), n, nil)
	s := newThresholdState("high CPU", "warning", 10*time.Second, time.Minute)
	now := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	a.evaluateThreshold(context.Background(), s, now, true, map[string]string{"x": "1"})
	a.evaluateThreshold(context.Background(), s, now.Add(9*time.Second), true, map[string]string{"x": "1"})
	if len(n.messages) != 0 {
		t.Fatalf("unexpected messages: %v", n.messages)
	}
	a.evaluateThreshold(context.Background(), s, now.Add(10*time.Second), true, map[string]string{"x": "1"})
	if len(n.messages) != 1 {
		t.Fatalf("messages = %d", len(n.messages))
	}
}

func TestEmitEventDoesNotBlockWhenQueueIsFull(t *testing.T) {
	a := New(config.Default(), &captureNotifier{}, nil)
	a.events = make(chan event.Event, 1)
	a.events <- event.Event{Title: "queued"}

	done := make(chan struct{})
	go func() {
		a.emitEvent(context.Background(), event.Event{Title: "dropped"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("emitEvent blocked on full notification queue")
	}
}
