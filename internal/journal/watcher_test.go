package journal

import (
	"context"
	"errors"
	"fmt"
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
	args := Watcher{Identifier: "vps-watchdog-login", CursorFile: path}.journalctlArgs(cursorOf(path))
	if !contains(args, "--since=now") {
		t.Fatalf("args missing --since=now: %v", args)
	}
}

func TestJournalctlArgsResumeWhenCursorFileExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "login.cursor")
	if err := os.WriteFile(path, []byte(testCursor+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	args := Watcher{Identifier: "vps-watchdog-login", CursorFile: path}.journalctlArgs(cursorOf(path))
	if contains(args, "--since=now") {
		t.Fatalf("args should not include --since=now when cursor exists: %v", args)
	}
	if !contains(args, "--cursor="+testCursor) {
		t.Fatalf("args missing --cursor: %v", args)
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--cursor-file") {
			t.Fatalf("journalctl must not manage the cursor file: %v", args)
		}
	}
}

func TestJournalctlArgsStartAtNowWhenCursorFileInvalid(t *testing.T) {
	for name, content := range map[string]string{
		"empty":       "",
		"word":        "cursor",
		"unknown key": "a=b",
		"no position": "s=1;b=2",
		"empty value": "s=;i=1",
		"spaces":      "s=1; i=2",
		"control":     "s=1;i=\x002",
		"huge":        "t=" + strings.Repeat("1", maxCursorBytes),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "login.cursor")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			args := Watcher{Identifier: "vps-watchdog-login", CursorFile: path}.journalctlArgs(cursorOf(path))
			if !contains(args, "--since=now") {
				t.Fatalf("args missing --since=now: %v", args)
			}
		})
	}
}

func TestSaveCursorAfterPersistsHandledEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "login.cursor")
	h := Watcher{CursorFile: path}.saveCursorAfter(func(context.Context, Entry) error { return nil })
	if err := h(context.Background(), Entry{Fields: map[string]string{"__CURSOR": "s=1;i=2;t=5"}}); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got, ok := readCursor(path); !ok || got != "s=1;i=2;t=5" {
		t.Fatalf("cursor = %q, %t", got, ok)
	}
	if err := h(context.Background(), Entry{Fields: map[string]string{}}); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got, _ := readCursor(path); got != "s=1;i=2;t=5" {
		t.Fatalf("entry without cursor overwrote it: %q", got)
	}
}

func TestSaveCursorAfterSkipsFailedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "login.cursor")
	h := Watcher{CursorFile: path}.saveCursorAfter(func(context.Context, Entry) error { return errors.New("boom") })
	if err := h(context.Background(), Entry{Fields: map[string]string{"__CURSOR": testCursor}}); err == nil {
		t.Fatal("expected handler error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cursor written for failed entry: %v", err)
	}
}

func TestSaveCursorAfterReportsWriteFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-dir", "login.cursor")
	var logged string
	h := Watcher{CursorFile: path, Logf: func(format string, args ...any) { logged = fmt.Sprintf(format, args...) }}.
		saveCursorAfter(func(context.Context, Entry) error { return nil })
	if err := h(context.Background(), Entry{Fields: map[string]string{"__CURSOR": testCursor}}); err != nil {
		t.Fatalf("write failure must not stop the watcher: %v", err)
	}
	if !strings.Contains(logged, "save journal cursor") {
		t.Fatalf("write failure not reported: %q", logged)
	}
}

