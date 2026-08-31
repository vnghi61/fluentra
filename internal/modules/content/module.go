package content

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/repository"
	"github.com/fluentra/fluentra/internal/modules/content/service"
	contenthttp "github.com/fluentra/fluentra/internal/modules/content/transport/http"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// Guard is the authorization interface required by the module.
type Guard = contenthttp.Guard

// Deps are the dependencies supplied by the composition root.
type Deps struct {
	Pool  *pgxpool.Pool
	Clock clock.Clock
	Guard Guard
}

// Module is the content module, assembled. It is the only symbol cmd/ imports.
type Module struct {
	service *service.Service
	handler *contenthttp.Handler
}

// New wires the content module for a process that serves HTTP.
//
// It fails closed without a guard, and that is deliberate: the admin authoring
// routes would otherwise be mounted unprotected. TestNewFailsClosedWithoutAGuard
// pins it.
func New(deps Deps) *Module {
	module := newModule(deps)

	handler, err := contenthttp.NewHandler(module.service, deps.Guard)
	if err != nil {
		panic(err)
	}
	module.handler = handler
	return module
}

// NewAuthoring wires content for a process with no HTTP surface at all.
//
// cmd/worker needs Author() to generate practice content and mounts no routes,
// so it has no guard to give. It used to call New anyway, which built the
// handler, which fails closed — and the worker panicked on boot with
// GUARD_REQUIRED three call frames from anything that mentioned HTTP.
//
// The consequence was not local to the worker. A worker that will not start
// drains no outbox, so no OTP email is ever sent: every E2E journey times out
// waiting for one, and in production registration stops working while the API
// looks perfectly healthy.
//
// A separate constructor rather than a nil-guard branch inside New, because the
// distinction being made is "this process serves no routes" — which is a fact
// about the caller, not a missing argument. Handing New a permissive guard
// would have worked today and quietly disarmed the check the first time
// somebody mounted a route in the worker.
func NewAuthoring(deps Deps) *Module {
	return newModule(deps)
}

// newModule builds everything both constructors share: the service, and nothing
// that needs authorization.
func newModule(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	repo := repository.New(deps.Pool)
	repoAdapter := repositoryAdapter{Repository: repo}

	events := outboxWriter{Writer: outbox.NewWriter()}

	svc := service.New(service.Deps{
		Pool:   deps.Pool,
		Repo:   repoAdapter,
		Events: events,
		Clock:  timekeeper,
		NewID:  func() uuid.UUID { return uuid.Must(uuid.NewV7()) },
	})

	// No handler here. New attaches one; NewAuthoring deliberately does not.
	return &Module{service: svc}
}

// Reader returns the public read contract implementation for other modules.
func (m *Module) Reader() contract.Reader {
	return m.service
}

// Author is the machine-authoring surface, addressed by slug and idempotent.
// It is not the review state machine, which models decisions a person makes.
func (m *Module) Author() contract.Author { return m.service }

// Service returns the underlying service instance.
func (m *Module) Service() *service.Service {
	return m.service
}

// Routes mounts the learner-facing content routes on router.
func (m *Module) Routes(router chi.Router) {
	m.handler.Routes(router)
}

// AdminRoutes mounts the back-office / authoring content routes on admin router.
func (m *Module) AdminRoutes(admin chi.Router) {
	m.handler.AdminRoutes(admin)
}

// repositoryAdapter bridges repository to service.Repository interface.
type repositoryAdapter struct {
	*repository.Repository
}

func (a repositoryAdapter) WithTx(tx pgx.Tx) service.Repository {
	return repositoryAdapter{Repository: a.Repository.WithTx(tx)}
}

// outboxWriter adapts shared/outbox to the service's EventWriter.
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
