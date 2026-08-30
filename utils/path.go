package utils

import (
	"errors"
	"fmt"
	"github.com/irains/fileharbor/conf"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	MaxListEntries            = 100
	InternalUploadStagePrefix = ".fileharbor-upload-"
)

var (
	ErrInvalidPath        = &OperationError{Code: "invalid_path"}
	ErrNotFound           = &OperationError{Code: "not_found"}
	ErrNotDirectory       = &OperationError{Code: "not_directory"}
	ErrRootOperation      = &OperationError{Code: "root_operation_forbidden"}
	ErrDestinationExists  = &OperationError{Code: "destination_exists"}
	ErrSelfDescendant     = &OperationError{Code: "self_descendant"}
	ErrInvalidName        = &OperationError{Code: "invalid_name"}
	ErrUnsupportedType    = &OperationError{Code: "unsupported_file_type"}
	ErrSourceChanged      = &OperationError{Code: "source_changed"}
	ErrBatchLimitExceeded = &OperationError{Code: "batch_limit_exceeded"}
)

// OperationError intentionally carries a stable public code only. Handlers must
// not return internal path names or underlying operating-system errors.
type OperationError struct{ Code string }

func (e *OperationError) Error() string { return e.Code }

func operationError(code string) error { return &OperationError{Code: code} }

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var op *OperationError
	if errors.As(err, &op) {
		return op.Code
	}
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound.Code
	}
	return "io_error"
}

// Root resolves the configured managed directory. Startup verifies it exists;
// doing this here also keeps all handlers testable with a temporary root.
func Root() (string, error) {
	if conf.FileHarbor == "" {
		return "", ErrInvalidPath
	}
	absolute, err := filepath.Abs(conf.FileHarbor)
	if err != nil {
		return "", ErrInvalidPath
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", ErrInvalidPath
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", ErrInvalidPath
	}
	return filepath.Clean(resolved), nil
}

// CleanRelative accepts a slash-style root-relative web path. It rejects raw
// traversal rather than trying to repair it, preventing surprising operations.
func CleanRelative(raw string, allowRoot bool) (string, error) {
	if strings.ContainsRune(raw, 0) || !utf8.ValidString(raw) {
		return "", ErrInvalidPath
	}
	raw = strings.ReplaceAll(raw, "\\", "/")
	if raw == "" || raw == "." {
		if allowRoot {
			return "", nil
		}
		return "", ErrRootOperation
	}
	if strings.HasPrefix(raw, "/") || strings.Contains(raw, ":") {
		return "", ErrInvalidPath
	}
	for _, part := range strings.Split(raw, "/") {
		if part == ".." || part == "" && raw != "" || ValidateLeafName(part) != nil {
			return "", ErrInvalidPath
		}
		for _, r := range part {
			if r < 32 || r == 127 {
				return "", ErrInvalidPath
			}
		}
	}
	cleaned := path.Clean(raw)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", ErrInvalidPath
	}
	return cleaned, nil
}

// Contained reports whether candidate is the directory root or one of its descendants.
// Both paths must already be absolute and cleaned by the caller.
func Contained(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func contained(root, candidate string) bool {
	return Contained(root, candidate)
}

// ResolveExisting returns a symlink-safe target. The final component is also
// returned via Lstat so callers can explicitly reject links and special files.
func ResolveExisting(raw string, allowRoot bool) (string, string, os.FileInfo, error) {
	rel, err := CleanRelative(raw, allowRoot)
	if err != nil {
		return "", "", nil, err
	}
	root, err := Root()
	if err != nil {
		return "", "", nil, err
	}
	candidate := root
	if rel != "" {
		candidate = filepath.Join(root, filepath.FromSlash(rel))
	}
	if !contained(root, candidate) {
		return "", "", nil, ErrInvalidPath
	}
	info, err := os.Lstat(candidate)
	if errors.Is(err, fs.ErrNotExist) {
		return "", "", nil, ErrNotFound
	}
	if err != nil {
		return "", "", nil, operationError("io_error")
	}
	if err := rejectSymlinkParents(root, candidate); err != nil {
		return "", "", nil, err
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil || !contained(root, resolved) {
		return "", "", nil, ErrInvalidPath
	}
	// Refresh the Lstat result after resolving link containment. Callers use this
	// metadata for type and version checks, so returning the earlier snapshot would
	// permit a changed final entry to be acted on before their next revalidation.
	info, err = safeEntryInfo(candidate)
	if err != nil {
		return "", "", nil, err
	}
	return candidate, rel, info, nil
}

func ResolveDirectory(raw string, allowRoot bool) (string, string, os.FileInfo, error) {
	absolute, rel, _, err := ResolveExisting(raw, allowRoot)
	if err != nil {
		return "", "", nil, err
	}
	info, err := safeEntryInfo(absolute)
	if err != nil {
		return "", "", nil, err
	}
	if !info.IsDir() {
		return "", "", nil, ErrNotDirectory
	}
	return absolute, rel, info, nil
}

// ValidateLeafName applies the portable subset of filename rules, so a name
// accepted on every FileHarbor-supported platform.
func ValidateLeafName(name string) error {
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, InternalUploadStagePrefix) || !utf8.ValidString(name) || strings.ContainsAny(name, "/\\<>:\"|?*") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return ErrInvalidName
	}
	for _, r := range name {
		if r < 32 || r == 127 {
			return ErrInvalidName
		}
	}
	// Windows treats every extension of a device name as the device itself, so
	// inspect the portion before the first dot rather than only the final suffix.
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	reserved := map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true, "COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true, "LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true}
	if reserved[base] {
		return ErrInvalidName
	}
	return nil
}

