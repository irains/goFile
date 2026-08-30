//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package main

import "os"

func shutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
