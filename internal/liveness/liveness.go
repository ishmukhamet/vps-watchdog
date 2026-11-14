// Package liveness pings an external dead man's switch, such as Healthchecks,
// so that silence from the host raises an alarm off-host (ADR 0008).
//
// The ping URL identifies the check and is a secret: it never appears in
// errors returned by this package.
package liveness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/state"
)

const maxURLBytes = 2048

// Pinger sends one request per call to a fixed URL.
type Pinger struct {
	url  string
	HTTP *http.Client
}

// New validates rawURL and returns a pinger for it.
func New(rawURL string) (*Pinger, error) {
	if err := Validate(rawURL); err != nil {
		return nil, err
	}
	return &Pinger{
		url: strings.TrimSpace(rawURL),
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
			// A redirect could lead to plain HTTP or another host; treat it
			// as a failure instead of following it.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

// LoadFile reads the ping URL from path. Errors name the file, never its
// content.
func LoadFile(path string) (*Pinger, error) {
	data, err := state.ReadSmall(path, maxURLBytes)
	if err != nil {
		return nil, fmt.Errorf("read ping url file: %w", err)
	}
	p, err := New(string(data))
	if err != nil {
		return nil, fmt.Errorf("ping url file %s: %w", path, err)
	}
	return p, nil
}

// Validate checks that rawURL is an absolute HTTPS URL. The returned error
// never contains the URL.
func Validate(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return errors.New("ping url is empty")
	}
	if strings.ContainsAny(rawURL, " \t\r\n") {
		return errors.New("ping url must be a single line without spaces")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("ping url is not a valid URL")
	}
	if u.Scheme != "https" {
		return errors.New("ping url must use https")
	}
	if u.Host == "" {
		return errors.New("ping url has no host")
	}
	return nil
}

// Ping sends one GET request. The request carries no host data.
func (p *Pinger) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return errors.New("ping failed: cannot build request")
	}
	req.Header.Set("User-Agent", "vps-watchdog")
	client := p.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ping failed: %s", p.sanitize(err))
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ping failed: http status %d", resp.StatusCode)
	}
	return nil
}

// sanitize drops the URL that net/http embeds in its errors and redacts any
// remaining occurrence of the URL or its path.
func (p *Pinger) sanitize(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	text := strings.ReplaceAll(err.Error(), "\n", " ")
	text = strings.ReplaceAll(text, p.url, "<redacted>")
	if u, perr := url.Parse(p.url); perr == nil {
		if u.Path != "" && u.Path != "/" {
			text = strings.ReplaceAll(text, u.Path, "<redacted>")
		}
		if u.RawQuery != "" {
			text = strings.ReplaceAll(text, u.RawQuery, "<redacted>")
		}
	}
	if len(text) > 300 {
		text = text[:300] + "..."
	}
	return text
}
