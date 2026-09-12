package utils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
		header := &tar.Header{Name: entry.name, Mode: int64(entry.mode.Perm()), Size: int64(len(entry.data)), Typeflag: entry.kind, Format: entry.format}
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
	name   string
	data   []byte
	mode   os.FileMode
	kind   byte
	format tar.Format
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
		"dot_prefix": {{name: "./file", data: []byte("x"), mode: 0644}},
		"traversal":  {{name: "../escape", data: []byte("x"), mode: 0644}},
		"link":       {{name: "link", data: []byte("target"), mode: os.ModeSymlink | 0777}},
		"duplicate":  {{name: "file", data: []byte("a"), mode: 0644}, {name: "file", data: []byte("b"), mode: 0644}},
		"case_fold":  {{name: "File", data: []byte("a"), mode: 0644}, {name: "file", data: []byte("b"), mode: 0644}},
		"file_dir":   {{name: "item", data: []byte("a"), mode: 0644}, {name: "item/child", data: []byte("b"), mode: 0644}},
		"reserved":   {{name: ".fileharbor-extract-hidden", data: []byte("a"), mode: 0644}},
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

func TestTarRelativePathsExtract(t *testing.T) {
	for _, format := range []tar.Format{tar.FormatGNU, tar.FormatUSTAR, tar.FormatPAX} {
		for _, compressed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/gzip=%v", format, compressed), func(t *testing.T) {
				root := withArchiveRoot(t)
				longName := strings.Repeat("n", 120)
				if format == tar.FormatUSTAR {
					longName = strings.Repeat("n", 80) + "/" + strings.Repeat("m", 80)
				}
				entries := []tarFixtureEntry{
					{name: ".", mode: 0000, kind: tar.TypeDir},
					{name: "./folder/", mode: 0755, kind: tar.TypeDir},
					{name: "./folder/file.txt", data: []byte("tar payload"), mode: 0644, kind: tar.TypeReg},
					{name: "././late/" + longName, data: []byte("long-name payload"), mode: 0644, kind: tar.TypeReg},
					{name: "./late/", mode: 0755, kind: tar.TypeDir},
					{name: "./", mode: 0777, kind: tar.TypeDir},
				}
				for index := range entries {
					entries[index].format = format
				}
				contents := tarFixture(t, entries)
				if format == tar.FormatGNU && !bytes.Contains(contents, []byte("././@LongLink")) {
					t.Fatal("fixture lacks a GNU long-name record")
				}
				if format == tar.FormatPAX && !bytes.Contains(contents, []byte("path=././late/")) {
					t.Fatal("fixture lacks a PAX path record")
				}
				name := "sample.tar"
				if compressed {
					name += ".gz"
					writeCompressedFixture(t, filepath.Join(root, name), func(w io.Writer) (io.WriteCloser, error) { return gzip.NewWriter(w), nil }, contents)
				} else if err := os.WriteFile(filepath.Join(root, name), contents, 0600); err != nil {
					t.Fatal(err)
				}
				before, err := os.Stat(root)
				if err != nil {
					t.Fatal(err)
				}
				if got, err := ExtractArchive(name); err != nil || got != name {
					t.Fatalf("ExtractArchive = %q, %v", got, err)
				}
				for name, want := range map[string]string{"folder/file.txt": "tar payload", "late/" + longName: "long-name payload"} {
					if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name))); err != nil || string(data) != want {
						t.Fatalf("output %q = %q, %v", name, data, err)
					}
				}
				after, err := os.Stat(root)
				if err != nil || before.Mode() != after.Mode() {
					t.Fatalf("output root mode changed: %v", err)
				}
			})
		}
	}
}

func TestTarGzipSuffixAcceptsRawTAR(t *testing.T) {
	for _, suffix := range []string{".tar.gz", ".tgz", ".tar.gzip", ".TaR.Gz", ".TGZ", ".TaR.GZip"} {
		for _, compressed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/gzip=%v", suffix, compressed), func(t *testing.T) {
				root := withArchiveRoot(t)
				name := "sample" + suffix
				contents := tarFixture(t, []tarFixtureEntry{{name: "file.txt", data: []byte("payload"), mode: 0644, kind: tar.TypeReg}})
				if compressed {
					writeCompressedFixture(t, filepath.Join(root, name), func(w io.Writer) (io.WriteCloser, error) { return gzip.NewWriter(w), nil }, contents)
				} else if err := os.WriteFile(filepath.Join(root, name), contents, 0600); err != nil {
					t.Fatal(err)
				}
				if got, err := ExtractArchive(name); err != nil || got != name {
					t.Fatalf("ExtractArchive = %q, %v", got, err)
				}
				if data, err := os.ReadFile(filepath.Join(root, "file.txt")); err != nil || string(data) != "payload" {
					t.Fatalf("output = %q, %v", data, err)
				}
			})
		}
	}
}

