package main

import (
	"flag"
	"fmt"
	"goFile/auth"
	"io"
	"strings"
	"unicode"
)

// startupConfig contains all settings required before the server can start.
type startupConfig struct {
	Path             string
	BasePath         string
	Port             string
	Host             string
	ReadOnly         bool
	UploadReadOnly   bool
	CookieSecure     bool
	AllowInsecureLAN bool
	Auth             auth.Config
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

	flags := flag.NewFlagSet("goFile", flag.ContinueOnError)
	flags.SetOutput(errOut)

	config := startupConfig{}
	flags.StringVar(&config.Path, "path", "./", "managed directory")
	flags.StringVar(&config.BasePath, "base-path", "", "public URL base path, for example /gofile")
	flags.StringVar(&config.Port, "port", "8089", "web port")
	flags.StringVar(&config.Host, "host", "127.0.0.1", "listen host")
	flags.BoolVar(&config.ReadOnly, "r", false, "read-only mode")
	flags.BoolVar(&config.UploadReadOnly, "ru", false, "read-only mode with upload")
	flags.BoolVar(&config.CookieSecure, "cookie-secure", false, "mark session cookies Secure (required behind HTTPS)")
	flags.BoolVar(&config.AllowInsecureLAN, "allow-insecure-lan", false, "allow plain HTTP and non-Secure cookies on a non-loopback host (unsafe)")

	var username, passwordHash, sessionSecret, apiToken optionalStringFlag
	flags.Var(&username, "admin-username", "administrator username (overrides GOFILE_ADMIN_USERNAME)")
	flags.Var(&passwordHash, "admin-password-hash", "bcrypt password hash (overrides GOFILE_ADMIN_PASSWORD_HASH)")
	flags.Var(&sessionSecret, "session-secret", "session signing secret (overrides GOFILE_SESSION_SECRET)")
	flags.Var(&apiToken, "api-token", "API upload token (overrides GOFILE_API_TOKEN)")

	if err := flags.Parse(args); err != nil {
		return startupConfig{}, err
	}
	if len(flags.Args()) != 0 {
		return startupConfig{}, fmt.Errorf("unexpected positional arguments")
	}
	basePath, err := normalizeBasePath(config.BasePath)
	if err != nil {
		return startupConfig{}, fmt.Errorf("invalid -base-path: %w", err)
	}
	config.BasePath = basePath
	legacyPasswordConfigured := lookup != nil && lookup("GOFILE_ADMIN_PASSWORD") != ""
	if config.UploadReadOnly {
		config.ReadOnly = true
	}
	if !isLoopbackHost(config.Host) && !config.CookieSecure && !config.AllowInsecureLAN {
		return startupConfig{}, fmt.Errorf("refusing non-loopback HTTP with non-Secure session cookies: use HTTPS with -cookie-secure, or explicitly acknowledge the risk with -allow-insecure-lan")
	}

	config.Auth = auth.ConfigFromLookup(lookup, config.CookieSecure)
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
	if legacyPasswordConfigured && config.Auth.PasswordHash == "" {
		return startupConfig{}, fmt.Errorf("GOFILE_ADMIN_PASSWORD is no longer supported; generate a bcrypt hash with 'goFile hash-password' and configure GOFILE_ADMIN_PASSWORD_HASH instead")
	}
	if err := config.Auth.Validate(); err != nil {
		return startupConfig{}, fmt.Errorf("authentication configuration error: set GOFILE_ADMIN_USERNAME, GOFILE_ADMIN_PASSWORD_HASH (valid bcrypt hash), GOFILE_SESSION_SECRET (at least 32 characters), and GOFILE_API_TOKEN (at least 32 characters), or pass -admin-username, -admin-password-hash, -session-secret, and -api-token: %w", err)
	}
	return config, nil
}
