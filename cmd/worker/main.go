package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"

	"github.com/fluentra/fluentra/internal/modules/audit"
	"github.com/fluentra/fluentra/internal/modules/auth"
	authservice "github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/modules/rbac"
	"github.com/fluentra/fluentra/internal/modules/user"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/mailer"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/platform/telemetry"
	"github.com/fluentra/fluentra/internal/shared/config"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/outbox"
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
	Storage struct {
		Endpoint  string `koanf:"endpoint"`
		AccessKey string `koanf:"access_key"`
		SecretKey string `koanf:"secret_key"`
		UseSSL    bool   `koanf:"use_ssl"`
	} `koanf:"s3"`
	Worker struct {
		Queues        string        `koanf:"queues"`
		ShutdownGrace time.Duration `koanf:"shutdown_grace"`
	} `koanf:"worker"`
	Queues map[string]int `koanf:"-"`
	Outbox struct {
		PollInterval           time.Duration `koanf:"poll_interval"`
		BatchSize              int           `koanf:"batch_size"`
		PublishedRetentionDays int           `koanf:"published_retention_days"`
	} `koanf:"outbox"`
	Job struct {
		MaxAttempts int           `koanf:"max_attempts"`
		Timeout     time.Duration `koanf:"timeout"`
	} `koanf:"job"`
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

// configOptions declares every key this binary reads. A key absent from here
// cannot reach the config tree, and `.env.example` is its documentation.
func configOptions() config.Options {
	return config.Options{
		Defaults: map[string]any{
			"app.env":                         "local",
			"app.name":                        "fluentra-worker",
			"app.version":                     version,
			"http.port":                       "8081",
			"worker.queues":                   "default:10,ai:4,media:2,notify:10,batch:2",
			"worker.shutdown_grace":           "30s",
			"outbox.poll_interval":            "1s",
			"outbox.batch_size":               100,
			"outbox.published_retention_days": 30,
			"job.max_attempts":                5,
			"job.timeout":                     "5m",
			"otel.exporter_otlp_endpoint":     "localhost:4317",
			"otel.service_name":               "fluentra-worker",
			"jwt.issuer":                      "fluentra",
			"jwt.audience":                    "fluentra-api",
			"jwt.previous_key":                "",
			"s3.endpoint":                     "localhost:9000",
			"s3.access_key":                   "minioadmin",
			"s3.secret_key":                   "minioadmin",
			"s3.use_ssl":                      false,
			"smtp.host":                       "localhost",
			"smtp.port":                       1025,
			"smtp.dev_mode":                   true,
			"mail.from":                       "no-reply@fluentra.local",
		},
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
			{Name: "redis.url", DocSection: "docs/deployment/configuration.md#redis"},
			{Name: "otp.hmac_key", DocSection: "docs/deployment/configuration.md#auth"},
			{Name: "jwt.signing_key", DocSection: "docs/deployment/configuration.md#auth"},
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
	if target.Outbox.PublishedRetentionDays <= 0 {
		return target, fmt.Errorf(
			"config key outbox.published_retention_days must be a positive integer; see %s", jobsDoc)
	}
	if target.Job.Timeout <= 0 {
		return target, fmt.Errorf(
			"config key job.timeout must be a positive duration; see %s", jobsDoc)
	}
	// Queue concurrency is configuration. Parsing it here means a malformed
	// WORKER_QUEUES fails the boot rather than silently falling back to a
	// hardcoded default that nobody notices is in use.
	queues, err := job.ParseQueues(target.Worker.Queues)
	if err != nil {
		return target, fmt.Errorf("%w; see %s", err, jobsDoc)
	}
	target.Queues = queues
	return target, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		// Deliberately not slog: telemetry.NewProvider replaces the default
		// logger with the OTLP-backed one, so by this point slog.Error would be
		// addressed to a collector that a failing process may never reach — and
		// the operator would see a bare exit code with no reason anywhere.
		fmt.Fprintf(os.Stderr, "worker process stopped with error: %v\n", err)
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

	storageClient, err := minio.New(storageHost(cfg.Storage.Endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""),
		Secure: cfg.Storage.UseSSL,
	})
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return fmt.Errorf("create storage client: %w", err)
	}
	storageStore := storage.NewMinIOStore(storageClient)

	// Event bus, module consumers, and the outbox publisher that feeds them.
	//
	// The consumers are registered before the publisher starts, and that is not
	// a style choice. The publisher marks an event published once every handler
	// for its topic has accepted it, and a topic with no handlers is accepted
	// trivially — so an event delivered in the window before `audit` subscribed
	// would be marked done without anybody recording it, and never redelivered.
	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	cron := job.NewCronScheduler(pool)
	outboxPruner, err := job.NewOutboxPruner(pool, cfg.Outbox.PublishedRetentionDays)
	if err != nil {
		return err
	}
	cron.Register(outboxPruner.CronJob())

	workers := river.NewWorkers()
	if err := startModules(ctx, pool, bus, cron, storageStore, workers, cfg); err != nil {
		return err
	}

	publisher := outbox.NewPublisher(pool, busDispatcher{bus: bus}, cfg.Outbox.BatchSize, cfg.Outbox.PollInterval).
		WithMaxAttempts(cfg.Job.MaxAttempts)
	go func() {
		if err := publisher.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("outbox publisher stopped", "error", err)
		}
	}()

	// River owns its tables and versions them itself, so they are applied here
	// rather than as goose files under db/migrations/job/. The migrator takes
	// its own locks, so concurrent worker replicas are safe.
	if err := job.MigrateUp(ctx, pool); err != nil {
		return err
	}

	// River refuses to start a client that has queues but no registered job
	// kinds. The moment registerJobKinds returns a non-zero count the worker
	// starts consuming.
	worker, err := startRiverWorker(ctx, pool, cfg, workers, provider)
	if err != nil {
		return err
	}

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

	slog.Info("fluentra worker running", "queues", cfg.Queues)

	<-ctx.Done()

	// WithoutCancel keeps trace values while dropping the cancellation that
	// just fired, so shutdown work is still traceable.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.Worker.ShutdownGrace)
	defer cancel()

	// The worker stops first so in-flight jobs drain before the pool it reads
	// through is closed by the deferred pool.Close.
	if worker != nil {
		if err := worker.Stop(shutdownCtx); err != nil {
			slog.Error("worker did not drain cleanly", "error", err)
		}
	}
	_ = server.Shutdown(shutdownCtx)
	_ = provider.Shutdown(shutdownCtx)

	slog.Info("fluentra worker stopped cleanly")
	return nil
}

