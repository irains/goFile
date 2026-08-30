//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package main

import "errors"

func acquireStateLock(string) (*stateLock, error) {
	return nil, errors.New("runtime state locking is unsupported on this platform")
}
