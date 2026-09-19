package studio

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/studio/job"
	"github.com/fluentra/fluentra/internal/modules/studio/repository"
	"github.com/fluentra/fluentra/internal/modules/studio/service"
	studiohttp "github.com/fluentra/fluentra/internal/modules/studio/transport/http"
	platformjob "github.com/fluentra/fluentra/internal/platform/job"
)

// Dependencies represents the external dependencies required to construct the studio Module.
type Dependencies struct {
	Pool          *pgxpool.Pool
	Guard         studiohttp.Guard
	ItemVerifier  learningcontract.ItemVerifier
	LessonAuthor  lessoncontract.Author
	ContentAuthor contentcontract.Author
}

// Module encapsulates the studio and moderation domain capabilities.
type Module struct {
	repo    repository.Repository
	service *service.Service
	handler *studiohttp.Handler
	worker  *job.VerificationWorker
}

// NewModule constructs a new studio module.
func NewModule(deps Dependencies) (*Module, error) {
	repo := repository.NewRepository(deps.Pool)
	svc := service.NewService(repo, deps.ItemVerifier, deps.LessonAuthor, deps.ContentAuthor)
	handler := studiohttp.NewHandler(svc, deps.Guard)
	worker := job.NewVerificationWorker(repo, svc)

	return &Module{
		repo:    repo,
		service: svc,
		handler: handler,
		worker:  worker,
	}, nil
}

// Routes mounts creator-facing endpoints under the authenticated router.
func (m *Module) Routes(r chi.Router) {
	m.handler.Routes(r)
}

// ModerationRoutes mounts staff moderation endpoints under the admin/moderation router.
func (m *Module) ModerationRoutes(r chi.Router) {
	m.handler.ModerationRoutes(r)
}

// CronJobs returns scheduled tasks owned by the studio module (Gate 1 verification worker).
func (m *Module) CronJobs() []platformjob.CronJob {
	if m.worker == nil {
		return nil
	}
	return []platformjob.CronJob{
		m.worker.CronJob(),
	}
}

// Service returns the studio service.
func (m *Module) Service() *service.Service {
	return m.service
}

// Repository returns the studio repository.
func (m *Module) Repository() repository.Repository {
	return m.repo
}