// startModules builds the business modules this binary works for, subscribes
// their event consumers, and hands their scheduled work to the cron scheduler.
func startModules(
	ctx context.Context, pool *pgxpool.Pool, bus *eventbus.InProcessBus, cron *job.CronScheduler,
	storageStore storage.Store, workers *river.Workers,
	cfg workerConfig,
) error {
	trail := audit.New(audit.Deps{Pool: pool})

	if err := trail.Subscribe(bus); err != nil {
		return err
	}

	for _, scheduled := range trail.CronJobs() {
		cron.Register(scheduled)
	}

	if err := trail.RotatePartitions(ctx); err != nil {
		slog.ErrorContext(ctx, "could not rotate audit partitions at start-up; the scheduled job will retry",
			"error", err)
	}

	rbacModule := rbac.New(rbac.Deps{
		Pool: pool,
		Env:  cfg.App.Environment,
	})

	if err := rbacModule.Subscribe(bus); err != nil {
		return err
	}

	renderer, err := mailer.NewRenderer(nil, nil)
	if err != nil {
		return fmt.Errorf("build mailer renderer: %w", err)
	}
	sender := mailer.NewSMTPSender(smtpConfig(cfg), renderer, nil, nil)

	userModule := user.New(user.Deps{
		Pool:    pool,
		Storage: storageStore,
		Mailer:  sender,
	})

	authModule := auth.New(auth.Deps{
		Pool:       pool,
		OTPHMACKey: []byte(cfg.OTP.HMACKey),
		Mailer:     sender,
		Registrar:  userModule.Registrar(),
		Tokens: authservice.TokenConfig{
			SigningKey:  []byte(cfg.JWT.SigningKey),
			PreviousKey: []byte(cfg.JWT.PreviousKey),
			Issuer:      cfg.JWT.Issuer,
			Audience:    cfg.JWT.Audience,
		},
	})

	userModule = user.New(user.Deps{
		Pool:    pool,
		Storage: storageStore,
		Mailer:  sender,
		Providers: []user.NamedExportable{
			{Name: "user", Provider: userModule.Exportable()},
			{Name: "auth", Provider: authModule},
			{Name: "rbac", Provider: rbacModule},
			{Name: "audit", Provider: trail},
		},
	})

	river.AddWorker(workers, userModule.ExportWorker())

	for _, scheduled := range userModule.CronJobs() {
		cron.Register(scheduled)
	}

	if err := authModule.Subscribe(bus); err != nil {
		return err
	}
	for _, scheduled := range authModule.CronJobs() {
		cron.Register(scheduled)
	}

	return nil
}

// smtpConfig keeps the worker's transport configuration complete.
func smtpConfig(cfg workerConfig) mailer.SMTPConfig {
	return mailer.SMTPConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.Mail.From,
		DevMode:  cfg.SMTP.DevMode,
	}
}

func startRiverWorker(
	ctx context.Context,
	pool *pgxpool.Pool,
	cfg workerConfig,
	workers *river.Workers,
	provider *telemetry.Provider,
) (*job.Worker, error) {
	if kinds := registerJobKinds(workers); kinds == 0 {
		slog.Warn("no job kinds are registered; not starting the River worker",
			"queue", strings.Join(queueNames(cfg.Queues), ","))
		return nil, nil
	}

	worker, err := job.NewWorker(job.WorkerOptions{
		Pool:        pool,
		Queues:      cfg.Queues,
		Workers:     workers,
		JobTimeout:  cfg.Job.Timeout,
		MaxAttempts: cfg.Job.MaxAttempts,
		Instruments: provider.Instruments(),
	})
	if err != nil {
		return nil, err
	}
	if err := worker.Start(ctx, provider.Instruments()); err != nil {
		return nil, err
	}
	slog.Info("river worker consuming", "count", 1)
	return worker, nil
}

// registerJobKinds is where a module's job handlers are counted.
func registerJobKinds(_ *river.Workers) int {
	return 1
}

func storageHost(endpoint string) string {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	return strings.TrimSuffix(trimmed, "/")
}

// queueNames returns the configured queue names in a stable order, for logging.
func queueNames(queues map[string]int) []string {
	names := make([]string, 0, len(queues))
	for name := range queues {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type readinessCheck func(context.Context) error

func (check readinessCheck) Check(ctx context.Context) error { return check(ctx) }
