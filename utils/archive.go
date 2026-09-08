package utils

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxArchiveEntries = 10000
	maxArchiveBytes   = int64(2 << 30) // 2 GiB decompressed / streamed input budget
)

// CreateDirectoryZip writes a single directory ZIP next to its source. The
// archive is first created under a private name and never overwrites a file.
func CreateDirectoryZip(rawPath string) (string, error) {
	operationMu.Lock()
	defer operationMu.Unlock()
	dir, rel, _, err := ResolveDirectory(rawPath, false)
	if err != nil {
		return "", err
	}
	current, err := safeEntryInfo(dir)
	if err != nil {
		return "", err
	}
	if !current.IsDir() {
		return "", ErrNotDirectory
	}
	output := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+".zip")
	if _, err := os.Lstat(output); err == nil {
		return "", ErrDestinationExists
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", operationError("io_error")
	}
	temp, err := os.CreateTemp(filepath.Dir(dir), InternalArchiveZipPrefix+"*")
	if err != nil {
		return "", operationError("io_error")
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := writeZip(temp, []SelectedItem{{Name: filepath.Base(dir), Relative: rel, Absolute: dir, Info: current}}); err != nil {
		_ = temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", operationError("io_error")
	}
	// os.Link publishes atomically without replacing an existing destination on
	// both NTFS and POSIX filesystems. The temporary file is removed by defer.
	if err := os.Link(tempName, output); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", ErrDestinationExists
		}
		return "", operationError("io_error")
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(rel), filepath.Base(dir)+".zip")), nil
}

// PreflightSelectionZip walks every selected descendant before a download
// response is committed. This prevents an ordinary unsupported child from
// producing a successful-looking partial archive response.
func PreflightSelectionZip(selection Selection) error {
	if len(selection.Items) == 0 || len(selection.Items) > MaxListEntries {
		return operationError("invalid_selection")
	}
	state := archiveState{}
	for _, item := range selection.Items {
		current, err := safeEntryInfo(item.Absolute)
		if err != nil {
			return err
		}
		if versionFor(current) != versionFor(item.Info) {
			return ErrSourceChanged
		}
		if err := preflightZipItem(item.Absolute, current, &state); err != nil {
			return err
		}
	}
	return nil
}

func preflightZipItem(absolute string, info os.FileInfo, state *archiveState) error {
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return ErrUnsupportedType
	}
	if !info.IsDir() {
		return state.add(info.Size())
	}
	if err := state.add(0); err != nil {
		return err
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return operationError("io_error")
	}
	for _, entry := range entries {
		childPath := filepath.Join(absolute, entry.Name())
		childInfo, err := safeEntryInfo(childPath)
		if err != nil {
			return err
		}
		if err := preflightZipItem(childPath, childInfo, state); err != nil {
			return err
		}
	}
	return nil
}

// PrepareSelectionZip fully builds a batch download in the supplied private
// temporary directory before an HTTP handler writes success headers. The caller
// owns the returned file and must invoke cleanup after it finishes streaming.
func PrepareSelectionZip(selection Selection, tempDir string) (*os.File, func(), error) {
	if err := PreflightSelectionZip(selection); err != nil {
		return nil, nil, err
	}
	file, err := os.CreateTemp(tempDir, "fileharbor-selection-*.zip")
	if err != nil {
		return nil, nil, operationError("io_error")
	}
	cleanup := func() {
		name := file.Name()
		_ = file.Close()
		_ = os.Remove(name)
	}
	if err := writeZip(file, selection.Items); err != nil {
		cleanup()
		return nil, nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, nil, operationError("io_error")
	}
	return file, cleanup, nil
}

// StreamSelectionZip writes a safe ZIP directly to a response stream. It does
// not create a managed-root artifact and it rejects links and special files.
func StreamSelectionZip(writer io.Writer, selection Selection) error {
	if err := PreflightSelectionZip(selection); err != nil {
		return err
	}
	return writeZip(writer, selection.Items)
}

func writeZip(destination io.Writer, items []SelectedItem) error {
	zipWriter := zip.NewWriter(destination)
	state := archiveState{}
	for _, item := range items {
		current, err := safeEntryInfo(item.Absolute)
		if err != nil {
			_ = zipWriter.Close()
			return err
		}
		if versionFor(current) != versionFor(item.Info) {
			_ = zipWriter.Close()
			return ErrSourceChanged
		}
		if err := addZipItem(zipWriter, item.Absolute, item.Name, current, &state); err != nil {
			_ = zipWriter.Close()
			return err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return operationError("io_error")
	}
	return nil
}

type archiveState struct {
	entries int
	bytes   int64
}

func (s *archiveState) add(size int64) error {
	s.entries++
	s.bytes += size
	if s.entries > maxArchiveEntries || s.bytes > maxArchiveBytes {
		return operationError("archive_limit_exceeded")
	}
	return nil
}

func addZipItem(zipWriter *zip.Writer, absolute, archiveName string, info os.FileInfo, state *archiveState) error {
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return ErrUnsupportedType
	}
	archiveName = path.Clean(filepath.ToSlash(archiveName))
	if archiveName == "." || strings.HasPrefix(archiveName, "../") || strings.HasPrefix(archiveName, "/") {
		return ErrInvalidPath
	}
	if info.IsDir() {
		if err := state.add(0); err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return operationError("io_error")
		}
		header.Name = strings.TrimSuffix(archiveName, "/") + "/"
		if _, err := zipWriter.CreateHeader(header); err != nil {
			return operationError("io_error")
		}
		entries, err := os.ReadDir(absolute)
		if err != nil {
			return operationError("io_error")
		}
		for _, entry := range entries {
			childPath := filepath.Join(absolute, entry.Name())
			childInfo, err := safeEntryInfo(childPath)
			if err != nil {
				return err
			}
			if err := addZipItem(zipWriter, childPath, path.Join(archiveName, entry.Name()), childInfo, state); err != nil {
				return err
			}
		}
		return nil
	}
	if err := state.add(info.Size()); err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return operationError("io_error")
	}
	header.Name = archiveName
	header.Method = zip.Deflate
	output, err := zipWriter.CreateHeader(header)
	if err != nil {
		return operationError("io_error")
	}
	input, err := os.Open(absolute)
	if err != nil {
		return operationError("io_error")
	}
	limited := io.LimitReader(input, info.Size()+1)
	written, copyErr := io.Copy(output, limited)
	inputCloseErr := input.Close()
	if copyErr != nil || inputCloseErr != nil {
		return operationError("io_error")
	}
	if written != info.Size() {
		return ErrSourceChanged
	}
	return nil
}

// Extraction compatibility helpers and format-specific extraction live in
// archive_extract.go. Keeping ZIP creation here prevents extraction changes from
// affecting existing download and CreateDirectoryZip behavior.
