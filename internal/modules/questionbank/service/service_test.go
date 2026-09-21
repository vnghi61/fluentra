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
}

func (m *mockGenerator) Generate(
	_ context.Context, _ learningcontract.GenerateRequest,
) ([]learningcontract.GeneratedItem, error) {
	return m.items, m.err
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
