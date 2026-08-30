package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/irains/fileharbor/auth"
)

// startupConfig contains all settings required before the server can start.
type startupConfig struct {
	Path             string
	StateDir         string
	BasePath         string
	Port             string
	Host             string
	ReadOnly         bool
	UploadReadOnly   bool
	CookieSecure     bool
	AllowInsecureLAN bool
	Auth             auth.Config
	Upload           UploadConfig
}

// UploadConfig bounds the durable resumable-upload service. The existing
// direct and legacy chunk routes keep their current compatibility limits.
type UploadConfig struct {
	MaxFileBytes       int64
	ChunkBytes         int64
	MaxParts           int
	MaxActive          int
	MaxPendingBytes    int64
	MaxConcurrentParts int
	InactivityTTL      time.Duration
	CompletionTTL      time.Duration
	MinFreeBytes       int64
}

func defaultUploadConfig() UploadConfig {
	return UploadConfig{
		MaxFileBytes:       8 << 30,
		ChunkBytes:         8 << 20,
		MaxParts:           4096,
		MaxActive:          64,
		MaxPendingBytes:    16 << 30,
		MaxConcurrentParts: 8,
		InactivityTTL:      24 * time.Hour,
		CompletionTTL:      time.Hour,
		MinFreeBytes:       256 << 20,
	}
}

func (config UploadConfig) Validate() error {
	if config.MaxFileBytes < 0 || config.ChunkBytes < 1<<20 || config.ChunkBytes > 64<<20 ||
		config.MaxParts < 1 || config.MaxActive < 1 || config.MaxPendingBytes < config.MaxFileBytes ||
		config.MaxConcurrentParts < 1 || config.InactivityTTL <= 0 || config.CompletionTTL <= 0 || config.MinFreeBytes < 0 {
		return fmt.Errorf("invalid reliable upload limits")
	}
	if config.MaxFileBytes > 0 && (config.MaxFileBytes+config.ChunkBytes-1)/config.ChunkBytes > int64(config.MaxParts) {
		return fmt.Errorf("reliable upload maximum requires more parts than allowed")
	}
	return nil
}

func normalizeBasePath(raw string) (string, error) {
	if raw == "" || raw == "/" {
		return "", nil
	}
	if !strings.HasPrefix(raw, "/") || strings.HasSuffix(raw, "/") || strings.Contains(raw, "//") || strings.ContainsAny(raw, "\\;?#%") || strings.IndexFunc(raw, func(r rune) bool { return r > unicode.MaxASCII || unicode.IsControl(r) }) >= 0 {
		return "", fmt.Errorf("must be an absolute path without a trailing slash or ambiguous characters")
	}
	for _, segment := range strings.Split(strings.TrimPrefix(raw, "/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("must not contain empty, dot, or traversal segments")
		}
	}
	return raw, nil
}

func defaultStateDir() string {
	if path, err := os.UserCacheDir(); err == nil && path != "" {
		return filepath.Join(path, "fileharbor")
	}
	return ""
}

// optionalStringFlag records whether a credential argument was explicitly
// provided. Its String method never returns the stored value so help output
// cannot disclose credentials.
type optionalStringFlag struct {
	value string
	set   bool
}

func (f *optionalStringFlag) String() string { return "" }

func (f *optionalStringFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}

