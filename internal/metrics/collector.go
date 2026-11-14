package metrics

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type CPUStat struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOWait  uint64
	IRQ     uint64
	SoftIRQ uint64
	Steal   uint64
}

func (s CPUStat) idleAll() uint64 { return s.Idle + s.IOWait }
func (s CPUStat) total() uint64 {
	return s.User + s.Nice + s.System + s.Idle + s.IOWait + s.IRQ + s.SoftIRQ + s.Steal
}

func ParseCPUStat(r io.Reader) (CPUStat, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			return CPUStat{}, fmt.Errorf("cpu line has too few fields")
		}
		vals := make([]uint64, 8)
		for i := range vals {
			v, err := strconv.ParseUint(fields[i+1], 10, 64)
			if err != nil {
				return CPUStat{}, err
			}
			vals[i] = v
		}
		return CPUStat{User: vals[0], Nice: vals[1], System: vals[2], Idle: vals[3], IOWait: vals[4], IRQ: vals[5], SoftIRQ: vals[6], Steal: vals[7]}, nil
	}
	if err := scanner.Err(); err != nil {
		return CPUStat{}, err
	}
	return CPUStat{}, fmt.Errorf("cpu line not found")
}

func BusyPercent(prev, curr CPUStat) (float64, bool) {
	if curr.total() < prev.total() || curr.idleAll() < prev.idleAll() {
		return 0, false
	}
	totalDelta := curr.total() - prev.total()
	idleDelta := curr.idleAll() - prev.idleAll()
	if totalDelta == 0 || idleDelta > totalDelta {
		return 0, false
	}
	return 100 * float64(totalDelta-idleDelta) / float64(totalDelta), true
}

type MemInfo struct {
	TotalKB     uint64
	AvailableKB uint64
}

func (m MemInfo) AvailablePercent() (float64, bool) {
	if m.TotalKB == 0 {
		return 0, false
	}
	return 100 * float64(m.AvailableKB) / float64(m.TotalKB), true
}

func ParseMemInfo(r io.Reader) (MemInfo, error) {
	scanner := bufio.NewScanner(r)
	var out MemInfo
	foundTotal := false
	foundAvailable := false
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			out.TotalKB = value
			foundTotal = true
		case "MemAvailable":
			out.AvailableKB = value
			foundAvailable = true
		}
	}
	if err := scanner.Err(); err != nil {
		return MemInfo{}, err
	}
	if !foundTotal || !foundAvailable || out.TotalKB == 0 {
		return MemInfo{}, fmt.Errorf("MemTotal or MemAvailable not found")
	}
	return out, nil
}

type PSI struct {
	SomeAvg10  float64
	SomeAvg60  float64
	SomeAvg300 float64
	FullAvg10  float64
	FullAvg60  float64
	FullAvg300 float64
	Available  bool
}

func ParsePSI(r io.Reader) (PSI, error) {
	scanner := bufio.NewScanner(r)
	var out PSI
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		target := &out.SomeAvg10
		prefix := fields[0]
		if prefix != "some" && prefix != "full" {
			continue
		}
		for _, f := range fields[1:] {
			k, v, ok := strings.Cut(f, "=")
			if !ok {
				continue
			}
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				continue
			}
			switch prefix + "." + k {
			case "some.avg10":
				target = &out.SomeAvg10
			case "some.avg60":
				target = &out.SomeAvg60
			case "some.avg300":
				target = &out.SomeAvg300
			case "full.avg10":
				target = &out.FullAvg10
			case "full.avg60":
				target = &out.FullAvg60
			case "full.avg300":
				target = &out.FullAvg300
			default:
				continue
			}
			*target = parsed
			out.Available = true
		}
	}
	if err := scanner.Err(); err != nil {
		return PSI{}, err
	}
	if !out.Available {
		return PSI{}, fmt.Errorf("psi data not found")
	}
	return out, nil
}

type Snapshot struct {
	CPUBusyPercent      float64
	CPUAvailable        bool
	MemAvailablePercent float64
	MemAvailable        bool
	PSIIO               PSI
}

type Collector struct {
	ProcRoot string
	lastCPU  CPUStat
	hasCPU   bool
}

func NewCollector() *Collector {
	return &Collector{ProcRoot: "/proc"}
}

func (c *Collector) Snapshot() (Snapshot, error) {
	root := c.ProcRoot
	if root == "" {
		root = "/proc"
	}
	var snap Snapshot
	var errs []string

	if data, err := os.Open(root + "/stat"); err == nil {
		cpu, err := ParseCPUStat(data)
		_ = data.Close()
		if err == nil {
			if c.hasCPU {
				if pct, ok := BusyPercent(c.lastCPU, cpu); ok {
					snap.CPUBusyPercent = pct
					snap.CPUAvailable = true
				}
			}
			c.lastCPU = cpu
			c.hasCPU = true
		} else {
			errs = append(errs, err.Error())
		}
	} else {
		errs = append(errs, err.Error())
	}

	if data, err := os.Open(root + "/meminfo"); err == nil {
		mem, err := ParseMemInfo(data)
		_ = data.Close()
		if err == nil {
			if pct, ok := mem.AvailablePercent(); ok {
				snap.MemAvailablePercent = pct
				snap.MemAvailable = true
			}
		} else {
			errs = append(errs, err.Error())
		}
	} else {
		errs = append(errs, err.Error())
	}

	if data, err := os.Open(root + "/pressure/io"); err == nil {
		psi, err := ParsePSI(data)
		_ = data.Close()
		if err == nil {
			snap.PSIIO = psi
		}
	}

	if !snap.CPUAvailable && !snap.MemAvailable && !snap.PSIIO.Available && len(errs) > 0 {
		return snap, errors.New(strings.Join(errs, "; "))
	}
	return snap, nil
}