// journalctl --follow does not save --cursor-file when it is killed, so the
// cursor must survive cancellation of a still-running subprocess.
func TestRunKeepsCursorWhenSubprocessIsKilled(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-journalctl")
	body := "#!/bin/sh\n" +
		`case "$*" in *--show-cursor*) exit 1;; esac` + "\n" +
		`echo '{"__CURSOR":"s=1;i=1;t=4","SYSLOG_IDENTIFIER":"probe","MESSAGE":"one"}'` + "\n" +
		`echo '{"__CURSOR":"s=1;i=2;t=5","SYSLOG_IDENTIFIER":"probe","MESSAGE":"two"}'` + "\n" +
		"exec sleep 30\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cursorFile := filepath.Join(dir, "probe.cursor")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	seen := 0
	w := Watcher{Identifier: "probe", CursorFile: cursorFile, Command: script}
	done := make(chan error, 1)
	go func() {
		done <- w.Run(ctx, func(context.Context, Entry) error {
			seen++
			if seen == 2 {
				cancel()
			}
			return nil
		})
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("watcher did not stop")
	}
	if got, ok := readCursor(cursorFile); !ok || got != "s=1;i=2;t=5" {
		t.Fatalf("cursor = %q, %t", got, ok)
	}
}

func TestJournalctlArgsCanFollowAllJournalsWithExplicitMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sudo.cursor")
	args := Watcher{
		CursorFile:  path,
		AllJournals: true,
		Matches:     []string{"_COMM=sudo", "+", "_EXE=/usr/bin/sudo", "+", "SYSLOG_IDENTIFIER=sudo"},
	}.journalctlArgs(cursorOf(path))
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

const testCursor = "s=0123abcd;i=3;b=4567ef;m=7f2585cd0d;t=660365726cb57;x=843d35fd20a90b4a"

func TestParseShowCursor(t *testing.T) {
	got, ok := parseShowCursor([]byte("-- No entries --\n-- cursor: " + testCursor + "\n"))
	if !ok || got != testCursor {
		t.Fatalf("parseShowCursor = %q, %t", got, ok)
	}
	if _, ok := parseShowCursor([]byte("-- No entries --\n")); ok {
		t.Fatal("expected no cursor")
	}
}

func writeFakeJournalctl(t *testing.T, dir, body string) string {
	t.Helper()
	script := filepath.Join(dir, "fake-journalctl")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return script
}

// The first start seeds the cursor from the journal end; the second start
// resumes after the entry handled by the first, without --since=now.
func TestRunSeedsCursorAndResumesFromIt(t *testing.T) {
	dir := t.TempDir()
	argsLog := filepath.Join(dir, "args.log")
	script := writeFakeJournalctl(t, dir,
		`echo "$*" >> `+argsLog+"\n"+
			`case "$*" in *--show-cursor*) echo "-- cursor: `+testCursor+`"; exit 0;; esac`+"\n"+
			`echo '{"__CURSOR":"s=1;i=9;t=9","MESSAGE":"one"}'`+"\n")
	cursorFile := filepath.Join(dir, "probe.cursor")
	w := Watcher{Identifier: "probe", CursorFile: cursorFile, Command: script}
	for i := 0; i < 2; i++ {
		if err := w.Run(context.Background(), func(context.Context, Entry) error { return nil }); err == nil || strings.Contains(err.Error(), "signal") {
			t.Fatalf("Run %d: want a plain exit error, got %v", i, err)
		}
	}
	data, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("journalctl calls = %q", lines)
	}
	if !strings.Contains(lines[0], "--show-cursor") {
		t.Fatalf("first call should seed the cursor: %q", lines[0])
	}
	if !strings.Contains(lines[1], "--cursor="+testCursor) {
		t.Fatalf("first follow should start after the seeded cursor: %q", lines[1])
	}
	if !strings.Contains(lines[2], "--cursor=s=1;i=9;t=9") || strings.Contains(lines[2], "--since") {
		t.Fatalf("second follow should resume after the handled entry: %q", lines[2])
	}
}

func TestRunFallsBackToNowWhenSeedingFails(t *testing.T) {
	dir := t.TempDir()
	argsLog := filepath.Join(dir, "args.log")
	script := writeFakeJournalctl(t, dir,
		`echo "$*" >> `+argsLog+"\n"+
			`case "$*" in *--show-cursor*) exit 1;; esac`+"\n")
	w := Watcher{Identifier: "probe", CursorFile: filepath.Join(dir, "probe.cursor"), Command: script}
	_ = w.Run(context.Background(), func(context.Context, Entry) error { return nil })
	data, _ := os.ReadFile(argsLog)
	if !strings.Contains(string(data), "--since=now") {
		t.Fatalf("follow should fall back to --since=now: %q", data)
	}
}