func assertArchiveRootEntries(t *testing.T, root string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	if len(entries) != len(wanted) {
		t.Fatalf("unexpected output or staging: got %d entries, want %d", len(entries), len(wanted))
	}
	for _, entry := range entries {
		if !wanted[entry.Name()] {
			t.Fatalf("unexpected output or staging: %q", entry.Name())
		}
	}
}

func assertTarRejected(t *testing.T, name string, contents []byte, code string) {
	t.Helper()
	root := withArchiveRoot(t)
	archivePath := filepath.Join(root, name)
	if err := os.WriteFile(archivePath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractArchive(name); ErrorCode(err) != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
	assertArchiveRootEntries(t, root, name)
	after, err := os.ReadFile(archivePath)
	if err != nil || sha256.Sum256(after) != sha256.Sum256(contents) {
		t.Fatalf("source archive changed: %v", err)
	}
}

func TestTarRejectsUnsafePathsAndTypes(t *testing.T) {
	for index, name := range []string{
		"./../escape", "/absolute", ".//absolute", "./dir/../escape", "./dir/./file",
		"./dir//file", "./C:/drive", `./C:\drive`, `./\server\share`, `./dir\..\escape`,
		"./CON", "./file:stream", "./.fileharbor-extract-hidden", "./file.", "./file ",
		strings.Repeat("./", 2048) + "file", "./" + strings.Repeat("x", 256),
	} {
		t.Run(fmt.Sprintf("path-%d", index), func(t *testing.T) {
			contents := tarFixture(t, []tarFixtureEntry{
				{name: "./safe", data: []byte("staged only"), mode: 0644, kind: tar.TypeReg},
				{name: name, data: []byte("unsafe"), mode: 0644, kind: tar.TypeReg},
			})
			assertTarRejected(t, "unsafe.tar", contents, "archive_unsafe_entry")
		})
	}
	for _, kind := range []byte{tar.TypeSymlink, tar.TypeLink, tar.TypeFifo, tar.TypeChar, tar.TypeBlock} {
		t.Run(fmt.Sprintf("type-%c", kind), func(t *testing.T) {
			contents := tarFixture(t, []tarFixtureEntry{
				{name: "./safe", data: []byte("staged only"), mode: 0644, kind: tar.TypeReg},
				{name: "./unsafe", mode: 0644, kind: kind},
			})
			assertTarRejected(t, "unsafe.tar.gz", contents, "archive_unsafe_entry")
		})
	}
	for _, name := range []string{"", ".", "./", "./.", "././"} {
		for _, kind := range []byte{tar.TypeReg, tar.TypeDir, tar.TypeSymlink, tar.TypeLink} {
			if kind == tar.TypeDir && (name == "." || name == "./") {
				continue
			}
			t.Run(fmt.Sprintf("root-%q-type-%c", name, kind), func(t *testing.T) {
				// Patch a synthetic header to exercise names the TAR writer refuses.
				contents := tarFixture(t, []tarFixtureEntry{{name: "placeholder", mode: 0644, kind: kind}})
				clear(contents[:100])
				copy(contents[:100], name)
				for i := 148; i < 156; i++ {
					contents[i] = ' '
				}
				var checksum int
				for _, value := range contents[:512] {
					checksum += int(value)
				}
				copy(contents[148:156], fmt.Sprintf("%06o\x00 ", checksum))
				assertTarRejected(t, "root.tar", contents, "archive_unsafe_entry")
			})
		}
	}
}

func TestTarRejectsNormalizedCollisions(t *testing.T) {
	file := func(name string) tarFixtureEntry {
		return tarFixtureEntry{name: name, data: []byte("file"), mode: 0644, kind: tar.TypeReg}
	}
	dir := func(name string) tarFixtureEntry { return tarFixtureEntry{name: name, mode: 0755, kind: tar.TypeDir} }
	for name, entries := range map[string][]tarFixtureEntry{
		"same_file":      {file("file"), file("./file")},
		"case_fold":      {file("./File"), file("./file")},
		"same_dir":       {dir("dir/"), dir("./dir/")},
		"file_parent":    {file("./item"), file("./item/child")},
		"replace_parent": {file("./item/child"), file("./item")},
		"file_to_dir":    {file("./item"), dir("./item/")},
		"dir_to_file":    {dir("./item/"), file("./item")},
	} {
		t.Run(name, func(t *testing.T) {
			assertTarRejected(t, "collision.tar.gz", tarFixture(t, entries), "archive_unsafe_entry")
		})
	}
}

func TestTarRootMetadataQuotaAndEmpty(t *testing.T) {
	for _, rootName := range []string{".", "./"} {
		t.Run("empty-"+rootName, func(t *testing.T) {
			assertTarRejected(t, "empty.tar", tarFixture(t, []tarFixtureEntry{{name: rootName, kind: tar.TypeDir}}), "corrupt_archive")
		})
	}
	for _, count := range []int{maxArchiveEntries - 1, maxArchiveEntries} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			entries := []tarFixtureEntry{{name: "./file", data: []byte("ok"), mode: 0644, kind: tar.TypeReg}}
			for index := 0; index < count; index++ {
				entries = append(entries, tarFixtureEntry{name: "./", kind: tar.TypeDir})
			}
			contents := tarFixture(t, entries)
			if count == maxArchiveEntries {
				assertTarRejected(t, "limit.tar", contents, "archive_limit_exceeded")
				return
			}
			root := withArchiveRoot(t)
			if err := os.WriteFile(filepath.Join(root, "limit.tar"), contents, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ExtractArchive("limit.tar"); err != nil {
				t.Fatal(err)
			}
			assertArchiveRootEntries(t, root, "limit.tar", "file")
		})
	}
}

