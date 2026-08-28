package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
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
	temp, err := os.CreateTemp(filepath.Dir(dir), ".gofile-zip-*")
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

// PrepareSelectionZip fully builds a batch download in the operating system's
// temporary directory before an HTTP handler writes success headers. The caller
// owns the returned file and must invoke cleanup after it finishes streaming.
func PrepareSelectionZip(selection Selection) (*os.File, func(), error) {
	if err := PreflightSelectionZip(selection); err != nil {
		return nil, nil, err
	}
	file, err := os.CreateTemp("", "gofile-selection-*.zip")
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

// Unzip extracts an archive relative to the managed root into its containing
// directory. Extraction is staged and published only after a complete success.
func Unzip(src string) bool {
	_, err := ExtractArchive(src)
	return err == nil
}

func ExtractArchive(rawPath string) (string, error) {
	operationMu.Lock()
	defer operationMu.Unlock()
	absolute, rel, _, err := ResolveExisting(rawPath, false)
	if err != nil {
		return "", err
	}
	info, err := safeEntryInfo(absolute)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", ErrUnsupportedType
	}
	extension := strings.ToLower(filepath.Ext(rel))
	switch extension {
	case ".zip":
		return rel, extractZip(absolute, filepath.Dir(absolute))
	case ".gz", ".tgz":
		return rel, extractTarGz(absolute, filepath.Dir(absolute))
	default:
		return "", operationError("unsupported_archive")
	}
}

func archiveDestination(base, name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.HasPrefix(name, "/") || path.IsAbs(name) {
		return "", ErrInvalidPath
	}
	cleanName := path.Clean(name)
	if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, "../") {
		return "", ErrInvalidPath
	}
	for _, part := range strings.Split(cleanName, "/") {
		if ValidateLeafName(part) != nil {
			return "", ErrInvalidPath
		}
	}
	target := filepath.Join(base, filepath.FromSlash(cleanName))
	cleanBase := filepath.Clean(base)
	if !contained(cleanBase, target) {
		return "", ErrInvalidPath
	}
	root, err := Root()
	if err != nil || !contained(root, target) {
		return "", ErrInvalidPath
	}
	// Existing symlink parents cannot be trusted.
	for parent := filepath.Dir(target); parent != cleanBase && parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
		if stat, err := os.Lstat(parent); err == nil && stat.Mode()&os.ModeSymlink != 0 {
			return "", ErrInvalidPath
		}
	}
	return target, nil
}

func topLevelArchiveName(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	cleanName := path.Clean(name)
	if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, "../") || strings.HasPrefix(cleanName, "/") {
		return "", ErrInvalidPath
	}
	top := strings.Split(cleanName, "/")[0]
	if ValidateLeafName(top) != nil {
		return "", ErrInvalidPath
	}
	return top, nil
}

func preflightExtractionTargets(outputDir string, names []string) ([]string, error) {
	tops := make(map[string]struct{})
	for _, name := range names {
		if _, err := archiveDestination(outputDir, name); err != nil {
			return nil, err
		}
		top, err := topLevelArchiveName(name)
		if err != nil {
			return nil, err
		}
		tops[top] = struct{}{}
	}
	result := make([]string, 0, len(tops))
	for top := range tops {
		target := filepath.Join(outputDir, top)
		if _, err := os.Lstat(target); err == nil {
			return nil, ErrDestinationExists
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, operationError("io_error")
		}
		result = append(result, top)
	}
	sort.Strings(result)
	return result, nil
}

func newExtractionStage(outputDir string) (string, error) {
	stage, err := os.MkdirTemp(outputDir, ".gofile-extract-")
	if err != nil {
		return "", operationError("io_error")
	}
	return stage, nil
}

func promoteExtractionStage(stage, outputDir string, tops []string) error {
	promoted := make([]string, 0, len(tops))
	for _, top := range tops {
		source := filepath.Join(stage, top)
		target := filepath.Join(outputDir, top)
		if err := os.Rename(source, target); err != nil {
			for i := len(promoted) - 1; i >= 0; i-- {
				_ = os.Rename(filepath.Join(outputDir, promoted[i]), filepath.Join(stage, promoted[i]))
			}
			return operationError("io_error")
		}
		promoted = append(promoted, top)
	}
	return nil
}

