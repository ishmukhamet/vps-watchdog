package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type errorRoundTripper struct{}

func (errorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("dial failed for %s", req.URL.String())
}

func TestClientSend(t *testing.T) {
	var gotPath string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		gotBody = r.Form.Get("text")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	client := &Client{Token: "TOKEN", ChatID: "123", BaseURL: server.URL, ParseMode: "HTML", HTTP: server.Client()}
	if err := client.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotPath != "/botTOKEN/sendMessage" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody != "hello" {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestClientSendHTTPErrorDoesNotIncludeTokenInError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad SECRET_TOKEN", http.StatusBadRequest)
	}))
	defer server.Close()

	client := &Client{Token: "SECRET_TOKEN", ChatID: "123", BaseURL: server.URL, HTTP: server.Client()}
	err := client.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "SECRET_TOKEN") {
		t.Fatalf("error leaked token: %v", err)
	}
}

func TestClientSendNetworkErrorDoesNotIncludeTokenInError(t *testing.T) {
	client := &Client{
		Token:   "SECRET_TOKEN",
		ChatID:  "123",
		BaseURL: "https://api.telegram.test",
		HTTP:    &http.Client{Transport: errorRoundTripper{}},
	}
	err := client.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "SECRET_TOKEN") {
		t.Fatalf("error leaked token: %v", err)
	}
}

func TestClientSendRejectsInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client := &Client{Token: "TOKEN", ChatID: "123", BaseURL: server.URL, HTTP: server.Client()}
	err := client.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid response") {
		t.Fatalf("error = %v", err)
	}
}
