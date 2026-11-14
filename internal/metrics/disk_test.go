package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUsageFromStatMatchesDf(t *testing.T) {
	// 1000 blocks, 100 free of which 50 are available to non-root users:
	// df shows used=900, avail=50, so free = 50/950.
	u, err := UsageFromStat("/", 1, FSStat{BlockSize: 4096, Blocks: 1000, BlocksFree: 100, BlocksAvail: 50, Files: 200, FilesFree: 20})
	if err != nil {
		t.Fatalf("UsageFromStat: %v", err)
	}
	if got := u.FreePercent; got < 5.26 || got > 5.27 {
		t.Fatalf("FreePercent = %v", got)
	}
	if u.AvailBytes != 50*4096 || u.SizeBytes != 1000*4096 {
		t.Fatalf("bytes = %d/%d", u.AvailBytes, u.SizeBytes)
	}
	if !u.InodesKnown || u.InodesFreePercent != 10 {
		t.Fatalf("inodes = %+v", u)
	}
}

func TestUsageFromStatWithoutInodeTable(t *testing.T) {
	u, err := UsageFromStat("/", 1, FSStat{BlockSize: 4096, Blocks: 10, BlocksFree: 5, BlocksAvail: 5})
	if err != nil {
		t.Fatalf("UsageFromStat: %v", err)
	}
	if u.InodesKnown {
		t.Fatal("inodes should be unknown when the filesystem reports none")
	}
}

func TestUsageFromStatRejectsNonsense(t *testing.T) {
	for name, st := range map[string]FSStat{
		"no blocks":       {BlockSize: 4096},
		"free over total": {BlockSize: 4096, Blocks: 10, BlocksFree: 11, BlocksAvail: 1},
		"avail over free": {BlockSize: 4096, Blocks: 10, BlocksFree: 1, BlocksAvail: 2},
	} {
		if _, err := UsageFromStat("/", 1, st); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	for n, want := range map[uint64]string{0: "0 B", 1023: "1023 B", 1536: "1.5 KiB", 5 << 30: "5.0 GiB"} {
		if got := HumanBytes(n); got != want {
			t.Fatalf("HumanBytes(%d) = %q want %q", n, got, want)
		}
	}
}

func TestReadBootID(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sys/kernel/random")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	id := "3223bc29-6995-44d6-82e7-38ae2d4e7e83"
	if err := os.WriteFile(filepath.Join(dir, "boot_id"), []byte(id+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBootID(root)
	if err != nil || got != id {
		t.Fatalf("ReadBootID = %q, %v", got, err)
	}
	if ValidBootID("not-a-boot-id") || ValidBootID(strings.ToUpper(id)) {
		t.Fatal("invalid boot id accepted")
	}
}

func TestParseBootTime(t *testing.T) {
	got, err := ParseBootTime(strings.NewReader("cpu 1 2 3\nbtime 1777200000\nprocesses 5\n"))
	if err != nil || !got.Equal(time.Unix(1777200000, 0)) {
		t.Fatalf("ParseBootTime = %v, %v", got, err)
	}
	if _, err := ParseBootTime(strings.NewReader("cpu 1\n")); err == nil {
		t.Fatal("expected error without btime")
	}
}
