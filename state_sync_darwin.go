//go:build darwin

package main

// APFS and HFS+ reject fsync on directory descriptors. Audit-file data is
// flushed before rotation; metadata durability relies on the platform rename.
func syncRuntimeDirectory(string) error {
	return nil
}
