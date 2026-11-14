package agent

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/config"
	"github.com/ishmukhamet/vps-watchdog/internal/event"
	"github.com/ishmukhamet/vps-watchdog/internal/journal"
	"github.com/ishmukhamet/vps-watchdog/internal/liveness"
	"github.com/ishmukhamet/vps-watchdog/internal/metrics"
	"github.com/ishmukhamet/vps-watchdog/internal/notifier"
	"github.com/ishmukhamet/vps-watchdog/internal/state"
)

type Agent struct {
	cfg      config.Config
	notifier notifier.Notifier
	log      *log.Logger
	state    state.Store
	events   chan event.Event

	procRoot   string
	readDisk   func(path string) (metrics.DiskUsage, error)
	loadPinger func(path string) (pinger, error)

	watchMu sync.Mutex
	watchUp map[string]time.Time // journal watcher name -> running since

	bg sync.WaitGroup // background goroutines that touch the state directory
}

func New(cfg config.Config, n notifier.Notifier, logger *log.Logger) *Agent {
	if logger == nil {
		logger = log.Default()
	}
	return &Agent{
		cfg:        cfg,
		notifier:   n,
		log:        logger,
		state:      state.New(state.DefaultDir),
		procRoot:   "/proc",
		readDisk:   metrics.ReadDiskUsage,
		loadPinger: func(path string) (pinger, error) { return liveness.LoadFile(path) },
		watchUp:    map[string]time.Time{},
	}
}

func (a *Agent) Run(ctx context.Context) error {
	if err := a.state.Ensure(); err != nil {
		return err
	}

	a.events = make(chan event.Event, 64)
	go a.runNotifier(ctx)

	errCh := make(chan error, 8)
	if a.cfg.Login.Enabled {
		a.expectWatcher("login")
		go a.runJournalLoop(ctx, "login", journal.Watcher{
			Identifier: "vps-watchdog-login",
			CursorFile: a.state.Path("login.cursor"),
			Logf:       a.log.Printf,
		}, a.handleLoginEntry, errCh)
	}
	if a.cfg.Sudo.Enabled {
		a.expectWatcher("sudo")
		go a.runJournalLoop(ctx, "sudo", journal.Watcher{
			Identifier:  "sudo",
			CursorFile:  a.state.Path("sudo.cursor"),
			AllJournals: true,
			Logf:        a.log.Printf,
			Matches: []string{
				"_COMM=sudo",
				"+",
				"_EXE=/usr/bin/sudo",
				"+",
				"SYSLOG_IDENTIFIER=sudo",
			},
		}, a.handleSudoEntry, errCh)
	}
	if a.cfg.Load.Enabled {
		go func() { errCh <- a.runLoadMonitor(ctx) }()
	}
	if a.cfg.Disk.Enabled {
		go func() { errCh <- a.runDiskMonitor(ctx) }()
	}
	if a.cfg.Reboot.Enabled {
		a.startRebootDetection(ctx, time.Now())
	} else {
		// Stale state would report a bogus reboot when detection is
		// enabled again.
		a.forgetRebootState()
	}
	if a.cfg.Liveness.Enabled {
		a.startLiveness(ctx)
	}

	a.emitEvent(ctx, event.Event{
		Time:     time.Now().UTC(),
		Host:     a.cfg.HostID,
		Type:     event.TypeAgent,
		Severity: event.SeverityInfo,
		Title:    "vps-watchdog started",
	})

	// Watchers report nil when the context is cancelled, so errCh can win the
	// race against ctx.Done on a normal stop; both paths must record it.
	var err error
	select {
	case <-ctx.Done():
	case err = <-errCh:
	}
	if ctx.Err() == nil {
		return err
	}
	a.bg.Wait()
	if a.cfg.Reboot.Enabled {
		a.markCleanStop(time.Now())
	}
	a.sendEvent(context.Background(), event.Event{
		Time:     time.Now().UTC(),
		Host:     a.cfg.HostID,
		Type:     event.TypeAgent,
		Severity: event.SeverityInfo,
		Title:    "vps-watchdog stopped",
	})
	return nil
}

