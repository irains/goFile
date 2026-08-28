//go:build windows

package utils

import "syscall"

// ERROR_NOT_SAME_DEVICE is 17 on Windows and is returned by MoveFile/Rename
// when a native rename crosses volumes.
var errCrossDevice = syscall.Errno(17)
