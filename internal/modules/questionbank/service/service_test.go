package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	"github.com/fluentra/fluentra/internal/modules/questionbank/domain"
	"github.com/fluentra/fluentra/internal/modules/questionbank/service"
)

type mockGenerator struct {
	items []learningcontract.GeneratedItem
	err   error
	// lastRequest is the request the service passed, so a test can assert what
	// reached the generator.
	lastRequest *learningcontract.GenerateRequest
}

func (m *mockGenerator) Generate(
	_ context.Context, req learningcontract.GenerateRequest,
) ([]learningcontract.GeneratedItem, error) {
	m.lastRequest = &req
	return m.items, m.err
}

// mockExamParts answers questionbank's exam-part lookup with one constraint.
type mockExamParts struct {
	constraints *learningcontract.ExamPartConstraints
	err         error
}

func (m *mockExamParts) PartConstraints(
	_ context.Context, _ uuid.UUID,
) (*learningcontract.ExamPartConstraints, error) {
	return m.constraints, m.err
}

type mockLessonAuthor struct {
	appendedActivities []lessoncontract.ActivitySpec
}

func (m *mockLessonAuthor) EnsureCourse(_ context.Context, _ lessoncontract.CourseSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (m *mockLessonAuthor) EnsureUnit(_ context.Context, _ lessoncontract.UnitSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (m *mockLessonAuthor) EnsureLesson(_ context.Context, _ lessoncontract.LessonSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (m *mockLessonAuthor) AppendActivity(
	_ context.Context, _ uuid.UUID, spec lessoncontract.ActivitySpec,
) (uuid.UUID, error) {
	m.appendedActivities = append(m.appendedActivities, spec)
	return uuid.New(), nil
}

func (m *mockLessonAuthor) SyncActivities(_ context.Context, _ uuid.UUID, _ []lessoncontract.ActivitySpec) error {
	return nil
}

func (m *mockLessonAuthor) UpdateLessonStatus(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func TestQuestionbank_Generate_FingerprintDeduplication(t *testing.T) {
	rawBody := json.RawMessage(`{
		"prompt": "Choose the best option to complete the sentence.",
		"sentence": "The train ___ at 8 AM tomorrow.",
		"options": ["leaves", "leaving", "left", "have left"],
		"key": "leaves",
		"explanation": "Schedule uses present simple."
	}`)

	fp, err := domain.FingerprintFromBody(contract.KindMcqGap, rawBody)
	require.NoError(t, err)
	assert.NotEmpty(t, fp)

	// Same body with permuted options should yield identical fingerprint
	permutedBody := json.RawMessage(`{
		"prompt": "Choose the best option to complete the sentence.",
		"sentence": "The train ___ at 8 AM tomorrow.",
		"options": ["left", "have left", "leaves", "leaving"],
		"key": "leaves",
		"explanation": "Schedule uses present simple."
	}`)

	fp2, err := domain.FingerprintFromBody(contract.KindMcqGap, permutedBody)
	require.NoError(t, err)
	assert.Equal(t, fp, fp2, "Permuted options must produce identical fingerprint")
}

func TestQuestionbank_ServiceInitialization(t *testing.T) {
	svc := service.New(service.Config{
		LessonAuthor: &mockLessonAuthor{},
		Generator:    &mockGenerator{},
	})
	assert.NotNil(t, svc)
}

// TestGenerateQuestions_PassesTheExamPartConstraintsToTheGenerator is the
// questionbank half of WO 22 Stage I enforcement: the part's published format
// reaches the generator.
func TestGenerateQuestions_PassesTheExamPartConstraintsToTheGenerator(t *testing.T) {
	gen := &mockGenerator{}
	parts := &mockExamParts{constraints: &learningcontract.ExamPartConstraints{
		OptionCount:       3,
		QuestionsPerGroup: 1,
		AudioRequired:     true,
	}}
	svc := service.New(service.Config{Generator: gen, ExamParts: parts})

	partID := uuid.New()
	_, err := svc.GenerateQuestions(context.Background(), contract.GenerateRequest{
		ExamPartID: &partID,
		Kind:       "question_response",
		CEFRLevel:  "B1",
		NodeCodes:  []string{"PRESENT_SIMPLE"},
		Count:      1,
	})
	require.NoError(t, err)
	require.NotNil(t, gen.lastRequest, "the generator must be called")
	require.NotNil(t, gen.lastRequest.ExamConstraints, "the part's spec must reach the generator")
	assert.Equal(t, 3, gen.lastRequest.ExamConstraints.OptionCount)
	assert.True(t, gen.lastRequest.ExamConstraints.AudioRequired)
}
