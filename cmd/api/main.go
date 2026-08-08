package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/fluentra/fluentra/internal/platform/telemetry"
	"github.com/fluentra/fluentra/internal/shared/config"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

const shutdownGrace = 30 * time.Second

var (
	version   = "dev"
	commitSHA = "unknown"
)

type applicationConfig struct {
	App struct {
		Environment string `koanf:"env"`
		Name        string `koanf:"name"`
		Version     string `koanf:"version"`
	} `koanf:"app"`
	HTTP struct {
		Port           string `koanf:"port"`
		ReadTimeout    string `koanf:"read_timeout"`
		WriteTimeout   string `koanf:"write_timeout"`
		IdleTimeout    string `koanf:"idle_timeout"`
		RequestTimeout string `koanf:"request_timeout"`
	} `koanf:"http"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"db"`
	Redis struct {
		URL string `koanf:"url"`
	} `koanf:"redis"`
	Storage struct {
		Endpoint  string `koanf:"endpoint"`
		AccessKey string `koanf:"access_key"`
		SecretKey string `koanf:"secret_key"`
		UseSSL    bool   `koanf:"use_ssl"`
	} `koanf:"s3"`
	Telemetry struct {
		Endpoint    string `koanf:"exporter_otlp_endpoint"`
		ServiceName string `koanf:"service_name"`
	} `koanf:"otel"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("API server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := loadConfig(ctx)
	if err != nil {
		return err
	}

	provider, err := telemetry.NewProvider(ctx, telemetry.Config{ServiceName: cfg.Telemetry.ServiceName, Version: cfg.App.Version, Environment: cfg.App.Environment, CommitSHA: commitSHA, Endpoint: cfg.Telemetry.Endpoint})
	if err != nil {
		return err
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("parse database configuration: %w", err)
	}
	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithDisableConnectionDetailsInAttributes())
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}

	redisOptions, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		pool.Close()
		return fmt.Errorf("parse Redis configuration: %w", err)
	}
	redisClient := redis.NewClient(redisOptions)
	if err := redisotel.InstrumentTracing(redisClient); err != nil {
		pool.Close()
		return fmt.Errorf("instrument Redis tracing: %w", err)
	}

	if _, err := minio.New(cfg.Storage.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""), Secure: cfg.Storage.UseSSL}); err != nil {
		_ = redisClient.Close()
		pool.Close()
		return fmt.Errorf("create storage client: %w", err)
	}

	requestTimeout, err := time.ParseDuration(cfg.HTTP.RequestTimeout)
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return fmt.Errorf("parse request timeout: %w", err)
	}
	health := telemetry.NewHealthHandler(cfg.App.Version, readinessCheck(pool.Ping), readinessCheck(redisPing(redisClient)))
	server := &http.Server{
		Addr: ":" + cfg.HTTP.Port,
		Handler: httpx.NewRouter(httpx.RouterDependencies{
			Database:       pool,
			Cache:          redisClient,
			Health:         health.Health,
			Ready:          health.Ready,
			Version:        health.Version,
			RequestTimeout: requestTimeout,
			Middleware: func(next http.Handler) http.Handler {
				return telemetry.Middleware(routePattern, next)
			},
		}),
		ReadTimeout:  mustDuration(cfg.HTTP.ReadTimeout),
		WriteTimeout: mustDuration(cfg.HTTP.WriteTimeout),
		IdleTimeout:  mustDuration(cfg.HTTP.IdleTimeout),
	}

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()

	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			_ = redisClient.Close()
			pool.Close()
			return fmt.Errorf("serve HTTP: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	redisErr := redisClient.Close()
	pool.Close()
	telemetryErr := provider.Shutdown(shutdownCtx)
	return errors.Join(shutdownErr, redisErr, telemetryErr)
}

func loadConfig(ctx context.Context) (applicationConfig, error) {
	var target applicationConfig
	err := config.Load(ctx, config.Options{
		Defaults: map[string]any{
			"app.env":                     "local",
			"app.name":                    "fluentra",
			"app.version":                 version,
			"http.port":                   "8080",
			"http.read_timeout":           "15s",
			"http.write_timeout":          "30s",
			"http.idle_timeout":           "120s",
			"http.request_timeout":        "30s",
			"otel.exporter_otlp_endpoint": "localhost:4317",
			"otel.service_name":           "fluentra-api",
		},
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
			{Name: "redis.url", DocSection: "docs/deployment/configuration.md#redis"},
			{Name: "s3.endpoint", DocSection: "docs/deployment/configuration.md#storage"},
			{Name: "s3.access_key", DocSection: "docs/deployment/configuration.md#storage"},
			{Name: "s3.secret_key", DocSection: "docs/deployment/configuration.md#storage"},
		},
	}, &target)
	return target, err
}

func mustDuration(value string) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return duration
}

type readinessCheck func(context.Context) error

func (check readinessCheck) Check(ctx context.Context) error { return check(ctx) }

func redisPing(client *redis.Client) func(context.Context) error {
	return func(ctx context.Context) error { return client.Ping(ctx).Err() }
}

func routePattern(request *http.Request) string {
	if routeContext := chi.RouteContext(request.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return request.URL.Path
}
