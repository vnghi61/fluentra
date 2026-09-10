package reading

import (
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/reading/contract"
	"github.com/fluentra/fluentra/internal/modules/reading/service"
)

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Content contentcontract.Reader
}

// Module represents the wired reading module.
type Module struct {
	grader *service.Grader
}

// New constructs a reading module.
func New(deps Deps) *Module {
	return &Module{
		grader: service.NewGrader(deps.Content),
	}
}

// Grader exposes the reading exercise grader.
func (m *Module) Grader() contract.Grader {
	return m.grader
}
