package utils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/irains/fileharbor/conf"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

func withArchiveRoot(t *testing.T) string {
	t.Helper()
	previous := conf.FileHarbor
	root := t.TempDir()
	conf.FileHarbor = root
	t.Cleanup(func() {
		conf.FileHarbor = previous
		archiveBeforePublishHook = nil
	})
	return root
}

func writeZIPFixture(t *testing.T, filename string, entries []zipFixtureEntry) {
	t.Helper()
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		part, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

type zipFixtureEntry struct {
	name string
	data []byte
	mode os.FileMode
}

func tarFixture(t *testing.T, entries []tarFixtureEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: int64(entry.mode.Perm()), Size: int64(len(entry.data)), Typeflag: entry.kind}
		if entry.kind == tar.TypeDir {
			header.Size = 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if len(entry.data) > 0 {
			if _, err := writer.Write(entry.data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

type tarFixtureEntry struct {
	name string
	data []byte
	mode os.FileMode
	kind byte
}

func newZlibFixtureWriter(w io.Writer) (io.WriteCloser, error) {
	return zlib.NewWriter(w), nil
}

func writeCompressedFixture(t *testing.T, filename string, write func(io.Writer) (io.WriteCloser, error), data []byte) {
	t.Helper()
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := write(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClassifyArchiveLongestSuffixAndAliases(t *testing.T) {
	for index := 1; index < len(archiveSuffixes); index++ {
		if len(archiveSuffixes[index-1].suffix) < len(archiveSuffixes[index].suffix) {
			t.Fatalf("suffix registry is not longest-first at %q before %q", archiveSuffixes[index-1].suffix, archiveSuffixes[index].suffix)
		}
	}
	for _, candidate := range archiveSuffixes {
		format, suffix, ok := ClassifyArchive("fixture" + strings.ToUpper(candidate.suffix))
		if !ok || format != candidate.format || !strings.EqualFold(suffix, candidate.suffix) {
			t.Errorf("registry alias %q = %q, %q, %v", candidate.suffix, format, suffix, ok)
		}
	}
	for name, want := range map[string]ArchiveFormat{
		"DATA.TAR.GZ":     ArchiveTARGzip,
		"data.tgz":        ArchiveTARGzip,
		"data.tar.bzip2":  ArchiveTARBzip2,
		"data.tbz":        ArchiveTARBzip2,
		"data.tbz2":       ArchiveTARBzip2,
		"data.tar.xz":     ArchiveTARXZ,
		"data.txz":        ArchiveTARXZ,
		"data.tar.zstd":   ArchiveTARZstd,
		"data.tzst":       ArchiveTARZstd,
		"data.tar.snappy": ArchiveTARSnappy,
		"data.tar.zlib":   ArchiveTARZlib,
		"data.GZIP":       ArchiveGzip,
		"data.SZ":         ArchiveSnappy,
		"data.rar":        ArchiveRAR,
	} {
		format, _, ok := ClassifyArchive(name)
		if !ok || format != want {
			t.Errorf("ClassifyArchive(%q) = %q, %v; want %q", name, format, ok, want)
		}
		if !IsArchive(name) {
			t.Errorf("IsArchive(%q) = false", name)
		}
	}
	if IsArchive("report.zip.txt") {
		t.Fatal("ordinary file classified as archive")
	}
}

func TestArchiveExtractsZIPWithPreamble(t *testing.T) {
	root := withArchiveRoot(t)
	zipPath := filepath.Join(root, "plain.zip")
	writeZIPFixture(t, zipPath, []zipFixtureEntry{{name: "inside.txt", data: []byte("safe"), mode: 0644}})
	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "prefixed.zip"), append([]byte("#!/bin/sh\necho launcher\n"), zipData...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractArchive("prefixed.zip"); err != nil {
		t.Fatalf("ExtractArchive(prefixed.zip): %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "inside.txt"))
	if err != nil || string(data) != "safe" {
		t.Fatalf("prefixed ZIP output = %q, %v", data, err)
	}
}

func TestStandaloneFormatsExtractAndDoNotOverwrite(t *testing.T) {
	root := withArchiveRoot(t)
	payload := []byte("hello archive")
	bzipData, err := base64.StdEncoding.DecodeString("QlpoOTFBWSZTWbcz9B0AAAKRgEAAKmSRACAAMQAwINBiGkySA4T8XckU4UJC3M/QdA==")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bzip.txt.bz2"), bzipData, 0600); err != nil {
		t.Fatal(err)
	}

	fixtures := map[string]func(io.Writer) (io.WriteCloser, error){
		"gzip.txt.gz": func(w io.Writer) (io.WriteCloser, error) { return gzip.NewWriter(w), nil },
		"zlib.txt.zz": newZlibFixtureWriter,
		"zstd.txt.zst": func(w io.Writer) (io.WriteCloser, error) {
			return zstd.NewWriter(w, zstd.WithEncoderConcurrency(1), zstd.WithWindowSize(1<<20))
		},
		"lz4.txt.lz4": func(w io.Writer) (io.WriteCloser, error) {
			writer := lz4.NewWriter(w)
			return writer, writer.Apply(lz4.ConcurrencyOption(1))
		},
		"brotli.txt.br": func(w io.Writer) (io.WriteCloser, error) { return brotli.NewWriter(w), nil },
		"snappy.txt.sz": func(w io.Writer) (io.WriteCloser, error) {
			return s2.NewWriter(w, s2.WriterSnappyCompat(), s2.WriterConcurrency(1)), nil
		},
	}
	for name, newWriter := range fixtures {
		writeCompressedFixture(t, filepath.Join(root, name), newWriter, payload)
	}

	for _, name := range append([]string{"bzip.txt.bz2"}, mapKeys(fixtures)...) {
		if _, err := ExtractArchive(name); err != nil {
			t.Fatalf("ExtractArchive(%q): %v", name, err)
		}
		output := filepath.Join(root, strings.Split(name, ".")[0]+".txt")
		data, err := os.ReadFile(output)
		if err != nil || !bytes.Equal(data, payload) {
			t.Fatalf("%q output = %q, %v", name, data, err)
		}
		if _, err := ExtractArchive(name); ErrorCode(err) != "destination_exists" {
			t.Fatalf("second ExtractArchive(%q) = %v", name, err)
		}
	}
}

func mapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func TestTarFormatsExtract(t *testing.T) {
	root := withArchiveRoot(t)
	contents := tarFixture(t, []tarFixtureEntry{
		{name: "folder/", mode: 03777, kind: tar.TypeDir},
		{name: "folder/file.txt", data: []byte("tar payload"), mode: 06755, kind: tar.TypeReg},
	})
	bzipData, err := base64.StdEncoding.DecodeString("QlpoOTFBWSZTWQS9eBUAAI77gMqAAgBAAf+AICBnJN5gCAggAHQSSm1TT1Gmg0eUxHqG9UEko0DQAAAD7qaudOSAyjSQh1nJBe8KLmHkohDAMpNGOrgnGBFKyEeHMuVHacubXs1ZyZHuitrZVM9ItwPkTc7cVmbEQYbQZT7iQfxdyRThQkAS9eBU")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sample.tar.bz2"), bzipData, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sample.tar"), contents, 0600); err != nil {
		t.Fatal(err)
	}
	writers := map[string]func(io.Writer) (io.WriteCloser, error){
		"sample.tar.gz": func(w io.Writer) (io.WriteCloser, error) { return gzip.NewWriter(w), nil },
		"sample.tar.zz": newZlibFixtureWriter,
		"sample.tar.zst": func(w io.Writer) (io.WriteCloser, error) {
			return zstd.NewWriter(w, zstd.WithEncoderConcurrency(1), zstd.WithWindowSize(1<<20))
		},
		"sample.tar.lz4": func(w io.Writer) (io.WriteCloser, error) {
			writer := lz4.NewWriter(w)
			return writer, writer.Apply(lz4.ConcurrencyOption(1))
		},
		"sample.tar.br": func(w io.Writer) (io.WriteCloser, error) { return brotli.NewWriter(w), nil },
		"sample.tar.sz": func(w io.Writer) (io.WriteCloser, error) {
			return s2.NewWriter(w, s2.WriterSnappyCompat(), s2.WriterConcurrency(1)), nil
		},
	}
	fixtureNames := append([]string{"sample.tar", "sample.tar.bz2"}, mapKeys(writers)...)
	for name, newWriter := range writers {
		writeCompressedFixture(t, filepath.Join(root, name), newWriter, contents)
	}
	for _, name := range fixtureNames {
		t.Run(name, func(t *testing.T) {
			if _, err := ExtractArchive(name); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(root, "folder", "file.txt"))
			if err != nil || string(data) != "tar payload" {
				t.Fatalf("output = %q, %v", data, err)
			}
			mode, err := os.Stat(filepath.Join(root, "folder", "file.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if mode.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
				t.Fatalf("unsafe mode retained: %v", mode.Mode())
			}
			if err := os.RemoveAll(filepath.Join(root, "folder")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestArchiveRejectsUnsafeEntriesAndCollisions(t *testing.T) {
	for name, entries := range map[string][]zipFixtureEntry{
		"traversal": {{name: "../escape", data: []byte("x"), mode: 0644}},
		"link":      {{name: "link", data: []byte("target"), mode: os.ModeSymlink | 0777}},
		"duplicate": {{name: "file", data: []byte("a"), mode: 0644}, {name: "file", data: []byte("b"), mode: 0644}},
		"case_fold": {{name: "File", data: []byte("a"), mode: 0644}, {name: "file", data: []byte("b"), mode: 0644}},
		"file_dir":  {{name: "item", data: []byte("a"), mode: 0644}, {name: "item/child", data: []byte("b"), mode: 0644}},
		"reserved":  {{name: ".fileharbor-extract-hidden", data: []byte("a"), mode: 0644}},
	} {
		t.Run(name, func(t *testing.T) {
			root := withArchiveRoot(t)
			writeZIPFixture(t, filepath.Join(root, "unsafe.zip"), entries)
			if _, err := ExtractArchive("unsafe.zip"); ErrorCode(err) != "archive_unsafe_entry" {
				t.Fatalf("error = %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "escape")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("partial output published: %v", err)
			}
		})
	}
}

func TestArchiveRejectsEmptyCorruptMagicEncryptedAndCancelled(t *testing.T) {
	root := withArchiveRoot(t)
	for name, data := range map[string][]byte{
		"empty.zip":    nil,
		"wrong.zip":    []byte("not a zip"),
		"wrong.tar.gz": []byte("not gzip"),
		"wrong.rar":    []byte("not rar"),
	} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ExtractArchive(name); ErrorCode(err) != "corrupt_archive" {
			t.Errorf("ExtractArchive(%q) = %v", name, err)
		}
	}
	writeZIPFixture(t, filepath.Join(root, "cancel.zip"), []zipFixtureEntry{{name: "created", data: []byte("x"), mode: 0644}})

	encryptedPath := filepath.Join(root, "encrypted.zip")
	writeZIPFixture(t, encryptedPath, []zipFixtureEntry{{name: "secret", data: []byte("x"), mode: 0644}})
	encryptedData, err := os.ReadFile(encryptedPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(encryptedData) < 8 {
		t.Fatal("unexpectedly short ZIP fixture")
	}
	encryptedData[6] |= 1
	centralOffset := bytes.Index(encryptedData, []byte("PK\x01\x02"))
	if centralOffset < 0 || centralOffset+10 > len(encryptedData) {
		t.Fatal("missing ZIP central directory")
	}
	encryptedData[centralOffset+8] |= 1
	if err := os.WriteFile(encryptedPath, encryptedData, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractArchive("encrypted.zip"); ErrorCode(err) != "encrypted_archive" {
		t.Fatalf("encrypted error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ExtractArchiveContext(ctx, "cancel.zip"); ErrorCode(err) != "request_cancelled" {
		t.Fatalf("cancel error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "created")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled output published: %v", err)
	}
}

type cancelAfterReadsContext struct {
	context.Context
	remaining atomic.Int32
	cancel    context.CancelFunc
	cancelled atomic.Bool
}

func (ctx *cancelAfterReadsContext) Done() <-chan struct{} {
	if ctx.remaining.Add(-1) == 0 {
		ctx.cancelled.Store(true)
		ctx.cancel()
	}
	return ctx.Context.Done()
}

func TestArchiveContextCancelsDuringCopy(t *testing.T) {
	root := withArchiveRoot(t)
	payload := bytes.Repeat([]byte("cancel-me"), 1<<18)
	writeCompressedFixture(t, filepath.Join(root, "large.txt.gz"), func(w io.Writer) (io.WriteCloser, error) {
		return gzip.NewWriter(w), nil
	}, payload)
	base, cancel := context.WithCancel(context.Background())
	ctx := &cancelAfterReadsContext{Context: base, cancel: cancel}
	ctx.remaining.Store(40)
	defer cancel()
	if _, err := ExtractArchiveContext(ctx, "large.txt.gz"); ErrorCode(err) != "request_cancelled" {
		t.Fatalf("mid-copy cancel error = %v", err)
	}
	if !ctx.cancelled.Load() || ctx.remaining.Load() > 0 {
		t.Fatalf("context did not reach the configured mid-operation cancellation point: remaining=%d", ctx.remaining.Load())
	}
	if _, err := os.Stat(filepath.Join(root, "large.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled output published: %v", err)
	}
}

func TestArchiveDetectsSourceChangeBeforePublish(t *testing.T) {
	root := withArchiveRoot(t)
	archivePath := filepath.Join(root, "change.zip")
	writeZIPFixture(t, archivePath, []zipFixtureEntry{{name: "created", data: []byte("x"), mode: 0644}})
	originalInfo, err := os.Stat(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	archiveBeforePublishHook = func() {
		data, readErr := os.ReadFile(archivePath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		data[len(data)/2] ^= 1
		if writeErr := os.WriteFile(archivePath, data, 0600); writeErr != nil {
			t.Fatal(writeErr)
		}
		if chtimesErr := os.Chtimes(archivePath, time.Now(), originalInfo.ModTime()); chtimesErr != nil {
			t.Fatal(chtimesErr)
		}
	}
	if _, err := ExtractArchive("change.zip"); ErrorCode(err) != "source_changed" {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "created")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("changed source output published: %v", err)
	}
}

func TestArchiveListingUsesRegistryAndHidesInternalStages(t *testing.T) {
	root := withArchiveRoot(t)
	for _, name := range []string{"A.TAR.XZ", "data.rar", "notes.txt", ".fileharbor-zip-secret", ".FILEHARBOR-EXTRACT-secret"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	listing, err := ListDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, entry := range listing.Entries {
		seen[entry.Name] = entry.IsArchive
	}
	if !seen["A.TAR.XZ"] || !seen["data.rar"] || seen["notes.txt"] {
		t.Fatalf("archive classification = %#v", seen)
	}
	if _, found := seen[".fileharbor-zip-secret"]; found {
		t.Fatal("ZIP stage was listed")
	}
	if _, found := seen[".FILEHARBOR-EXTRACT-secret"]; found {
		t.Fatal("extraction stage was listed")
	}
}

func TestArchiveDirectoryMetadataDoesNotConsumeOutputQuota(t *testing.T) {
	root := withArchiveRoot(t)
	archivePath := filepath.Join(root, "directory-metadata.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	directory := &zip.FileHeader{Name: "metadata/", Method: zip.Store, UncompressedSize64: uint64(maxArchiveBytes) + 1, CompressedSize64: uint64(maxArchiveBytes) + 1}
	directory.SetMode(os.ModeDir | 0755)
	if _, err := writer.CreateRaw(directory); err != nil {
		t.Fatal(err)
	}
	part, err := writer.Create("metadata/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractArchive("directory-metadata.zip"); err != nil {
		t.Fatalf("directory metadata extraction failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "metadata", "file.txt"))
	if err != nil || string(data) != "ok" {
		t.Fatalf("metadata output = %q, %v", data, err)
	}
}

func TestArchiveEntryLimitPreflight(t *testing.T) {
	root := withArchiveRoot(t)
	archivePath := filepath.Join(root, "entries.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for index := 0; index <= maxArchiveEntries; index++ {
		header := &zip.FileHeader{Name: fmt.Sprintf("entry-%05d", index), Method: zip.Store}
		if _, err := writer.CreateHeader(header); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractArchive("entries.zip"); ErrorCode(err) != "archive_limit_exceeded" {
		t.Fatalf("entry limit error = %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "entries.zip" {
		t.Fatalf("entry-limit extraction left output: %#v", entries)
	}
}

func TestArchivePromotionRaceDoesNotOverwrite(t *testing.T) {
	root := withArchiveRoot(t)
	writeZIPFixture(t, filepath.Join(root, "race.zip"), []zipFixtureEntry{{name: "target", data: []byte("archive"), mode: 0644}})
	archiveBeforePublishHook = func() {
		if err := os.WriteFile(filepath.Join(root, "target"), []byte("existing"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ExtractArchive("race.zip"); ErrorCode(err) != "destination_exists" {
		t.Fatalf("promotion race error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "target"))
	if err != nil || string(data) != "existing" {
		t.Fatalf("race target = %q, %v", data, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(strings.ToLower(entry.Name()), InternalArchiveExtractPrefix) {
			t.Fatalf("staging directory was not removed: %q", entry.Name())
		}
	}
}

func TestArchiveLimitAndUnsupportedCompression(t *testing.T) {
	root := withArchiveRoot(t)
	archivePath := filepath.Join(root, "limit.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	writer.RegisterCompressor(zip.Deflate, func(output io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(output, flate.BestSpeed)
	})
	header := &zip.FileHeader{Name: "huge", Method: zip.Store, UncompressedSize64: uint64(maxArchiveBytes) + 1}
	if _, err := writer.CreateRaw(header); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractArchive("limit.zip"); ErrorCode(err) != "archive_limit_exceeded" {
		t.Fatalf("limit error = %v", err)
	}
}
