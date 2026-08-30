//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package main

import "os"

func protectPrivateDirectory(path string) error {
	return os.Chmod(path, 0700)
}

func protectPrivateFile(path string) error {
	return os.Chmod(path, 0600)
}
