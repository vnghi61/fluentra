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
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	authdomain "github.com/fluentra/fluentra/internal/modules/auth/domain"
	authservice "github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/modules/auth/service/oauth/google"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/mailer"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/platform/telemetry"
	"github.com/fluentra/fluentra/internal/shared/config"
	"github.com/fluentra/fluentra/internal/shared/httpx"
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
	// CORS_ALLOWED_ORIGINS, a comma-separated allowlist. Empty means the
	// same-origin deployment and no cross-origin client is expected; the local
	// dev split sets it to the Vite dev server's origin.
	CORS struct {
		AllowedOrigins string `koanf:"allowed_origins"`
	} `koanf:"cors"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"db"`
	Redis struct {
		URL string `koanf:"url"`
	} `koanf:"redis"`
	Storage struct {
		Endpoint      string `koanf:"endpoint"`
		AccessKey     string `koanf:"access_key"`
		SecretKey     string `koanf:"secret_key"`
		Region        string `koanf:"region"`
		UseSSL        bool   `koanf:"use_ssl"`
		UsePostPolicy bool   `koanf:"use_post_policy"`
	} `koanf:"s3"`
	Telemetry struct {
		Endpoint    string `koanf:"exporter_otlp_endpoint"`
		ServiceName string `koanf:"service_name"`
	} `koanf:"otel"`
	OTP struct {
		HMACKey                string `koanf:"hmac_key"`
		IssuePerIPPerHour      int    `koanf:"issue_per_ip_per_hour"`
		IssuePerSubjectPerHour int    `koanf:"issue_per_subject_per_hour"`
	} `koanf:"otp"`
	// PASSWORD_RESET_TTL, under the first-underscore-becomes-a-dot rule.
	Password struct {
		ResetTTL time.Duration `koanf:"reset_ttl"`
	} `koanf:"password"`
	// SESSION_* becomes `session.*`, under the same first-underscore rule.
	Session struct {
		IdleWindow        time.Duration `koanf:"idle_window"`
		IdleWindowTrusted time.Duration `koanf:"idle_window_trusted"`
		AbsoluteTTL       time.Duration `koanf:"absolute_ttl"`
		IdleWindowAdmin   time.Duration `koanf:"idle_window_admin"`
		AbsoluteTTLAdmin  time.Duration `koanf:"absolute_ttl_admin"`
	} `koanf:"session"`
	// RATE_LIMIT_* becomes `rate.limit_*`: the convention replaces the FIRST
	// underscore with a dot and leaves the rest alone.
	Rate struct {
		LimitAnonPerMin    int `koanf:"limit_anon_per_min"`
		LimitUserPerMin    int `koanf:"limit_user_per_min"`
		LimitAuthPerMin    int `koanf:"limit_auth_per_min"`
		LimitUploadPerHour int `koanf:"limit_upload_per_hour"`
	} `koanf:"rate"`
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
	// REFRESH_TOKEN_TTL, under the same first-underscore-becomes-a-dot rule.
	Refresh struct {
		TokenTTL time.Duration `koanf:"token_ttl"`
	} `koanf:"refresh"`
	// OAUTH_* becomes `oauth.*`, under the same first-underscore rule — so
	// OAUTH_GOOGLE_CLIENT_ID is `oauth.google_client_id` and not
	// `oauth.google.client_id`. Getting this wrong produces a key that is read
	// and can never be set from the environment.
	OAuth struct {
		GoogleEnabled      bool          `koanf:"google_enabled"`
		GoogleClientID     string        `koanf:"google_client_id"`
		GoogleClientSecret string        `koanf:"google_client_secret"`
		GoogleRedirectURL  string        `koanf:"google_redirect_url"`
		GoogleJWKSURL      string        `koanf:"google_jwks_url"`
		GoogleIssuer       string        `koanf:"google_issuer"`
		JWKSCacheTTL       time.Duration `koanf:"jwks_cache_ttl"`
		StateTTL           time.Duration `koanf:"state_ttl"`
	} `koanf:"oauth"`
	Mail struct {
		Transport string `koanf:"transport"`
		From      string `koanf:"from"`
	} `koanf:"mail"`
	SMTP struct {
		Host     string `koanf:"host"`
		Port     int    `koanf:"port"`
		Username string `koanf:"username"`
		Password string `koanf:"password"`
		DevMode  bool   `koanf:"dev_mode"`
	} `koanf:"smtp"`
	Resend struct {
		APIKey string `koanf:"api_key"`
	} `koanf:"resend"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("API server stopped", "error", err)
		fmt.Fprintf(os.Stderr, "API server error: %v\n", err)
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
	poolConfig.ConnConfig.Tracer = telemetry.NewDBQueryTracer(
		otelpgx.NewTracer(otelpgx.WithDisableConnectionDetailsInAttributes()),
		provider.Instruments(),
	)
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	if _, err := provider.Instruments().ObserveDBPoolConnections(pool); err != nil {
		// Losing the gauge is not a reason to refuse startup; queries still work.
		slog.Warn("db pool gauge not registered", "error", err)
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
		Region: cfg.Storage.Region,
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
	// One limiter shared by every class. The classes differ in their keys and
	// their budgets, not in their counting.
	rateLimiter := httpx.RateLimit(httpx.RateLimitConfig{
		Limiter:             rateLimiterAdapter{inner: cache.NewRedisLimiter(redisClient)},
		AnonymousPerMinute:  cfg.Rate.LimitAnonPerMin,
		UserPerMinute:       cfg.Rate.LimitUserPerMin,
		CredentialPerMinute: cfg.Rate.LimitAuthPerMin,
		UploadPerHour:       cfg.Rate.LimitUploadPerHour,
		ChallengeIPPerHour:  cfg.OTP.IssuePerIPPerHour,
		Env:                 cfg.App.Environment,
	})

	jobClient, err := job.NewClientFromPool(pool)
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return fmt.Errorf("create job client: %w", err)
	}

	modules := newIdentity(identityDeps{
		Pool:        pool,
		Redis:       redisClient,
		Cache:       cache.NewRedisCache[[]string](redisClient),
		Limiter:     cache.NewRedisLimiter(redisClient),
		Storage:     newStorageStore(storageClient, cfg.Storage.UsePostPolicy),
		Enqueuer:    jobClient,
		Instruments: provider.Instruments(),
		Env:         cfg.App.Environment,
		OTPHMACKey:  []byte(cfg.OTP.HMACKey),
		Tokens: authservice.TokenConfig{
			SigningKey:  []byte(cfg.JWT.SigningKey),
			PreviousKey: []byte(cfg.JWT.PreviousKey),
			Issuer:      cfg.JWT.Issuer,
			Audience:    cfg.JWT.Audience,
			AccessTTL:   cfg.Access.TokenTTL,
		},
		RefreshTTL:              cfg.Refresh.TokenTTL,
		PasswordResetTTL:        cfg.Password.ResetTTL,
		IssuesPerIPPerHour:      cfg.OTP.IssuePerIPPerHour,
		IssuesPerSubjectPerHour: cfg.OTP.IssuePerSubjectPerHour,
		RateLimit:               rateLimiter,
		// OAUTH_GOOGLE_ENABLED gates the credentials rather than the routes. The
		// operations stay mounted either way and answer the same refusal a bad
		// code gets, because a deployment with the provider switched off should
		// look to a client like one Google is not cooperating with — not like a
		// different version of the API.
		Google:        googleOptions(cfg),
		OAuthStateTTL: cfg.OAuth.StateTTL,
		Windows: authdomain.WindowConfig{
			Idle:          cfg.Session.IdleWindow,
			IdleTrusted:   cfg.Session.IdleWindowTrusted,
			Absolute:      cfg.Session.AbsoluteTTL,
			IdleAdmin:     cfg.Session.IdleWindowAdmin,
			AbsoluteAdmin: cfg.Session.AbsoluteTTLAdmin,
		},
		// A separate typed cache from the permission one. They share the Redis
		// client but not the value type, and Cache[T] is generic per type.
		Denylist: cache.NewRedisCache[bool](redisClient),
		Mailer:   newAPIMailSender(cfg, pool),
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
			CORS:           httpx.CORS(splitList(cfg.CORS.AllowedOrigins)),
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
			"app.env":                        "local",
			"app.name":                       "fluentra",
			"app.version":                    version,
			"http.port":                      "8080",
			"http.read_timeout":              "15s",
			"http.write_timeout":             defaultTimeout,
			"http.idle_timeout":              "120s",
			"http.request_timeout":           defaultTimeout,
			"http.shutdown_grace":            defaultTimeout,
			"http.trusted_proxies":           "",
			"cors.allowed_origins":           "",
			"s3.use_ssl":                     false,
			"s3.use_post_policy":             true,
			"s3.region":                      "us-east-1",
			"otel.exporter_otlp_endpoint":    defaultOTLPEndpoint,
			"otel.service_name":              "fluentra-api",
			"smtp.host":                      "localhost",
			"smtp.port":                      1025,
			"smtp.dev_mode":                  true,
			"mail.transport":                 "smtp",
			"mail.from":                      "no-reply@fluentra.local",
			"resend.api_key":                 "",
			"jwt.issuer":                     "fluentra",
			"jwt.audience":                   "fluentra-api",
			"jwt.previous_key":               "",
			"access.token_ttl":               "15m",
			"refresh.token_ttl":              "720h",
			"password.reset_ttl":             "30m",
			"session.idle_window":            "720h",
			"session.idle_window_trusted":    "2160h",
			"session.absolute_ttl":           "4320h",
			"session.idle_window_admin":      "12h",
			"session.absolute_ttl_admin":     "168h",
			"rate.limit_anon_per_min":        60,
			"rate.limit_user_per_min":        600,
			"rate.limit_auth_per_min":        5,
			"rate.limit_upload_per_hour":     30,
			"otp.issue_per_ip_per_hour":      20,
			"otp.issue_per_subject_per_hour": 3,
			"oauth.google_enabled":           false,
			"oauth.google_jwks_url":          "https://www.googleapis.com/oauth2/v3/certs",
			"oauth.google_issuer":            "https://accounts.google.com",
			"oauth.jwks_cache_ttl":           "6h",
			"oauth.state_ttl":                "10m",
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

// googleOptions turns the OAUTH_GOOGLE_* keys into the provider's options.
//
// A disabled provider gets no credentials rather than no routes. The operations
// stay mounted and refuse, which is what a client should see from a deployment
// Google is not configured for — the alternative, unmounting them, makes the API
// surface depend on configuration and turns a missing key into a 404 that looks
// like a version mismatch.
//
// The absence of credentials while enabled is a warning and not a fatal error.
// It is a misconfiguration of one optional sign-in method, and refusing to start
// the whole API over it would take down password sign-in as well — which is the
// one every learner has.
func googleOptions(cfg applicationConfig) google.Options {
	if !cfg.OAuth.GoogleEnabled {
		return google.Options{}
	}
	if cfg.OAuth.GoogleClientID == "" || cfg.OAuth.GoogleClientSecret == "" {
		slog.Warn("google sign-in is enabled but has no client credentials; every attempt will be refused",
			"key", "OAUTH_GOOGLE_CLIENT_ID")
	}
	return google.Options{
		ClientID:     cfg.OAuth.GoogleClientID,
		ClientSecret: cfg.OAuth.GoogleClientSecret,
		RedirectURL:  cfg.OAuth.GoogleRedirectURL,
		Issuer:       cfg.OAuth.GoogleIssuer,
		JWKSURL:      cfg.OAuth.GoogleJWKSURL,
		JWKSTTL:      cfg.OAuth.JWKSCacheTTL,
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

// newStorageStore builds the storage facade, choosing the constructor that
// matches the store's capabilities. Object stores that do not implement S3 POST
// policy (Cloudflare R2) must be configured with s3.use_post_policy=false.
func newStorageStore(client *minio.Client, usePostPolicy bool) storage.Store {
	if !usePostPolicy {
		return storage.NewMinIOStoreNoPostPolicy(client)
	}
	return storage.NewMinIOStore(client)
}

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

// newAPIMailSender builds the appropriate Sender based on MAIL_TRANSPORT.
// "resend" uses the Resend HTTP API (port 443, works on Render Free).
// Everything else falls back to SMTP (works locally with Mailpit).
func newAPIMailSender(cfg applicationConfig, pool *pgxpool.Pool) mailer.Sender {
	renderer, err := mailer.NewRenderer(nil, nil)
	if err != nil {
		panic("mailer.NewRenderer: " + err.Error())
	}
	var suppressions mailer.SuppressionStore
	var recorder mailer.DeliveryRecorder
	if pool != nil {
		suppressions = mailer.NewPostgresSuppressionStore(pool)
		recorder = mailer.NewPostgresRecorder(pool)
	}
	if cfg.Mail.Transport == "resend" {
		return mailer.NewResendSender(mailer.ResendConfig{
			APIKey: cfg.Resend.APIKey,
			From:   cfg.Mail.From,
		}, renderer, suppressions, recorder)
	}
	return mailer.NewSMTPSender(mailer.SMTPConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.Mail.From,
		DevMode:  cfg.SMTP.DevMode,
	}, renderer, suppressions, recorder)
}
