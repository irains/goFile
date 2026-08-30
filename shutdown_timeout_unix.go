//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package main

import "time"

func shutdownTimeout() time.Duration {
	return 15 * time.Second
}
