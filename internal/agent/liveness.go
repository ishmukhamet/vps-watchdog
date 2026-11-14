package agent

import (
	"context"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/event"
)

// minWatcherUptime is how long every journal watcher must have been running
// before a ping is sent. A watcher that cannot read the journal exits within
// milliseconds, so it never counts as running.
const minWatcherUptime = 5 * time.Second

type pinger interface {
	Ping(ctx context.Context) error
}

func (a *Agent) expectWatcher(name string) {
	a.watchMu.Lock()
	defer a.watchMu.Unlock()
	a.watchUp[name] = time.Time{}
}

func (a *Agent) setWatcherUp(name string, since time.Time) {
	a.watchMu.Lock()
	defer a.watchMu.Unlock()
	a.watchUp[name] = since
}

// watchersHealthy reports whether every enabled journal watcher has been
// running for at least minWatcherUptime. With no watchers enabled, the agent
// counts as healthy while it runs.
func (a *Agent) watchersHealthy(now time.Time) bool {
	a.watchMu.Lock()
	defer a.watchMu.Unlock()
	for _, since := range a.watchUp {
		if since.IsZero() || now.Sub(since) < minWatcherUptime {
			return false
		}
	}
	return true
}

// startLiveness pings the external dead man's switch while the agent is
// watching (ADR 0008). It runs outside the event queue; failures are logged
// and never block collection or delivery.
func (a *Agent) startLiveness(ctx context.Context) {
	p, err := a.loadPinger(a.cfg.Liveness.PingURLFile)
	if err != nil {
		a.log.Printf("liveness pings disabled: %v", err)
		a.emitEvent(ctx, event.Event{
			Time:     time.Now().UTC(),
			Host:     a.cfg.HostID,
			Type:     event.TypeAgent,
			Severity: event.SeverityWarning,
			Title:    "liveness pings disabled",
			Fields:   map[string]string{"error": err.Error()},
		})
		return
	}
	// The first attempt comes as soon as watchers can count as running, so a
	// restart is confirmed quickly instead of after a full interval.
	go a.runLiveness(ctx, p, minWatcherUptime+time.Second, time.Duration(a.cfg.Liveness.IntervalSec)*time.Second)
}

func (a *Agent) runLiveness(ctx context.Context, p pinger, first, interval time.Duration) {
	timer := time.NewTimer(first)
	defer timer.Stop()
	failing, skipping := false, false
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		timer.Reset(interval)

		if !a.watchersHealthy(time.Now()) {
			if !skipping {
				a.log.Printf("liveness ping skipped: a journal watcher is not running")
				skipping = true
			}
			continue
		}
		skipping = false

		pingCtx, cancel := context.WithTimeout(ctx, interval/2)
		err := p.Ping(pingCtx)
		cancel()
		switch {
		case err != nil && ctx.Err() == nil && !failing:
			a.log.Printf("liveness %v", err)
			failing = true
		case err == nil && failing:
			a.log.Printf("liveness ping recovered")
			failing = false
		}
	}
}
