package event

import (
	"html"
	"sort"
	"strings"
	"time"
)

const maxFieldValueRunes = 1000

type Type string

const (
	TypeAgent Type = "agent"
	TypeLogin Type = "login"
	TypeSudo  Type = "sudo"
	TypeLoad  Type = "load"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Event struct {
	Time     time.Time
	Host     string
	Type     Type
	Severity Severity
	Title    string
	Fields   map[string]string
}

func FormatTelegramHTML(e Event) string {
	icon := severityIcon(e.Severity)
	var b strings.Builder
	b.WriteString(icon)
	b.WriteString(" <b>")
	b.WriteString(escape(e.Title))
	b.WriteString("</b>\n")
	if e.Host != "" {
		b.WriteString("host: <code>")
		b.WriteString(escape(e.Host))
		b.WriteString("</code>\n")
	}
	if !e.Time.IsZero() {
		b.WriteString("time: <code>")
		b.WriteString(escape(e.Time.UTC().Format(time.RFC3339)))
		b.WriteString("</code>\n")
	}
	if e.Type != "" {
		b.WriteString("type: <code>")
		b.WriteString(escape(string(e.Type)))
		b.WriteString("</code>\n")
	}

	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := truncateValue(strings.TrimSpace(e.Fields[k]))
		if v == "" {
			continue
		}
		b.WriteString(escape(k))
		b.WriteString(": <code>")
		b.WriteString(escape(v))
		b.WriteString("</code>\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func severityIcon(s Severity) string {
	switch s {
	case SeverityCritical:
		return "🚨"
	case SeverityWarning:
		return "⚠️"
	default:
		return "ℹ️"
	}
}

func escape(s string) string {
	return html.EscapeString(s)
}

func truncateValue(value string) string {
	runes := []rune(value)
	if len(runes) <= maxFieldValueRunes {
		return value
	}
	return string(runes[:maxFieldValueRunes]) + "... [truncated]"
}
