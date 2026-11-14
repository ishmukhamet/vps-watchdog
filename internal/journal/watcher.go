package journal

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/state"
)

const (
	maxJournalLineBytes = 1024 * 1024
	maxStderrBytes      = 4096
	maxCursorBytes      = 4096
	exitGracePeriod     = 2 * time.Second
)

type Entry struct {
	Timestamp  time.Time
	Identifier string
	Message    string
	Fields     map[string]string
}

type Watcher struct {
	Identifier string
	// CursorFile stores the journal cursor of the last handled entry. The
	// watcher writes it itself instead of using journalctl --cursor-file,
	// which is saved only on a clean exit; the watcher stops journalctl with
	// SIGKILL, and some versions also skip the save on SIGTERM.
	CursorFile  string
	Command     string
	AllJournals bool
	Matches     []string
	// Logf reports non-fatal problems such as a failed cursor save. Optional.
	Logf func(format string, args ...any)
}

type Handler func(context.Context, Entry) error

func (w Watcher) Run(ctx context.Context, handler Handler) error {
	if len(w.matchArgs()) == 0 {
		return fmt.Errorf("journal match is required")
	}
	cmdName := w.Command
	if cmdName == "" {
		cmdName = "journalctl"
	}

	startCursor := w.ensureCursor(ctx, cmdName)
	args := w.journalctlArgs(startCursor)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(runCtx, cmdName, args...)
	// Bounds Wait if something else keeps the pipes open after journalctl
	// is killed.
	cmd.WaitDelay = exitGracePeriod
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("journal stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("journal stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start journalctl: %w", err)
	}

	// cmd.Wait closes the pipes, so it runs only after both readers have
	// finished; otherwise entries written just before exit can be lost.
	stderrCh := make(chan error, 1)
	scanCh := make(chan error, 1)
	go func() { stderrCh <- consumeStderr(stderr) }()
	go func() { scanCh <- scanEntries(runCtx, stdout, w.saveCursorAfter(skipCursor(startCursor, handler))) }()

	var firstErr, stderrErr error
	pending := 2
	doneCh := ctx.Done()
	for pending > 0 {
		select {
		case err := <-scanCh:
			scanCh = nil
			pending--
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				cancel()
			} else {
				// stdout reached EOF: journalctl is exiting. Give it a
				// moment to report its status, then kill it.
				grace := time.AfterFunc(exitGracePeriod, cancel)
				defer grace.Stop()
			}
		case err := <-stderrCh:
			stderrCh = nil
			pending--
			stderrErr = err
			if err != nil && firstErr == nil {
				firstErr = err
				cancel()
			}
		case <-doneCh:
			if firstErr == nil {
				firstErr = ctx.Err()
			}
			doneCh = nil
			cancel()
		}
	}
	waitErr := cmd.Wait()
	if firstErr == nil && ctx.Err() == nil {
		if waitErr != nil {
			firstErr = fmt.Errorf("journalctl exited: %w", waitErr)
		} else {
			firstErr = errors.New("journalctl exited")
		}
	}
	if stderrErr != nil && w.CursorFile != "" && strings.Contains(stderrErr.Error(), "Failed to seek to cursor") {
		w.logf("journal cursor rejected by journalctl")
		w.discardCursor()
	}
	return firstErr
}

// ensureCursor returns the stored cursor, seeding a missing one with the
// current end of the journal, so that every later start resumes from a cursor
// even if no matching entry has been handled yet. It returns "" when there is
// no usable cursor; the watcher then starts at the current time.
func (w Watcher) ensureCursor(ctx context.Context, cmdName string) string {
	if w.CursorFile == "" {
		return ""
	}
	if cursor, ok := readCursor(w.CursorFile); ok {
		return cursor
	}
	if _, err := os.Stat(w.CursorFile); err == nil {
		w.logf("journal cursor %s has an invalid format; starting from the current end of the journal", w.CursorFile)
		w.discardCursor()
	}
	args := []string{"--lines=0", "--show-cursor", "--no-pager", "--quiet"}
	if !w.AllJournals {
		args = append([]string{"--system"}, args...)
	}
	seedCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	seed := exec.CommandContext(seedCtx, cmdName, args...)
	seed.WaitDelay = exitGracePeriod
	out, err := seed.Output()
	if err != nil {
		if ctx.Err() == nil {
			w.logf("seed journal cursor: %v", err)
		}
		return ""
	}
	cursor, ok := parseShowCursor(out)
	if !ok {
		w.logf("seed journal cursor: no cursor in journalctl output")
		return ""
	}
	if err := writeCursor(w.CursorFile, cursor); err != nil {
		w.logf("seed journal cursor: %v", err)
		return ""
	}
	return cursor
}

func parseShowCursor(out []byte) (string, bool) {
	for _, line := range strings.Split(string(out), "\n") {
		if cursor, ok := strings.CutPrefix(strings.TrimSpace(line), "-- cursor: "); ok {
			cursor = strings.TrimSpace(cursor)
			return cursor, validCursor(cursor)
		}
	}
	return "", false
}

// discardCursor moves aside an unusable cursor, keeping it for inspection,
// so the next start does not fail on it again.
func (w Watcher) discardCursor() {
	if err := os.Rename(w.CursorFile, w.CursorFile+".rejected"); err != nil {
		w.logf("discard journal cursor: %v", err)
		return
	}
	w.logf("journal cursor moved to %s.rejected", w.CursorFile)
}

func (w Watcher) logf(format string, args ...any) {
	if w.Logf != nil {
		w.Logf(format, args...)
	}
}

