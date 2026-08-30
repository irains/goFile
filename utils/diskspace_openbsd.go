//go:build openbsd

package utils

import "golang.org/x/sys/unix"

// DiskUsage returns total and free bytes for the filesystem containing path.
func DiskUsage(path string) (total, free uint64) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil || stat.F_bavail < 0 {
		return 0, 0
	}
	return stat.F_blocks * uint64(stat.F_bsize), uint64(stat.F_bavail) * uint64(stat.F_bsize)
}
