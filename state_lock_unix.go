//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package main

import (
	"errors"
	"fmt"
	"syscall"
)

func acquireStateLock(path string) (*stateLock, error) {
	lock, err := openStateLockFile(path)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.New("state directory is already in use")
		}
		return nil, fmt.Errorf("could not acquire state lock: %w", err)
	}
	return lock, nil
}
