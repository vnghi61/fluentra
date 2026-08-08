package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/telemetry"
	"github.com/fluentra/fluentra/internal/shared/config"
	"github.com/fluentra/fluentra/internal/shared/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	version   = "dev"
	commitSHA = "unknown"
)

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
	Telemetry struct {
		Endpoint    string `koanf:"exporter_otlp_endpoint"`
		ServiceName string `koanf:"service_name"`
	} `koanf:"otel"`
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
	var cfg workerConfig
	if err := config.Load(ctx, config.Options{
		Defaults: map[string]any{
			"app.env":                     "local",
			"app.name":                    "fluentra-worker",
			"app.version":                 version,
			"http.port":                   "8081",
			"otel.exporter_otlp_endpoint": "localhost:4317",
			"otel.service_name":           "fluentra-worker",
		},
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
			{Name: "redis.url", DocSection: "docs/deployment/configuration.md#redis"},
		},
	}, &cfg); err != nil {
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
	defer redisClient.Close()

	// Outbox publisher loop
	publisher := outbox.NewPublisher(pool, nil, 50, 500*time.Millisecond)
	go func() {
		if err := publisher.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("outbox publisher stopped", "error", err)
		}
	}()

	// Cron scheduler
	cron := job.NewCronScheduler(pool)
	cron.Start(ctx)

	// Metrics / Health HTTP server
	health := telemetry.NewHealthHandler(cfg.App.Version)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health.Health)
	mux.HandleFunc("/ready", health.Ready)

	server := &http.Server{
		Addr:    ":" + cfg.HTTP.Port,
		Handler: mux,
	}

	go func() {
		_ = server.ListenAndServe()
	}()

	slog.Info("fluentra worker running", "queues", job.DefaultQueues())

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = server.Shutdown(shutdownCtx)
	_ = provider.Shutdown(shutdownCtx)

	slog.Info("fluentra worker stopped cleanly")
	return nil
}
