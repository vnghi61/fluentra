package resource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/job"
	"github.com/fluentra/fluentra/internal/modules/resource/repository"
	"github.com/fluentra/fluentra/internal/modules/resource/service"
	resourcehttp "github.com/fluentra/fluentra/internal/modules/resource/transport/http"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	platformai "github.com/fluentra/fluentra/internal/platform/ai"
	platformjob "github.com/fluentra/fluentra/internal/platform/job"
	platformmedia "github.com/fluentra/fluentra/internal/platform/media"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
)

// Deps defines the infrastructure dependencies for the resource module.
type Deps struct {
	Pool         *pgxpool.Pool
	Storage      storage.Store
	Enqueuer     platformjob.Enqueuer
	WorkerNudger service.WorkerNudger
	URLFetcher   service.URLFetcher
	MediaRender  service.MediaRenderRequester
	Taxonomies   contentcontract.TaxonomyResolver
	Transcriber  platformmedia.Transcriber
	AIClient     platformai.Client
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
	if deps.MediaRender != nil {
		svc.SetMediaRender(deps.MediaRender)
	}
	if deps.Taxonomies != nil {
		svc.SetTaxonomies(deps.Taxonomies)
	}
	if deps.Transcriber != nil {
		svc.SetTranscriber(deps.Transcriber)
	}
	if deps.AIClient != nil {
		svc.SetAIClient(deps.AIClient)
	}
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

// TranscribeWorker returns the River worker for async audio/video transcription.
func (m *Module) TranscribeWorker() *job.TranscribeResourceWorker {
	return job.NewTranscribeResourceWorker(m.service)
}

// ClassifyWorker returns the River worker for async resource classification.
func (m *Module) ClassifyWorker() *job.ClassifyResourceWorker {
	return job.NewClassifyResourceWorker(m.service)
}

// SweepJob returns the cron task for sweeping abandoned upload intents.
func (m *Module) SweepJob() platformjob.CronJob {
	return job.SweepPendingJob(m.service)
}

// Reader returns the read-only contract implementation.
func (m *Module) Reader() contract.ResourceReader {
	return &readerAdapter{service: m.service}
}

// MaterialPublisher returns the read-and-copy surface a course publisher uses.
func (m *Module) MaterialPublisher() contract.MaterialPublisher {
	return m.service
}

// Service returns the underlying domain service (used for worker/jobs and module integration tests).
func (m *Module) Service() *service.Service {
	return m.service
}

type readerAdapter struct {
	service *service.Service
}

func (r *readerAdapter) GetResource(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error) {
	return r.service.GetResource(ctx, id, userID)
}

// Subscribe registers the module's consumers in the worker.
//
// Account erasure anonymises the user row, so ON DELETE CASCADE never runs;
// nothing but this consumer removes the resource rows, uploads and renditions.
func (m *Module) Subscribe(bus eventbus.EventBus) error {
	if err := bus.Subscribe(usercontract.EventDeleted, m.handleUserDeleted); err != nil {
		return fmt.Errorf("subscribe resource consumer to %s: %w", usercontract.EventDeleted, err)
	}
	return nil
}

func (m *Module) handleUserDeleted(ctx context.Context, msg eventbus.Message) error {
	var payload usercontract.UserDeleted
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode %s payload: %w", usercontract.EventDeleted, err)
	}
	if payload.UserID == uuid.Nil {
		return nil
	}
	return m.service.DeleteUserResources(ctx, payload.UserID)
}
