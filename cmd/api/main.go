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

	"github.com/exaring/otelpgx"
	authservice "github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/platform/mailer"
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

var (
	version   = "dev"
	commitSHA = "unknown"
)

// applicationConfig mirrors `.env.example`. Every field maps to exactly one
// environment variable under the convention documented in shared/config.
type applicationConfig struct {
	App struct {
		Environment string `koanf:"env"`
		Name        string `koanf:"name"`
		Version     string `koanf:"version"`
	} `koanf:"app"`
	HTTP struct {
		Port           string        `koanf:"port"`
		ReadTimeout    time.Duration `koanf:"read_timeout"`
		WriteTimeout   time.Duration `koanf:"write_timeout"`
		IdleTimeout    time.Duration `koanf:"idle_timeout"`
		RequestTimeout time.Duration `koanf:"request_timeout"`
		ShutdownGrace  time.Duration `koanf:"shutdown_grace"`
		TrustedProxies string        `koanf:"trusted_proxies"`
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
	OTP struct {
		HMACKey string `koanf:"hmac_key"`
	} `koanf:"otp"`
	JWT struct {
		SigningKey  string `koanf:"signing_key"`
		PreviousKey string `koanf:"previous_key"`
		Issuer      string `koanf:"issuer"`
		Audience    string `koanf:"audience"`
	} `koanf:"jwt"`
	// ACCESS_TOKEN_TTL maps to `access.token_ttl`, not `access_token.ttl`: the
	// convention replaces the FIRST underscore with a dot and leaves the rest
	// alone (see shared/config). Getting this wrong produces a key that is read
	// but can never be set from the environment.
	Access struct {
		TokenTTL time.Duration `koanf:"token_ttl"`
	} `koanf:"access"`
	Mail struct {
		From string `koanf:"from"`
	} `koanf:"mail"`
	SMTP struct {
		Host     string `koanf:"host"`
		Port     int    `koanf:"port"`
		Username string `koanf:"username"`
		Password string `koanf:"password"`
		DevMode  bool   `koanf:"dev_mode"`
	} `koanf:"smtp"`
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

	storageClient, err := minio.New(storageHost(cfg.Storage.Endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""),
		Secure: cfg.Storage.UseSSL,
	})
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return fmt.Errorf("create storage client: %w", err)
	}

	clientIP, err := httpx.NewClientIPResolver(splitList(cfg.HTTP.TrustedProxies))
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return fmt.Errorf("config key http.trusted_proxies: %w", err)
	}

	// The business modules, after the infrastructure they are built over and
	// before the router that mounts them (ARCHITECTURE.md §6.3).
	//
	// The cache is passed as rbac's own narrow interface rather than the Redis
	// client: a permission check falls through to the database when the cache
	// is unavailable, and the module is what decides that, not this file.
	modules := newIdentity(identityDeps{
		Pool:       pool,
		Cache:      cache.NewRedisCache[[]string](redisClient),
		Limiter:    cache.NewRedisLimiter(redisClient),
		Env:        cfg.App.Environment,
		OTPHMACKey: []byte(cfg.OTP.HMACKey),
		Tokens: authservice.TokenConfig{
			SigningKey:  []byte(cfg.JWT.SigningKey),
			PreviousKey: []byte(cfg.JWT.PreviousKey),
			Issuer:      cfg.JWT.Issuer,
			Audience:    cfg.JWT.Audience,
			AccessTTL:   cfg.Access.TokenTTL,
		},
		// A separate typed cache from the permission one. They share the Redis
		// client but not the value type, and Cache[T] is generic per type.
		Denylist: cache.NewRedisCache[bool](redisClient),
		SMTP: mailer.SMTPConfig{
			Host:     cfg.SMTP.Host,
			Port:     cfg.SMTP.Port,
			Username: cfg.SMTP.Username,
			Password: cfg.SMTP.Password,
			From:     cfg.Mail.From,
			DevMode:  cfg.SMTP.DevMode,
		},
	})

	health := telemetry.NewHealthHandler(cfg.App.Version,
		readinessCheck(pool.Ping),
		readinessCheck(redisPing(redisClient)),
		readinessCheck(storagePing(storageClient)),
	)
	server := &http.Server{
		Addr: ":" + cfg.HTTP.Port,
		Handler: httpx.NewRouter(httpx.RouterDependencies{
			Database:       pool,
			Cache:          redisClient,
			Health:         health.Health,
			Ready:          health.Ready,
			Version:        health.Version,
			RequestTimeout: cfg.HTTP.RequestTimeout,
			ClientIP:       clientIP,
			Modules:        modules.Routes,
			Middleware: func(next http.Handler) http.Handler {
				return telemetry.Middleware(routePattern, next)
			},
		}),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
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

	// WithoutCancel keeps the trace and request values while dropping the
	// cancellation that just fired, so shutdown work is still traceable.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.HTTP.ShutdownGrace)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	redisErr := redisClient.Close()
	pool.Close()
	telemetryErr := provider.Shutdown(shutdownCtx)
	return errors.Join(shutdownErr, redisErr, telemetryErr)
}

