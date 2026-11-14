package metrics

import (
	"fmt"
	"syscall"
)

// ReadDiskUsage measures the filesystem that contains path.
func ReadDiskUsage(path string) (DiskUsage, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return DiskUsage{}, fmt.Errorf("stat %s: %w", path, err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return DiskUsage{}, fmt.Errorf("statfs %s: %w", path, err)
	}
	size := uint64(fs.Frsize)
	if size == 0 {
		size = uint64(fs.Bsize)
	}
	return UsageFromStat(path, uint64(st.Dev), FSStat{
		BlockSize:   size,
		Blocks:      fs.Blocks,
		BlocksFree:  fs.Bfree,
		BlocksAvail: fs.Bavail,
		Files:       fs.Files,
		FilesFree:   fs.Ffree,
	})
}
