//go:build netbsd

package utils

import "golang.org/x/sys/unix"

// DiskUsage returns total and free bytes for the filesystem containing path.
func DiskUsage(path string) (total, free uint64) {
	var stat unix.Statvfs_t
	if err := unix.Statvfs(path, &stat); err != nil {
		return 0, 0
	}
	return stat.Blocks * stat.Frsize, stat.Bavail * stat.Frsize
}
