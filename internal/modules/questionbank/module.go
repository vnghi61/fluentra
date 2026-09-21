package questionbank

import (
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
	ContentAuthor contentcontract.Author
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
		ContentAuthor: deps.ContentAuthor,
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

// AdminRoutes mounts questionbank admin endpoints.
func (m *Module) AdminRoutes(r chi.Router) {
	m.handler.AdminRoutes(r)
}
