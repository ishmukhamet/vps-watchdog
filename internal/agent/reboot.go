package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/event"
	"github.com/ishmukhamet/vps-watchdog/internal/metrics"
	"github.com/ishmukhamet/vps-watchdog/internal/state"
)

// State files for reboot detection (ADR 0009).
const (
	bootIDFile     = "boot_id"
	lastAliveFile  = "last_alive"
	cleanStopFile  = "clean_stop"
	pendingReboot  = "reboot.pending"
	maxStateBytes  = 16 * 1024
	aliveInterval  = 60 * time.Second
	retryMinDelay  = 10 * time.Second
	retryMaxDelay  = 5 * time.Minute
	sendTimeout    = 30 * time.Second
	timeFieldFmt   = "2006-01-02 15:04:05 UTC"
	uncleanSummary = "no clean stop recorded: forced reset, power loss, or crash"
)

// startRebootDetection compares the boot id with the saved one, queues a
// durable reboot event when it changed, and starts refreshing the "last alive"
// timestamp.
func (a *Agent) startRebootDetection(ctx context.Context, now time.Time) {
	current, err := metrics.ReadBootID(a.procRoot)
	if err != nil {
		a.log.Printf("reboot detection disabled: %v", err)
		return
	}
	saved, hasSaved := a.readBootID()
	lastAlive, hasAlive := a.readUnixTime(lastAliveFile)
	stopped, cleanStop := a.readUnixTime(cleanStopFile)

	// Order matters for a crash during startup: the pending event is written
	// before the new boot id, and the clean stop marker is removed last, so
	// a crash in between detects the same reboot again with the same facts.
	if hasSaved && saved != current {
		bootedAt, err := metrics.ReadBootTime(a.procRoot)
		if err != nil {
			a.log.Printf("reboot detection: boot time: %v", err)
		}
		e := a.rebootEvent(now, bootedAt, lastAlive, hasAlive, stopped, cleanStop)
		if err := a.addPending(e); err != nil {
			a.log.Printf("reboot detection: save pending event: %v", err)
			a.emitEvent(ctx, e)
		}
	}
	if !hasSaved || saved != current {
		if err := state.WriteAtomic(a.state.Path(bootIDFile), []byte(current+"\n")); err != nil {
			a.log.Printf("reboot detection: save boot id: %v", err)
		}
	}
	if err := state.Remove(a.state.Path(cleanStopFile)); err != nil {
		a.log.Printf("reboot detection: remove clean stop marker: %v", err)
	}

	a.bg.Add(2)
	go func() { defer a.bg.Done(); a.deliverPending(ctx) }()
	go func() { defer a.bg.Done(); a.refreshAlive(ctx) }()
}

// forgetRebootState removes reboot detection state, so that enabling
// detection later starts quietly instead of comparing against an old boot.
func (a *Agent) forgetRebootState() {
	for _, name := range []string{bootIDFile, lastAliveFile, cleanStopFile} {
		if err := state.Remove(a.state.Path(name)); err != nil {
			a.log.Printf("reboot detection: remove %s: %v", name, err)
		}
	}
}

func (a *Agent) rebootEvent(now, bootedAt, lastAlive time.Time, hasAlive bool, stopped time.Time, cleanStop bool) event.Event {
	fields := map[string]string{}
	lastSeen, hasLastSeen := lastAlive, hasAlive
	if cleanStop {
		fields["previous_run"] = "stopped cleanly at " + stopped.UTC().Format(timeFieldFmt)
		if !hasLastSeen || stopped.After(lastSeen) {
			lastSeen, hasLastSeen = stopped, true
		}
	} else {
		fields["previous_run"] = uncleanSummary
	}
	if hasLastSeen {
		fields["last_seen"] = lastSeen.UTC().Format(timeFieldFmt)
	}
	if !bootedAt.IsZero() {
		fields["booted_at"] = bootedAt.UTC().Format(timeFieldFmt)
		if hasLastSeen && bootedAt.After(lastSeen) {
			fields["downtime"] = "about " + bootedAt.Sub(lastSeen).Round(time.Second).String()
		}
	}
	return event.Event{
		Time:     now.UTC(),
		Host:     a.cfg.HostID,
		Type:     event.TypeReboot,
		Severity: event.SeverityWarning,
		Title:    "host rebooted",
		Fields:   fields,
	}
}

