package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/fluentra/fluentra/internal/platform/mailer"
	"github.com/fluentra/fluentra/internal/shared/config"
)

// envQueues is the variable that decides queue concurrency at run time.
const envQueues = "WORKER_QUEUES"

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
	assertWorkerConfig(t, cfg)
}

func assertWorkerConfig(t *testing.T, cfg workerConfig) {
	t.Helper()
	if cfg.Database.DSN == "" || cfg.Redis.URL == "" {
		t.Errorf("required connection settings not decoded: %#v", cfg)
	}
	if cfg.Worker.ShutdownGrace != 30*time.Second {
		t.Errorf("Worker.ShutdownGrace = %s, want 30s", cfg.Worker.ShutdownGrace)
	}
	if cfg.Outbox.PollInterval != time.Second || cfg.Outbox.BatchSize != 100 || cfg.Outbox.PublishedRetentionDays != 30 {
		t.Errorf("outbox settings = %s / %d / %d",
			cfg.Outbox.PollInterval, cfg.Outbox.BatchSize, cfg.Outbox.PublishedRetentionDays)
	}
	if cfg.Job.MaxAttempts != 5 {
		t.Errorf("Job.MaxAttempts = %d, want 5", cfg.Job.MaxAttempts)
	}
	if cfg.Worker.Queues == "" {
		t.Error("Worker.Queues not decoded from WORKER_QUEUES")
	}
	if cfg.Job.Timeout != 5*time.Minute {
		t.Errorf("Job.Timeout = %s, want 5m", cfg.Job.Timeout)
	}
	// The parsed map is what the worker actually runs on. Decoding the string
	// but never parsing it is how the five queues stayed decorative.
	if len(cfg.Queues) != 5 || cfg.Queues["default"] != 10 || cfg.Queues["ai"] != 4 {
		t.Errorf("parsed queues = %v, want the five queues from .env.example", cfg.Queues)
	}
	if cfg.Telemetry.Endpoint != "localhost:4317" {
		t.Errorf("Telemetry.Endpoint = %q, want the scheme stripped for gRPC", cfg.Telemetry.Endpoint)
	}
	if cfg.Mail.From != "Fluentra <no-reply@fluentra.dev>" {
		t.Errorf("Mail.From = %q, want the documented MAIL_FROM value", cfg.Mail.From)
	}
}

func TestNewMailSender_SMTPTransport(t *testing.T) {
	cfg := workerConfig{}
	cfg.Mail.Transport = "smtp"
	cfg.SMTP.Host = "smtp.example.test"
	cfg.SMTP.Port = 587
	cfg.SMTP.Username = "mailer-user"
	cfg.SMTP.Password = "mailer-password"
	cfg.Mail.From = "no-reply@example.test"

	renderer, err := mailer.NewRenderer(nil, nil)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	sender := newMailSender(cfg, renderer, nil)
	if sender == nil {
		t.Fatal("newMailSender returned nil")
	}
	if _, ok := sender.(*mailer.SMTPSender); !ok {
		t.Errorf("expected *mailer.SMTPSender, got %T", sender)
	}
}

func TestNewMailSender_ResendTransport(t *testing.T) {
	cfg := workerConfig{}
	cfg.Mail.Transport = "resend"
	cfg.Resend.APIKey = "re_test_key"
	cfg.Mail.From = "no-reply@example.test"

	renderer, err := mailer.NewRenderer(nil, nil)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	sender := newMailSender(cfg, renderer, nil)
	if sender == nil {
		t.Fatal("newMailSender returned nil")
	}
	if _, ok := sender.(*mailer.ResendSender); !ok {
		t.Errorf("expected *mailer.ResendSender, got %T", sender)
	}
}

// TestLoadConfig_RejectsUnusableJobSettings keeps a malformed value from booting
// a worker that silently runs on defaults nobody chose.
func TestLoadConfig_RejectsUnusableJobSettings(t *testing.T) {
	for name, override := range map[string]struct{ key, value string }{
		"malformed queue spec":             {envQueues, "default"},
		"zero concurrency":                 {envQueues, "default:0"},
		"empty queue spec":                 {envQueues, " "},
		"non-positive timeout":             {"JOB_TIMEOUT", "0s"},
		"non-positive published retention": {"OUTBOX_PUBLISHED_RETENTION_DAYS", "0"},
	} {
		t.Run(name, func(t *testing.T) {
			for variable, value := range envExample(t) {
				t.Setenv(variable, value)
			}
			t.Setenv(override.key, override.value)

			if _, err := loadConfig(context.Background()); err == nil {
				t.Errorf("%s=%q was accepted", override.key, override.value)
			}
		})
	}
}

// TestRegisterJobKinds_IsTheSinglePlaceP1Adds asserts that registered job kinds
// includes the active workers (user.data_export).
func TestRegisterJobKinds_IsTheSinglePlaceP1Adds(t *testing.T) {
	if got := registerJobKinds(river.NewWorkers()); got != 1 {
		t.Errorf("registered kinds = %d; want 1", got)
	}
}

func TestQueueNames_AreSortedForStableLogging(t *testing.T) {
	t.Parallel()
	got := queueNames(map[string]int{"notify": 1, "ai": 2, "batch": 3})
	want := []string{"ai", "batch", "notify"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("queueNames = %v, want %v", got, want)
		}
	}
}
