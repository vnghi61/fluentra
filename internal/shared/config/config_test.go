package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_EnvironmentOverridesDefaults(t *testing.T) {
	t.Setenv("HTTP_PORT", "9090")
	var target struct {
		HTTP struct {
			Port int `koanf:"port"`
		} `koanf:"http"`
	}
	err := Load(context.Background(), Options{Defaults: map[string]any{"http.port": 8080}}, &target)
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
	err := Load(context.Background(), Options{Defaults: map[string]any{"http.port": 8080}, File: path}, &target)
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
	if err := Load(context.Background(), Options{Defaults: map[string]any{"http.port": 8080}}, nil); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestLoad_MissingRequiredKeyNamesDocumentation(t *testing.T) {
	var target struct{}
	err := Load(context.Background(), Options{Required: []RequiredKey{{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"}}}, &target)
	if err == nil || !strings.Contains(err.Error(), "db.dsn") || !strings.Contains(err.Error(), "configuration.md") {
		t.Fatalf("unexpected error: %v", err)
	}
}
