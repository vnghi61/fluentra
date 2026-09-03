package learning

import (
	"context"
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/generated/learning/sqlc"
	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	learninghttp "github.com/fluentra/fluentra/internal/modules/learning/transport/http"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	srscontract "github.com/fluentra/fluentra/internal/modules/srs/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/telemetry"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// Advisory lock id for learning module partition maintenance.
// Value is unique across the repository based on migration timestamp 1700000210.
const rotatePartitionsLockID int64 = 1_700_000_210

// rotateInterval is how frequently partition creation is checked.
const rotateInterval = 6 * time.Hour

// Guard is the authorization interface required by HTTP handlers.
type Guard = learninghttp.Guard

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Pool          *pgxpool.Pool
	Clock         clock.Clock
	Guard         Guard
	Lesson        lessoncontract.Reader
	SRSDue        srscontract.QueueReader
	SRSCards      srscontract.CardWriter
	Graders       map[string]contract.ExerciseGrader
	DeclaredKinds []string
	Metrics       telemetry.Instruments
	Caches        service.LearningCaches
	Env           string
	AI            ai.Client
}

// Module represents the learning module, assembled.
type Module struct {
	pool    *pgxpool.Pool
	clock   clock.Clock
	queries *sqlc.Queries
	service *service.Service
	handler *learninghttp.Handler
	graders *domain.GraderRegistry
}

// New constructs and wires the learning module.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	var queries *sqlc.Queries
	var repo service.Repository
	if deps.Pool != nil {
		queries = sqlc.New(deps.Pool)
		repo = repositoryAdapter{Repository: repository.New(deps.Pool)}
	}

	// The registry holds exactly what the composition root hands over. It does
	// not invent a fallback.
	//
	// It used to: with no graders supplied it registered domain.FakeGrader for
	// "quiz", "multiple_choice" and "reading_mcq", and cmd/api supplies none — so
	// every submission in production would have been scored 100/100 by a grader
	// that never reads the response. That is the opposite of this module's own
	// recorded decision that scoring is server-side "without exception, because
	// progress data that cannot be trusted is worthless".
	//
	// It also made ADR-0015's startup validation decorative, because the set being
	// validated was the set the constructor had just populated itself.
	registry := domain.NewGraderRegistry()
	for kind, grader := range deps.Graders {
		if err := registry.Register(kind, grader); err != nil {
			panic(fmt.Sprintf("register grader %q: %v", kind, err))
		}
	}

	// ADR-0015: "The grader registry is validated at startup." DeclaredKinds is
	// what a deployment claims it can grade; a kind declared with no grader behind
	// it fails the process here, naming the kind, rather than failing a learner's
	// request at 22:00. Phase 2 declares nothing, because WP9 writes the first
	// real grader — see learning/DECISIONS.md.
	if err := registry.Validate(deps.DeclaredKinds); err != nil {
		panic(err)
	}

	events := outboxWriter{Writer: outbox.NewWriter()}

	svc := service.New(service.Deps{
		Pool:     deps.Pool,
		Repo:     repo,
		Lesson:   deps.Lesson,
		SRSDue:   deps.SRSDue,
		SRSCards: deps.SRSCards,
		Graders:  registry,
		Events:   events,
		Metrics:  deps.Metrics,
		Clock:    timekeeper,
		Caches:   deps.Caches,
		Env:      deps.Env,
		AI:       deps.AI,
	})

	var handler *learninghttp.Handler
	if deps.Guard != nil {
		var err error
		handler, err = learninghttp.NewHandler(svc, deps.Guard)
		if err != nil {
			panic(fmt.Sprintf("construct learning http handler: %v", err))
		}
	}

	return &Module{
		pool:    deps.Pool,
		clock:   timekeeper,
		queries: queries,
		service: svc,
		handler: handler,
		graders: registry,
	}
}

// Grader reports the grader registered for an activity kind, if any. Exposed so
// the composition root and its tests can assert what a deployment can actually
// grade rather than assume it.
func (m *Module) Grader(kind string) (contract.ExerciseGrader, bool) {
	return m.graders.Get(kind)
}

// ProgressReader returns the public ProgressReader contract implementation.
func (m *Module) ProgressReader() contract.ProgressReader {
	return m.service
}

// UnlockChecker returns the public UnlockChecker contract implementation.
func (m *Module) UnlockChecker() contract.UnlockChecker {
	return m.service
}

// Routes mounts learner-facing attempt endpoints under the authenticated router.
func (m *Module) Routes(router chi.Router) {
	if m.handler != nil {
		m.handler.Routes(router)
	}
}

// CronJobs returns the scheduled partition maintenance job.
func (m *Module) CronJobs() []job.CronJob {
	return []job.CronJob{
		{
			Name:     "learning.rotate_partitions",
			LockID:   rotatePartitionsLockID,
			Interval: rotateInterval,
			Task:     m.RotatePartitions,
		},
	}
}

// RotatePartitions creates future partitions for the attempts table.
// Exported so cmd/worker can invoke it at start-up to avoid partition lapse outages.
func (m *Module) RotatePartitions(ctx context.Context) error {
	if m.queries == nil {
		return fmt.Errorf("learning module pool is nil")
	}
	_, err := m.queries.EnsurePartitions(ctx, 3)
	if err != nil {
		return fmt.Errorf("ensure learning partitions: %w", err)
	}
	return nil
}

// repositoryAdapter bridges repository to the service.Repository interface, so
// that a repository bound to a transaction is still the interface the service
// declared rather than the concrete struct.
type repositoryAdapter struct {
	*repository.Repository
}

func (a repositoryAdapter) WithTx(tx pgx.Tx) service.Repository {
	return repositoryAdapter{Repository: a.Repository.WithTx(tx)}
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
