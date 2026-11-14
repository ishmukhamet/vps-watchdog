//go:build !linux

package metrics

import "errors"

// ReadDiskUsage is implemented only on Linux, the supported target.
func ReadDiskUsage(path string) (DiskUsage, error) {
	return DiskUsage{}, errors.New("disk usage is supported only on linux")
}
