package lesson

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/repository"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
	lessonhttp "github.com/fluentra/fluentra/internal/modules/lesson/transport/http"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// Guard is the authorization interface required by the module.
type Guard = lessonhttp.Guard

// Deps are the dependencies supplied by the composition root.
type Deps struct {
	Pool     *pgxpool.Pool
	Caches   service.LessonCaches
	Clock    clock.Clock
	Guard    Guard
	Content  contentcontract.Reader
	Unlocker service.UnlockChecker
	Env      string
}

// Module is the lesson module, assembled. It is the only symbol cmd/ imports.
type Module struct {
	service *service.Service
	handler *lessonhttp.Handler
}

// New wires the lesson module.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	repo := repository.New(deps.Pool)
	repoAdapter := repositoryAdapter{Repository: repo}

	events := outboxWriter{Writer: outbox.NewWriter()}

	svc := service.New(service.Deps{
		Pool:     deps.Pool,
		Repo:     repoAdapter,
		Content:  deps.Content,
		Unlocker: deps.Unlocker,
		Events:   events,
		Caches:   deps.Caches,
		Clock:    timekeeper,
		NewID:    func() uuid.UUID { return uuid.Must(uuid.NewV7()) },
		Env:      deps.Env,
	})

	handler, err := lessonhttp.NewHandler(svc, deps.Guard)
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

// Service returns the underlying service instance.
func (m *Module) Service() *service.Service {
	return m.service
}

// Routes mounts learner-facing lesson routes on router.
func (m *Module) Routes(router chi.Router) {
	if m.handler != nil {
		m.handler.Routes(router)
	}
}

// AdminRoutes mounts back-office / authoring lesson routes on admin router.
func (m *Module) AdminRoutes(admin chi.Router) {
	if m.handler != nil {
		m.handler.AdminRoutes(admin)
	}
}

// Subscribe registers event consumers this module runs in the background.
func (m *Module) Subscribe(bus eventbus.EventBus) error {
	if err := bus.Subscribe(contentcontract.EventContentArchived, m.handleContentArchived); err != nil {
		return fmt.Errorf("subscribe lesson consumer to %s: %w", contentcontract.EventContentArchived, err)
	}
	if err := bus.Subscribe(contract.EventLessonPublished, m.handleLessonPublished); err != nil {
		return fmt.Errorf("subscribe lesson consumer to %s: %w", contract.EventLessonPublished, err)
	}
	return nil
}

func (m *Module) handleContentArchived(ctx context.Context, msg eventbus.Message) error {
	var payload contentcontract.Archived
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode %s payload: %w", contentcontract.EventContentArchived, err)
	}
	if payload.VersionID == uuid.Nil {
		return nil
	}
	return m.service.HandleContentArchived(ctx, payload.VersionID)
}

// handleLessonPublished is the backstop for the invalidation PublishLesson
// already ran synchronously; see service.HandleLessonPublished.
func (m *Module) handleLessonPublished(ctx context.Context, msg eventbus.Message) error {
	var payload contract.Published
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode %s payload: %w", contract.EventLessonPublished, err)
	}
	if payload.LessonID == uuid.Nil {
		return nil
	}
	return m.service.HandleLessonPublished(ctx, payload.LessonID, payload.CourseID)
}

// repositoryAdapter bridges repository to service.Repository interface.
type repositoryAdapter struct {
	*repository.Repository
}

func (a repositoryAdapter) WithTx(tx pgx.Tx) service.Repository {
	return repositoryAdapter{Repository: a.Repository.WithTx(tx)}
}

func (a repositoryAdapter) CreateCourse(
	ctx context.Context, params service.CreateCourseParams,
) (*contract.Course, error) {
	return a.Repository.CreateCourse(ctx, repository.CreateCourseParams{
		Slug:           params.Slug,
		Title:          params.Title,
		Description:    params.Description,
		CEFRFrom:       params.CEFRFrom,
		CEFRTo:         params.CEFRTo,
		Status:         params.Status,
		EstimatedHours: params.EstimatedHours,
	})
}

func (a repositoryAdapter) ListPrerequisitesByLessonID(
	ctx context.Context, lessonID uuid.UUID,
) ([]service.PrerequisiteItem, error) {
	items, err := a.Repository.ListPrerequisitesByLessonID(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	res := make([]service.PrerequisiteItem, len(items))
	for i, it := range items {
		res[i] = service.PrerequisiteItem{
			LessonID:            it.LessonID,
			RequiresLessonID:    it.RequiresLessonID,
			MinScore:            it.MinScore,
			RequiresLessonTitle: it.RequiresLessonTitle,
		}
	}
	return res, nil
}

func (a repositoryAdapter) ListPrerequisitesForLessons(
	ctx context.Context, lessonIDs []uuid.UUID,
) ([]service.PrerequisiteItem, error) {
	items, err := a.Repository.ListPrerequisitesForLessons(ctx, lessonIDs)
	if err != nil {
		return nil, err
	}
	res := make([]service.PrerequisiteItem, len(items))
	for i, it := range items {
		res[i] = service.PrerequisiteItem{
			LessonID:            it.LessonID,
			RequiresLessonID:    it.RequiresLessonID,
			MinScore:            it.MinScore,
			RequiresLessonTitle: it.RequiresLessonTitle,
		}
	}
	return res, nil
}

func (a repositoryAdapter) ListLessonIDsByContentVersionID(
	ctx context.Context, versionID uuid.UUID,
) ([]uuid.UUID, error) {
	return a.Repository.ListLessonIDsByContentVersionID(ctx, versionID)
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