const (
	// defaultTimeout is the shared default for the request-scoped HTTP timeouts.
	defaultTimeout = "30s"
	// defaultOTLPEndpoint is the collector address in the local compose stack.
	defaultOTLPEndpoint = "localhost:4317"
	// docSectionStorage is the anchor a missing S3 key points the operator at.
	docSectionStorage = "docs/deployment/configuration.md#storage"
)

// configOptions declares every key this binary reads. A key absent from here
// cannot reach the config tree, and `.env.example` is its documentation.
func configOptions() config.Options {
	return config.Options{
		Defaults: map[string]any{
			"app.env":                     "local",
			"app.name":                    "fluentra",
			"app.version":                 version,
			"http.port":                   "8080",
			"http.read_timeout":           "15s",
			"http.write_timeout":          defaultTimeout,
			"http.idle_timeout":           "120s",
			"http.request_timeout":        defaultTimeout,
			"http.shutdown_grace":         defaultTimeout,
			"http.trusted_proxies":        "",
			"s3.use_ssl":                  false,
			"otel.exporter_otlp_endpoint": defaultOTLPEndpoint,
			"otel.service_name":           "fluentra-api",
			"smtp.host":                   "localhost",
			"smtp.port":                   1025,
			"smtp.dev_mode":               true,
			"mail.from":                   "no-reply@fluentra.local",
			"jwt.issuer":                  "fluentra",
			"jwt.audience":                "fluentra-api",
			"jwt.previous_key":            "",
			"access.token_ttl":            "15m",
		},
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
			{Name: "redis.url", DocSection: "docs/deployment/configuration.md#redis"},
			{Name: "s3.endpoint", DocSection: docSectionStorage},
			{Name: "s3.access_key", DocSection: docSectionStorage},
			{Name: "s3.secret_key", DocSection: docSectionStorage},
			{Name: "otp.hmac_key", DocSection: "docs/deployment/configuration.md#auth"},
			{Name: "jwt.signing_key", DocSection: "docs/deployment/configuration.md#auth"},
		},
	}
}

func loadConfig(ctx context.Context) (applicationConfig, error) {
	var target applicationConfig
	if err := config.Load(ctx, configOptions(), &target); err != nil {
		return target, err
	}
	target.Telemetry.Endpoint = grpcEndpoint(target.Telemetry.Endpoint)
	for _, timeout := range []struct {
		key   string
		value time.Duration
	}{
		{"http.read_timeout", target.HTTP.ReadTimeout},
		{"http.write_timeout", target.HTTP.WriteTimeout},
		{"http.idle_timeout", target.HTTP.IdleTimeout},
		{"http.request_timeout", target.HTTP.RequestTimeout},
		{"http.shutdown_grace", target.HTTP.ShutdownGrace},
	} {
		if timeout.value <= 0 {
			return target, fmt.Errorf(
				"config key %s must be a positive duration; see docs/deployment/configuration.md#app",
				timeout.key)
		}
	}
	return target, nil
}

// grpcEndpoint strips a URL scheme so an OTLP endpoint documented as
// `http://host:4317` reaches the gRPC exporter as `host:4317`.
func grpcEndpoint(endpoint string) string {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	return strings.TrimSuffix(trimmed, "/")
}

// splitList parses a comma-separated config value into trimmed entries.
func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	entries := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			entries = append(entries, trimmed)
		}
	}
	return entries
}

// storageHost strips a URL scheme so an S3 endpoint documented as
// `http://host:9000` reaches minio-go as `host:9000`.
func storageHost(endpoint string) string { return grpcEndpoint(endpoint) }

type readinessCheck func(context.Context) error

func (check readinessCheck) Check(ctx context.Context) error { return check(ctx) }

func redisPing(client *redis.Client) func(context.Context) error {
	return func(ctx context.Context) error { return client.Ping(ctx).Err() }
}

func storagePing(client *minio.Client) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := client.ListBuckets(ctx)
		return err
	}
}

func routePattern(request *http.Request) string {
	if routeContext := chi.RouteContext(request.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return request.URL.Path
}
