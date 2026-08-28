package utils

import (
	"goFile/auth"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxPropertyEntries = 100000
	maxPropertyBytes   = int64(2 << 30)
)

// Properties holds read-only, portable information about a managed item.
type Properties struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Extension  string `json:"extension,omitempty"`
	Size       int64  `json:"size"`
	Modified   int64  `json:"modified"`
	Mode       string `json:"mode"`
	EntryCount int    `json:"entry_count,omitempty"`
	Incomplete bool   `json:"incomplete,omitempty"`
}

func GetProperties(rawPath string) (Properties, error) {
	absolute, rel, _, err := ResolveExisting(rawPath, false)
	if err != nil {
		return Properties{}, err
	}
	info, err := safeEntryInfo(absolute)
	if err != nil {
		return Properties{}, err
	}
	kind := "file"
	if info.IsDir() {
		kind = "directory"
	}
	properties := Properties{
		Name:      filepath.Base(rel),
		Path:      rel,
		Kind:      kind,
		Extension: strings.TrimPrefix(strings.ToLower(filepath.Ext(rel)), "."),
		Size:      info.Size(),
		Modified:  info.ModTime().Unix(),
		Mode:      info.Mode().String(),
	}
	if !info.IsDir() {
		return properties, nil
	}
	var size int64
	var count int
	incomplete := false
	err = filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absolute {
			return nil
		}
		count++
		if count > maxPropertyEntries {
			incomplete = true
			return fs.SkipAll
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			incomplete = true
			return fs.SkipDir
		}
		if !info.IsDir() {
			size += info.Size()
			if size > maxPropertyBytes {
				incomplete = true
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return Properties{}, operationError("io_error")
	}
	properties.Size = size
	properties.EntryCount = count
	properties.Incomplete = incomplete
	return properties, nil
}

// SelectionFromArchiveItems revalidates a one-use ticket immediately before ZIP
// streaming. It rejects changed items rather than silently archiving a new file.
func SelectionFromArchiveItems(items []auth.ArchiveItem) (Selection, error) {
	if len(items) == 0 || len(items) > MaxListEntries {
		return Selection{}, operationError("invalid_selection")
	}
	selection := Selection{}
	var directory string
	for _, item := range items {
		absolute, rel, _, err := ResolveExisting(item.Path, false)
		if err != nil {
			return Selection{}, err
		}
		info, err := safeEntryInfo(absolute)
		if err != nil {
			return Selection{}, err
		}
		if versionFor(info) != item.Version {
			return Selection{}, ErrSourceChanged
		}
		itemDirectory := filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel)))
		if itemDirectory == "." {
			itemDirectory = ""
		}
		if len(selection.Items) == 0 {
			directory = itemDirectory
		} else if directory != itemDirectory {
			return Selection{}, operationError("invalid_selection")
		}
		selection.Items = append(selection.Items, SelectedItem{Name: filepath.Base(rel), Relative: rel, Absolute: absolute, Info: info})
	}
	selection.Directory = directory
	return revalidateSelection(selection)
}