// parseStartupConfig reads command-line arguments and merges explicitly supplied
// credential flags over their corresponding environment values.
func parseStartupConfig(args []string, lookup func(string) string, errOut io.Writer) (startupConfig, error) {
	if errOut == nil {
		errOut = io.Discard
	}

	flags := flag.NewFlagSet("fileharbor", flag.ContinueOnError)
	flags.SetOutput(errOut)

	config := startupConfig{Upload: defaultUploadConfig()}
	stateDir := defaultStateDir()
	flags.StringVar(&config.StateDir, "state-dir", "", "private runtime state directory")
	flags.StringVar(&config.Path, "path", "./", "managed directory")
	flags.StringVar(&config.BasePath, "base-path", "", "public URL base path, for example /fileharbor")
	flags.StringVar(&config.Port, "port", "8089", "web port")
	flags.StringVar(&config.Host, "host", "127.0.0.1", "listen host")
	flags.BoolVar(&config.ReadOnly, "r", false, "read-only mode")
	flags.BoolVar(&config.UploadReadOnly, "ru", false, "read-only mode with upload")
	flags.BoolVar(&config.CookieSecure, "cookie-secure", false, "mark session cookies Secure (required behind HTTPS)")
	flags.BoolVar(&config.AllowInsecureLAN, "allow-insecure-lan", false, "allow plain HTTP and non-Secure cookies on a non-loopback host (unsafe)")
	flags.Int64Var(&config.Upload.MaxFileBytes, "upload-max-bytes", config.Upload.MaxFileBytes, "maximum reliable upload size in bytes")
	flags.Int64Var(&config.Upload.ChunkBytes, "upload-chunk-bytes", config.Upload.ChunkBytes, "reliable upload chunk size in bytes")
	flags.IntVar(&config.Upload.MaxParts, "upload-max-parts", config.Upload.MaxParts, "maximum reliable upload parts per file")
	flags.IntVar(&config.Upload.MaxActive, "upload-max-active", config.Upload.MaxActive, "maximum active reliable uploads")
	flags.Int64Var(&config.Upload.MaxPendingBytes, "upload-max-pending-bytes", config.Upload.MaxPendingBytes, "maximum reserved reliable upload bytes")
	flags.IntVar(&config.Upload.MaxConcurrentParts, "upload-max-concurrent-parts", config.Upload.MaxConcurrentParts, "maximum concurrent reliable upload part writes")
	flags.DurationVar(&config.Upload.InactivityTTL, "upload-inactivity-ttl", config.Upload.InactivityTTL, "reliable upload inactivity retention")
	flags.DurationVar(&config.Upload.CompletionTTL, "upload-completion-ttl", config.Upload.CompletionTTL, "reliable upload completion retention")
	flags.Int64Var(&config.Upload.MinFreeBytes, "upload-min-free-bytes", config.Upload.MinFreeBytes, "minimum free bytes reserved on reliable upload volumes")

	var username, passwordHash, sessionSecret, apiToken optionalStringFlag
	flags.Var(&username, "admin-username", "administrator username (overrides FILEHARBOR_ADMIN_USERNAME)")
	flags.Var(&passwordHash, "admin-password-hash", "bcrypt password hash (overrides FILEHARBOR_ADMIN_PASSWORD_HASH)")
	flags.Var(&sessionSecret, "session-secret", "session signing secret (overrides FILEHARBOR_SESSION_SECRET)")
	flags.Var(&apiToken, "api-token", "API upload token (overrides FILEHARBOR_API_TOKEN)")

	if err := flags.Parse(args); err != nil {
		return startupConfig{}, err
	}
	if len(flags.Args()) != 0 {
		return startupConfig{}, fmt.Errorf("unexpected positional arguments")
	}
	stateExplicit := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "state-dir" {
			stateExplicit = true
		}
	})
	if !stateExplicit {
		primaryStateDir := ""
		legacyStateDir := ""
		if lookup != nil {
			primaryStateDir = lookup("FILEHARBOR_STATE_DIR")
			legacyStateDir = lookup("GOFILE_STATE_DIR")
		}
		if primaryStateDir != "" && legacyStateDir != "" && primaryStateDir != legacyStateDir {
			return startupConfig{}, fmt.Errorf("state directory configuration conflicts between FILEHARBOR_STATE_DIR and GOFILE_STATE_DIR")
		}
		switch {
		case primaryStateDir != "":
			config.StateDir = primaryStateDir
		case legacyStateDir != "":
			config.StateDir = legacyStateDir
		default:
			config.StateDir = stateDir
		}
	}
	if config.StateDir == "" {
		return startupConfig{}, fmt.Errorf("state directory is required: set -state-dir or FILEHARBOR_STATE_DIR")
	}
	basePath, err := normalizeBasePath(config.BasePath)
	if err != nil {
		return startupConfig{}, fmt.Errorf("invalid -base-path: %w", err)
	}
	config.BasePath = basePath
	legacyPasswordConfigured := lookup != nil && (lookup("FILEHARBOR_ADMIN_PASSWORD") != "" || lookup("GOFILE_ADMIN_PASSWORD") != "")
	if config.UploadReadOnly {
		config.ReadOnly = true
	}
	if err := config.Upload.Validate(); err != nil {
		return startupConfig{}, err
	}
	if !isLoopbackHost(config.Host) && !config.CookieSecure && !config.AllowInsecureLAN {
		return startupConfig{}, fmt.Errorf("refusing non-loopback HTTP with non-Secure session cookies: use HTTPS with -cookie-secure, or explicitly acknowledge the risk with -allow-insecure-lan")
	}

	config.Auth, err = auth.ConfigFromLookup(lookup, config.CookieSecure)
	if err != nil {
		return startupConfig{}, fmt.Errorf("authentication configuration error: FILEHARBOR_* and legacy GOFILE_* credential values conflict: %w", err)
	}
	config.Auth.CookiePath = config.BasePath
	if username.set {
		config.Auth.Username = username.value
	}
	if passwordHash.set {
		config.Auth.PasswordHash = passwordHash.value
	}
	if sessionSecret.set {
		config.Auth.SessionSecret = sessionSecret.value
	}
	if apiToken.set {
		config.Auth.APIToken = apiToken.value
	}
	if legacyPasswordConfigured {
		return startupConfig{}, fmt.Errorf("FILEHARBOR_ADMIN_PASSWORD and GOFILE_ADMIN_PASSWORD are no longer supported; generate a bcrypt hash with 'fileharbor hash-password' and configure FILEHARBOR_ADMIN_PASSWORD_HASH instead")
	}
	if err := config.Auth.Validate(); err != nil {
		return startupConfig{}, fmt.Errorf("authentication configuration error: set FILEHARBOR_ADMIN_USERNAME, FILEHARBOR_ADMIN_PASSWORD_HASH (valid bcrypt hash), FILEHARBOR_SESSION_SECRET (at least 32 characters), and FILEHARBOR_API_TOKEN (at least 32 characters), or pass -admin-username, -admin-password-hash, -session-secret, and -api-token: %w", err)
	}
	return config, nil
}
