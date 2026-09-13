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
	case TaskEnrichExamples:
		return p.enrichExamples(req)
	case TaskGradeWriting:
		return p.gradeWriting(req)
	default:
		return Response{}, fmt.Errorf("ai: mock provider has no answer for task %q", req.Task)
	}
}

func (p *MockProvider) gradeWriting(req Request) (Response, error) {
	submission := strings.TrimSpace(stringVar(req.Vars, "Submission"))
	score := 85
	correct := true
	feedback := "Good writing response with clear vocabulary."
	feedbackVi := "Bài viết tốt với vốn từ vựng rõ ràng."
	if len(strings.Fields(submission)) < 3 {
		score = 30
		correct = false
		feedback = "The submission is too short to evaluate properly."
		feedbackVi = "Bài viết quá ngắn để đánh giá chi tiết."
	}
	payload, err := json.Marshal(map[string]any{
		"overall_band": 6.5,
		"score":        score,
		"correct":      correct,
		"feedback":     feedback,
		"feedback_en":  feedback,
		"feedback_vi":  feedbackVi,
		"criteria": []map[string]any{
			mockCriterion("task_response", 7.0, "Good response to the prompt.", "Phản hồi tốt yêu cầu đề bài."),
			mockCriterion("coherence_cohesion", 6.5, "Clear progression of ideas.", "Ý tứ phát triển rõ ràng."),
			mockCriterion("lexical_resource", 6.5, "Varied vocabulary used appropriately.", "Sử dụng từ vựng đa dạng, phù hợp."),
			mockCriterion("grammatical_range", 6.0, "Good range of grammatical structures.", "Cấu trúc ngữ pháp tương đối tốt."),
		},
		"annotations": []map[string]any{},
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock writing grade: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
}

func (p *MockProvider) enrichExamples(req Request) (Response, error) {
	term := strings.TrimSpace(stringVar(req.Vars, "Term"))
	if term == "" {
		return Response{}, fmt.Errorf("ai: mock enrichment needs a term")
	}
	count := intVar(req.Vars, "Count", 5, maxMockExamples)
	examples := make([]map[string]string, 0, count)
	for i := 1; i <= count; i++ {
		examples = append(examples, map[string]string{
			"sentence":    fmt.Sprintf("Enriched example %d for %q.", i, term),
			"sentence_vi": fmt.Sprintf("Ví dụ mở rộng %d cho %q.", i, term),
		})
	}
	payload, err := json.Marshal(map[string]any{
		"examples": examples,
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai: encode mock answer: %w", err)
	}
	return Response{Text: string(payload), Model: MockModelName}, nil
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

	count := intVar(req.Vars, "ExampleCount", 5, maxMockExamples)
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

// maxMockExamples bounds every count the mock will honour.
//
// The real caller asks for at most `examplesPerSweep`, and the vocabulary
// domain caps a sense at fifteen sentences. This ceiling is deliberately well
// above both: it is not a business rule, it is the point past which a number
// arriving in `Vars` stops being a request and starts being an allocation.
const maxMockExamples = 50

// intVar reads a positive int out of the prompt variables.
//
// `ceiling` is a required argument rather than a default because the value it
// bounds reaches `make(..., count)` directly. `Vars` is an untyped map filled
// by callers, so a count of a billion is a memory-exhaustion vector rather than
// a typo — and this is the mock, which means it runs wherever no real provider
// is configured, including production before the AI slots are filled in.
func intVar(vars map[string]any, key string, fallback, ceiling int) int {
	value, ok := vars[key].(int)
	if !ok || value <= 0 {
		return fallback
	}
	if value > ceiling {
		return ceiling
	}
	return value
}

var _ Client = (*MockProvider)(nil)

// mockCriterion is one rubric criterion in the mock's writing grade.
func mockCriterion(name string, band float64, commentEn, commentVi string) map[string]any {
	return map[string]any{"name": name, "band": band, "comment_en": commentEn, "comment_vi": commentVi}
}
