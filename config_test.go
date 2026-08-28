package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"

	"goFile/auth"
	"golang.org/x/crypto/bcrypt"
)

const testPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func validAuthEnvironment() map[string]string {
	return map[string]string{
		"GOFILE_ADMIN_USERNAME":      "environment-admin",
		"GOFILE_ADMIN_PASSWORD_HASH": testPasswordHash,
		"GOFILE_SESSION_SECRET":       "environment-session-secret-0123456789",
		"GOFILE_API_TOKEN":            "environment-api-token-012345678901234",
	}
}

func lookupEnvironment(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestParseStartupConfigUsesEnvironmentFallback(t *testing.T) {
	values := validAuthEnvironment()
	config, err := parseStartupConfig(nil, lookupEnvironment(values), nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Auth.Username != values["GOFILE_ADMIN_USERNAME"] ||
		config.Auth.PasswordHash != values["GOFILE_ADMIN_PASSWORD_HASH"] ||
		config.Auth.SessionSecret != values["GOFILE_SESSION_SECRET"] ||
		config.Auth.APIToken != values["GOFILE_API_TOKEN"] {
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
	config, err := parseStartupConfig(args, lookupEnvironment(nil), nil)
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
			config, err := parseStartupConfig([]string{test.flag + "=" + test.value}, lookupEnvironment(values), nil)
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
		_, err := parseStartupConfig(args, lookupEnvironment(values), nil)
		if !errors.Is(err, auth.ErrInvalidConfig) {
			t.Fatalf("args %q error = %v, want invalid auth configuration", args, err)
		}
	}

	legacyOnly := validAuthEnvironment()
	legacyOnly["GOFILE_ADMIN_PASSWORD_HASH"] = ""
	legacyOnly["GOFILE_ADMIN_PASSWORD"] = "legacy-plaintext-password"
	_, err := parseStartupConfig(nil, lookupEnvironment(legacyOnly), nil)
	if err == nil || !strings.Contains(err.Error(), "GOFILE_ADMIN_PASSWORD") {
		t.Fatalf("legacy password error = %v", err)
	}
}

func TestParseStartupConfigAllowsHashDuringLegacyEnvironmentMigration(t *testing.T) {
	values := validAuthEnvironment()
	values["GOFILE_ADMIN_PASSWORD"] = "legacy-plaintext-password"

	config, err := parseStartupConfig(nil, lookupEnvironment(values), nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Auth.PasswordHash != testPasswordHash {
		t.Fatal("migration configuration did not retain the bcrypt hash")
	}

	config, err = parseStartupConfig([]string{"-admin-password-hash=" + testPasswordHash}, lookupEnvironment(map[string]string{
		"GOFILE_ADMIN_USERNAME": "environment-admin",
		"GOFILE_ADMIN_PASSWORD": "legacy-plaintext-password",
		"GOFILE_SESSION_SECRET":  "environment-session-secret-0123456789",
		"GOFILE_API_TOKEN":       "environment-api-token-012345678901234",
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Auth.PasswordHash != testPasswordHash {
		t.Fatal("command-line bcrypt hash was not accepted during migration")
	}
}

func TestParseStartupConfigRejectsBcryptCostAbovePolicy(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("a durable password"), 13)
	if err != nil {
		t.Fatal(err)
	}
	values := validAuthEnvironment()
	values["GOFILE_ADMIN_PASSWORD_HASH"] = string(hash)
	_, err = parseStartupConfig(nil, lookupEnvironment(values), nil)
	if !errors.Is(err, auth.ErrInvalidConfig) {
		t.Fatalf("cost 13 configuration error = %v", err)
	}
}

func TestParseStartupConfigPreservesModeAndNetworkGuards(t *testing.T) {
	values := validAuthEnvironment()
	config, err := parseStartupConfig([]string{
		"-path", "files",
		"-port", "9090",
		"-host", "0.0.0.0",
		"-r",
		"-ru",
		"-cookie-secure",
		"-allow-insecure-lan",
	}, lookupEnvironment(values), nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Path != "files" || config.Port != "9090" || config.Host != "0.0.0.0" ||
		!config.ReadOnly || !config.UploadReadOnly || !config.CookieSecure || !config.AllowInsecureLAN {
		t.Fatalf("unexpected mapped configuration: %#v", config)
	}

	for _, test := range []struct {
		name string
		args []string
		valid bool
	}{
		{"non-loopback is rejected", []string{"-host", "0.0.0.0"}, false},
		{"secure cookie permits non-loopback", []string{"-host", "0.0.0.0", "-cookie-secure"}, true},
		{"unsafe override permits non-loopback", []string{"-host", "0.0.0.0", "-allow-insecure-lan"}, true},
		{"loopback permits non-secure cookie", nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseStartupConfig(test.args, lookupEnvironment(values), nil)
			if (err == nil) != test.valid {
				t.Fatalf("parse error = %v, valid = %v", err, test.valid)
			}
		})
	}
}

func TestParseStartupConfigReportsHelpWithoutSecrets(t *testing.T) {
	values := validAuthEnvironment()
	values["GOFILE_ADMIN_PASSWORD_HASH"] = "$2a$10$password-hash-sentinel-123456789012345678901234567890123"
	values["GOFILE_SESSION_SECRET"] = "environment-session-secret-sentinel-123"
	values["GOFILE_API_TOKEN"] = "environment-api-token-sentinel-1234567"
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
	for _, secret := range []string{commandLineSecret, values["GOFILE_ADMIN_PASSWORD_HASH"], values["GOFILE_SESSION_SECRET"], values["GOFILE_API_TOKEN"]} {
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
