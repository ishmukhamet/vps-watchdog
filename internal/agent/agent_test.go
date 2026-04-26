package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/config"
	"github.com/ishmukhamet/vps-watchdog/internal/event"
	"github.com/ishmukhamet/vps-watchdog/internal/journal"
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

func TestSudoEventClassifiesExecutedCommand(t *testing.T) {
	a := New(config.Default(), &captureNotifier{}, nil)
	e, ok := a.sudoEvent(journal.Entry{Message: "ishmukhamet : TTY=pts/0 ; PWD=/home/ishmukhamet ; USER=root ; COMMAND=/usr/bin/true"})
	if !ok {
		t.Fatal("expected sudo event")
	}
	if e.Title != "sudo command executed" {
		t.Fatalf("title = %q", e.Title)
	}
	if e.Fields["actor"] != "ishmukhamet" || e.Fields["target_user"] != "root" || e.Fields["command"] != "/usr/bin/true" {
		t.Fatalf("fields = %+v", e.Fields)
	}
}

func TestSudoEventClassifiesDeniedUser(t *testing.T) {
	a := New(config.Default(), &captureNotifier{}, nil)
	e, ok := a.sudoEvent(journal.Entry{Message: "   cloud : user NOT in sudoers ; TTY=pts/1 ; PWD=/home/cloud ; USER=root ; COMMAND=/usr/bin/apt update"})
	if !ok {
		t.Fatal("expected sudo event")
	}
	if e.Title != "sudo denied" {
		t.Fatalf("title = %q", e.Title)
	}
	if e.Fields["actor"] != "cloud" || e.Fields["reason"] != "user NOT in sudoers" || e.Fields["command"] != "/usr/bin/apt update" {
		t.Fatalf("fields = %+v", e.Fields)
	}
}

func TestSudoEventClassifiesAuthenticationFailure(t *testing.T) {
	a := New(config.Default(), &captureNotifier{}, nil)
	e, ok := a.sudoEvent(journal.Entry{Message: "pam_unix(sudo:auth): authentication failure; logname=cloud uid=1001 euid=0 tty=/dev/pts/1 ruser=cloud rhost=  user=cloud"})
	if !ok {
		t.Fatal("expected sudo event")
	}
	if e.Title != "sudo authentication failed" {
		t.Fatalf("title = %q", e.Title)
	}
	if e.Fields["actor"] != "cloud" || e.Fields["target_user"] != "cloud" || e.Fields["tty"] != "/dev/pts/1" {
		t.Fatalf("fields = %+v", e.Fields)
	}
}

func TestSudoEventIgnoresSessionNoise(t *testing.T) {
	a := New(config.Default(), &captureNotifier{}, nil)
	for _, message := range []string{
		"pam_unix(sudo:session): session opened for user root(uid=0) by ishmukhamet(uid=1000)",
		"pam_unix(sudo:session): session closed for user root",
	} {
		if _, ok := a.sudoEvent(journal.Entry{Message: message}); ok {
			t.Fatalf("expected session noise to be ignored: %s", message)
		}
	}
}

func TestHandleSudoEntrySendsClassifiedEvent(t *testing.T) {
	n := &captureNotifier{}
	a := New(config.Default(), n, nil)
	if err := a.handleSudoEntry(context.Background(), journal.Entry{Message: "ishmukhamet : TTY=pts/0 ; PWD=/home/ishmukhamet ; USER=root ; COMMAND=/usr/bin/true"}); err != nil {
		t.Fatalf("handleSudoEntry() error = %v", err)
	}
	if len(n.messages) != 1 || !strings.Contains(n.messages[0], "sudo command executed") {
		t.Fatalf("messages = %+v", n.messages)
	}
}
