package agent

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/config"
	"github.com/ishmukhamet/vps-watchdog/internal/event"
	"github.com/ishmukhamet/vps-watchdog/internal/metrics"
	"github.com/ishmukhamet/vps-watchdog/internal/state"
)

const (
	bootA = "3223bc29-6995-44d6-82e7-38ae2d4e7e83"
	bootB = "8f0c1d2e-3b4a-4c5d-9e6f-7a8b9c0d1e2f"
)

type syncNotifier struct {
	mu       sync.Mutex
	messages []string
	fail     atomic.Int32 // number of upcoming sends that fail
}

func (n *syncNotifier) Send(_ context.Context, message string) error {
	if n.fail.Load() > 0 {
		n.fail.Add(-1)
		return errors.New("network down")
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, message)
	return nil
}

func (n *syncNotifier) all() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return strings.Join(n.messages, "\n---\n")
}

func testAgent(t *testing.T, n *syncNotifier) (*Agent, string) {
	t.Helper()
	dir := t.TempDir()
	a := New(config.Default(), n, log.New(io.Discard, "", 0))
	a.state = state.New(filepath.Join(dir, "state"))
	if err := a.state.Ensure(); err != nil {
		t.Fatal(err)
	}
	a.procRoot = filepath.Join(dir, "proc")
	return a, dir
}

// runCtx returns a context cancelled at test end, after which the agent's
// background goroutines are awaited before the temp dir is removed.
func runCtx(t *testing.T, a *Agent) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		a.bg.Wait()
	})
	return ctx
}

