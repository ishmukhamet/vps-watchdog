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
	TypeAgent  Type = "agent"
	TypeLogin  Type = "login"
	TypeSudo   Type = "sudo"
	TypeLoad   Type = "load"
	TypeDisk   Type = "disk"
	TypeReboot Type = "reboot"
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
	var b strings.Builder
	b.WriteString("[")
	b.WriteString(severityIcon(e.Severity))
	b.WriteString("] [")
	b.WriteString(eventClock(e.Time))
	b.WriteString("] [")
	b.WriteString(escape(eventHost(e.Host)))
	b.WriteString("]\n\n")
	if strings.TrimSpace(e.Title) != "" {
		b.WriteString("<b>")
		b.WriteString(escape(e.Title))
		b.WriteString("</b>")
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
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n\n") {
			b.WriteString("\n")
		}
		b.WriteString(escape(k))
		b.WriteString(": <code>")
		b.WriteString(escape(v))
		b.WriteString("</code>")
	}
	return strings.TrimRight(b.String(), "\n")
}

func eventClock(t time.Time) string {
	if t.IsZero() {
		return "--:--:--"
	}
	return t.UTC().Format("15:04:05")
}

func eventHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return "unknown"
	}
	return host
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