// markCleanStop records a graceful stop, so the next boot can tell a planned
// shutdown from a forced reset.
func (a *Agent) markCleanStop(now time.Time) {
	value := []byte(strconv.FormatInt(now.Unix(), 10) + "\n")
	if err := state.WriteAtomic(a.state.Path(cleanStopFile), value); err != nil {
		a.log.Printf("reboot detection: save clean stop marker: %v", err)
	}
}

func (a *Agent) refreshAlive(ctx context.Context) {
	ticker := time.NewTicker(aliveInterval)
	defer ticker.Stop()
	failing := false
	for {
		value := []byte(strconv.FormatInt(time.Now().Unix(), 10) + "\n")
		err := state.WriteAtomic(a.state.Path(lastAliveFile), value)
		if err != nil && !failing {
			a.log.Printf("reboot detection: save last alive time: %v", err)
		}
		failing = err != nil
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// deliverPending sends a stored reboot event, retrying with backoff until it
// is delivered or the agent stops; the file survives restarts.
func (a *Agent) deliverPending(ctx context.Context) {
	path := a.state.Path(pendingReboot)
	delay := retryMinDelay
	for {
		e, ok, err := a.readPending()
		var corrupt *pendingCorruptError
		if errors.As(err, &corrupt) {
			a.log.Printf("reboot detection: discard corrupt pending event: %v", err)
			_ = state.Remove(path)
			return
		}
		if err != nil {
			// Possibly transient (I/O error); keep the file for the next start.
			a.log.Printf("reboot detection: read pending event: %v", err)
			return
		}
		if !ok {
			return
		}
		sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
		err = a.notifier.Send(sendCtx, event.FormatTelegramHTML(e))
		cancel()
		if err == nil {
			if err := state.Remove(path); err != nil {
				a.log.Printf("reboot detection: remove delivered pending event: %v", err)
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		a.log.Printf("reboot notification failed, retrying in %s: %v", delay, err)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if delay *= 2; delay > retryMaxDelay {
			delay = retryMaxDelay
		}
	}
}

// addPending stores e for delivery. If an earlier reboot is still
// undelivered, the new event keeps a reference to it instead of replacing it
// silently; the count of older undelivered reboots is carried along.
func (a *Agent) addPending(e event.Event) error {
	if prev, ok, err := a.readPending(); err == nil && ok {
		earlier, _ := strconv.Atoi(prev.Fields[earlierCountKey])
		e.Fields[earlierCountKey] = strconv.Itoa(earlier + 1)
		if first := prev.Fields[earliestKey]; first != "" {
			e.Fields[earliestKey] = first
		} else {
			e.Fields[earliestKey] = prev.Time.UTC().Format(timeFieldFmt)
		}
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return state.WriteAtomic(a.state.Path(pendingReboot), data)
}

// Fields that summarise reboots whose alerts were never delivered.
const (
	earlierCountKey = "undelivered_earlier_reboots"
	earliestKey     = "earliest_undelivered_reboot_detected_at"
)

type pendingCorruptError struct{ err error }

func (e *pendingCorruptError) Error() string { return e.err.Error() }

func (a *Agent) readPending() (event.Event, bool, error) {
	data, err := state.ReadSmall(a.state.Path(pendingReboot), maxStateBytes)
	if errors.Is(err, os.ErrNotExist) {
		return event.Event{}, false, nil
	}
	if err != nil {
		if strings.Contains(err.Error(), "larger than") {
			return event.Event{}, false, &pendingCorruptError{err}
		}
		return event.Event{}, false, err
	}
	var e event.Event
	if err := json.Unmarshal(data, &e); err != nil {
		return event.Event{}, false, &pendingCorruptError{err}
	}
	if e.Type != event.TypeReboot || e.Fields == nil {
		return event.Event{}, false, &pendingCorruptError{errors.New("unexpected event content")}
	}
	return e, true, nil
}

func (a *Agent) readBootID() (string, bool) {
	data, err := state.ReadSmall(a.state.Path(bootIDFile), 64)
	if err != nil {
		return "", false
	}
	id := strings.TrimSpace(string(data))
	return id, metrics.ValidBootID(id)
}

func (a *Agent) readUnixTime(name string) (time.Time, bool) {
	data, err := state.ReadSmall(a.state.Path(name), 32)
	if err != nil {
		return time.Time{}, false
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil || sec <= 0 {
		return time.Time{}, false
	}
	return time.Unix(sec, 0).UTC(), true
}