func TestRunDiscardsCursorRejectedByJournalctl(t *testing.T) {
	dir := t.TempDir()
	script := writeFakeJournalctl(t, dir,
		`case "$*" in *--cursor=*) echo "Failed to seek to cursor: Invalid argument" >&2; exit 1;; esac`+"\n")
	cursorFile := filepath.Join(dir, "probe.cursor")
	if err := os.WriteFile(cursorFile, []byte(testCursor), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var logged []string
	w := Watcher{Identifier: "probe", CursorFile: cursorFile, Command: script,
		Logf: func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }}
	if err := w.Run(context.Background(), func(context.Context, Entry) error { return nil }); err == nil {
		t.Fatal("expected journalctl error")
	}
	if _, err := os.Stat(cursorFile); !os.IsNotExist(err) {
		t.Fatalf("rejected cursor still in place: %v", err)
	}
	if _, err := os.Stat(cursorFile + ".rejected"); err != nil {
		t.Fatalf("rejected cursor not kept for inspection: %v", err)
	}
	if len(logged) == 0 {
		t.Fatal("rejection not logged")
	}
}

func TestRunSkipsEntryAtResumeCursor(t *testing.T) {
	dir := t.TempDir()
	cursorFile := filepath.Join(dir, "probe.cursor")
	if err := os.WriteFile(cursorFile, []byte("s=1;i=1;t=1"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	script := writeFakeJournalctl(t, dir,
		`echo '{"__CURSOR":"s=1;i=1;t=1","MESSAGE":"old"}'`+"\n"+
			`echo '{"__CURSOR":"s=1;i=2;t=2","MESSAGE":"new"}'`+"\n")
	var got []string
	w := Watcher{Identifier: "probe", CursorFile: cursorFile, Command: script}
	_ = w.Run(context.Background(), func(_ context.Context, e Entry) error {
		got = append(got, e.Message)
		return nil
	})
	if strings.Join(got, ",") != "new" {
		t.Fatalf("handled = %v, want only the entry after the cursor", got)
	}
}

func cursorOf(path string) string {
	if cursor, ok := readCursor(path); ok {
		return cursor
	}
	return ""
}

func TestRunMovesAsideMalformedCursor(t *testing.T) {
	dir := t.TempDir()
	cursorFile := filepath.Join(dir, "probe.cursor")
	if err := os.WriteFile(cursorFile, []byte("garbage"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	script := writeFakeJournalctl(t, dir,
		`case "$*" in *--show-cursor*) echo "-- cursor: `+testCursor+`"; exit 0;; esac`+"\n")
	var logged []string
	w := Watcher{Identifier: "probe", CursorFile: cursorFile, Command: script,
		Logf: func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }}
	_ = w.Run(context.Background(), func(context.Context, Entry) error { return nil })
	if data, err := os.ReadFile(cursorFile + ".rejected"); err != nil || string(data) != "garbage" {
		t.Fatalf("malformed cursor not kept: %q, %v", data, err)
	}
	if got := cursorOf(cursorFile); got != testCursor {
		t.Fatalf("cursor not reseeded: %q", got)
	}
	if !strings.Contains(strings.Join(logged, "\n"), "invalid format") {
		t.Fatalf("malformed cursor not logged: %q", logged)
	}
}

// journalctl that closes stdout but keeps running must not block the watcher.
func TestRunStopsSubprocessThatOutlivesStdout(t *testing.T) {
	dir := t.TempDir()
	script := writeFakeJournalctl(t, dir,
		`case "$*" in *--show-cursor*) exit 1;; esac`+"\n"+
			"exec 1>&-\nexec sleep 30\n")
	w := Watcher{Identifier: "probe", Command: script}
	done := make(chan error, 1)
	go func() { done <- w.Run(context.Background(), func(context.Context, Entry) error { return nil }) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an exit error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run blocked after stdout closed")
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
