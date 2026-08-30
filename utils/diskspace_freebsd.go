//go:build freebsd

package utils

import "golang.org/x/sys/unix"

// DiskUsage returns total and free bytes for the filesystem containing path.
func DiskUsage(path string) (total, free uint64) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil || stat.Bavail < 0 {
		return 0, 0
	}
	return stat.Blocks * stat.Bsize, uint64(stat.Bavail) * stat.Bsize
}
