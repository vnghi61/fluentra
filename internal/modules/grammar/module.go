package grammar

import (
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/grammar/contract"
	"github.com/fluentra/fluentra/internal/modules/grammar/service"
)

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Content contentcontract.Reader
}

// Module represents the wired grammar module.
type Module struct {
	grader *service.Grader
}

// New constructs a grammar module.
func New(deps Deps) *Module {
	return &Module{
		grader: service.NewGrader(deps.Content),
	}
}

// Grader exposes the grammar exercise grader.
func (m *Module) Grader() contract.Grader {
	return m.grader
}
