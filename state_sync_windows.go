//go:build windows

package main

func syncRuntimeDirectory(string) error {
	// Windows does not support flushing directory handles with FlushFileBuffers.
	// Audit-file data is still flushed before a rotation; metadata durability uses
	// the platform filesystem's normal rename guarantees.
	return nil
}
