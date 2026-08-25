package srs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/generated/srs/sqlc"
	"github.com/fluentra/fluentra/internal/modules/srs/contract"
	"github.com/fluentra/fluentra/internal/modules/srs/repository"
	"github.com/fluentra/fluentra/internal/modules/srs/service"
	srshttp "github.com/fluentra/fluentra/internal/modules/srs/transport/http"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// Advisory lock id for srs module partition maintenance based on migration timestamp 1700000220.
const rotateSRSPartitionsLockID int64 = 1_700_000_220

const rotateInterval = 6 * time.Hour

// partitionMonthsAhead matches learning's: the current month plus three, so a
// deploy that misses a month-end still has a partition to insert into.
const partitionMonthsAhead = 3

// Guard is the authorization interface required by HTTP handlers.
type Guard = srshttp.Guard

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Pool   *pgxpool.Pool
	Clock  clock.Clock
	Guard  Guard
	Users  usercontract.Reader
	Caches service.SRSCaches
	Env    string
}

// Module represents the wired srs module.
type Module struct {
	pool    *pgxpool.Pool
	clock   clock.Clock
	queries *sqlc.Queries
	service *service.Service
	handler *srshttp.Handler
}

// New constructs and wires the srs module.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	var queries *sqlc.Queries
	if deps.Pool != nil {
		queries = sqlc.New(deps.Pool)
	}
	repo := repository.New(deps.Pool)

	events := outboxWriter{Writer: outbox.NewWriter()}

	srv := service.New(service.Deps{
		Pool:   deps.Pool,
		Repo:   repo,
		Users:  deps.Users,
		Events: events,
		Caches: deps.Caches,
		Clock:  timekeeper,
		Env:    deps.Env,
	})

	handler, err := srshttp.NewHandler(srv, deps.Guard)
	if err != nil {
		panic(fmt.Sprintf("failed to construct srs HTTP handler: %v", err))
	}

	return &Module{
		pool:    deps.Pool,
		clock:   timekeeper,
		queries: queries,
		service: srv,
		handler: handler,
	}
}

// Routes mounts the srs endpoints on the provided router.
func (m *Module) Routes(router chi.Router) {
	m.handler.Routes(router)
}

// CardWriter returns the card writer interface for learning exercise engine.
func (m *Module) CardWriter() contract.CardWriter {
	return m.service
}

// QueueReader returns the queue reader interface for dashboard and progress.
func (m *Module) QueueReader() contract.QueueReader {
	return m.service
}

// CronJobs returns the scheduled partition maintenance job.
func (m *Module) CronJobs() []job.CronJob {
	return []job.CronJob{
		{
			Name:     "srs.rotate_partitions",
			LockID:   rotateSRSPartitionsLockID,
			Interval: rotateInterval,
			Task:     m.RotatePartitions,
		},
	}
}

// RotatePartitions creates future partitions for the review_logs table.
// Exported so cmd/worker can invoke it at start-up to avoid partition lapse outages,
// the same way learning.Module.RotatePartitions is.
func (m *Module) RotatePartitions(ctx context.Context) error {
	if m.queries == nil {
		return fmt.Errorf("srs module pool is nil")
	}
	created, err := m.queries.EnsureSRSPartitions(ctx, partitionMonthsAhead)
	if err != nil {
		return fmt.Errorf("ensure srs partitions: %w", err)
	}
	if created > 0 {
		slog.InfoContext(ctx, "created future srs partitions", "count", created)
	}
	return nil
}

type outboxWriter struct {
	*outbox.Writer
}

func (w outboxWriter) Write(
	ctx context.Context, tx service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	return w.Writer.Write(ctx, outboxTx{tx}, aggregate, event, payload)
}

type outboxTx struct{ inner service.OutboxTx }

func (t outboxTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return t.inner.Exec(ctx, sql, arguments...)
}
