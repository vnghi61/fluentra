// Package listening wires the listening comprehension module.
package listening

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/listening/repository"
	"github.com/fluentra/fluentra/internal/modules/listening/service"
	listeninghttp "github.com/fluentra/fluentra/internal/modules/listening/transport/http"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// Deps holds the dependencies for the listening module.
type Deps struct {
	Pool      *pgxpool.Pool
	Content   contentcontract.Reader
	Learning  learningcontract.AttemptReader
	Storage   storage.Store
	Sittings  service.SittingPlayPolicy
	Placement service.PlacementPlayPolicy
	Audio     service.AudioLocator
	Clock     clock.Clock
}

// Module encapsulates the listening service, grader, and HTTP handler.
type Module struct {
	service *service.Service
	grader  *service.Grader
	handler *listeninghttp.Handler
}

// New constructs and wires a new listening module.
func New(deps Deps) *Module {
	repo := repository.New(deps.Pool)
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	svc := service.New(service.Deps{
		Repo:      repo,
		Content:   deps.Content,
		Learning:  deps.Learning,
		Storage:   deps.Storage,
		Sittings:  deps.Sittings,
		Placement: deps.Placement,
		Audio:     deps.Audio,
		Clock:     timekeeper,
	})

	grader := service.NewGrader(deps.Content)
	handler := listeninghttp.NewHandler(svc)

	return &Module{
		service: svc,
		grader:  grader,
		handler: handler,
	}
}

// Grader returns the ExerciseGrader for listening_comprehension.
func (m *Module) Grader() learningcontract.ExerciseGrader {
	return m.grader
}

// Service returns the underlying listening service.
func (m *Module) Service() *service.Service {
	return m.service
}

// Routes mounts the listening endpoints on the provided router.
func (m *Module) Routes(router chi.Router) {
	m.handler.Routes(router)
}