func TestTarRootMetadataDoesNotChangeStageMode(t *testing.T) {
	stage, output := t.TempDir(), t.TempDir()
	if err := os.Chmod(stage, 0700); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(stage)
	if err != nil {
		t.Fatal(err)
	}
	contents := tarFixture(t, []tarFixtureEntry{
		{name: "./", mode: 0777, kind: tar.TypeDir},
		{name: "./file", data: []byte("ok"), mode: 0644, kind: tar.TypeReg},
		{name: ".", mode: 0000, kind: tar.TypeDir},
	})
	if _, err := extractTARContext(context.Background(), bytes.NewReader(contents), ArchiveTAR, stage, output); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(stage)
	if err != nil || before.Mode() != after.Mode() {
		t.Fatalf("staging root mode changed: %v", err)
	}
	if runtime.GOOS != "windows" && after.Mode().Perm() != 0700 {
		t.Fatalf("staging root permissions = %v", after.Mode())
	}
}

func TestTarCorruptionAndFormatBoundaries(t *testing.T) {
	contents := tarFixture(t, []tarFixtureEntry{
		{name: "./safe", data: []byte("safe"), mode: 0644, kind: tar.TypeReg},
		{name: "./second", data: []byte("second payload"), mode: 0644, kind: tar.TypeReg},
		{name: "./", kind: tar.TypeDir},
	})
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	badGzip := bytes.Clone(buffer.Bytes())
	badGzip[len(badGzip)-8] ^= 1
	badTar := bytes.Clone(contents)
	badTar[0] ^= 1
	for name, data := range map[string][]byte{
		"checksum.tar":    badTar,
		"checksum.tar.gz": badTar,
		"short.tar.gz":    contents[:511],
		"empty.tar.gz":    nil,
		"zero.tar.gz":     make([]byte, 1024),
		"payload.tar":     contents[:1539],
		"payload.tar.gz":  contents[:1539],
		"header.tar":      contents[:1224],
		"header.tar.gz":   contents[:1224],
		"trailer.tar.gz":  badGzip,
		"truncated.tgz":   buffer.Bytes()[:buffer.Len()-1],
		"standalone.gz":   contents,
		"standalone.gzip": contents,
		"other.tar.bz2":   contents,
		"other.tar.xz":    contents,
	} {
		t.Run(name, func(t *testing.T) { assertTarRejected(t, name, data, "corrupt_archive") })
	}
}

