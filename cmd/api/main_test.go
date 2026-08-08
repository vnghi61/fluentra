package main

import (
	"context"
	"testing"
)

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
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	t.Setenv("OTEL_SERVICE_NAME", "fluentra-api-test")

	cfg, err := loadConfig(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.HTTP.Read.Timeout != "7s" || cfg.HTTP.Request.Timeout != "10s" {
		t.Fatalf("HTTP timeouts = read %q, request %q", cfg.HTTP.Read.Timeout, cfg.HTTP.Request.Timeout)
	}
	if cfg.Storage.Access.Key != "access" || cfg.Storage.Secret.Key != "secret" || !cfg.Storage.Use.SSL {
		t.Fatalf("storage environment was not decoded: %#v", cfg.Storage)
	}
	if cfg.Telemetry.Exporter.OTLP.Endpoint != "collector:4317" || cfg.Telemetry.Service.Name != "fluentra-api-test" {
		t.Fatalf("telemetry environment was not decoded: %#v", cfg.Telemetry)
	}
}