// rejectSymlinkParents prevents operations from treating a child below a managed
// symlink as a direct root-relative item. The final component is checked by the
// caller so a caller can return ErrUnsupportedType for that case.
func rejectSymlinkParents(root, candidate string) error {
	if candidate == root {
		return nil
	}
	parent := filepath.Dir(candidate)
	for {
		if !contained(root, parent) {
			return ErrInvalidPath
		}
		info, err := os.Lstat(parent)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return ErrNotFound
			}
			return operationError("io_error")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidPath
		}
		if parent == root {
			return nil
		}
		parent = filepath.Dir(parent)
	}
}

func safeEntryInfo(absolute string) (os.FileInfo, error) {
	root, err := Root()
	if err != nil || !contained(root, absolute) {
		return nil, ErrInvalidPath
	}
	if err := rejectSymlinkParents(root, absolute); err != nil {
		return nil, err
	}
	info, err := os.Lstat(absolute)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, operationError("io_error")
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return nil, ErrUnsupportedType
	}
	return info, nil
}

// EntryVersion returns the stable listing version used to detect a changed
// filesystem entry between selection and execution.
func EntryVersion(info os.FileInfo) string {
	return fmt.Sprintf("%d:%d:%d", info.Size(), info.ModTime().UnixNano(), uint32(info.Mode()))
}

func versionFor(info os.FileInfo) string {
	return EntryVersion(info)
}

// ListDirectory returns at most 100 safely inspectable direct children, sorted
// with directories first then names. Symlinks and special files are omitted.
func ListDirectory(raw string) (conf.Info, error) {
	dir, _, _, err := ResolveDirectory(raw, true)
	if err != nil {
		return conf.Info{}, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return conf.Info{}, operationError("io_error")
	}
	root, err := Root()
	if err != nil {
		return conf.Info{}, err
	}
	info := conf.Info{}
	for _, entry := range entries {
		name := entry.Name()
		if ValidateLeafName(name) != nil {
			continue
		}
		absolute := filepath.Join(dir, name)
		if err := rejectSymlinkParents(root, absolute); err != nil {
			continue
		}
		stat, err := safeEntryInfo(absolute)
		if err != nil {
			continue
		}
		rel, err := relativeToRoot(absolute)
		if err != nil {
			continue
		}
		kind := "file"
		if stat.IsDir() {
			kind = "directory"
		}
		extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		archive := extension == "zip" || extension == "gz" || extension == "tgz"
		item := conf.Entry{Name: name, Path: rel, Kind: kind, Size: stat.Size(), Modified: stat.ModTime(), Mode: stat.Mode().String(), Extension: extension, IsArchive: archive, Version: versionFor(stat)}
		info.Entries = append(info.Entries, item)
		if stat.IsDir() {
			info.Dirs = append(info.Dirs, conf.Dir{DirName: name, DirPath: rel})
		} else {
			info.Files = append(info.Files, conf.File{FileName: name, FilePath: rel, IsZip: archive})
		}
	}
	sort.SliceStable(info.Entries, func(i, j int) bool {
		if info.Entries[i].Kind != info.Entries[j].Kind {
			return info.Entries[i].Kind == "directory"
		}
		return strings.ToLower(info.Entries[i].Name) < strings.ToLower(info.Entries[j].Name)
	})
	if len(info.Entries) > MaxListEntries {
		info.Entries = info.Entries[:MaxListEntries]
		info.Files = info.Files[:0]
		info.Dirs = info.Dirs[:0]
		for _, item := range info.Entries {
			if item.Kind == "directory" {
				info.Dirs = append(info.Dirs, conf.Dir{DirName: item.Name, DirPath: item.Path})
			} else {
				info.Files = append(info.Files, conf.File{FileName: item.Name, FilePath: item.Path, IsZip: item.IsArchive})
			}
		}
	}
	return info, nil
}

func relativeToRoot(absolute string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, absolute)
	if err != nil || rel == "." || !contained(root, absolute) {
		return "", ErrInvalidPath
	}
	return filepath.ToSlash(rel), nil
}

// IsPathSafe is retained for external custom templates. New handlers should use
// the resolver functions above instead of composing paths themselves.
func IsPathSafe(absolute string) bool {
	root, err := Root()
	if err != nil {
		return false
	}
	return contained(root, filepath.Clean(absolute))
}

// Exist reports whether path exists on the filesystem.
func Exist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func GetPrevPath(cPath string) string {
	if cPath == "" || cPath == "/" {
		return "/"
	}
	prevPath := path.Dir(cPath)
	if prevPath == "." || prevPath == "/" {
		return "/"
	}
	return "/d/" + prevPath
}
