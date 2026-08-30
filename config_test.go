package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/irains/fileharbor/auth"
	"golang.org/x/crypto/bcrypt"
)

const testPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func validAuthEnvironment() map[string]string {
	return map[string]string{
		"FILEHARBOR_ADMIN_USERNAME":      "environment-admin",
		"FILEHARBOR_ADMIN_PASSWORD_HASH": testPasswordHash,
		"FILEHARBOR_SESSION_SECRET":      "environment-session-secret-0123456789",
		"FILEHARBOR_API_TOKEN":           "environment-api-token-012345678901234",
	}
}

func lookupEnvironment(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func configArgs(t *testing.T, args ...string) []string {
	t.Helper()
	return append([]string{"-state-dir", t.TempDir()}, args...)
}

func legacyAuthEnvironment() map[string]string {
	return map[string]string{
		"GOFILE_ADMIN_USERNAME":      "environment-admin",
		"GOFILE_ADMIN_PASSWORD_HASH": testPasswordHash,
		"GOFILE_SESSION_SECRET":      "environment-session-secret-0123456789",
		"GOFILE_API_TOKEN":           "environment-api-token-012345678901234",
	}
}

func TestNormalizeBasePath(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want string
		ok   bool
	}{
		{"", "", true},
		{"/", "", true},
		{"/gofile", "/gofile", true},
		{"/tools/gofile", "/tools/gofile", true},
		{"gofile", "", false},
		{"/gofile/", "", false},
		{"//gofile", "", false},
		{"/gofile//files", "", false},
		{"/gofile/../admin", "", false},
		{"/gofile?x=1", "", false},
		{"/gofile#fragment", "", false},
		{"/gofile%2fadmin", "", false},
		{"/gofile;admin", "", false},
		{"/gofile/管理", "", false},
		{"/gofile\\admin", "", false},
		{"/gofile\x01admin", "", false},
	} {
		t.Run(test.raw, func(t *testing.T) {
			got, err := normalizeBasePath(test.raw)
			if (err == nil) != test.ok || got != test.want {
				t.Fatalf("normalizeBasePath(%q) = %q, %v; want %q, valid=%t", test.raw, got, err, test.want, test.ok)
			}
		})
	}
}

