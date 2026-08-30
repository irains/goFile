package utils

import (
	"archive/zip"
	"bytes"
	"errors"
	"github.com/irains/fileharbor/conf"
	"os"
	"path/filepath"
	"testing"
)

func TestSelectionZipIncludesTopLevelNames(t *testing.T) {
	previous := conf.FileHarbor
	root := t.TempDir()
	conf.FileHarbor = root
	t.Cleanup(func() { conf.FileHarbor = previous })
	if err := os.Mkdir(filepath.Join(root, "logs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "logs", "app.log"), []byte("line"), 0644); err != nil {
		t.Fatal(err)
	}
	absolute, rel, info, err := ResolveExisting("logs", false)
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := StreamSelectionZip(&buffer, Selection{Items: []SelectedItem{{Name: "logs", Relative: rel, Absolute: absolute, Info: info}}}); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range reader.File {
		if entry.Name == "logs/app.log" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing top-level directory entry")
	}
}

func TestPrepareSelectionZipRejectsUnsafeDescendants(t *testing.T) {
	root := t.TempDir()
	previous := conf.FileHarbor
	conf.FileHarbor = root
	t.Cleanup(func() { conf.FileHarbor = previous })
	if err := os.Mkdir(filepath.Join(root, "logs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside"), filepath.Join(root, "logs", "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	absolute, rel, info, err := ResolveExisting("logs", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, cleanup, err := PrepareSelectionZip(Selection{Items: []SelectedItem{{Name: "logs", Relative: rel, Absolute: absolute, Info: info}}}, t.TempDir()); ErrorCode(err) != "unsupported_file_type" || cleanup != nil {
		t.Fatalf("expected unsafe descendant rejection, got %v", err)
	}
}

func TestExtractionRejectsInvalidArchiveWithoutPublishingEntries(t *testing.T) {
	previous := conf.FileHarbor
	root := t.TempDir()
	conf.FileHarbor = root
	t.Cleanup(func() { conf.FileHarbor = previous })
	archivePath := filepath.Join(root, "unsafe.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	first, err := writer.Create("created.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Write([]byte("created")); err != nil {
		t.Fatal(err)
	}
	second, err := writer.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Write([]byte("escape")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractArchive("unsafe.zip"); err == nil {
		t.Fatal("expected invalid archive rejection")
	}
	if _, err := os.Lstat(filepath.Join(root, "created.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial extraction was published: %v", err)
	}
}

func TestCreateDirectoryZipDoesNotOverwrite(t *testing.T) {
	previous := conf.FileHarbor
	root := t.TempDir()
	conf.FileHarbor = root
	t.Cleanup(func() { conf.FileHarbor = previous })
	if err := os.Mkdir(filepath.Join(root, "data"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateDirectoryZip("data"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "data.zip")); err != nil {
		t.Fatalf("expected published archive: %v", err)
	}
	if _, err := CreateDirectoryZip("data"); ErrorCode(err) != "destination_exists" {
		t.Fatalf("expected conflict, got %v", err)
	}
}
