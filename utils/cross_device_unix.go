//go:build !windows

package utils

import "syscall"

var errCrossDevice = syscall.EXDEV
