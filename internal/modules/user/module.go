package user

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/repository"
	"github.com/fluentra/fluentra/internal/modules/user/service"
	userhttp "github.com/fluentra/fluentra/internal/modules/user/transport/http"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/id"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// Deps are what the composition root supplies.
//
// Pool is the concrete pool rather than an interface declared here. The module
// needs both halves of it — a querier for reads and a transaction starter for
// writes — and naming the concrete type is honest about that, where a bespoke
// interface would only be a second name for *pgxpool.Pool.
type Deps struct {
	Pool    *pgxpool.Pool
	Clock   clock.Clock
	Storage storage.Store
}

// Module is the user module, assembled. It is the only symbol cmd/ imports.
type Module struct {
	service *service.Service
	handler *userhttp.Handler
}

// New wires the module. Reading down this function is the fastest way to see
// what the module is made of and what it depends on.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	repo := repositoryAdapter{Repository: repository.New(deps.Pool)}
	users := service.New(service.Deps{
		Pool:    deps.Pool,
		Repo:    repo,
		Events:  outboxWriter{Writer: outbox.NewWriter()},
		Clock:   timekeeper,
		NewID:   id.NewUUIDv7,
		Storage: deps.Storage,
	})

	return &Module{service: users, handler: userhttp.NewHandler(users)}
}

// Routes mounts the module's HTTP operations under the caller's router.
func (m *Module) Routes(router chi.Router) { m.handler.Routes(router) }

// Reader is this module's read contract, for other modules to depend on.
func (m *Module) Reader() contract.Reader { return m.service }

// Creator is this module's write contract. Only `auth` uses it.
func (m *Module) Creator() contract.Creator { return m.service }

// Registrar is the registration-lifecycle contract: create an account, ask
// whether an address is already claimed, mark it proved, and sweep the ones
// that never were. Only `auth` uses it.
func (m *Module) Registrar() contract.Registrar { return m.service }

// repositoryAdapter narrows *repository.Repository to the interface the
// service declares. Go has no covariant return types, so WithTx returning
// *Repository cannot satisfy a method returning service.Repository — this is
// the four lines that bridge that, and keeping it here means neither the
// service nor the repository has to know about the other's package.
type repositoryAdapter struct {
	*repository.Repository
}

func (a repositoryAdapter) WithTx(tx pgx.Tx) service.Repository {
	return repositoryAdapter{Repository: a.Repository.WithTx(tx)}
}

// outboxWriter adapts shared/outbox to the service's EventWriter. The
// signatures already match; the adapter exists only because the service
// declares its own transaction interface rather than importing the outbox
// package's, which is what lets the service be tested without one.
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
