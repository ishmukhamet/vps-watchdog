package config

import (
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	cfg, err := Parse([]byte(`
host_id: "web-01"
telegram:
  token_file: /tmp/token
  chat_id_file: /tmp/chat
login:
  enabled: true
sudo:
  enabled: false
load:
  enabled: true
  sample_interval_sec: 10
  cooldown_sec: 60
  cpu_busy_percent: 85
  cpu_duration_sec: 120
  mem_available_percent: 15
  mem_duration_sec: 180
  psi_io_some_avg60: 25.5
  io_duration_sec: 90
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.HostID != "web-01" {
		t.Fatalf("HostID = %q", cfg.HostID)
	}
	if cfg.Telegram.TokenFile != "/tmp/token" || cfg.Telegram.ChatIDFile != "/tmp/chat" {
		t.Fatalf("unexpected telegram config: %+v", cfg.Telegram)
	}
	if !cfg.Login.Enabled {
		t.Fatal("login should be enabled")
	}
	if cfg.Sudo.Enabled {
		t.Fatal("sudo should be disabled")
	}
	if cfg.Load.PSIIOSomeAvg60 != 25.5 {
		t.Fatalf("PSIIOSomeAvg60 = %v", cfg.Load.PSIIOSomeAvg60)
	}
}

func TestParseRejectsUnknownKey(t *testing.T) {
	_, err := Parse([]byte("unknown: true\n"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRejectsInvalidThreshold(t *testing.T) {
	_, err := Parse([]byte(`
host_id: x
telegram:
  token_file: /tmp/token
  chat_id_file: /tmp/chat
load:
  cpu_busy_percent: 101
`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseNewSections(t *testing.T) {
	cfg, err := Parse([]byte(`
disk:
  paths: "/, /var/lib/docker ,"
  free_percent: 5
  inodes_free_percent: 0
reboot:
  enabled: false
liveness:
  enabled: true
  ping_url_file: /tmp/ping
  interval_sec: 30
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := strings.Join(cfg.Disk.Paths, "|"); got != "/|/var/lib/docker" {
		t.Fatalf("Disk.Paths = %q", got)
	}
	if cfg.Disk.FreePercent != 5 || cfg.Disk.InodesFreePercent != 0 || !cfg.Disk.Enabled {
		t.Fatalf("unexpected disk config: %+v", cfg.Disk)
	}
	if cfg.Reboot.Enabled {
		t.Fatal("reboot should be disabled")
	}
	if !cfg.Liveness.Enabled || cfg.Liveness.PingURLFile != "/tmp/ping" || cfg.Liveness.IntervalSec != 30 {
		t.Fatalf("unexpected liveness config: %+v", cfg.Liveness)
	}
}

func TestDefaultsKeepLivenessOff(t *testing.T) {
	cfg, err := Parse([]byte("host_id: x\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Liveness.Enabled || !cfg.Reboot.Enabled || !cfg.Disk.Enabled {
		t.Fatalf("unexpected defaults: %+v %+v %+v", cfg.Liveness, cfg.Reboot, cfg.Disk)
	}
	if len(cfg.Disk.Paths) != 1 || cfg.Disk.Paths[0] != "/" {
		t.Fatalf("default disk paths = %v", cfg.Disk.Paths)
	}
}

func TestParseRejectsInvalidNewSections(t *testing.T) {
	for name, body := range map[string]string{
		"relative path":    "disk:\n  paths: var/lib\n",
		"home path":        "disk:\n  paths: /, /home\n",
		"under root home":  "disk:\n  paths: /root/data\n",
		"no paths":         "disk:\n  paths: \" , \"\n",
		"free 100":         "disk:\n  free_percent: 100\n",
		"inodes negative":  "disk:\n  inodes_free_percent: -1\n",
		"short interval":   "liveness:\n  interval_sec: 5\n",
		"enabled no file":  "liveness:\n  enabled: true\n  ping_url_file: \"\"\n",
		"bad reboot value": "reboot:\n  enabled: maybe\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(body)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseAcceptsHomeLookalikePath(t *testing.T) {
	if _, err := Parse([]byte("disk:\n  paths: /homework\n")); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
}
