package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/shared/config"
)

// testOTLPEndpoint is an override endpoint, deliberately different from
// defaultOTLPEndpoint so a test cannot pass on the default by accident.
const testOTLPEndpoint = "collector:4317"

// envExample is the documented configuration contract. Tests read it rather
// than restating keys, so a variable renamed here cannot silently stop working.
func envExample(t *testing.T) map[string]string {
	t.Helper()
	values, err := config.ParseEnvFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("parse .env.example: %v", err)
	}
	return values
}

// TestConfigOptions_EveryDeclaredKeyIsDocumented asserts the binary cannot read
// a key that `.env.example` does not document.
func TestConfigOptions_EveryDeclaredKeyIsDocumented(t *testing.T) {
	documented := make(map[string]string, len(envExample(t)))
	for variable := range envExample(t) {
		if key := config.EnvKey(variable); key != "" {
			documented[key] = variable
		}
	}

	options := configOptions()
	declared := make([]string, 0, len(options.Defaults)+len(options.Required))
	for key := range options.Defaults {
		declared = append(declared, key)
	}
	for _, required := range options.Required {
		declared = append(declared, required.Name)
	}

	for _, key := range declared {
		if _, ok := documented[key]; !ok {
			t.Errorf("config key %q is read by cmd/api but not documented in .env.example", key)
		}
	}
}

// TestLoadConfig_ResolvesEveryKeyFromEnvExample is the regression test for the
// mapping bug: `S3_ACCESS_KEY` must satisfy the required key `s3.access_key`.
func TestLoadConfig_ResolvesEveryKeyFromEnvExample(t *testing.T) {
	for variable, value := range envExample(t) {
		t.Setenv(variable, value)
	}

	cfg, err := loadConfig(context.Background())
	if err != nil {
		t.Fatalf("load config from .env.example: %v", err)
	}
	if cfg.Storage.AccessKey != "minioadmin" || cfg.Storage.SecretKey != "minioadmin" {
		t.Errorf("storage credentials = %q/%q", cfg.Storage.AccessKey, cfg.Storage.SecretKey)
	}
	if cfg.Database.DSN == "" || cfg.Redis.URL == "" || cfg.Storage.Endpoint == "" {
		t.Errorf("required connection settings not decoded: %#v", cfg)
	}
	if cfg.HTTP.RequestTimeout != 30*time.Second {
		t.Errorf("HTTP.RequestTimeout = %s, want 30s", cfg.HTTP.RequestTimeout)
	}
	if cfg.HTTP.ShutdownGrace != 30*time.Second {
		t.Errorf("HTTP.ShutdownGrace = %s, want 30s", cfg.HTTP.ShutdownGrace)
	}
	if cfg.Telemetry.Endpoint != defaultOTLPEndpoint {
		t.Errorf("Telemetry.Endpoint = %q, want the scheme stripped for gRPC", cfg.Telemetry.Endpoint)
	}
}

func TestLoadConfigReadsDocumentedEnvironmentKeys(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://app:password@db/fluentra")
	t.Setenv("REDIS_URL", "redis://redis:6379/0")
	t.Setenv("S3_ENDPOINT", "minio:9000")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("S3_USE_SSL", "true")
	t.Setenv("HTTP_READ_TIMEOUT", "7s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "8s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "9s")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "10s")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", testOTLPEndpoint)
	t.Setenv("OTEL_SERVICE_NAME", "fluentra-api-test")
	t.Setenv("OTP_HMAC_KEY", "test-otp-hmac-key-at-least-32-bytes-long")

	cfg, err := loadConfig(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.HTTP.ReadTimeout != 7*time.Second || cfg.HTTP.RequestTimeout != 10*time.Second {
		t.Fatalf("HTTP timeouts = read %s, request %s", cfg.HTTP.ReadTimeout, cfg.HTTP.RequestTimeout)
	}
	if cfg.Storage.AccessKey != "access" || cfg.Storage.SecretKey != "secret" || !cfg.Storage.UseSSL {
		t.Fatalf("storage environment was not decoded: %#v", cfg.Storage)
	}
	if cfg.Telemetry.Endpoint != testOTLPEndpoint || cfg.Telemetry.ServiceName != "fluentra-api-test" {
		t.Fatalf("telemetry environment was not decoded: %#v", cfg.Telemetry)
	}
}

func TestLoadConfig_RejectsNonPositiveDuration(t *testing.T) {
	for variable, value := range envExample(t) {
		t.Setenv(variable, value)
	}
	t.Setenv("HTTP_REQUEST_TIMEOUT", "0s")

	_, err := loadConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "http.request_timeout") {
		t.Fatalf("error = %v, want it to name http.request_timeout", err)
	}
}

func TestGRPCEndpointStripsScheme(t *testing.T) {
	for input, want := range map[string]string{
		"http://" + defaultOTLPEndpoint:    defaultOTLPEndpoint,
		"https://" + testOTLPEndpoint:      testOTLPEndpoint,
		testOTLPEndpoint:                   testOTLPEndpoint,
		"http://" + testOTLPEndpoint + "/": testOTLPEndpoint,
	} {
		if got := grpcEndpoint(input); got != want {
			t.Errorf("grpcEndpoint(%q) = %q, want %q", input, got, want)
		}
	}
}