func validateZipEntries(files []*zip.File, outputDir string) ([]string, error) {
	if len(files) > maxArchiveEntries {
		return nil, operationError("archive_limit_exceeded")
	}
	names := make([]string, 0, len(files))
	var total int64
	for _, file := range files {
		if file.Mode()&os.ModeSymlink != 0 || (!file.FileInfo().IsDir() && !file.Mode().IsRegular()) {
			return nil, ErrUnsupportedType
		}
		if file.UncompressedSize64 > uint64(maxArchiveBytes) {
			return nil, operationError("archive_limit_exceeded")
		}
		size := int64(file.UncompressedSize64)
		if total > maxArchiveBytes-size {
			return nil, operationError("archive_limit_exceeded")
		}
		total += size
		names = append(names, file.Name)
	}
	return preflightExtractionTargets(outputDir, names)
}

func extractZip(source, outputDir string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return operationError("io_error")
	}
	defer reader.Close()
	tops, err := validateZipEntries(reader.File, outputDir)
	if err != nil {
		return err
	}
	stage, err := newExtractionStage(outputDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for _, file := range reader.File {
		destination, err := archiveDestination(stage, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0755); err != nil {
				return operationError("io_error")
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return operationError("io_error")
		}
		input, err := file.Open()
		if err != nil {
			return operationError("io_error")
		}
		output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, file.Mode().Perm())
		if err != nil {
			input.Close()
			if errors.Is(err, fs.ErrExist) {
				return ErrDestinationExists
			}
			return operationError("io_error")
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		inputCloseErr := input.Close()
		if copyErr != nil || closeErr != nil || inputCloseErr != nil {
			return operationError("io_error")
		}
	}
	return promoteExtractionStage(stage, outputDir, tops)
}

func scanTarGz(source, outputDir string) ([]string, error) {
	file, err := os.Open(source)
	if err != nil {
		return nil, operationError("io_error")
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, operationError("io_error")
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	names := make([]string, 0)
	entries := 0
	var total int64
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, operationError("io_error")
		}
		entries++
		if header.Size < 0 || entries > maxArchiveEntries || header.Size > maxArchiveBytes || total > maxArchiveBytes-header.Size {
			return nil, operationError("archive_limit_exceeded")
		}
		total += header.Size
		if header.Typeflag != tar.TypeDir && header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, ErrUnsupportedType
		}
		names = append(names, header.Name)
	}
	return preflightExtractionTargets(outputDir, names)
}

func extractTarGz(source, outputDir string) error {
	tops, err := scanTarGz(source, outputDir)
	if err != nil {
		return err
	}
	stage, err := newExtractionStage(outputDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	file, err := os.Open(source)
	if err != nil {
		return operationError("io_error")
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return operationError("io_error")
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	entries := 0
	var total int64
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return operationError("io_error")
		}
		entries++
		if header.Size < 0 || entries > maxArchiveEntries || header.Size > maxArchiveBytes || total > maxArchiveBytes-header.Size {
			return operationError("archive_limit_exceeded")
		}
		total += header.Size
		if header.Typeflag != tar.TypeDir && header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return ErrUnsupportedType
		}
		destination, err := archiveDestination(stage, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, os.FileMode(header.Mode).Perm()); err != nil {
				return operationError("io_error")
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				return operationError("io_error")
			}
			output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(header.Mode).Perm())
			if err != nil {
				if errors.Is(err, fs.ErrExist) {
					return ErrDestinationExists
				}
				return operationError("io_error")
			}
			_, copyErr := io.Copy(output, tarReader)
			closeErr := output.Close()
			if copyErr != nil || closeErr != nil {
				return operationError("io_error")
			}
		}
	}
	return promoteExtractionStage(stage, outputDir, tops)
}
