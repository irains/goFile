package conf

import "time"

// Info is the safe, direct-child view of a directory.
type Info struct {
	Entries   []Entry
	Files     []File // Kept for compatibility with external template customizations.
	Dirs      []Dir  // Kept for compatibility with external template customizations.
	Truncated bool
}

// Entry is an item rendered in the file workspace. Version is an optimistic
// concurrency marker that must be revalidated by state-changing operations.
type Entry struct {
	Name      string
	Path      string
	Kind      string
	Size      int64
	Modified  time.Time
	Mode      string
	Extension string
	IsArchive bool
	Version   string
}

type File struct {
	FileName string
	FilePath string
	IsZip    bool
}

type Dir struct {
	DirName string `json:"name"`
	DirPath string `json:"path"`
}

var (
	FileHarborPort string
	FileHarbor     string
	Host           string
)
