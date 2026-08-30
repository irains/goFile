package utils

import (
	"errors"
	"github.com/irains/fileharbor/conf"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func useTestRoot(t *testing.T) string {
	t.Helper()
	previous := conf.FileHarbor
	root := t.TempDir()
	conf.FileHarbor = root
	t.Cleanup(func() { conf.FileHarbor = previous })
	return root
}

func TestCleanRelativeRejectsUnsafePaths(t *testing.T) {
	for _, value := range []string{"../secret", "/etc/passwd", "C:/Windows", "a//b", "a/../../b", "a/CON/file.txt", "a/name. ", "a/file?.txt", "a/NUL.txt", "a/COM1.backup/file.txt"} {
		if _, err := CleanRelative(value, true); err == nil {
			t.Fatalf("%q should be rejected", value)
		}
	}
	if value, err := CleanRelative("dir/file.txt", false); err != nil || value != "dir/file.txt" {
		t.Fatalf("expected safe relative path, got %q, %v", value, err)
	}
}

func TestFileOperationsRejectConflictsAndSelfDescendants(t *testing.T) {
	root := useTestRoot(t)
	if err := os.Mkdir(filepath.Join(root, "source"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source", "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "target"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := CopyItem("source/a.txt", "target", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := CopyItem("source/a.txt", "target", ""); ErrorCode(err) != "destination_exists" {
		t.Fatalf("expected conflict, got %v", err)
	}
	if _, err := CopyItem("source", "source", ""); ErrorCode(err) != "self_descendant" {
		t.Fatalf("expected self-descendant rejection, got %v", err)
	}
}

func TestBatchCopyPreservesPreexistingStagingPath(t *testing.T) {
	root := useTestRoot(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "target"), 0755); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(root, "target", "b.txt.fileharbor-copy-staging")
	if err := os.WriteFile(staging, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	list, err := ListDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	allowed := make(map[string]string)
	for _, entry := range list.Entries {
		if entry.Name == "a.txt" || entry.Name == "b.txt" {
			allowed[entry.Name] = entry.Version
		}
	}
	selection, err := ValidateSelection("", allowed, []ItemRequest{{Name: "a.txt", Version: allowed["a.txt"]}, {Name: "b.txt", Version: allowed["b.txt"]}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BatchCopy(selection, "target"); ErrorCode(err) != "destination_busy" {
		t.Fatalf("expected staging conflict, got %v", err)
	}
	data, err := os.ReadFile(staging)
	if err != nil || string(data) != "keep" {
		t.Fatalf("pre-existing staging path was changed: %q, %v", data, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "target", "a.txt.fileharbor-copy-staging")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("request staging path remained: %v", err)
	}
}

func TestBatchDeleteReportsChangedItem(t *testing.T) {
	root := useTestRoot(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	aAbsolute, aRel, aInfo, err := ResolveExisting("a.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	bAbsolute, bRel, bInfo, err := ResolveExisting("b.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(aAbsolute); err != nil {
		t.Fatal(err)
	}
	results := BatchDelete(Selection{Items: []SelectedItem{{Name: "a.txt", Relative: aRel, Absolute: aAbsolute, Info: aInfo}, {Name: "b.txt", Relative: bRel, Absolute: bAbsolute, Info: bInfo}}})
	if len(results) != 2 {
		t.Fatalf("expected two results, got %#v", results)
	}
	codes := map[string]string{}
	for _, result := range results {
		codes[result.Name] = result.Code
	}
	if codes["a.txt"] != "not_found" || codes["b.txt"] != "deleted" {
		t.Fatalf("unexpected deletion results: %#v", results)
	}
}

func TestBatchDeleteRejectsMismatchedSelectionDirectory(t *testing.T) {
	root := useTestRoot(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	absolute, rel, info, err := ResolveExisting("a.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	results := BatchDelete(Selection{Directory: "other", Items: []SelectedItem{{Name: "a.txt", Relative: rel, Absolute: absolute, Info: info}}})
	if len(results) != 1 || results[0].Code != "source_changed" {
		t.Fatalf("expected source_changed result, got %#v", results)
	}
	if _, err := os.Lstat(absolute); err != nil {
		t.Fatalf("mismatched selection deleted the source: %v", err)
	}
}

func TestBatchOperationRejectsMismatchedSelectionDirectory(t *testing.T) {
	root := useTestRoot(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "target"), 0755); err != nil {
		t.Fatal(err)
	}
	absolute, rel, info, err := ResolveExisting("a.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	selection := Selection{Directory: "other", Items: []SelectedItem{{Name: "a.txt", Relative: rel, Absolute: absolute, Info: info}}}
	if _, err := BatchCopy(selection, "target"); ErrorCode(err) != "source_changed" {
		t.Fatalf("expected source_changed rejection, got %v", err)
	}
	if _, err := os.Lstat(absolute); err != nil {
		t.Fatalf("mismatched selection copied or removed the source: %v", err)
	}
}

func TestBatchMoveUsesSingleVolume(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("volume-name preflight is Windows-specific")
	}
	root := useTestRoot(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "target"), 0755); err != nil {
		t.Fatal(err)
	}
	list, err := ListDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	var version string
	for _, entry := range list.Entries {
		if entry.Name == "a.txt" {
			version = entry.Version
		}
	}
	selection, err := ValidateSelection("", map[string]string{"a.txt": version}, []ItemRequest{{Name: "a.txt", Version: version}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureSameVolume(selection, "target"); err != nil {
		t.Fatalf("same-volume destination rejected: %v", err)
	}
}

func TestBatchSelectionPreflightRejectsStaleOrDuplicateEntries(t *testing.T) {
	root := useTestRoot(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	list, err := ListDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	version := list.Entries[0].Version
	if _, err := ValidateSelection("", map[string]string{"a.txt": version}, []ItemRequest{{Name: "a.txt", Version: version}, {Name: "a.txt", Version: version}}); ErrorCode(err) != "invalid_selection" {
		t.Fatalf("expected duplicate rejection, got %v", err)
	}
	if _, err := ValidateSelection("", map[string]string{"a.txt": version}, []ItemRequest{{Name: "a.txt", Version: "old"}}); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("expected stale rejection, got %v", err)
	}
}
