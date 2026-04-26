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
)

const maxJournalLineBytes = 1024 * 1024

type Entry struct {
	Timestamp  time.Time
	Identifier string
	Message    string
	Fields     map[string]string
}

type Watcher struct {
	Identifier string
	CursorFile string
	Command    string
}

type Handler func(context.Context, Entry) error

func (w Watcher) Run(ctx context.Context, handler Handler) error {
	if strings.TrimSpace(w.Identifier) == "" {
		return fmt.Errorf("journal identifier is required")
	}
	cmdName := w.Command
	if cmdName == "" {
		cmdName = "journalctl"
	}

	args := w.journalctlArgs()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(runCtx, cmdName, args...)
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

	stderrCh := make(chan error, 1)
	scanCh := make(chan error, 1)
	waitCh := make(chan error, 1)
	go func() { stderrCh <- consumeStderr(stderr) }()
	go func() { scanCh <- scanEntries(runCtx, stdout, handler) }()
	go func() { waitCh <- cmd.Wait() }()

	var firstErr error
	pending := 3
	doneCh := ctx.Done()
	for pending > 0 {
		select {
		case err := <-scanCh:
			scanCh = nil
			pending--
			if err != nil && firstErr == nil {
				firstErr = err
			}
			cancel()
		case err := <-stderrCh:
			stderrCh = nil
			pending--
			if err != nil && firstErr == nil {
				firstErr = err
				cancel()
			}
		case err := <-waitCh:
			waitCh = nil
			pending--
			if err != nil && firstErr == nil && ctx.Err() == nil {
				firstErr = fmt.Errorf("journalctl exited: %w", err)
			}
			cancel()
		case <-doneCh:
			if firstErr == nil {
				firstErr = ctx.Err()
			}
			doneCh = nil
			cancel()
		}
	}
	return firstErr
}

func (w Watcher) journalctlArgs() []string {
	args := []string{"--system", "--follow", "--no-pager", "--output=json"}
	if w.CursorFile != "" {
		args = append(args, "--cursor-file="+w.CursorFile)
		if _, err := os.Stat(w.CursorFile); os.IsNotExist(err) {
			args = append(args, "--since=now")
		}
	}
	args = append(args, "SYSLOG_IDENTIFIER="+w.Identifier)
	return args
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
	_, _ = io.Copy(io.Discard, r)
	return nil
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