func setBoot(t *testing.T, a *Agent, id string, btime int64) {
	t.Helper()
	dir := filepath.Join(a.procRoot, "sys/kernel/random")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "boot_id"), []byte(id+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stat := "cpu 1 2 3 4 5 6 7 8\nbtime " + strconv.FormatInt(btime, 10) + "\n"
	if err := os.WriteFile(filepath.Join(a.procRoot, "stat"), []byte(stat), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRebootDetectionFirstStartIsQuiet(t *testing.T) {
	n := &syncNotifier{}
	a, _ := testAgent(t, n)
	setBoot(t, a, bootA, 1777200000)
	ctx := runCtx(t, a)
	a.startRebootDetection(ctx, time.Now())
	if _, err := os.Stat(a.state.Path(pendingReboot)); !os.IsNotExist(err) {
		t.Fatalf("first start must not report a reboot: %v", err)
	}
	if id, ok := a.readBootID(); !ok || id != bootA {
		t.Fatalf("boot id not recorded: %q", id)
	}
}

func TestRebootDetectionSameBootIsNotAReboot(t *testing.T) {
	n := &syncNotifier{}
	a, _ := testAgent(t, n)
	setBoot(t, a, bootA, 1777200000)
	_ = state.WriteAtomic(a.state.Path(bootIDFile), []byte(bootA))
	ctx := runCtx(t, a)
	a.startRebootDetection(ctx, time.Now())
	if _, err := os.Stat(a.state.Path(pendingReboot)); !os.IsNotExist(err) {
		t.Fatalf("agent restart reported as reboot: %v", err)
	}
}

func TestRebootDetectionReportsUncleanRebootWithDowntime(t *testing.T) {
	n := &syncNotifier{}
	a, _ := testAgent(t, n)
	setBoot(t, a, bootB, 1777203600)
	_ = state.WriteAtomic(a.state.Path(bootIDFile), []byte(bootA))
	_ = state.WriteAtomic(a.state.Path(lastAliveFile), []byte("1777201200"))
	ctx := runCtx(t, a)
	a.startRebootDetection(ctx, time.Unix(1777203700, 0))
	waitFor(t, func() bool { return strings.Contains(n.all(), "host rebooted") })
	msg := n.all()
	for _, want := range []string{uncleanSummary, "about 40m0s", "booted_at"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q:\n%s", want, msg)
		}
	}
	waitFor(t, func() bool {
		_, err := os.Stat(a.state.Path(pendingReboot))
		return os.IsNotExist(err)
	})
	if id, _ := a.readBootID(); id != bootB {
		t.Fatalf("boot id not updated: %q", id)
	}
}

func TestRebootDetectionReportsCleanStop(t *testing.T) {
	n := &syncNotifier{}
	a, _ := testAgent(t, n)
	setBoot(t, a, bootB, 1777200030)
	_ = state.WriteAtomic(a.state.Path(bootIDFile), []byte(bootA))
	_ = state.WriteAtomic(a.state.Path(lastAliveFile), []byte("1777199990"))
	a.markCleanStop(time.Unix(1777200000, 0))
	ctx := runCtx(t, a)
	a.startRebootDetection(ctx, time.Unix(1777200040, 0))
	waitFor(t, func() bool { return strings.Contains(n.all(), "host rebooted") })
	msg := n.all()
	if !strings.Contains(msg, "stopped cleanly") || !strings.Contains(msg, "about 30s") {
		t.Fatalf("unexpected message:\n%s", msg)
	}
	if _, err := os.Stat(a.state.Path(cleanStopFile)); !os.IsNotExist(err) {
		t.Fatal("clean stop marker must be consumed on start")
	}
}

func TestPendingRebootSurvivesFailedDeliveryAndRestart(t *testing.T) {
	n := &syncNotifier{}
	n.fail.Store(1000)
	a, _ := testAgent(t, n)
	setBoot(t, a, bootB, 1777203600)
	_ = state.WriteAtomic(a.state.Path(bootIDFile), []byte(bootA))
	ctx, cancel := context.WithCancel(context.Background())
	a.startRebootDetection(ctx, time.Now())
	time.Sleep(100 * time.Millisecond)
	cancel()
	a.bg.Wait()
	if _, err := os.Stat(a.state.Path(pendingReboot)); err != nil {
		t.Fatalf("pending event lost after failed delivery: %v", err)
	}

	// Next start on the same boot: no new reboot, but the pending one is sent.
	n.fail.Store(0)
	a.startRebootDetection(runCtx(t, a), time.Now())
	waitFor(t, func() bool { return strings.Count(n.all(), "host rebooted") == 1 })
	waitFor(t, func() bool {
		_, err := os.Stat(a.state.Path(pendingReboot))
		return os.IsNotExist(err)
	})
}

func TestReadPendingRejectsGarbage(t *testing.T) {
	a, _ := testAgent(t, &syncNotifier{})
	_ = state.WriteAtomic(a.state.Path(pendingReboot), []byte(`{"Type":"login","Title":"x"}`))
	if _, _, err := a.readPending(); err == nil {
		t.Fatal("expected error for a non-reboot event")
	}
}

func diskAgent(t *testing.T, usage map[string]metrics.DiskUsage) (*Agent, *captureNotifier) {
	t.Helper()
	n := &captureNotifier{}
	cfg := config.Default()
	cfg.Disk.Paths = []string{"/", "/var", "/data"}
	cfg.Disk.DurationSec = 60
	a := New(cfg, n, log.New(io.Discard, "", 0))
	a.readDisk = func(path string) (metrics.DiskUsage, error) {
		u, ok := usage[path]
		if !ok {
			return metrics.DiskUsage{}, errors.New("no such mount")
		}
		return u, nil
	}
	return a, n
}

func TestDiskAlertsAfterDurationOncePerFilesystem(t *testing.T) {
	low := metrics.DiskUsage{Device: 1, FreePercent: 3, AvailBytes: 1 << 30, SizeBytes: 32 << 30, InodesKnown: true, InodesFreePercent: 50}
	a, n := diskAgent(t, map[string]metrics.DiskUsage{"/": low, "/var": low})
	m := newDiskMonitor()
	now := time.Unix(1777200000, 0)
	a.checkDisks(context.Background(), m, now)
	a.checkDisks(context.Background(), m, now.Add(30*time.Second))
	if len(n.messages) != 0 {
		t.Fatalf("alerted before duration: %v", n.messages)
	}
	a.checkDisks(context.Background(), m, now.Add(60*time.Second))
	if len(n.messages) != 1 {
		t.Fatalf("messages = %d, want one alert for the shared filesystem: %v", len(n.messages), n.messages)
	}
	if !strings.Contains(n.messages[0], "low disk space on /") || !strings.Contains(n.messages[0], "1.0 GiB of 32.0 GiB") {
		t.Fatalf("unexpected message: %s", n.messages[0])
	}
}

func TestDiskInodesAndRecovery(t *testing.T) {
	usage := map[string]metrics.DiskUsage{"/": {Device: 1, FreePercent: 50, InodesKnown: true, InodesFree: 5, InodesTotal: 100, InodesFreePercent: 5}}
	a, n := diskAgent(t, usage)
	a.cfg.Disk.DurationSec = 0
	m := newDiskMonitor()
	now := time.Unix(1777200000, 0)
	a.checkDisks(context.Background(), m, now)
	if len(n.messages) != 1 || !strings.Contains(n.messages[0], "low free inodes on /") {
		t.Fatalf("messages = %v", n.messages)
	}
	usage["/"] = metrics.DiskUsage{Device: 1, FreePercent: 50, InodesKnown: true, InodesFreePercent: 50}
	a.checkDisks(context.Background(), m, now.Add(time.Second))
	if len(n.messages) != 2 || !strings.Contains(n.messages[1], "recovered") {
		t.Fatalf("messages = %v", n.messages)
	}
}

func TestDiskIgnoresUnknownInodes(t *testing.T) {
	a, n := diskAgent(t, map[string]metrics.DiskUsage{"/": {Device: 1, FreePercent: 50}})
	a.cfg.Disk.DurationSec = 0
	m := newDiskMonitor()
	for i := 0; i < 3; i++ {
		a.checkDisks(context.Background(), m, time.Unix(int64(1777200000+i), 0))
	}
	if len(n.messages) != 0 {
		t.Fatalf("unexpected alerts: %v", n.messages)
	}
}

type countingPinger struct{ n atomic.Int32 }

func (p *countingPinger) Ping(context.Context) error {
	p.n.Add(1)
	return nil
}

func TestWatchersHealthy(t *testing.T) {
	a := New(config.Default(), &captureNotifier{}, log.New(io.Discard, "", 0))
	now := time.Now()
	if !a.watchersHealthy(now) {
		t.Fatal("no watchers enabled should count as healthy")
	}
	a.expectWatcher("login")
	if a.watchersHealthy(now) {
		t.Fatal("watcher that never started counted as healthy")
	}
	a.setWatcherUp("login", now.Add(-time.Second))
	if a.watchersHealthy(now) {
		t.Fatal("watcher running for less than the minimum counted as healthy")
	}
	a.setWatcherUp("login", now.Add(-minWatcherUptime))
	if !a.watchersHealthy(now) {
		t.Fatal("running watcher not counted as healthy")
	}
	a.setWatcherUp("login", time.Time{})
	if a.watchersHealthy(now) {
		t.Fatal("stopped watcher counted as healthy")
	}
}

func TestStartLivenessReportsBadURLFile(t *testing.T) {
	a := New(config.Default(), &captureNotifier{}, log.New(io.Discard, "", 0))
	a.events = make(chan event.Event, 4)
	a.loadPinger = func(string) (pinger, error) { return nil, errors.New("ping url must use https") }
	a.startLiveness(context.Background())
	select {
	case e := <-a.events:
		if e.Title != "liveness pings disabled" {
			t.Fatalf("unexpected event: %+v", e)
		}
	default:
		t.Fatal("misconfigured liveness must be reported")
	}
}

func TestRunLivenessPingsOnlyWhileWatchersRun(t *testing.T) {
	a := New(config.Default(), &captureNotifier{}, log.New(io.Discard, "", 0))
	a.expectWatcher("login")
	p := &countingPinger{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.runLiveness(ctx, p, time.Millisecond, 10*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	if got := p.n.Load(); got != 0 {
		t.Fatalf("pinged %d times while the watcher was not running", got)
	}
	a.setWatcherUp("login", time.Now().Add(-minWatcherUptime))
	waitFor(t, func() bool { return p.n.Load() >= 2 })
	cancel()
}

func TestSecondRebootKeepsUndeliveredOne(t *testing.T) {
	n := &syncNotifier{}
	n.fail.Store(1000)
	a, _ := testAgent(t, n)
	setBoot(t, a, bootB, 1777203600)
	_ = state.WriteAtomic(a.state.Path(bootIDFile), []byte(bootA))
	ctx, cancel := context.WithCancel(context.Background())
	a.startRebootDetection(ctx, time.Unix(1777203700, 0))
	cancel()
	a.bg.Wait()

	setBoot(t, a, bootA, 1777207200)
	n.fail.Store(0)
	a.startRebootDetection(runCtx(t, a), time.Unix(1777207300, 0))
	waitFor(t, func() bool { return strings.Contains(n.all(), "host rebooted") })
	msg := n.all()
	if strings.Count(msg, "host rebooted") != 1 || !strings.Contains(msg, earlierCountKey) || !strings.Contains(msg, "2026-04-26 11:41:40 UTC") {
		t.Fatalf("earlier undelivered reboot not reported:\n%s", msg)
	}
}

// A crash after the pending event is saved but before the boot id is
// updated must not turn a clean stop into an unclean one.
func TestCrashDuringStartupKeepsCleanStop(t *testing.T) {
	a, _ := testAgent(t, &syncNotifier{})
	setBoot(t, a, bootB, 1777200030)
	_ = state.WriteAtomic(a.state.Path(bootIDFile), []byte(bootA))
	a.markCleanStop(time.Unix(1777200000, 0))
	// Simulate the crash window: the event is stored, nothing else changed.
	stopped, clean := a.readUnixTime(cleanStopFile)
	if err := a.addPending(a.rebootEvent(time.Unix(1777200040, 0), time.Time{}, time.Time{}, false, stopped, clean)); err != nil {
		t.Fatal(err)
	}
	_ = state.Remove(a.state.Path(pendingReboot))
	if _, ok := a.readUnixTime(cleanStopFile); !ok {
		t.Fatal("clean stop marker must survive until the boot id is updated")
	}
}

func TestDisabledRebootDetectionForgetsState(t *testing.T) {
	a, _ := testAgent(t, &syncNotifier{})
	for _, name := range []string{bootIDFile, lastAliveFile, cleanStopFile} {
		_ = state.WriteAtomic(a.state.Path(name), []byte("1"))
	}
	a.forgetRebootState()
	for _, name := range []string{bootIDFile, lastAliveFile, cleanStopFile} {
		if _, err := os.Stat(a.state.Path(name)); !os.IsNotExist(err) {
			t.Fatalf("%s not removed: %v", name, err)
		}
	}
}

func TestCorruptPendingIsDiscarded(t *testing.T) {
	a, _ := testAgent(t, &syncNotifier{})
	_ = state.WriteAtomic(a.state.Path(pendingReboot), []byte("{not json"))
	a.deliverPending(context.Background())
	if _, err := os.Stat(a.state.Path(pendingReboot)); !os.IsNotExist(err) {
		t.Fatalf("corrupt pending event kept: %v", err)
	}
}

func TestDiskZeroDurationAlertsOnFirstSample(t *testing.T) {
	a, n := diskAgent(t, map[string]metrics.DiskUsage{"/": {Device: 1, FreePercent: 1}})
	a.cfg.Disk.DurationSec = 0
	a.checkDisks(context.Background(), newDiskMonitor(), time.Unix(1777200000, 0))
	if len(n.messages) != 1 {
		t.Fatalf("messages = %v", n.messages)
	}
}

// If the first path on a filesystem cannot be measured, the next path on the
// same filesystem continues the existing alert state instead of alerting again.
func TestDiskStateFollowsFilesystemNotPath(t *testing.T) {
	low := metrics.DiskUsage{Device: 1, FreePercent: 1}
	usage := map[string]metrics.DiskUsage{"/": low, "/var": low}
	a, n := diskAgent(t, usage)
	a.cfg.Disk.DurationSec = 0
	m := newDiskMonitor()
	now := time.Unix(1777200000, 0)
	a.checkDisks(context.Background(), m, now)
	delete(usage, "/")
	a.checkDisks(context.Background(), m, now.Add(time.Minute))
	if len(n.messages) != 1 {
		t.Fatalf("duplicate alert after the first path failed: %v", n.messages)
	}
}
