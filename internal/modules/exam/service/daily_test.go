package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	"github.com/fluentra/fluentra/internal/modules/exam/service"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
)

// testBlueprintName is the TOEIC blueprint the daily and fixed-test tests use.
const (
	testBlueprintName = "toeic_default"
	testVersionCode   = "TOEIC_LR_2026"
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
	repo.version = &domain.ExamVersion{ID: versionID, Code: testVersionCode}
	repo.parts = []*domain.ExamPart{part}
	repo.blueprint = &domain.Blueprint{
		ID:               blueprintID,
		VersionID:        versionID,
		Name:             testBlueprintName,
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

	composed, err := svc.GenerateDailyExam(context.Background(), testVersionCode)
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
	repo.version = &domain.ExamVersion{ID: versionID, Code: testVersionCode}
	repo.parts = []*domain.ExamPart{{
		ID: partID, VersionID: versionID, Section: testSkillListening, PartNumber: 1,
		Kind: kindListeningComprehension, QuestionCount: 3, GroupSize: 1,
	}}
	repo.blueprint = &domain.Blueprint{
		ID: uuid.New(), VersionID: versionID, Name: testBlueprintName,
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

// fakeBacklog answers the review backlog with a fixed number of days.
type fakeBacklog struct {
	days   int
	prefix string
}

func (f *fakeBacklog) PendingBatchDays(_ context.Context, prefix string) (int, error) {
	f.prefix = prefix
	return f.days, nil
}

// dailyRepo is one TOEIC version with one part of three questions.
func dailyRepo() *mockExamRepo {
	versionID := uuid.New()
	repo := newMockExamRepo()
	repo.version = &domain.ExamVersion{ID: versionID, Code: testVersionCode}
	repo.parts = []*domain.ExamPart{{
		ID: uuid.New(), VersionID: versionID, Section: testSkillListening, PartNumber: 1,
		Kind: kindListeningComprehension, QuestionCount: 3, GroupSize: 1,
	}}
	repo.blueprint = &domain.Blueprint{
		ID: uuid.New(), VersionID: versionID, Name: testBlueprintName,
		CefrDistribution: json.RawMessage(`{"B2": 1.0}`),
	}
	return repo
}

// TestGenerateDaily_SkipsAnExamWhoseDoubtsWait is Stage O trap 1: with an
// exam's escalated batches from two earlier days unreviewed, the day generates
// nothing for it.
func TestGenerateDaily_SkipsAnExamWhoseDoubtsWait(t *testing.T) {
	bank := &fakeBankAuthor{}
	backlog := &fakeBacklog{days: 2}
	svc := service.New(service.Deps{Repo: dailyRepo(), BankAuthor: bank, ReviewBacklog: backlog})

	require.NoError(t, svc.GenerateDaily(context.Background()))
	assert.Empty(t, bank.calls, "a backlogged exam generates nothing")
	assert.True(t, strings.HasPrefix(backlog.prefix, "bank:"), "the backlog is read by the exam's batch prefix")

	backlog.days = 1
	require.NoError(t, svc.GenerateDaily(context.Background()))
	assert.NotEmpty(t, bank.calls, "one earlier day of doubts does not stop the job")
}

// TestGenerateDailyExam_HonoursTheCap: a run asks for no more items than its cap.
func TestGenerateDailyExam_HonoursTheCap(t *testing.T) {
	bank := &fakeBankAuthor{}
	svc := service.New(service.Deps{Repo: dailyRepo(), BankAuthor: bank, DailyGenerationCap: 2})

	_, err := svc.GenerateDailyExam(context.Background(), testVersionCode)
	require.NoError(t, err)
	require.Len(t, bank.calls, 1)
	assert.Equal(t, 2, bank.calls[0].Count, "the cap bounds the run")
}