func (w Watcher) journalctlArgs(cursor string) []string {
	args := []string{"--follow", "--no-pager", "--output=json"}
	if !w.AllJournals {
		args = append([]string{"--system"}, args...)
	}
	if w.CursorFile != "" {
		// --cursor rather than --after-cursor: on some versions --after-cursor
		// drops the first matching entry when the cursor points at an entry
		// that does not match. The entry at the cursor is skipped by Run.
		if cursor != "" {
			args = append(args, "--cursor="+cursor)
		} else {
			args = append(args, "--since=now")
		}
	}
	args = append(args, w.matchArgs()...)
	return args
}

func (w Watcher) matchArgs() []string {
	if len(w.Matches) > 0 {
		out := make([]string, 0, len(w.Matches))
		for _, match := range w.Matches {
			match = strings.TrimSpace(match)
			if match != "" {
				out = append(out, match)
			}
		}
		return out
	}
	if strings.TrimSpace(w.Identifier) == "" {
		return nil
	}
	return []string{"SYSLOG_IDENTIFIER=" + w.Identifier}
}

// saveCursorAfter wraps handler so that the cursor of every successfully
// handled entry is persisted. A failed save is reported and does not stop the
// watcher: restarting would replay already handled entries.
func (w Watcher) saveCursorAfter(handler Handler) Handler {
	if w.CursorFile == "" {
		return handler
	}
	return func(ctx context.Context, entry Entry) error {
		if err := handler(ctx, entry); err != nil {
			return err
		}
		cursor := entry.Fields["__CURSOR"]
		if !validCursor(cursor) {
			return nil
		}
		if err := writeCursor(w.CursorFile, cursor); err != nil {
			w.logf("save journal cursor: %v", err)
		}
		return nil
	}
}

// skipCursor drops the entry stored as the resume cursor: it was handled
// before the restart.
func skipCursor(cursor string, handler Handler) Handler {
	if cursor == "" {
		return handler
	}
	return func(ctx context.Context, entry Entry) error {
		if entry.Fields["__CURSOR"] == cursor {
			return nil
		}
		return handler(ctx, entry)
	}
}

func readCursor(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > maxCursorBytes {
		return "", false
	}
	cursor := strings.TrimSpace(string(data))
	return cursor, validCursor(cursor)
}

// validCursor accepts only the shape journalctl can seek to: ";"-separated
// key=value items with known keys, including at least one complete position
// (s and i, b and m, or t). Anything else would make journalctl fail on start.
func validCursor(cursor string) bool {
	if cursor == "" || len(cursor) > maxCursorBytes {
		return false
	}
	for _, r := range cursor {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	seen := map[string]bool{}
	for _, item := range strings.Split(cursor, ";") {
		key, value, ok := strings.Cut(item, "=")
		if !ok || value == "" || !strings.Contains("sibmtx", key) || len(key) != 1 {
			return false
		}
		seen[key] = true
	}
	return (seen["s"] && seen["i"]) || (seen["b"] && seen["m"]) || seen["t"]
}

// writeCursor stores the cursor atomically; see state.WriteAtomic.
func writeCursor(path, cursor string) error {
	return state.WriteAtomic(path, []byte(cursor))
}

func scanEntries(ctx context.Context, r io.Reader, handler Handler) error {
	reader := bufio.NewReaderSize(r, 64*1024)
	var line []byte
	discarding := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fragment, err := reader.ReadSlice('\n')
		if len(fragment) > 0 {
			if discarding {
				if err != bufio.ErrBufferFull {
					discarding = false
				}
			} else if len(line)+len(fragment) > maxJournalLineBytes {
				line = line[:0]
				discarding = err == bufio.ErrBufferFull
			} else {
				line = append(line, fragment...)
			}
		}

		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if len(line) > 0 && !discarding {
			entry, parseErr := ParseJSONLine(line)
			if parseErr == nil {
				if handleErr := handler(ctx, entry); handleErr != nil {
					return handleErr
				}
			}
		}
		line = line[:0]
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
}

func consumeStderr(r io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(r, maxStderrBytes+1))
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil
	}
	if len(data) > maxStderrBytes {
		text += "... [truncated]"
	}
	return fmt.Errorf("journalctl stderr: %s", text)
}

func ParseJSONLine(line []byte) (Entry, error) {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return Entry{}, err
	}
	fields := make(map[string]string, len(raw))
	for k, v := range raw {
		fields[k] = fmt.Sprint(v)
	}
	entry := Entry{
		Identifier: fields["SYSLOG_IDENTIFIER"],
		Message:    fields["MESSAGE"],
		Fields:     fields,
	}
	if ts, ok := parseJournalTimestamp(fields["__REALTIME_TIMESTAMP"]); ok {
		entry.Timestamp = ts
	} else if ts, ok := parseJournalTimestamp(fields["_SOURCE_REALTIME_TIMESTAMP"]); ok {
		entry.Timestamp = ts
	}
	return entry, nil
}

func parseJournalTimestamp(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	micros, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, micros*int64(time.Microsecond)).UTC(), true
}

func ParseKeyValueMessage(message string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Fields(message) {
		k, v, ok := strings.Cut(part, "=")
		if !ok || k == "" {
			continue
		}
		out[k] = decodeMessageValue(strings.Trim(v, `"'`))
	}
	return out
}

func decodeMessageValue(value string) string {
	encoded, ok := strings.CutPrefix(value, "hex:")
	if !ok {
		return value
	}
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return value
	}
	return string(decoded)
}
