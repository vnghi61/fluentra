package questionbank

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	"github.com/fluentra/fluentra/internal/modules/questionbank/repository"
	"github.com/fluentra/fluentra/internal/modules/questionbank/service"
	questionbankhttp "github.com/fluentra/fluentra/internal/modules/questionbank/transport/http"
	rbaccontract "github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
)

// Deps defines dependencies required by questionbank module.
type Deps struct {
	Pool          *pgxpool.Pool
	RBAC          rbaccontract.Authorizer
	ContentReader contentcontract.Reader
	TagIndex      contentcontract.TagIndex
	LessonAuthor  lessoncontract.Author
	Generator     learningcontract.Generator
	Events        eventbus.EventBus
}

// Module wires questionbank domain, repository, service, and HTTP transport.
type Module struct {
	repo    *repository.Repository
	service *service.Service
	handler *questionbankhttp.Handler
}

// New constructs a questionbank module.
func New(deps Deps) *Module {
	repo := repository.New(deps.Pool)
	svc := service.New(service.Config{
		Repo:          repo,
		RBAC:          deps.RBAC,
		ContentReader: deps.ContentReader,
		TagIndex:      deps.TagIndex,
		LessonAuthor:  deps.LessonAuthor,
		Generator:     deps.Generator,
		Events:        deps.Events,
	})
	handler := questionbankhttp.NewHandler(svc)

	return &Module{
		repo:    repo,
		service: svc,
		handler: handler,
	}
}

// Reader exposes the questionbank reader contract for other modules.
func (m *Module) Reader() contract.Reader {
	return m.service
}

// Author exposes the questionbank author contract for other modules.
func (m *Module) Author() contract.Author {
	return m.service
}

// Service exposes the service implementation.
func (m *Module) Service() *service.Service {
	return m.service
}

// ReviewRoutes mounts the permission-gated read endpoints a moderator uses.
func (m *Module) ReviewRoutes(r chi.Router) {
	m.handler.ReviewRoutes(r)
}

// AdminRoutes mounts questionbank admin endpoints.
func (m *Module) AdminRoutes(r chi.Router) {
	m.handler.AdminRoutes(r)
}

// Subscribe registers the consumer that moves an approved question into the
// bank: content.published for a bank question's content item appends it to the
// bank course and marks it published.
func (m *Module) Subscribe(bus eventbus.EventBus) error {
	if err := bus.Subscribe(contentcontract.EventContentPublished, m.handleContentPublished); err != nil {
		return fmt.Errorf("subscribe questionbank consumer to %s: %w", contentcontract.EventContentPublished, err)
	}
	// An archived content item stops being drawn: a sample a person rejected,
	// or an item a learner reported (WO 22 Stage A.5).
	if err := bus.Subscribe(contentcontract.EventContentArchived, m.handleContentArchived); err != nil {
		return fmt.Errorf("subscribe questionbank consumer to %s: %w", contentcontract.EventContentArchived, err)
	}
	return nil
}

func (m *Module) handleContentArchived(ctx context.Context, msg eventbus.Message) error {
	var payload contentcontract.Archived
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode %s payload: %w", contentcontract.EventContentArchived, err)
	}
	return m.service.HandleContentArchived(ctx, payload)
}

func (m *Module) handleContentPublished(ctx context.Context, msg eventbus.Message) error {
	var payload contentcontract.Published
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode %s payload: %w", contentcontract.EventContentPublished, err)
	}
	return m.service.HandleContentPublished(ctx, payload)
}
