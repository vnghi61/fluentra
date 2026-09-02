package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MockProvider answers without a network call.
//
// It is the default, and that is deliberate. `make dev` has to produce a working
// stack for someone who has not signed up to anything, and a verification job
// that fails on every batch because no model is configured looks like a broken
// feature rather than an unconfigured one. The mock returns an answer of the
// right shape, so the pipeline around it — the job, the deck, the review cards,
// the XP — is exercised end to end on a laptop with no API key and no internet.
//
// It is not a model. It accepts any word it is given, which is exactly what a
// real verification must not do, so nothing that matters may rely on it. The
// summary a learner sees names the model that answered for this reason.
type MockProvider struct {
	registry *Registry
}

// NewMockProvider builds the offline provider.
func NewMockProvider(registry *Registry) *MockProvider {
	return &MockProvider{registry: registry}
}

// Name returns the provider identifier.
func (p *MockProvider) Name() string {
	return ProviderMock
}

// MockModelName is what a mocked answer reports as its model, so a stored
// verification can be told apart from a real one later.
const MockModelName = "mock"

// Complete implements Client interface.
func (p *MockProvider) Complete(_ context.Context, req Request) (Response, error) {
	// The template is still rendered and its errors still surface: a broken
	// prompt should fail in development, where the mock is what runs, rather
	// than first in production against a real provider.
	if p.registry != nil {
		tmpl, err := p.registry.Get(req.Task)
		if err != nil {
			return Response{}, err
		}
		if _, err := tmpl.Render(req.Vars); err != nil {
			return Response{}, err
		}
	}

	switch req.Task {
	case TaskVerifyVocabulary:
		return p.verifyVocabulary(req)
	default:
		return Response{}, fmt.Errorf("ai: mock provider has no answer for task %q", req.Task)
	}
}

func (p *MockProvider) verifyVocabulary(req Request) (Response, error) {
	term := strings.TrimSpace(stringVar(req.Vars, "Term"))
	if term == "" {
		return Response{}, fmt.Errorf("ai: mock verification needs a term")
	}

	definition := stringVar(req.Vars, "DictionaryDefinition")
	if definition == "" {
		definition = fmt.Sprintf("A placeholder definition of %q, written by the mock provider.", term)
	}
	partOfSpeech := stringVar(req.Vars, "PartOfSpeech")
	if partOfSpeech == "" {
		partOfSpeech = "noun"
	}

	count := intVar(req.Vars, "ExampleCount", 5)
	// Deliberately flat and repetitive. A mocked sentence that reads like a
	// real one is a mocked sentence that ships.
	examples := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		examples = append(examples,
			fmt.Sprintf("Example %d for %q, generated offline by the mock provider.", i, term))
	}

	payload, err := json.Marshal(map[string]any{
		"valid":           true,
		"reason":          "",
		"lemma":           strings.ToLower(term),
		"part_of_speech":  partOfSpeech,
		"cefr_level":      "B1",
		"definition":      definition,
		"meaning_matches": true,
		"examples":        examples,
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock answer: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

func stringVar(vars map[string]any, key string) string {
	if value, ok := vars[key].(string); ok {
		return value
	}
	return ""
}

func intVar(vars map[string]any, key string, fallback int) int {
	if value, ok := vars[key].(int); ok && value > 0 {
		return value
	}
	return fallback
}

var _ Client = (*MockProvider)(nil)
