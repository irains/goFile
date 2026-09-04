package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTextContent(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
		want bool
	}{
		{name: "empty", data: nil, want: true},
		{name: "unicode text", data: []byte("notes\n文件\tready"), want: true},
		{name: "invalid utf8", data: []byte{0xff, 0xfe}, want: false},
		{name: "nul", data: []byte("note\x00"), want: false},
		{name: "escape", data: []byte("\x1b[31m"), want: false},
		{name: "delete", data: []byte("note\x7f"), want: false},
		{name: "c1 control", data: []byte("note"), want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := IsTextContent(test.data); got != test.want {
				t.Fatalf("IsTextContent(%q) = %t, want %t", test.data, got, test.want)
			}
		})
	}
}

func TestTextEditorFilesAreContentBased(t *testing.T) {
	root := useTestRoot(t)
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("plain text"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "binary.txt"), []byte("not text\x00"), 0644); err != nil {
		t.Fatal(err)
	}

	if !IsTextFile("README", 1024) {
		t.Fatal("extensionless text file must be editable")
	}
	if IsTextFile("binary.txt", 1024) {
		t.Fatal("binary content must not become editable because of its extension")
	}
}

func TestWriteVersionedTextFileRejectsNonText(t *testing.T) {
	root := useTestRoot(t)
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := ReadTextFile("notes.txt", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteVersionedTextFile("notes.txt", EntryVersion(file.Info), []byte("after\x00"), 1024); ErrorCode(err) != "invalid_text_content" {
		t.Fatalf("non-text save error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "before" {
		t.Fatalf("rejected text save modified target: %q, %v", data, err)
	}
}
