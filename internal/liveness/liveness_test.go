package liveness

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const secretPath = "/0f1e2d3c-secret-check-id"

func testPinger(t *testing.T, srv *httptest.Server) *Pinger {
	t.Helper()
	p, err := New(srv.URL + secretPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p.HTTP.Transport = srv.Client().Transport
	return p
}

func TestPingSendsBareGet(t *testing.T) {
	var got *http.Request
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		_, _ = w.Write([]byte("OK"))
	}))
	defer srv.Close()
	if err := testPinger(t, srv).Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if got.Method != http.MethodGet || got.URL.Path != secretPath || got.ContentLength > 0 {
		t.Fatalf("unexpected request: %s %s len=%d", got.Method, got.URL.Path, got.ContentLength)
	}
}

func TestPingErrorsDoNotLeakURL(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("mode") {
		case "redirect":
			http.Redirect(w, r, "http://example.invalid"+secretPath, http.StatusFound)
		default:
			http.Error(w, "nope "+secretPath, http.StatusNotFound)
		}
	}))
	p := testPinger(t, srv)
	for _, mode := range []string{"status", "redirect"} {
		p.url = srv.URL + secretPath + "?mode=" + mode
		err := p.Ping(context.Background())
		if err == nil {
			t.Fatalf("%s: expected error", mode)
		}
		if strings.Contains(err.Error(), "secret-check-id") {
			t.Fatalf("%s: error leaks the URL: %v", mode, err)
		}
	}
	srv.Close()
	p.url = srv.URL + secretPath
	err := p.Ping(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret-check-id") || strings.Contains(err.Error(), srv.URL) {
		t.Fatalf("network error leaks the URL or is nil: %v", err)
	}
}

func TestValidate(t *testing.T) {
	for _, bad := range []string{"", "http://hc.example/x", "https://", "not a url", "https://hc.example/a\nb", "ftp://x/y"} {
		if err := Validate(bad); err == nil {
			t.Fatalf("Validate(%q) accepted", bad)
		} else if bad != "" && strings.Contains(err.Error(), bad) {
			t.Fatalf("Validate error leaks the URL: %v", err)
		}
	}
	if err := Validate(" https://hc.example/ping/abc \n"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good")
	if err := os.WriteFile(good, []byte("https://hc.example"+secretPath+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(good); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("http://hc.example"+secretPath), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(bad); err == nil || strings.Contains(err.Error(), "secret-check-id") {
		t.Fatalf("LoadFile(bad) = %v", err)
	}
	if _, err := LoadFile(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected error for a missing file")
	}
}
