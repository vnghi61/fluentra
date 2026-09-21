package resource

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/job"
	"github.com/fluentra/fluentra/internal/modules/resource/repository"
	"github.com/fluentra/fluentra/internal/modules/resource/service"
	resourcehttp "github.com/fluentra/fluentra/internal/modules/resource/transport/http"
	platformjob "github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/storage"
)

// Deps defines the infrastructure dependencies for the resource module.
type Deps struct {
	Pool         *pgxpool.Pool
	Storage      storage.Store
	Enqueuer     platformjob.Enqueuer
	WorkerNudger service.WorkerNudger
	URLFetcher   service.URLFetcher
}

// Module encapsulates the resource domain, repository, service, jobs, and HTTP transport.
type Module struct {
	repo    *repository.Repository
	service *service.Service
	handler *resourcehttp.Handler
}

// New constructs and wires a new resource Module.
func New(deps Deps) *Module {
	repo := repository.New(deps.Pool)
	svc := service.New(
		repo,
		deps.Storage,
		deps.Pool,
		deps.Enqueuer,
		deps.URLFetcher,
		deps.WorkerNudger,
	)
	handler := resourcehttp.NewHandler(svc)

	return &Module{
		repo:    repo,
		service: svc,
		handler: handler,
	}
}

// Routes mounts the resource HTTP endpoints under the authenticated router.
func (m *Module) Routes(r chi.Router) {
	m.handler.Routes(r)
}

// ValidateWorker returns the River worker for async resource validation.
func (m *Module) ValidateWorker() *job.ValidateResourceWorker {
	return job.NewValidateResourceWorker(m.service)
}

// SweepJob returns the cron task for sweeping abandoned upload intents.
func (m *Module) SweepJob() platformjob.CronJob {
	return job.SweepPendingJob(m.service)
}

// Reader returns the read-only contract implementation.
func (m *Module) Reader() contract.ResourceReader {
	return &readerAdapter{service: m.service}
}

type readerAdapter struct {
	service *service.Service
}

func (r *readerAdapter) GetResource(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error) {
	return r.service.GetResource(ctx, id, userID)
}
