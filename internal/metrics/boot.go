package metrics

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ReadBootID returns the kernel boot id, which changes on every boot.
func ReadBootID(procRoot string) (string, error) {
	if procRoot == "" {
		procRoot = "/proc"
	}
	data, err := os.ReadFile(filepath.Join(procRoot, "sys/kernel/random/boot_id"))
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(data))
	if !ValidBootID(id) {
		return "", fmt.Errorf("unexpected boot id format")
	}
	return id, nil
}

// ValidBootID accepts the UUID form the kernel uses.
func ValidBootID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdef", r) {
				return false
			}
		}
	}
	return true
}

// ParseBootTime reads the "btime" line of /proc/stat.
func ParseBootTime(r io.Reader) (time.Time, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "btime" {
			sec, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil || sec <= 0 {
				return time.Time{}, fmt.Errorf("invalid btime")
			}
			return time.Unix(sec, 0).UTC(), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return time.Time{}, err
	}
	return time.Time{}, fmt.Errorf("btime not found")
}

// ReadBootTime returns the time the kernel booted.
func ReadBootTime(procRoot string) (time.Time, error) {
	if procRoot == "" {
		procRoot = "/proc"
	}
	f, err := os.Open(filepath.Join(procRoot, "stat"))
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()
	return ParseBootTime(f)
}
