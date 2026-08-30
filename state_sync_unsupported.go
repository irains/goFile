//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package main

func syncRuntimeDirectory(string) error {
	return nil
}