func (a *Agent) runJournalLoop(ctx context.Context, name string, watcher journal.Watcher, handler journal.Handler, errCh chan<- error) {
	backoff := 2 * time.Second
	for {
		if ctx.Err() != nil {
			errCh <- nil
			return
		}
		a.log.Printf("starting %s journal watcher", name)
		a.setWatcherUp(name, time.Now())
		err := watcher.Run(ctx, handler)
		a.setWatcherUp(name, time.Time{})
		if ctx.Err() != nil {
			errCh <- nil
			return
		}
		a.log.Printf("%s journal watcher exited: %v", name, err)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			errCh <- nil
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (a *Agent) handleLoginEntry(ctx context.Context, entry journal.Entry) error {
	fields := journal.ParseKeyValueMessage(entry.Message)
	ts := entry.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	e := event.Event{
		Time:     ts,
		Host:     a.cfg.HostID,
		Type:     event.TypeLogin,
		Severity: event.SeverityWarning,
		Title:    "login detected",
		Fields: map[string]string{
			"service": valueOr(fields["service"], "unknown"),
			"user":    valueOr(fields["user"], "unknown"),
			"rhost":   valueOr(fields["rhost"], "local"),
			"tty":     valueOr(fields["tty"], "unknown"),
		},
	}
	a.emitEvent(ctx, e)
	return nil
}

func (a *Agent) handleSudoEntry(ctx context.Context, entry journal.Entry) error {
	e, ok := a.sudoEvent(entry)
	if !ok {
		return nil
	}
	a.emitEvent(ctx, e)
	return nil
}

func (a *Agent) sudoEvent(entry journal.Entry) (event.Event, bool) {
	message := strings.TrimSpace(entry.Message)
	if message == "" || isSudoSessionNoise(message) {
		return event.Event{}, false
	}

	ts := entry.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	e := event.Event{
		Time:     ts,
		Host:     a.cfg.HostID,
		Type:     event.TypeSudo,
		Severity: event.SeverityWarning,
		Title:    "sudo activity detected",
		Fields: map[string]string{
			"message": message,
		},
	}
	classifySudoMessage(message, e.Fields, &e.Title)
	if entry.Fields["_UID"] != "" {
		e.Fields["uid"] = entry.Fields["_UID"]
	}
	return e, true
}

func isSudoSessionNoise(message string) bool {
	return strings.Contains(message, "pam_unix(sudo:session): session opened") ||
		strings.Contains(message, "pam_unix(sudo:session): session closed")
}

func classifySudoMessage(message string, fields map[string]string, title *string) {
	if strings.Contains(message, "pam_unix(sudo:auth): authentication failure") {
		*title = "sudo authentication failed"
		fields["reason"] = "authentication failure"
		for key, value := range parseSpaceKeyValues(strings.NewReplacer(";", " ").Replace(message)) {
			switch key {
			case "logname":
				fields["actor"] = value
			case "user":
				fields["target_user"] = value
			case "tty":
				fields["tty"] = value
			case "ruser":
				fields["ruser"] = value
			}
		}
		return
	}

	actor, parts, ok := parseSudoColonMessage(message)
	if !ok {
		return
	}
	if actor != "" {
		fields["actor"] = actor
	}
	for _, part := range parts {
		if strings.Contains(part, "user NOT in sudoers") {
			*title = "sudo denied"
			fields["reason"] = "user NOT in sudoers"
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "TTY":
			fields["tty"] = strings.TrimSpace(value)
		case "PWD":
			fields["pwd"] = strings.TrimSpace(value)
		case "USER":
			fields["target_user"] = strings.TrimSpace(value)
		case "COMMAND":
			fields["command"] = strings.TrimSpace(value)
			if *title != "sudo denied" {
				*title = "sudo command executed"
			}
		}
	}
}

func parseSudoColonMessage(message string) (string, []string, bool) {
	actor, rest, ok := strings.Cut(message, ":")
	if !ok {
		return "", nil, false
	}
	actor = strings.TrimSpace(actor)
	if actor == "" || strings.Contains(actor, "(") {
		return "", nil, false
	}
	parts := strings.Split(rest, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return actor, out, len(out) > 0
}

func parseSpaceKeyValues(message string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Fields(message) {
		key, value, ok := strings.Cut(part, "=")
		if ok && key != "" {
			out[key] = value
		}
	}
	return out
}

func (a *Agent) runLoadMonitor(ctx context.Context) error {
	collector := metrics.NewCollector()
	interval := time.Duration(a.cfg.Load.SampleIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	cpuState := newThresholdState("high CPU", event.SeverityWarning, time.Duration(a.cfg.Load.CPUDurationSec)*time.Second, time.Duration(a.cfg.Load.CooldownSec)*time.Second)
	memState := newThresholdState("low memory", event.SeverityWarning, time.Duration(a.cfg.Load.MemDurationSec)*time.Second, time.Duration(a.cfg.Load.CooldownSec)*time.Second)
	ioState := newThresholdState("high IO pressure", event.SeverityWarning, time.Duration(a.cfg.Load.IODurationSec)*time.Second, time.Duration(a.cfg.Load.CooldownSec)*time.Second)

	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			snap, err := collector.Snapshot()
			if err != nil {
				a.log.Printf("metrics snapshot failed: %v", err)
				continue
			}
			if snap.CPUAvailable {
				a.evaluateThreshold(ctx, cpuState, now, snap.CPUBusyPercent >= a.cfg.Load.CPUBusyPercent, map[string]string{
					"cpu_busy_percent": fmt.Sprintf("%.1f", snap.CPUBusyPercent),
					"threshold":        fmt.Sprintf("%.1f", a.cfg.Load.CPUBusyPercent),
				})
			}
			if snap.MemAvailable {
				a.evaluateThreshold(ctx, memState, now, snap.MemAvailablePercent <= a.cfg.Load.MemAvailablePercent, map[string]string{
					"mem_available_percent": fmt.Sprintf("%.1f", snap.MemAvailablePercent),
					"threshold":             fmt.Sprintf("%.1f", a.cfg.Load.MemAvailablePercent),
				})
			}
			if snap.PSIIO.Available {
				a.evaluateThreshold(ctx, ioState, now, snap.PSIIO.SomeAvg60 >= a.cfg.Load.PSIIOSomeAvg60, map[string]string{
					"psi_io_some_avg60": fmt.Sprintf("%.2f", snap.PSIIO.SomeAvg60),
					"threshold":         fmt.Sprintf("%.2f", a.cfg.Load.PSIIOSomeAvg60),
				})
			}
		}
	}
}

type thresholdState struct {
	Type      event.Type
	Name      string
	Severity  event.Severity
	Duration  time.Duration
	Cooldown  time.Duration
	Since     time.Time
	Alerted   bool
	LastAlert time.Time
}

func newThresholdState(name string, severity event.Severity, duration, cooldown time.Duration) *thresholdState {
	return &thresholdState{Type: event.TypeLoad, Name: name, Severity: severity, Duration: duration, Cooldown: cooldown}
}

func (a *Agent) evaluateThreshold(ctx context.Context, s *thresholdState, now time.Time, breached bool, fields map[string]string) {
	if !breached {
		if s.Alerted {
			a.emitEvent(ctx, event.Event{
				Time:     now.UTC(),
				Host:     a.cfg.HostID,
				Type:     s.Type,
				Severity: event.SeverityInfo,
				Title:    s.Name + " recovered",
				Fields:   fields,
			})
		}
		s.Since = time.Time{}
		s.Alerted = false
		return
	}
	if s.Since.IsZero() {
		s.Since = now
		if s.Duration > 0 {
			return
		}
	}
	if now.Sub(s.Since) < s.Duration {
		return
	}
	if s.Alerted && now.Sub(s.LastAlert) < s.Cooldown {
		return
	}
	fields["duration_sec"] = strconv.Itoa(int(now.Sub(s.Since).Seconds()))
	a.emitEvent(ctx, event.Event{
		Time:     now.UTC(),
		Host:     a.cfg.HostID,
		Type:     s.Type,
		Severity: s.Severity,
		Title:    s.Name,
		Fields:   fields,
	})
	s.Alerted = true
	s.LastAlert = now
}

func (a *Agent) runNotifier(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-a.events:
			a.sendEvent(ctx, e)
		}
	}
}

func (a *Agent) emitEvent(ctx context.Context, e event.Event) {
	if a.events == nil {
		a.sendEvent(ctx, e)
		return
	}
	select {
	case a.events <- e:
	case <-ctx.Done():
	default:
		a.log.Printf("notification queue full; dropping event type=%s title=%q", e.Type, e.Title)
	}
}

func (a *Agent) sendEvent(ctx context.Context, e event.Event) {
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	if err := a.notifier.Send(ctx, event.FormatTelegramHTML(e)); err != nil {
		a.log.Printf("notification failed: %v", err)
	}
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
