package metrics

import (
	"strings"
	"testing"
)

func TestBusyPercent(t *testing.T) {
	prev := CPUStat{User: 100, System: 100, Idle: 800}
	curr := CPUStat{User: 200, System: 200, Idle: 900}
	got, ok := BusyPercent(prev, curr)
	if !ok {
		t.Fatal("expected busy percent")
	}
	if got < 66 || got > 67 {
		t.Fatalf("busy = %v", got)
	}
}

func TestBusyPercentRejectsCounterReset(t *testing.T) {
	prev := CPUStat{User: 100, System: 100, Idle: 800}
	curr := CPUStat{User: 10, System: 10, Idle: 80}
	if got, ok := BusyPercent(prev, curr); ok || got != 0 {
		t.Fatalf("busy = %v ok=%v", got, ok)
	}
}

func TestParseCPUStatRejectsShortLine(t *testing.T) {
	_, err := ParseCPUStat(strings.NewReader("cpu  1 2 3 4 5 6 7\n"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseMemInfo(t *testing.T) {
	mem, err := ParseMemInfo(strings.NewReader("MemTotal:       1000 kB\nMemAvailable:    250 kB\n"))
	if err != nil {
		t.Fatalf("ParseMemInfo() error = %v", err)
	}
	pct, ok := mem.AvailablePercent()
	if !ok || pct != 25 {
		t.Fatalf("pct = %v ok=%v", pct, ok)
	}
}

func TestParseMemInfoAllowsZeroAvailable(t *testing.T) {
	mem, err := ParseMemInfo(strings.NewReader("MemTotal:       1000 kB\nMemAvailable:    0 kB\n"))
	if err != nil {
		t.Fatalf("ParseMemInfo() error = %v", err)
	}
	pct, ok := mem.AvailablePercent()
	if !ok || pct != 0 {
		t.Fatalf("pct = %v ok=%v", pct, ok)
	}
}

func TestParsePSI(t *testing.T) {
	psi, err := ParsePSI(strings.NewReader("some avg10=1.23 avg60=4.56 avg300=7.89 total=123\nfull avg10=0.10 avg60=0.20 avg300=0.30 total=10\n"))
	if err != nil {
		t.Fatalf("ParsePSI() error = %v", err)
	}
	if !psi.Available || psi.SomeAvg60 != 4.56 || psi.FullAvg60 != 0.20 {
		t.Fatalf("unexpected psi: %+v", psi)
	}
}
