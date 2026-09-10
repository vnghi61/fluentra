package writing

import (
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/writing/contract"
	"github.com/fluentra/fluentra/internal/modules/writing/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Content contentcontract.Reader
	AI      ai.Client
}

// Module represents the wired writing module.
type Module struct {
	grader *service.Grader
}

// New constructs a writing module.
func New(deps Deps) *Module {
	return &Module{
		grader: service.NewGrader(deps.Content, deps.AI),
	}
}

// Grader exposes the writing exercise grader.
func (m *Module) Grader() contract.Grader {
	return m.grader
}