func TestParseStartupConfigConfiguresReliableUploadLimits(t *testing.T) {
	config, err := parseStartupConfig(configArgs(t,
		"-upload-max-bytes=33554432",
		"-upload-chunk-bytes=1048576",
		"-upload-max-parts=32",
		"-upload-max-active=3",
		"-upload-max-pending-bytes=67108864",
		"-upload-max-concurrent-parts=2",
		"-upload-inactivity-ttl=2h",
		"-upload-completion-ttl=15m",
		"-upload-min-free-bytes=1048576",
	), lookupEnvironment(validAuthEnvironment()), nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Upload.MaxFileBytes != 32<<20 || config.Upload.ChunkBytes != 1<<20 || config.Upload.MaxParts != 32 ||
		config.Upload.MaxActive != 3 || config.Upload.MaxPendingBytes != 64<<20 || config.Upload.MaxConcurrentParts != 2 ||
		config.Upload.InactivityTTL != 2*time.Hour || config.Upload.CompletionTTL != 15*time.Minute || config.Upload.MinFreeBytes != 1<<20 {
		t.Fatalf("reliable upload limits = %#v", config.Upload)
	}
	for _, argument := range []string{"-upload-max-parts=0", "-upload-min-free-bytes=-1"} {
		if _, err := parseStartupConfig(configArgs(t, argument), lookupEnvironment(validAuthEnvironment()), nil); err == nil {
			t.Fatalf("invalid reliable upload argument %q was accepted", argument)
		}
	}
}

func TestParseStartupConfigConfiguresBasePathCookieScope(t *testing.T) {
	config, err := parseStartupConfig(configArgs(t, "-base-path=/gofile"), lookupEnvironment(validAuthEnvironment()), nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.BasePath != "/gofile" || config.Auth.CookiePath != "/gofile" {
		t.Fatalf("base path configuration = %#v", config)
	}

	config, err = parseStartupConfig(configArgs(t), lookupEnvironment(validAuthEnvironment()), nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.BasePath != "" || config.Auth.CookiePath != "" {
		t.Fatalf("root base path configuration = %#v", config)
	}
}

func TestParseStartupConfigUsesEnvironmentFallback(t *testing.T) {
	values := validAuthEnvironment()
	config, err := parseStartupConfig(configArgs(t), lookupEnvironment(values), nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Auth.Username != values["FILEHARBOR_ADMIN_USERNAME"] ||
		config.Auth.PasswordHash != values["FILEHARBOR_ADMIN_PASSWORD_HASH"] ||
		config.Auth.SessionSecret != values["FILEHARBOR_SESSION_SECRET"] ||
		config.Auth.APIToken != values["FILEHARBOR_API_TOKEN"] {
		t.Fatal("configuration did not preserve environment credentials")
	}
	if config.Path != "./" || config.Port != "8089" || config.Host != "127.0.0.1" {
		t.Fatalf("unexpected defaults: %#v", config)
	}
}

func TestParseStartupConfigSupportsCommandLineCredentials(t *testing.T) {
	args := []string{
		"-admin-username=flag-admin",
		"-admin-password-hash=" + testPasswordHash,
		"-session-secret=flag-session-secret-012345678901234",
		"-api-token=flag-api-token-0123456789012345678",
	}
	config, err := parseStartupConfig(configArgs(t, args...), lookupEnvironment(nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Auth.Username != "flag-admin" || config.Auth.PasswordHash != testPasswordHash ||
		config.Auth.SessionSecret != "flag-session-secret-012345678901234" ||
		config.Auth.APIToken != "flag-api-token-0123456789012345678" {
		t.Fatal("configuration did not use credential flags")
	}
}

func TestParseStartupConfigCredentialFlagsOverrideEnvironmentIndividually(t *testing.T) {
	values := validAuthEnvironment()
	tests := []struct {
		name     string
		flag     string
		value    string
		actual   func(auth.Config) string
		expected string
	}{
		{"username", "-admin-username", "flag-admin", func(c auth.Config) string { return c.Username }, "flag-admin"},
		{"password hash", "-admin-password-hash", testPasswordHash, func(c auth.Config) string { return c.PasswordHash }, testPasswordHash},
		{"session secret", "-session-secret", "flag-session-secret-012345678901234", func(c auth.Config) string { return c.SessionSecret }, "flag-session-secret-012345678901234"},
		{"API token", "-api-token", "flag-api-token-0123456789012345678", func(c auth.Config) string { return c.APIToken }, "flag-api-token-0123456789012345678"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config, err := parseStartupConfig(configArgs(t, test.flag+"="+test.value), lookupEnvironment(values), nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := test.actual(config.Auth); got != test.expected {
				t.Fatalf("override = %q, want %q", got, test.expected)
			}
		})
	}
}

func TestParseStartupConfigRejectsInvalidCredentials(t *testing.T) {
	values := validAuthEnvironment()
	for _, args := range [][]string{
		{"-admin-password-hash="},
		{"-admin-password-hash=not-a-bcrypt-hash"},
		{"-session-secret=short"},
		{"-api-token=short"},
	} {
		_, err := parseStartupConfig(append(configArgs(t), args...), lookupEnvironment(values), nil)
		if !errors.Is(err, auth.ErrInvalidConfig) {
			t.Fatalf("args %q error = %v, want invalid auth configuration", args, err)
		}
	}

	legacyOnly := validAuthEnvironment()
	legacyOnly["FILEHARBOR_ADMIN_PASSWORD_HASH"] = ""
	legacyOnly["FILEHARBOR_ADMIN_PASSWORD"] = "legacy-plaintext-password"
	_, err := parseStartupConfig(configArgs(t), lookupEnvironment(legacyOnly), nil)
	if err == nil || !strings.Contains(err.Error(), "FILEHARBOR_ADMIN_PASSWORD") {
		t.Fatalf("legacy password error = %v", err)
	}
}

func TestParseStartupConfigRejectsPlaintextPasswordConfiguration(t *testing.T) {
	for _, plaintextKey := range []string{"FILEHARBOR_ADMIN_PASSWORD", "GOFILE_ADMIN_PASSWORD"} {
		t.Run(plaintextKey, func(t *testing.T) {
			values := validAuthEnvironment()
			values[plaintextKey] = "legacy-plaintext-password"
			_, err := parseStartupConfig(configArgs(t), lookupEnvironment(values), nil)
			if err == nil || !strings.Contains(err.Error(), plaintextKey) {
				t.Fatalf("plaintext password error = %v", err)
			}
		})
	}
}

func TestParseStartupConfigSupportsLegacyNamespaceOnly(t *testing.T) {
	values := legacyAuthEnvironment()
	config, err := parseStartupConfig(configArgs(t), lookupEnvironment(values), nil)
	if err != nil || config.Auth.Username != values["GOFILE_ADMIN_USERNAME"] {
		t.Fatalf("legacy configuration = %#v, %v", config, err)
	}
}

func TestParseStartupConfigRejectsMixedCredentialNamespaces(t *testing.T) {
	values := validAuthEnvironment()
	values["GOFILE_API_TOKEN"] = values["FILEHARBOR_API_TOKEN"]
	_, err := parseStartupConfig(configArgs(t), lookupEnvironment(values), nil)
	if !errors.Is(err, auth.ErrInvalidConfig) {
		t.Fatalf("mixed namespaces error = %v", err)
	}
}

func TestParseStartupConfigStateDirectoryPrecedence(t *testing.T) {
	primary := t.TempDir()
	legacy := t.TempDir()
	values := validAuthEnvironment()
	values["FILEHARBOR_STATE_DIR"] = primary
	config, err := parseStartupConfig(nil, lookupEnvironment(values), nil)
	if err != nil || config.StateDir != primary {
		t.Fatalf("primary state directory = %q, %v", config.StateDir, err)
	}
	values = validAuthEnvironment()
	values["GOFILE_STATE_DIR"] = legacy
	config, err = parseStartupConfig(nil, lookupEnvironment(values), nil)
	if err != nil || config.StateDir != legacy {
		t.Fatalf("legacy state directory = %q, %v", config.StateDir, err)
	}
	values["FILEHARBOR_STATE_DIR"] = primary
	if _, err := parseStartupConfig(nil, lookupEnvironment(values), nil); err == nil {
		t.Fatal("expected conflicting state directories to fail")
	}
	config, err = parseStartupConfig(configArgs(t), lookupEnvironment(values), nil)
	if err != nil || config.StateDir == "" || config.StateDir == primary || config.StateDir == legacy {
		t.Fatalf("explicit state directory = %q, %v", config.StateDir, err)
	}
}

func TestParseStartupConfigRejectsBcryptCostAbovePolicy(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("a durable password"), 13)
	if err != nil {
		t.Fatal(err)
	}
	values := validAuthEnvironment()
	values["FILEHARBOR_ADMIN_PASSWORD_HASH"] = string(hash)
	_, err = parseStartupConfig(configArgs(t), lookupEnvironment(values), nil)
	if !errors.Is(err, auth.ErrInvalidConfig) {
		t.Fatalf("cost 13 configuration error = %v", err)
	}
}

func TestParseStartupConfigPreservesModeAndNetworkGuards(t *testing.T) {
	values := validAuthEnvironment()
	config, err := parseStartupConfig(configArgs(t,
		"-path", "files",
		"-port", "9090",
		"-host", "0.0.0.0",
		"-r",
		"-ru",
		"-cookie-secure",
		"-allow-insecure-lan",
	), lookupEnvironment(values), nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Path != "files" || config.Port != "9090" || config.Host != "0.0.0.0" ||
		!config.ReadOnly || !config.UploadReadOnly || !config.CookieSecure || !config.AllowInsecureLAN {
		t.Fatalf("unexpected mapped configuration: %#v", config)
	}

	for _, test := range []struct {
		name  string
		args  []string
		valid bool
	}{
		{"non-loopback is rejected", []string{"-host", "0.0.0.0"}, false},
		{"secure cookie permits non-loopback", []string{"-host", "0.0.0.0", "-cookie-secure"}, true},
		{"unsafe override permits non-loopback", []string{"-host", "0.0.0.0", "-allow-insecure-lan"}, true},
		{"loopback permits non-secure cookie", nil, true},
		{"hostname requires secure cookie", []string{"-host", "localhost"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseStartupConfig(append(configArgs(t), test.args...), lookupEnvironment(values), nil)
			if (err == nil) != test.valid {
				t.Fatalf("parse error = %v, valid = %v", err, test.valid)
			}
		})
	}
}

func TestParseStartupConfigReportsHelpWithoutSecrets(t *testing.T) {
	values := validAuthEnvironment()
	values["FILEHARBOR_ADMIN_PASSWORD_HASH"] = "$2a$10$password-hash-sentinel-123456789012345678901234567890123"
	values["FILEHARBOR_SESSION_SECRET"] = "environment-session-secret-sentinel-123"
	values["FILEHARBOR_API_TOKEN"] = "environment-api-token-sentinel-1234567"
	const commandLineSecret = "$2a$10$command-line-hash-sentinel-123456789012345678901234567890"

	var output bytes.Buffer
	_, err := parseStartupConfig([]string{"-admin-password-hash=" + commandLineSecret, "-h"}, lookupEnvironment(values), &output)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v", err)
	}
	help := output.String()
	for _, name := range []string{"-admin-username", "-admin-password-hash", "-session-secret", "-api-token"} {
		if !strings.Contains(help, name) {
			t.Fatalf("help did not describe %s: %q", name, help)
		}
	}
	for _, secret := range []string{commandLineSecret, values["FILEHARBOR_ADMIN_PASSWORD_HASH"], values["FILEHARBOR_SESSION_SECRET"], values["FILEHARBOR_API_TOKEN"]} {
		if strings.Contains(help, secret) {
			t.Fatalf("help leaked a secret: %q", secret)
		}
	}
}

func TestParseStartupConfigReturnsFlagErrors(t *testing.T) {
	for _, args := range [][]string{{"-not-a-real-flag"}, {"-admin-password=legacy"}, {"unexpected"}} {
		_, err := parseStartupConfig(args, lookupEnvironment(validAuthEnvironment()), nil)
		if err == nil {
			t.Fatalf("args %q should fail", args)
		}
	}
}
