package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/shared/config"
)

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
	documented := make(map[string]struct{})
	for variable := range envExample(t) {
		if key := config.EnvKey(variable); key != "" {
			documented[key] = struct{}{}
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
			t.Errorf("config key %q is read by cmd/worker but not documented in .env.example", key)
		}
	}
}

func TestLoadConfig_ResolvesEveryKeyFromEnvExample(t *testing.T) {
	for variable, value := range envExample(t) {
		t.Setenv(variable, value)
	}

	cfg, err := loadConfig(context.Background())
	if err != nil {
		t.Fatalf("load config from .env.example: %v", err)
	}
	if cfg.Database.DSN == "" || cfg.Redis.URL == "" {
		t.Errorf("required connection settings not decoded: %#v", cfg)
	}
	if cfg.Worker.ShutdownGrace != 30*time.Second {
		t.Errorf("Worker.ShutdownGrace = %s, want 30s", cfg.Worker.ShutdownGrace)
	}
	if cfg.Outbox.PollInterval != time.Second || cfg.Outbox.BatchSize != 100 {
		t.Errorf("outbox settings = %s / %d", cfg.Outbox.PollInterval, cfg.Outbox.BatchSize)
	}
	if cfg.Job.MaxAttempts != 5 {
		t.Errorf("Job.MaxAttempts = %d, want 5", cfg.Job.MaxAttempts)
	}
	if cfg.Worker.Queues == "" {
		t.Error("Worker.Queues not decoded from WORKER_QUEUES")
	}
	if cfg.Telemetry.Endpoint != "localhost:4317" {
		t.Errorf("Telemetry.Endpoint = %q, want the scheme stripped for gRPC", cfg.Telemetry.Endpoint)
	}
}
