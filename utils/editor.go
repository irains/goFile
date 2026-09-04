package utils

import (
	"io"
	"os"
	"unicode"
	"unicode/utf8"
)

// EditorFile is a managed regular file whose bounded contents have passed the
// text-editor policy. Callers must still treat the data as untrusted content.
type EditorFile struct {
	Absolute string
	Relative string
	Info     os.FileInfo
	Data     []byte
}

// IsTextContent accepts UTF-8 textual content supported by the browser editor.
// Tab, carriage return, and line feed are normal text whitespace; other control
// characters indicate a binary or terminal-control payload and are rejected.
func IsTextContent(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	for _, value := range string(data) {
		if value == '\t' || value == '\r' || value == '\n' {
			continue
		}
		if unicode.IsControl(value) {
			return false
		}
	}
	return true
}

// ReadTextFile resolves and inspects a managed regular file under the operation
// lock. It never reads more than maxBytes+1 bytes and rejects non-text content.
func ReadTextFile(rawPath string, maxBytes int64) (EditorFile, error) {
	operationMu.Lock()
	defer operationMu.Unlock()
	return readTextFileLocked(rawPath, maxBytes)
}

func readTextFileLocked(rawPath string, maxBytes int64) (EditorFile, error) {
	absolute, relative, info, err := ResolveExisting(rawPath, false)
	if err != nil {
		return EditorFile{}, err
	}
	if !info.Mode().IsRegular() {
		return EditorFile{}, ErrUnsupportedType
	}
	if info.Size() > maxBytes {
		return EditorFile{}, ErrBatchLimitExceeded
	}

	file, err := os.Open(absolute)
	if err != nil {
		return EditorFile{}, operationError("io_error")
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return EditorFile{}, operationError("io_error")
	}
	if int64(len(data)) > maxBytes {
		return EditorFile{}, ErrBatchLimitExceeded
	}
	if !IsTextContent(data) {
		return EditorFile{}, ErrUnsupportedType
	}

	verified, err := safeEntryInfo(absolute)
	if err != nil {
		return EditorFile{}, err
	}
	if !verified.Mode().IsRegular() {
		return EditorFile{}, ErrUnsupportedType
	}
	if verified.Size() > maxBytes {
		return EditorFile{}, ErrBatchLimitExceeded
	}
	if EntryVersion(verified) != EntryVersion(info) {
		return EditorFile{}, ErrSourceChanged
	}
	return EditorFile{Absolute: absolute, Relative: relative, Info: verified, Data: data}, nil
}

// IsTextFile is a non-authoritative listing hint. Editor endpoints must call
// ReadTextFile or WriteVersionedTextFile again before serving or changing data.
func IsTextFile(rawPath string, maxBytes int64) bool {
	_, err := ReadTextFile(rawPath, maxBytes)
	return err == nil
}

// WriteVersionedTextFile revalidates a managed textual regular file under the
// filesystem operation lock before replacing its contents. The caller must
// record the audit attempt before calling this helper and success afterward.
func WriteVersionedTextFile(rawPath, expectedVersion string, data []byte, maxBytes int64) (os.FileInfo, error) {
	if int64(len(data)) > maxBytes {
		return nil, ErrBatchLimitExceeded
	}
	if !IsTextContent(data) {
		return nil, ErrInvalidTextContent
	}

	operationMu.Lock()
	defer operationMu.Unlock()

	current, err := readTextFileLocked(rawPath, maxBytes)
	if err != nil {
		return nil, err
	}
	if expectedVersion == "" || EntryVersion(current.Info) != expectedVersion {
		return nil, ErrSourceChanged
	}
	if err := os.WriteFile(current.Absolute, data, current.Info.Mode().Perm()); err != nil {
		return nil, operationError("io_error")
	}
	updated, err := safeEntryInfo(current.Absolute)
	if err != nil {
		return nil, err
	}
	if !updated.Mode().IsRegular() {
		return nil, ErrUnsupportedType
	}
	return updated, nil
}
