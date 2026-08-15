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
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"

	"github.com/fluentra/fluentra/internal/modules/audit"
	"github.com/fluentra/fluentra/internal/modules/auth"
	authservice "github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/modules/user"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/mailer"
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
	Worker struct {
		Queues        string        `koanf:"queues"`
		ShutdownGrace time.Duration `koanf:"shutdown_grace"`
	} `koanf:"worker"`
	Queues map[string]int `koanf:"-"`
	Outbox struct {
		PollInterval time.Duration `koanf:"poll_interval"`
		BatchSize    int           `koanf:"batch_size"`
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
			"app.env":                     "local",
			"app.name":                    "fluentra-worker",
			"app.version":                 version,
			"http.port":                   "8081",
			"worker.queues":               "default:10,ai:4,media:2,notify:10,batch:2",
			"worker.shutdown_grace":       "30s",
			"outbox.poll_interval":        "1s",
			"outbox.batch_size":           100,
			"job.max_attempts":            5,
			"job.timeout":                 "5m",
			"otel.exporter_otlp_endpoint": "localhost:4317",
			"otel.service_name":           "fluentra-worker",
			"jwt.issuer":                  "fluentra",
			"jwt.audience":                "fluentra-api",
			"jwt.previous_key":            "",
			"smtp.host":                   "localhost",
			"smtp.port":                   1025,
			"smtp.dev_mode":               true,
			"mail.from":                   "no-reply@fluentra.local",
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

	// Event bus, module consumers, and the outbox publisher that feeds them.
	//
	// The consumers are registered before the publisher starts, and that is not
	// a style choice. The publisher marks an event published once every handler
	// for its topic has accepted it, and a topic with no handlers is accepted
	// trivially — so an event delivered in the window before `audit` subscribed
	// would be marked done without anybody recording it, and never redelivered.
	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	cron := job.NewCronScheduler(pool)
	if err := startModules(ctx, pool, bus, cron, cfg); err != nil {
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
	// kinds, and today no module has one — every business module is still a
	// stub. Rather than fail the boot (which would break `make dev` for
	// everyone until the first P1 module lands) the worker says so and skips
	// the consumer. The moment registerJobKinds returns a non-zero count the
	// worker starts consuming, with no further change here.
	workers := river.NewWorkers()
	var worker *job.Worker
	if kinds := registerJobKinds(workers); kinds == 0 {
		slog.Warn("no job kinds are registered; not starting the River worker",
			"queue", strings.Join(queueNames(cfg.Queues), ","))
	} else {
		worker, err = job.NewWorker(job.WorkerOptions{
			Pool:        pool,
			Queues:      cfg.Queues,
			Workers:     workers,
			JobTimeout:  cfg.Job.Timeout,
			MaxAttempts: cfg.Job.MaxAttempts,
			Instruments: provider.Instruments(),
		})
		if err != nil {
			return err
		}
		if err := worker.Start(ctx, provider.Instruments()); err != nil {
			return err
		}
		slog.Info("river worker consuming", "count", kinds)
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
//
// The `auth` module's consumer delivers registration emails and warning emails
// for the already-verified path, using the outbox events written inside the
// same transaction as the business write. The `audit` consumer records every
// event. Both subscribe before the outbox publisher starts (in run()) so no
// event can be marked published before a consumer has accepted it.
func startModules(
	ctx context.Context, pool *pgxpool.Pool, bus *eventbus.InProcessBus, cron *job.CronScheduler,
	cfg workerConfig,
) error {
	trail := audit.New(audit.Deps{Pool: pool})

	if err := trail.Subscribe(bus); err != nil {
		return err
	}

	// The scheduler guards each job with a Postgres advisory lock, so a second
	// replica running the same tick is harmless.
	for _, scheduled := range trail.CronJobs() {
		cron.Register(scheduled)
	}

	// Rotation runs once at boot rather than waiting for the first tick. A
	// deployment onto a database whose partitions have lapsed would otherwise
	// refuse every audited write until the interval elapsed, and the whole
	// point of creating three months ahead is that nobody is ever waiting.
	//
	// A failure here is logged rather than returned: the tick will retry, and
	// refusing to boot would also stop the outbox publisher and every queue.
	if err := trail.RotatePartitions(ctx); err != nil {
		slog.ErrorContext(ctx, "could not rotate audit partitions at start-up; the scheduled job will retry",
			"error", err)
	}

	// user is constructed here rather than imported from cmd/api because the two
	// processes are independent — they share a pool and a database, but each
	// builds its own module graph. The worker needs user only as the Registrar
	// surface that auth adapts to.
	userModule := user.New(user.Deps{Pool: pool})

	// The renderer is built at startup so a missing or malformed template fails
	// the worker boot rather than one consumer invocation.
	renderer, err := mailer.NewRenderer(nil, nil)
	if err != nil {
		return fmt.Errorf("build mailer renderer: %w", err)
	}
	sender := mailer.NewSMTPSender(smtpConfig(cfg), renderer, nil, nil)

	// The worker neither issues nor verifies a token — it runs the mailer
	// consumer and the purge sweep. It is handed the signing material anyway
	// because auth.New builds the whole module and refuses to start without a
	// usable key, which is the right refusal for the API and an inherited cost
	// here. The alternative is a token service that silently cannot sign, and a
	// process that boots and then fails every login is worse than one that does
	// not boot. Both binaries deploy from one environment, so this distributes
	// no secret that was not already present. Tracked in auth/TODO.md.
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

	if err := authModule.Subscribe(bus); err != nil {
		return err
	}
	for _, scheduled := range authModule.CronJobs() {
		cron.Register(scheduled)
	}

	return nil
}

// smtpConfig keeps the worker's transport configuration complete. The API and
// worker run in separate processes: registration writes the outbox event in
// the API, but this worker is the process that authenticates to SMTP and sends
// the OTP, so omitting credentials here makes every production delivery fail.
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

// registerJobKinds is where a module's job handlers are added to the bundle, and
// it returns how many were registered.
//
// It is empty on purpose: job handlers are owned by the module whose data they
// touch (see internal/platform/job/AGENT.md §2), and no business module exists
// yet. This is the single place P1 adds `river.AddWorker(workers, ...)`.
func registerJobKinds(*river.Workers) int {
	return 0
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
