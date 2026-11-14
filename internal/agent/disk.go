package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/event"
	"github.com/ishmukhamet/vps-watchdog/internal/metrics"
)

// diskMonitor holds per-filesystem alert state for low disk space and inodes
// (ADR 0010). Paths on the same filesystem are reported once, under the first
// configured path that could be measured.
type diskMonitor struct {
	space   map[uint64]*thresholdState
	inodes  map[uint64]*thresholdState
	lastErr map[string]string
}

func newDiskMonitor() *diskMonitor {
	return &diskMonitor{
		space:   map[uint64]*thresholdState{},
		inodes:  map[uint64]*thresholdState{},
		lastErr: map[string]string{},
	}
}

func (a *Agent) runDiskMonitor(ctx context.Context) error {
	m := newDiskMonitor()
	ticker := time.NewTicker(time.Duration(a.cfg.Disk.SampleIntervalSec) * time.Second)
	defer ticker.Stop()
	a.checkDisks(ctx, m, time.Now())
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			a.checkDisks(ctx, m, now)
		}
	}
}

func (a *Agent) checkDisks(ctx context.Context, m *diskMonitor, now time.Time) {
	cfg := a.cfg.Disk
	duration := time.Duration(cfg.DurationSec) * time.Second
	cooldown := time.Duration(cfg.CooldownSec) * time.Second
	seen := map[uint64]bool{}
	for _, path := range cfg.Paths {
		u, err := a.readDisk(path)
		if err != nil {
			// Log only changes, so a missing mount does not flood the log.
			if msg := err.Error(); m.lastErr[path] != msg {
				a.log.Printf("disk check %s: %v", path, err)
				m.lastErr[path] = msg
			}
			continue
		}
		delete(m.lastErr, path)
		if seen[u.Device] {
			continue
		}
		seen[u.Device] = true

		space := m.space[u.Device]
		if space == nil {
			space = newThresholdState("low disk space on "+path, event.SeverityWarning, duration, cooldown)
			space.Type = event.TypeDisk
			m.space[u.Device] = space
		}
		a.evaluateThreshold(ctx, space, now, u.FreePercent < cfg.FreePercent, map[string]string{
			"path":         path,
			"free":         fmt.Sprintf("%s of %s", metrics.HumanBytes(u.AvailBytes), metrics.HumanBytes(u.SizeBytes)),
			"free_percent": fmt.Sprintf("%.1f", u.FreePercent),
			"threshold":    fmt.Sprintf("%.1f", cfg.FreePercent),
		})

		if !u.InodesKnown || cfg.InodesFreePercent <= 0 {
			continue
		}
		inodes := m.inodes[u.Device]
		if inodes == nil {
			inodes = newThresholdState("low free inodes on "+path, event.SeverityWarning, duration, cooldown)
			inodes.Type = event.TypeDisk
			m.inodes[u.Device] = inodes
		}
		a.evaluateThreshold(ctx, inodes, now, u.InodesFreePercent < cfg.InodesFreePercent, map[string]string{
			"path":                path,
			"inodes_free":         fmt.Sprintf("%d of %d", u.InodesFree, u.InodesTotal),
			"inodes_free_percent": fmt.Sprintf("%.1f", u.InodesFreePercent),
			"threshold":           fmt.Sprintf("%.1f", cfg.InodesFreePercent),
		})
	}
}
