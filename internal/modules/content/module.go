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

// New wires the content module.
func New(deps Deps) *Module {
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

	handler, err := contenthttp.NewHandler(svc, deps.Guard)
	if err != nil {
		panic(err)
	}

	return &Module{
		service: svc,
		handler: handler,
	}
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
