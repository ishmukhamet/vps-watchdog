package metrics

import (
	"errors"
	"fmt"
)

// FSStat is the subset of statfs(2) needed for free space checks, in a
// platform-neutral form.
type FSStat struct {
	BlockSize   uint64 // fragment size, the unit of the block counts
	Blocks      uint64
	BlocksFree  uint64 // free, including blocks reserved for root
	BlocksAvail uint64 // free for unprivileged processes
	Files       uint64
	FilesFree   uint64
}

// DiskUsage describes one filesystem as an unprivileged process sees it.
type DiskUsage struct {
	Path              string
	Device            uint64
	SizeBytes         uint64
	AvailBytes        uint64
	FreePercent       float64
	InodesTotal       uint64
	InodesFree        uint64
	InodesFreePercent float64
	// InodesKnown is false on filesystems without a fixed inode table, such
	// as btrfs, which report zero inodes.
	InodesKnown bool
}

// UsageFromStat computes usage the way df(1) does: the percentage is relative
// to used + available space, so blocks reserved for root count as unusable.
func UsageFromStat(path string, device uint64, st FSStat) (DiskUsage, error) {
	if st.BlockSize == 0 || st.Blocks == 0 {
		return DiskUsage{}, errors.New("filesystem reports no blocks")
	}
	if st.BlocksFree > st.Blocks || st.BlocksAvail > st.BlocksFree {
		return DiskUsage{}, errors.New("inconsistent block counts")
	}
	used := st.Blocks - st.BlocksFree
	usable := used + st.BlocksAvail
	u := DiskUsage{
		Path:       path,
		Device:     device,
		SizeBytes:  st.Blocks * st.BlockSize,
		AvailBytes: st.BlocksAvail * st.BlockSize,
	}
	if usable > 0 {
		u.FreePercent = 100 * float64(st.BlocksAvail) / float64(usable)
	}
	if st.Files > 0 && st.FilesFree <= st.Files {
		u.InodesTotal = st.Files
		u.InodesFree = st.FilesFree
		u.InodesFreePercent = 100 * float64(st.FilesFree) / float64(st.Files)
		u.InodesKnown = true
	}
	return u, nil
}

// HumanBytes formats a byte count with binary units, e.g. "1.5 GiB".
func HumanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit && exp < 5; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
