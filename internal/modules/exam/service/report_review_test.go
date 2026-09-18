package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/exam/service"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
)

// reviewLessons resolves an activity to the item as authored. Only
// ResolveActivity is called by the exam service; the embedded interface keeps
// the fake honest about that.
type reviewLessons struct {
	lessoncontract.Reader
	configs map[uuid.UUID]json.RawMessage
}

func (r reviewLessons) ResolveActivity(_ context.Context, id uuid.UUID) (*lessoncontract.ActivityHierarchy, error) {
	return &lessoncontract.ActivityHierarchy{ActivityID: id, Config: r.configs[id]}, nil
}

// A report told the learner "1 of 4 questions correct" and nothing more. It now
// carries, for a submitted sitting, what was answered and the item as authored,
// so the review can show each question, the choice, the right answer and why.
func TestReport_CarriesTheAnswerAndTheItemForReview(t *testing.T) {
	f := newFixture(t)
	listening := f.activity(1)
	authored := json.RawMessage(`{"script":"Flight 402 is delayed.","questions":[{"id":"q1","correct_option_id":"A"}]}`)
	f.svc = service.New(service.Deps{
		Repo:       f.repo,
		Learning:   f.learning,
		Attempts:   f.learning,
		Exposures:  f.exposures,
		Drawer:     &fakeDrawer{sections: f.sections},
		Clock:      f.clock,
		DailyLimit: 5,
		Enqueuer:   &fakeJobEnqueuer{},
		Lesson:     reviewLessons{configs: map[uuid.UUID]json.RawMessage{listening.ID: authored}},
	})
	userID := uuid.New()
	att := submitPractice(t, f, userID, answer(listening.ID, `{"answers":{"q1":"B"}}`))

	report, err := f.svc.GetScoreReport(context.Background(), userID, att.ID)
	require.NoError(t, err)

	item := report.PerSection[0].Items[0]
	assert.JSONEq(t, `{"answers":{"q1":"B"}}`, string(item.Response))
	assert.JSONEq(t, string(authored), string(item.Content))

	unanswered := report.PerSection[1].Items[0]
	assert.Empty(t, unanswered.Response, "an unanswered item has no response to show")
}
