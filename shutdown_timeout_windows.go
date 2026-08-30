//go:build windows

package main

import "time"

// Windows console-control shutdown handlers have a short completion window.
func shutdownTimeout() time.Duration {
	return 4 * time.Second
}
