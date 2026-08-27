package user

import (
	"context"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	userjob "github.com/fluentra/fluentra/internal/modules/user/job"
	"github.com/fluentra/fluentra/internal/modules/user/repository"
	"github.com/fluentra/fluentra/internal/modules/user/service"
	userhttp "github.com/fluentra/fluentra/internal/modules/user/transport/http"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/mailer"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/id"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// NamedExportable is re-exported from user/job for wiring in main.
type NamedExportable = userjob.NamedExportable

// Deps are what the composition root supplies.
type Deps struct {
	Pool      *pgxpool.Pool
	Clock     clock.Clock
	Storage   storage.Store
	Mailer    mailer.Sender
	Enqueuer  job.Enqueuer
	Providers []NamedExportable
	Bucket    string
	LinkTTL   time.Duration
	Retention time.Duration
	// Roles grants the baseline role to every account this module creates.
	// The composition root supplies rbac's module; a build without it creates
	// accounts that hold nothing, which is the bug this exists to close.
	Roles BaselineRoleGranter
}

// BaselineRoleGranter is satisfied by the rbac module. It is named here so
// cmd/ can read the wiring without opening the service package.
type BaselineRoleGranter = service.BaselineRoles

// Module is the user module, assembled. It is the only symbol cmd/ imports.
type Module struct {
	service        *service.Service
	handler        *userhttp.Handler
	worker         *userjob.ExportWorker
	cleaner        *userjob.ExportCleaner
	deletionExec   *userjob.DeletionExecutor
	erasureChecker *userjob.ErasureChecker
}

// New wires the module.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	repo := repository.New(deps.Pool)
	repoAdapter := repositoryAdapter{Repository: repo}

	var enqueuer service.JobEnqueuer
	if deps.Enqueuer != nil {
		enqueuer = jobEnqueuerAdapter{enqueuer: deps.Enqueuer}
	}

	events := outboxWriter{Writer: outbox.NewWriter()}

	users := service.New(service.Deps{
		Pool:     deps.Pool,
		Repo:     repoAdapter,
		Events:   events,
		Clock:    timekeeper,
		NewID:    id.NewUUIDv7,
		Storage:  deps.Storage,
		Enqueuer: enqueuer,
		Roles:    deps.Roles,
	})

	worker := userjob.NewExportWorker(userjob.ExportWorkerOptions{
		Repo:        repo,
		Storage:     deps.Storage,
		Mailer:      deps.Mailer,
		UserContact: users,
		Providers:   deps.Providers,
		Clock:       timekeeper,
		Bucket:      deps.Bucket,
		LinkTTL:     deps.LinkTTL,
		Retention:   deps.Retention,
	})

	cleaner := userjob.NewExportCleaner(repo, deps.Storage, deps.Bucket)
	deletionExec := userjob.NewDeletionExecutor(
		deps.Pool,
		repo,
		deps.Storage,
		outbox.NewWriter(),
		storage.BucketAvatars,
	)
	erasureChecker := userjob.NewErasureChecker(repo, deps.Providers)

	return &Module{
		service:        users,
		handler:        userhttp.NewHandler(users),
		worker:         worker,
		cleaner:        cleaner,
		deletionExec:   deletionExec,
		erasureChecker: erasureChecker,
	}
}

// Routes mounts the module's HTTP operations under the caller's router.
func (m *Module) Routes(router chi.Router) { m.handler.Routes(router) }

// Reader is this module's read contract, for other modules to depend on.
func (m *Module) Reader() contract.Reader { return m.service }

// Creator is this module's write contract. Only `auth` uses it.
func (m *Module) Creator() contract.Creator { return m.service }

// Registrar is the registration-lifecycle contract.
func (m *Module) Registrar() contract.Registrar { return m.service }

// Exportable is this module's GDPR export contract.
func (m *Module) Exportable() contract.Exportable { return m.service }

// AdminReader is this module's read contract for admin user management.
func (m *Module) AdminReader() contract.AdminReader { return m.service }

// AdminManager is this module's state management contract for admin user operations.
func (m *Module) AdminManager() contract.AdminManager { return m.service }

// ExportWorker returns the River worker for user data export jobs.
func (m *Module) ExportWorker() *userjob.ExportWorker { return m.worker }

// CronJobs returns background scheduled jobs owned by the user module.
func (m *Module) CronJobs() []job.CronJob {
	jobs := make([]job.CronJob, 0, 3)
	if m.cleaner != nil {
		jobs = append(jobs, m.cleaner.CronJob())
	}
	if m.deletionExec != nil {
		jobs = append(jobs, m.deletionExec.CronJob())
	}
	if m.erasureChecker != nil {
		jobs = append(jobs, m.erasureChecker.CronJob())
	}
	return jobs
}

// repositoryAdapter narrows *repository.Repository to the interface the service declares.
type repositoryAdapter struct {
	*repository.Repository
}

func (a repositoryAdapter) WithTx(tx pgx.Tx) service.Repository {
	return repositoryAdapter{Repository: a.Repository.WithTx(tx)}
}

func (a repositoryAdapter) SearchUsersAdmin(
	ctx context.Context,
	filter contract.UserFilter,
	cursorID *uuid.UUID,
	cursorTime *time.Time,
	limit int,
) ([]service.SearchUserRow, error) {
	params := repository.UserFilterParams{
		EmailPrefix:   filter.EmailPrefix,
		DisplayName:   filter.DisplayName,
		Status:        filter.Status,
		CreatedAfter:  filter.CreatedAfter,
		CreatedBefore: filter.CreatedBefore,
	}
	rows, err := a.Repository.SearchUsersAdmin(ctx, params, cursorID, cursorTime, limit)
	if err != nil {
		return nil, err
	}
	result := make([]service.SearchUserRow, 0, len(rows))
	for _, r := range rows {
		result = append(result, service.SearchUserRow{
			ID:          r.ID,
			Email:       r.Email,
			Status:      r.Status,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
			DisplayName: r.DisplayName,
			Locale:      r.Locale,
			Timezone:    r.Timezone,
		})
	}
	return result, nil
}

// jobEnqueuerAdapter adapts job.Enqueuer to service.JobEnqueuer.
type jobEnqueuerAdapter struct {
	enqueuer job.Enqueuer
}

func (a jobEnqueuerAdapter) EnqueueExportTx(ctx context.Context, tx pgx.Tx, exportID, userID uuid.UUID) error {
	args := userjob.ExportArgs{
		ExportID: exportID,
		UserID:   userID,
	}
	_, err := a.enqueuer.EnqueueTx(ctx, tx, args, nil)
	return err
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
