package config

import "testing"

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
