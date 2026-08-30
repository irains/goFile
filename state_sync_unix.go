//go:build dragonfly || freebsd || linux || netbsd || openbsd

package main

import "os"

func syncRuntimeDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
