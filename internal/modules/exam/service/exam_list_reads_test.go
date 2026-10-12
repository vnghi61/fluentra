package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/exam/service"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
)

// countsOnlyQuestionbank answers coverage from counts and fails the test if the
// questions themselves are read.
type countsOnlyQuestionbank struct {
	*fakeQuestionbank
	t     *testing.T
	reads int
}

func (c *countsOnlyQuestionbank) DrawableForPart(
	context.Context, uuid.UUID,
) ([]*questionbankcontract.Question, error) {
	c.t.Fatal("the exam list must count the bank, not read every question of every part")
	return nil, nil
}

func (c *countsOnlyQuestionbank) DrawableSizes(
	ctx context.Context, partIDs []uuid.UUID,
) (map[uuid.UUID]map[int]int, error) {
	c.reads++
	return c.fakeQuestionbank.DrawableSizes(ctx, partIDs)
}

// TestListCurrentExamVersions_CountsTheBankOnce pins the exam list's N+1: it
// read every drawable question of every part, one part at a time, to show one
// number per version.
func TestListCurrentExamVersions_CountsTheBankOnce(t *testing.T) {
	repo := dailyRepo()
	activity := uuid.New()
	questions := make([]*questionbankcontract.Question, 3)
	for i := range questions {
		questions[i] = &questionbankcontract.Question{ID: uuid.New(), ActivityID: &activity, QuestionCount: 1}
	}
	bank := &countsOnlyQuestionbank{
		fakeQuestionbank: &fakeQuestionbank{byPart: map[uuid.UUID][]*questionbankcontract.Question{
			repo.parts[0].ID: questions,
		}},
		t: t,
	}
	svc := service.New(service.Deps{Repo: repo, Questionbank: bank})

	versions, err := svc.ListCurrentExamVersions(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Len(t, versions, 1)
	assert.Equal(t, 1, versions[0].DistinctTestsPossible, "three one-question items fill one three-question part")
	assert.Equal(t, 1, bank.reads, "every part is counted in one read")
}
