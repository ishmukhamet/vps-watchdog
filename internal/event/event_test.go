package event

import (
	"strings"
	"testing"
	"time"
)

func TestFormatTelegramHTMLEscapesFields(t *testing.T) {
	msg := FormatTelegramHTML(Event{
		Time:     time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC),
		Host:     "web<01>",
		Type:     TypeSudo,
		Severity: SeverityWarning,
		Title:    "sudo used",
		Fields: map[string]string{
			"cmd": "cat < /etc/shadow & true",
		},
	})
	if strings.Contains(msg, "cat <") || strings.Contains(msg, "& true") {
		t.Fatalf("message not escaped: %s", msg)
	}
	if !strings.Contains(msg, "&lt;") || !strings.Contains(msg, "&amp;") {
		t.Fatalf("expected escaped entities: %s", msg)
	}
}

func TestFormatTelegramHTMLTruncatesLongFields(t *testing.T) {
	msg := FormatTelegramHTML(Event{
		Title:    "sudo used",
		Severity: SeverityWarning,
		Fields: map[string]string{
			"message": strings.Repeat("x", maxFieldValueRunes+50),
		},
	})
	if !strings.Contains(msg, "[truncated]") {
		t.Fatalf("expected truncation marker: %s", msg)
	}
	if strings.Contains(msg, strings.Repeat("x", maxFieldValueRunes+1)) {
		t.Fatalf("message was not truncated")
	}
}
