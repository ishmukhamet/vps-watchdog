package journal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseJSONLine(t *testing.T) {
	entry, err := ParseJSONLine([]byte(`{"__REALTIME_TIMESTAMP":"1777200000000000","SYSLOG_IDENTIFIER":"sudo","MESSAGE":"alice : TTY=pts/0 ; COMMAND=/bin/true"}`))
	if err != nil {
		t.Fatalf("ParseJSONLine() error = %v", err)
	}
	if entry.Identifier != "sudo" || entry.Message == "" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	want := time.Unix(0, 1777200000000000*int64(time.Microsecond)).UTC()
	if !entry.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %s want %s", entry.Timestamp, want)
	}
}

func TestParseKeyValueMessage(t *testing.T) {
	got := ParseKeyValueMessage(`event=login service=sshd user=alice rhost=203.0.113.10 tty=ssh type=open_session`)
	if got["user"] != "alice" || got["service"] != "sshd" {
		t.Fatalf("unexpected fields: %+v", got)
	}
}

func TestParseKeyValueMessageDecodesHexValues(t *testing.T) {
	got := ParseKeyValueMessage(`event=login service=hex:73736864 user=hex:616c6963652061646d696e rhost=hex:3230332e302e3131332e3130 tty=hex:7074732f30`)
	if got["user"] != "alice admin" || got["service"] != "sshd" || got["tty"] != "pts/0" {
		t.Fatalf("unexpected fields: %+v", got)
	}
}

func TestJournalctlArgsStartAtNowWhenCursorFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "login.cursor")
	args := Watcher{Identifier: "vps-watchdog-login", CursorFile: path}.journalctlArgs()
	if !contains(args, "--since=now") {
		t.Fatalf("args missing --since=now: %v", args)
	}
}

func TestJournalctlArgsResumeWhenCursorFileExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "login.cursor")
	if err := os.WriteFile(path, []byte("cursor"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	args := Watcher{Identifier: "vps-watchdog-login", CursorFile: path}.journalctlArgs()
	if contains(args, "--since=now") {
		t.Fatalf("args should not include --since=now when cursor exists: %v", args)
	}
}

func TestJournalctlArgsCanFollowAllJournalsWithExplicitMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sudo.cursor")
	args := Watcher{
		CursorFile:  path,
		AllJournals: true,
		Matches:     []string{"_COMM=sudo", "+", "_EXE=/usr/bin/sudo", "+", "SYSLOG_IDENTIFIER=sudo"},
	}.journalctlArgs()
	if contains(args, "--system") {
		t.Fatalf("sudo watcher should not be system-journal-only: %v", args)
	}
	for _, want := range []string{"_COMM=sudo", "+", "_EXE=/usr/bin/sudo", "SYSLOG_IDENTIFIER=sudo"} {
		if !contains(args, want) {
			t.Fatalf("args missing %q: %v", want, args)
		}
	}
}

func TestScanEntriesSkipsOversizedLine(t *testing.T) {
	input := strings.Repeat("x", maxJournalLineBytes+1) + "\n" + `{"SYSLOG_IDENTIFIER":"sudo","MESSAGE":"ok"}` + "\n"
	var entries []Entry
	err := scanEntries(context.Background(), strings.NewReader(input), func(_ context.Context, entry Entry) error {
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		t.Fatalf("scanEntries() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Message != "ok" {
		t.Fatalf("entries = %+v", entries)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
