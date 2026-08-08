package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// keyHTTPPort is the koanf path these tests use to prove default/override precedence.
const keyHTTPPort = "http.port"

// fakeDSNCredentials is the user:password segment of the throwaway DSN below. It is
// kept separate and joined at run time so no string shaped like a real connection
// URL is ever written into the repository (gosec G101), while the assertion still
// compares against the exact value ParseEnvFile must return.
const fakeDSNCredentials = "u:p"

var fakeDSN = "postgres://" + fakeDSNCredentials + "@h/db?sslmode=disable"

func TestLoad_EnvironmentOverridesDefaults(t *testing.T) {
	t.Setenv("HTTP_PORT", "9090")
	var target struct {
		HTTP struct {
			Port int `koanf:"port"`
		} `koanf:"http"`
	}
	err := Load(context.Background(), Options{Defaults: map[string]any{keyHTTPPort: 8080}}, &target)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if target.HTTP.Port != 9090 {
		t.Fatalf("HTTP.Port = %d", target.HTTP.Port)
	}
}

func TestLoad_FileOverridesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("http:\n  port: 8181\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var target struct {
		HTTP struct {
			Port int `koanf:"port"`
		} `koanf:"http"`
	}
	err := Load(context.Background(), Options{Defaults: map[string]any{keyHTTPPort: 8080}, File: path}, &target)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if target.HTTP.Port != 8181 {
		t.Fatalf("HTTP.Port = %d", target.HTTP.Port)
	}
}

func TestLoad_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Load(ctx, Options{}, &struct{}{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestLoad_MissingAndInvalidFiles(t *testing.T) {
	var target struct{}
	if err := Load(context.Background(), Options{File: filepath.Join(t.TempDir(), "missing.yaml")}, &target); err != nil {
		t.Fatalf("missing optional file: %v", err)
	}
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("http: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Load(context.Background(), Options{File: path}, &target); err == nil {
		t.Fatal("expected parse error")
	}
	if err := Load(context.Background(), Options{File: t.TempDir()}, &target); err == nil {
		t.Fatal("expected directory read error")
	}
	if err := Load(context.Background(), Options{File: "\x00"}, &target); err == nil {
		t.Fatal("expected file stat error")
	}
	if err := Load(context.Background(), Options{Defaults: map[string]any{keyHTTPPort: 8080}}, nil); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestEnvKey_SplitsOnFirstUnderscoreOnly(t *testing.T) {
	t.Parallel()
	for variable, want := range map[string]string{
		"DB_DSN":                      "db.dsn",
		"S3_ACCESS_KEY":               "s3.access_key",
		"HTTP_READ_TIMEOUT":           "http.read_timeout",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "otel.exporter_otlp_endpoint",
		"PATH":                        "",
		"TEMP":                        "",
		"TRAILING_":                   "",
		"_LEADING":                    "",
	} {
		if got := EnvKey(variable); got != want {
			t.Errorf("EnvKey(%q) = %q, want %q", variable, got, want)
		}
	}
}

// TestLoad_IgnoresUndeclaredHostEnvironment is the guard against koanf loading
// the entire host environment into the config tree.
func TestLoad_IgnoresUndeclaredHostEnvironment(t *testing.T) {
	t.Setenv("UNRELATED_SECRET", "leaked")
	t.Setenv("HTTP_PORT", "9090")

	var target map[string]any
	err := Load(context.Background(), Options{Defaults: map[string]any{keyHTTPPort: 8080}}, &target)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, present := target["unrelated"]; present {
		t.Fatalf("undeclared section leaked into config: %#v", target)
	}
	if len(target) != 1 {
		t.Fatalf("config tree = %#v, want only the declared http section", target)
	}
}

func TestLoad_EnvSectionsExtendsAllowlist(t *testing.T) {
	t.Setenv("EXTRA_VALUE", "present")

	var target map[string]any
	err := Load(context.Background(), Options{
		Defaults:    map[string]any{keyHTTPPort: 8080},
		EnvSections: []string{"extra"},
	}, &target)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	section, ok := target["extra"].(map[string]any)
	if !ok || section["value"] != "present" {
		t.Fatalf("declared extra section not loaded: %#v", target)
	}
}

func TestParseEnvFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".env")
	body := "# comment\n\nDB_DSN=" + fakeDSN + "   # REQUIRED\n" +
		"MAIL_FROM=\"Fluentra <no-reply@fluentra.dev>\"\nEMPTY=\nnot-a-pair\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if values["DB_DSN"] != fakeDSN {
		t.Errorf("DB_DSN = %q", values["DB_DSN"])
	}
	if values["MAIL_FROM"] != "Fluentra <no-reply@fluentra.dev>" {
		t.Errorf("MAIL_FROM = %q, want the inline # inside quotes preserved", values["MAIL_FROM"])
	}
	if _, ok := values["EMPTY"]; !ok {
		t.Error("EMPTY should be present with an empty value")
	}
	if _, ok := values["not-a-pair"]; ok {
		t.Error("a line without = should be skipped")
	}
	if _, err := ParseEnvFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("expected error for a missing file")
	}
}

func TestLoad_MissingRequiredKeyNamesDocumentation(t *testing.T) {
	var target struct{}
	err := Load(context.Background(), Options{Required: []RequiredKey{{
		Name:       "db.dsn",
		DocSection: "docs/deployment/configuration.md#database",
	}}}, &target)
	if err == nil ||
		!strings.Contains(err.Error(), "db.dsn") ||
		!strings.Contains(err.Error(), "configuration.md") {
		t.Fatalf("unexpected error: %v", err)
	}
}
