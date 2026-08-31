package utils

import (
	"os"
)

// WriteVersionedFile revalidates a managed regular file under the filesystem
// operation lock before replacing its contents. The caller must record the audit
// attempt before calling this helper and the success record after it returns.
func WriteVersionedFile(rawPath, expectedVersion string, data []byte) (os.FileInfo, error) {
	operationMu.Lock()
	defer operationMu.Unlock()

	absolute, _, info, err := ResolveExisting(rawPath, false)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrUnsupportedType
	}
	if expectedVersion == "" || EntryVersion(info) != expectedVersion {
		return nil, ErrSourceChanged
	}
	if err := os.WriteFile(absolute, data, info.Mode().Perm()); err != nil {
		return nil, operationError("io_error")
	}
	updated, err := safeEntryInfo(absolute)
	if err != nil {
		return nil, err
	}
	if !updated.Mode().IsRegular() {
		return nil, ErrUnsupportedType
	}
	return updated, nil
}
