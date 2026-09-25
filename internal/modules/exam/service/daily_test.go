package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	"github.com/fluentra/fluentra/internal/modules/exam/service"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
)

// fakeBankAuthor records the generation requests the daily job makes.
type fakeBankAuthor struct {
	calls []questionbankcontract.GenerateRequest
}

func (f *fakeBankAuthor) CreateQuestion(
	_ context.Context, _ *questionbankcontract.Question,
) (*questionbankcontract.Question, error) {
	return nil, nil
}

func (f *fakeBankAuthor) GenerateQuestions(
	_ context.Context, req questionbankcontract.GenerateRequest,
) ([]*questionbankcontract.Question, error) {
	f.calls = append(f.calls, req)
	return nil, nil
}

func (f *fakeBankAuthor) PublishQuestion(
	_ context.Context, _ uuid.UUID,
) (*questionbankcontract.Question, error) {
	return nil, nil
}

func (f *fakeBankAuthor) RetireQuestion(
	_ context.Context, _ uuid.UUID,
) (*questionbankcontract.Question, error) {
	return nil, nil
}

func TestDailyExamForDay_CyclesThroughTheRotation(t *testing.T) {
	seen := map[string]bool{}
	for day := 0; day < len(service.DailyExamRotation); day++ {
		date := time.Date(2026, 1, 1+day, 0, 0, 0, 0, time.UTC)
		seen[service.DailyExamForDay(date)] = true
	}
	assert.Len(t, seen, len(service.DailyExamRotation),
		"consecutive days must pick different exams")
}

func TestGenerateDailyExam_GeneratesEachPartThenComposes(t *testing.T) {
	partID := uuid.New()
	blueprintID := uuid.New()
	versionID := uuid.New()

	part := &domain.ExamPart{
		ID:            partID,
		VersionID:     versionID,
		Section:       testSkillListening,
		PartNumber:    1,
		Kind:          kindListeningComprehension,
		QuestionCount: 3,
		GroupSize:     1,
	}
	repo := newMockExamRepo()
	repo.version = &domain.ExamVersion{ID: versionID, Code: "TOEIC_LR_2026"}
	repo.parts = []*domain.ExamPart{part}
	repo.blueprint = &domain.Blueprint{
		ID:               blueprintID,
		VersionID:        versionID,
		Name:             "toeic_default",
		CefrDistribution: json.RawMessage(`{"B2": 0.6, "B1": 0.4}`),
	}

	// Three activities, exactly one three-question test's worth.
	questions := make([]*questionbankcontract.Question, 0, 3)
	for i := 0; i < 3; i++ {
		actID := uuid.New()
		questions = append(questions, &questionbankcontract.Question{
			ID:            uuid.New(),
			ActivityID:    &actID,
			ExamPartID:    &partID,
			Kind:          kindListeningComprehension,
			CEFRLevel:     "B2",
			QuestionCount: 1,
		})
	}
	bank := &fakeBankAuthor{}
	svc := service.New(service.Deps{
		Repo:         repo,
		Questionbank: &fakeQuestionbank{byPart: map[uuid.UUID][]*questionbankcontract.Question{partID: questions}},
		BankAuthor:   bank,
	})

	composed, err := svc.GenerateDailyExam(context.Background(), "TOEIC_LR_2026")
	require.NoError(t, err)

	require.Len(t, bank.calls, 1, "one part generates once")
	assert.Equal(t, partID, *bank.calls[0].ExamPartID)
	assert.Equal(t, "B2", bank.calls[0].CEFRLevel, "the blueprint's largest share is the level")
	// One test's worth (3 groups) plus the margin.
	assert.Equal(t, 3+service.DailyGenerationMargin, bank.calls[0].Count)
	assert.Equal(t, 1, composed, "a full test's worth composes one numbered test")
}

// TestComposeAllFixedTests_ComposesWhatTheBankCanFill is the Stage O sweep: a
// bank a person or a verifier filled after the daily job gets its next
// numbered test, with no generation involved.
func TestComposeAllFixedTests_ComposesWhatTheBankCanFill(t *testing.T) {
	partID := uuid.New()
	versionID := uuid.New()

	repo := newMockExamRepo()
	repo.version = &domain.ExamVersion{ID: versionID, Code: "TOEIC_LR_2026"}
	repo.parts = []*domain.ExamPart{{
		ID: partID, VersionID: versionID, Section: testSkillListening, PartNumber: 1,
		Kind: kindListeningComprehension, QuestionCount: 3, GroupSize: 1,
	}}
	repo.blueprint = &domain.Blueprint{
		ID: uuid.New(), VersionID: versionID, Name: "toeic_default",
		CefrDistribution: json.RawMessage(`{"B2": 1.0}`),
	}
	questions := make([]*questionbankcontract.Question, 0, 3)
	for i := 0; i < 3; i++ {
		actID := uuid.New()
		questions = append(questions, &questionbankcontract.Question{
			ID: uuid.New(), ActivityID: &actID, ExamPartID: &partID,
			Kind: kindListeningComprehension, CEFRLevel: "B2", QuestionCount: 1,
		})
	}
	svc := service.New(service.Deps{
		Repo:         repo,
		Questionbank: &fakeQuestionbank{byPart: map[uuid.UUID][]*questionbankcontract.Question{partID: questions}},
	})

	require.NoError(t, svc.ComposeAllFixedTests(context.Background()))
	require.Len(t, repo.mockTests, 1, "one test's worth composes exactly one numbered test")
	require.NotNil(t, repo.mockTests[0].Number)
	assert.Equal(t, 1, *repo.mockTests[0].Number)
}