func TestTarPublicationGuards(t *testing.T) {
	for _, scenario := range []string{"existing", "race", "source", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			root := withArchiveRoot(t)
			contents := tarFixture(t, []tarFixtureEntry{{name: "./target", data: []byte("archive"), mode: 0644, kind: tar.TypeReg}})
			archivePath := filepath.Join(root, "sample.tar.gz")
			if err := os.WriteFile(archivePath, contents, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := "destination_exists"
			createTarget := func() {
				if err := os.WriteFile(filepath.Join(root, "target"), []byte("existing"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "existing":
				createTarget()
			case "race":
				archiveBeforePublishHook = createTarget
			case "source":
				want = "source_changed"
				before, err := os.Stat(archivePath)
				if err != nil {
					t.Fatal(err)
				}
				archiveBeforePublishHook = func() {
					changed := bytes.Clone(contents)
					changed[512] ^= 1
					if err := os.WriteFile(archivePath, changed, 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.Chtimes(archivePath, before.ModTime(), before.ModTime()); err != nil {
						t.Fatal(err)
					}
				}
			case "cancel":
				want = "request_cancelled"
				archiveBeforePublishHook = cancel
			}
			if _, err := ExtractArchiveContext(ctx, "sample.tar.gz"); ErrorCode(err) != want {
				t.Fatalf("error = %v, want %s", err, want)
			}
			if want == "destination_exists" {
				assertArchiveRootEntries(t, root, "sample.tar.gz", "target")
				if data, err := os.ReadFile(filepath.Join(root, "target")); err != nil || string(data) != "existing" {
					t.Fatalf("existing target changed: %v", err)
				}
			} else {
				assertArchiveRootEntries(t, root, "sample.tar.gz")
			}
			if scenario != "source" {
				after, err := os.ReadFile(archivePath)
				if err != nil || sha256.Sum256(after) != sha256.Sum256(contents) {
					t.Fatalf("source changed: %v", err)
				}
			}
		})
	}
}

func TestTarCancellationDuringCopy(t *testing.T) {
	root := withArchiveRoot(t)
	contents := tarFixture(t, []tarFixtureEntry{
		{name: "./", kind: tar.TypeDir},
		{name: "./large", data: bytes.Repeat([]byte("cancel-me"), 1<<18), mode: 0644, kind: tar.TypeReg},
	})
	archivePath := filepath.Join(root, "large.tar.gz")
	writeCompressedFixture(t, archivePath, func(w io.Writer) (io.WriteCloser, error) { return gzip.NewWriter(w), nil }, contents)
	before, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	base, cancel := context.WithCancel(context.Background())
	ctx := &cancelAfterReadsContext{Context: base, cancel: cancel}
	ctx.remaining.Store(40)
	defer cancel()
	if _, err := ExtractArchiveContext(ctx, "large.tar.gz"); ErrorCode(err) != "request_cancelled" {
		t.Fatalf("mid-copy cancel = %v", err)
	}
	if !ctx.cancelled.Load() {
		t.Fatal("did not reach cancellation point")
	}
	assertArchiveRootEntries(t, root, "large.tar.gz")
	after, err := os.ReadFile(archivePath)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatalf("source changed: %v", err)
	}
}

func TestExtractionFormatGzipSignatureAndIOError(t *testing.T) {
	root := withArchiveRoot(t)
	contents := tarFixture(t, []tarFixtureEntry{{name: "file", mode: 0644, kind: tar.TypeReg}})
	// Both routing signatures match. Gzip must take precedence, even if invalid.
	contents[0], contents[1] = 0x1f, 0x8b
	for index := 148; index < 156; index++ {
		contents[index] = ' '
	}
	var checksum int
	for _, value := range contents[:512] {
		checksum += int(value)
	}
	copy(contents[148:156], fmt.Sprintf("%06o\x00 ", checksum))
	if !validTarHeader(contents[:512]) {
		t.Fatal("fixture must have a valid TAR checksum")
	}
	archivePath := filepath.Join(root, "signature.tar.gz")
	if err := os.WriteFile(archivePath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	format, err := resolveExtractionFormat(source, ArchiveTARGzip, int64(len(contents)))
	if err != nil || format != ArchiveTARGzip {
		t.Fatalf("format = %q, %v", format, err)
	}
	if _, err := ExtractArchive("signature.tar.gz"); ErrorCode(err) != "corrupt_archive" {
		t.Fatalf("invalid gzip = %v", err)
	}
	assertArchiveRootEntries(t, root, "signature.tar.gz")
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveExtractionFormat(source, ArchiveTARGzip, int64(len(contents))); ErrorCode(err) != "io_error" {
		t.Fatalf("closed source error = %v", err)
	}
}
