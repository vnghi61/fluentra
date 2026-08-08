package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/telemetry"
	"github.com/fluentra/fluentra/internal/shared/config"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	version   = "dev"
	commitSHA = "unknown"
)

// workerConfig mirrors `.env.example`. Every field maps to exactly one
// environment variable under the convention documented in shared/config.
type workerConfig struct {
	App struct {
		Environment string `koanf:"env"`
		Name        string `koanf:"name"`
		Version     string `koanf:"version"`
	} `koanf:"app"`
	HTTP struct {
		Port string `koanf:"port"`
	} `koanf:"http"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"db"`
	Redis struct {
		URL string `koanf:"url"`
	} `koanf:"redis"`
	Worker struct {
		Queues        string        `koanf:"queues"`
		ShutdownGrace time.Duration `koanf:"shutdown_grace"`
	} `koanf:"worker"`
	Outbox struct {
		PollInterval time.Duration `koanf:"poll_interval"`
		BatchSize    int           `koanf:"batch_size"`
	} `koanf:"outbox"`
	Job struct {
		MaxAttempts int `koanf:"max_attempts"`
	} `koanf:"job"`
	Telemetry struct {
		Endpoint    string `koanf:"exporter_otlp_endpoint"`
		ServiceName string `koanf:"service_name"`
	} `koanf:"otel"`
}

// configOptions declares every key this binary reads. A key absent from here
// cannot reach the config tree, and `.env.example` is its documentation.
func configOptions() config.Options {
	return config.Options{
		Defaults: map[string]any{
			"app.env":                     "local",
			"app.name":                    "fluentra-worker",
			"app.version":                 version,
			"http.port":                   "8081",
			"worker.queues":               "default:10,ai:4,media:2,notify:10,batch:2",
			"worker.shutdown_grace":       "30s",
			"outbox.poll_interval":        "1s",
			"outbox.batch_size":           100,
			"job.max_attempts":            5,
			"otel.exporter_otlp_endpoint": "localhost:4317",
			"otel.service_name":           "fluentra-worker",
		},
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
			{Name: "redis.url", DocSection: "docs/deployment/configuration.md#redis"},
		},
	}
}

func loadConfig(ctx context.Context) (workerConfig, error) {
	var target workerConfig
	if err := config.Load(ctx, configOptions(), &target); err != nil {
		return target, err
	}
	trimmed := strings.TrimPrefix(strings.TrimPrefix(target.Telemetry.Endpoint, "https://"), "http://")
	target.Telemetry.Endpoint = strings.TrimSuffix(trimmed, "/")
	const jobsDoc = "docs/deployment/configuration.md#jobs"
	if target.Worker.ShutdownGrace <= 0 {
		return target, fmt.Errorf(
			"config key worker.shutdown_grace must be a positive duration; see %s", jobsDoc)
	}
	if target.Outbox.PollInterval <= 0 {
		return target, fmt.Errorf(
			"config key outbox.poll_interval must be a positive duration; see %s", jobsDoc)
	}
	if target.Outbox.BatchSize <= 0 {
		return target, fmt.Errorf(
			"config key outbox.batch_size must be a positive integer; see %s", jobsDoc)
	}
	return target, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("worker process stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := loadConfig(ctx)
	if err != nil {
		return err
	}

	provider, err := telemetry.NewProvider(ctx, telemetry.Config{
		ServiceName: cfg.Telemetry.ServiceName,
		Version:     cfg.App.Version,
		Environment: cfg.App.Environment,
		CommitSHA:   commitSHA,
		Endpoint:    cfg.Telemetry.Endpoint,
	})
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	redisOpt, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		return err
	}
	redisClient := redis.NewClient(redisOpt)
	defer func() { _ = redisClient.Close() }()

	// Event bus and outbox publisher. Module consumers register their handlers
	// on the bus; the publisher is what moves committed events onto it.
	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	publisher := outbox.NewPublisher(pool, busDispatcher{bus: bus}, cfg.Outbox.BatchSize, cfg.Outbox.PollInterval).
		WithMaxAttempts(cfg.Job.MaxAttempts)
	go func() {
		if err := publisher.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("outbox publisher stopped", "error", err)
		}
	}()

	// Cron scheduler
	cron := job.NewCronScheduler(pool)
	cron.Start(ctx)

	// Metrics / Health HTTP server
	health := telemetry.NewHealthHandler(cfg.App.Version, readinessCheck(pool.Ping))
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health.Health)
	mux.HandleFunc("/ready", health.Ready)

	server := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		_ = server.ListenAndServe()
	}()

	slog.Info("fluentra worker running", "queues", job.DefaultQueues())

	<-ctx.Done()

	// WithoutCancel keeps trace values while dropping the cancellation that
	// just fired, so shutdown work is still traceable.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.Worker.ShutdownGrace)
	defer cancel()

	_ = server.Shutdown(shutdownCtx)
	_ = provider.Shutdown(shutdownCtx)

	slog.Info("fluentra worker stopped cleanly")
	return nil
}

type readinessCheck func(context.Context) error

func (check readinessCheck) Check(ctx context.Context) error { return check(ctx) }
