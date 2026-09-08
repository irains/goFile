package utils

import "strings"

// ArchiveFormat identifies one supported archive or compression stream format.
// Values are stable for callers that need to classify directory listings.
type ArchiveFormat string

const (
	ArchiveZIP       ArchiveFormat = "zip"
	ArchiveTAR       ArchiveFormat = "tar"
	ArchiveTARGzip   ArchiveFormat = "tar_gzip"
	ArchiveTARBzip2  ArchiveFormat = "tar_bzip2"
	ArchiveTARXZ     ArchiveFormat = "tar_xz"
	ArchiveTARZstd   ArchiveFormat = "tar_zstd"
	ArchiveTARLZ4    ArchiveFormat = "tar_lz4"
	ArchiveTARBrotli ArchiveFormat = "tar_brotli"
	ArchiveTARSnappy ArchiveFormat = "tar_snappy"
	ArchiveTARZlib   ArchiveFormat = "tar_zlib"
	ArchiveRAR       ArchiveFormat = "rar"
	ArchiveGzip      ArchiveFormat = "gzip"
	ArchiveBzip2     ArchiveFormat = "bzip2"
	ArchiveXZ        ArchiveFormat = "xz"
	ArchiveZstd      ArchiveFormat = "zstd"
	ArchiveLZ4       ArchiveFormat = "lz4"
	ArchiveBrotli    ArchiveFormat = "brotli"
	ArchiveSnappy    ArchiveFormat = "snappy"
	ArchiveZlib      ArchiveFormat = "zlib"
)

type archiveSuffix struct {
	suffix string
	format ArchiveFormat
}

// Longest suffixes come first so compound TAR names cannot be mistaken for
// standalone compressed streams. Matching is ASCII case-insensitive.
var archiveSuffixes = []archiveSuffix{
	{".tar.snappy", ArchiveTARSnappy},
	{".tar.bzip2", ArchiveTARBzip2},
	{".tar.zstd", ArchiveTARZstd},
	{".tar.zlib", ArchiveTARZlib},
	{".tar.gzip", ArchiveTARGzip},
	{".tar.bz2", ArchiveTARBzip2},
	{".tar.lz4", ArchiveTARLZ4},
	{".tar.zst", ArchiveTARZstd},
	{".tar.xz", ArchiveTARXZ},
	{".tar.br", ArchiveTARBrotli},
	{".tar.gz", ArchiveTARGzip},
	{".tar.sz", ArchiveTARSnappy},
	{".tar.zz", ArchiveTARZlib},
	{".snappy", ArchiveSnappy},
	{".bzip2", ArchiveBzip2},
	{".gzip", ArchiveGzip},
	{".zstd", ArchiveZstd},
	{".zlib", ArchiveZlib},
	{".tbz2", ArchiveTARBzip2},
	{".tzst", ArchiveTARZstd},
	{".tgz", ArchiveTARGzip},
	{".tbz", ArchiveTARBzip2},
	{".txz", ArchiveTARXZ},
	{".rar", ArchiveRAR},
	{".zip", ArchiveZIP},
	{".tar", ArchiveTAR},
	{".bz2", ArchiveBzip2},
	{".zst", ArchiveZstd},
	{".lz4", ArchiveLZ4},
	{".xz", ArchiveXZ},
	{".br", ArchiveBrotli},
	{".sz", ArchiveSnappy},
	{".zz", ArchiveZlib},
	{".gz", ArchiveGzip},
}

// ClassifyArchive applies the central longest-suffix registry used by both
// extraction and directory listings. The returned suffix is the exact number
// of bytes to remove when naming a standalone decompressed output.
func ClassifyArchive(name string) (format ArchiveFormat, suffix string, ok bool) {
	lower := strings.ToLower(name)
	for _, candidate := range archiveSuffixes {
		if strings.HasSuffix(lower, candidate.suffix) {
			return candidate.format, name[len(name)-len(candidate.suffix):], true
		}
	}
	return "", "", false
}

func IsArchive(name string) bool {
	_, _, ok := ClassifyArchive(name)
	return ok
}

func isTarArchive(format ArchiveFormat) bool {
	switch format {
	case ArchiveTAR, ArchiveTARGzip, ArchiveTARBzip2, ArchiveTARXZ, ArchiveTARZstd, ArchiveTARLZ4, ArchiveTARBrotli, ArchiveTARSnappy, ArchiveTARZlib:
		return true
	default:
		return false
	}
}
