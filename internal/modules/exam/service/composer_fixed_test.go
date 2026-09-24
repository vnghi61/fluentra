package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	"github.com/fluentra/fluentra/internal/modules/exam/service"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
)

// kindListeningComprehension is the part kind this gate's bank holds.
const kindListeningComprehension = "listening_comprehension"

// fakeQuestionbank answers DrawableForPart from a fixed per-part list.
type fakeQuestionbank struct {
	byPart map[uuid.UUID][]*questionbankcontract.Question
}

func (f *fakeQuestionbank) GetQuestion(
	_ context.Context, _ uuid.UUID,
) (*questionbankcontract.Question, error) {
	return nil, nil
}

func (f *fakeQuestionbank) ListQuestions(
	_ context.Context, _ questionbankcontract.Filter,
) ([]*questionbankcontract.Question, int, error) {
	return nil, 0, nil
}

func (f *fakeQuestionbank) SampleQuestions(
	_ context.Context, _ questionbankcontract.SampleCriteria,
) ([]*questionbankcontract.Question, error) {
	return nil, nil
}

func (f *fakeQuestionbank) GetQuestionStats(
	_ context.Context, _ uuid.UUID,
) (*questionbankcontract.QuestionStats, error) {
	return nil, nil
}

func (f *fakeQuestionbank) DrawableForPart(
	_ context.Context, partID uuid.UUID,
) ([]*questionbankcontract.Question, error) {
	return f.byPart[partID], nil
}

// TestComposeNextFixedTests_DisjointAndStopsWhenShort is Stage J's gate: a bank
// holding exactly five tests' worth yields Tests 1–5, refuses Test 6 naming the
// short part, and no question appears twice.
func TestComposeNextFixedTests_DisjointAndStopsWhenShort(t *testing.T) {
	partID := uuid.New()
	blueprintID := uuid.New()
	versionID := uuid.New()

	part := &domain.ExamPart{
		ID:            partID,
		VersionID:     versionID,
		Section:       "listening",
		PartNumber:    1,
		Kind:          kindListeningComprehension,
		QuestionCount: 3,
		GroupSize:     1,
	}

	// Exactly five tests' worth: fifteen single-question activities.
	questions := make([]*questionbankcontract.Question, 0, 15)
	for i := 0; i < 15; i++ {
		actID := uuid.New()
		questions = append(questions, &questionbankcontract.Question{
			ID:            uuid.New(),
			ActivityID:    &actID,
			ExamPartID:    &partID,
			Kind:          kindListeningComprehension,
			CEFRLevel:     "B1",
			QuestionCount: 1,
		})
	}

	repo := newMockExamRepo()
	repo.blueprint = &domain.Blueprint{
		ID:               blueprintID,
		VersionID:        versionID,
		Name:             "toeic_default",
		CefrDistribution: json.RawMessage(`{"B1": 1.0}`),
	}
	repo.parts = []*domain.ExamPart{part}

	svc := service.New(service.Deps{
		Repo:         repo,
		Questionbank: &fakeQuestionbank{byPart: map[uuid.UUID][]*questionbankcontract.Question{partID: questions}},
	})

	composed, err := svc.ComposeNextFixedTests(context.Background(), blueprintID)
	require.NoError(t, err)
	assert.Equal(t, 5, composed)
	require.Len(t, repo.mockTests, 5)

	// No question appears twice across the five tests.
	seen := map[uuid.UUID]bool{}
	for _, mt := range repo.mockTests {
		require.NotNil(t, mt.Number)
		for _, comp := range mt.Composition {
			for _, id := range comp.ActivityIDs {
				assert.Falsef(t, seen[id], "activity %s reused across fixed tests", id)
				seen[id] = true
			}
		}
	}
	assert.Len(t, seen, 15)

	// Test 6 has nothing left: refused, naming the part, and no row is written.
	_, err = svc.ComposeFixedTest(context.Background(), blueprintID, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), partID.String())
	assert.Len(t, repo.mockTests, 5)

	// Asking again for a stored test returns it, not a new composition.
	stored, err := svc.ComposeFixedTest(context.Background(), blueprintID, 1)
	require.NoError(t, err)
	require.NotNil(t, stored.Number)
	assert.Equal(t, 1, *stored.Number)
	assert.Len(t, repo.mockTests, 5)
}
