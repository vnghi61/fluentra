package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/fluentra/fluentra/internal/generated/exam/sqlc"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
)

// resolvedKind is the kind every batch-resolved activity carries.
const resolvedKind = "vocab_multiple_choice"

// batchOnlyLessons resolves activities only in batches and fails the test on a
// one-by-one read.
type batchOnlyLessons struct {
	stubLessonReader
	t       *testing.T
	batches int
}

func (b *batchOnlyLessons) ResolveActivity(context.Context, uuid.UUID) (*lessoncontract.ActivityHierarchy, error) {
	b.t.Fatal("activities must be resolved in one batch, not one query each")
	return nil, nil
}

func (b *batchOnlyLessons) ResolveActivities(
	_ context.Context, ids []uuid.UUID,
) (map[uuid.UUID]*lessoncontract.ActivityHierarchy, error) {
	b.batches++
	out := make(map[uuid.UUID]*lessoncontract.ActivityHierarchy, len(ids))
	for _, id := range ids {
		out[id] = &lessoncontract.ActivityHierarchy{
			ActivityID: id, Kind: resolvedKind, Config: []byte(`{"prompt":"p"}`),
		}
	}
	return out, nil
}

// TestSittingAndReview_ResolveEveryActivityInOneRead pins the exam N+1: a TOEIC
// test holds 142 activities, and starting it or opening its report resolved
// them with a query each.
func TestSittingAndReview_ResolveEveryActivityInOneRead(t *testing.T) {
	lessons := &batchOnlyLessons{t: t}
	svc := New(Deps{Lesson: lessons})

	comp := []domain.MockTestPartComposition{
		{PartID: uuid.New(), ActivityIDs: []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}},
		{PartID: uuid.New(), ActivityIDs: []uuid.UUID{uuid.New(), uuid.New()}},
	}
	drawn, _ := svc.compositionToSectionActivities(context.Background(), comp, nil)
	require.Len(t, drawn, 1)
	require.Len(t, drawn[0].Activities, 5)
	for _, act := range drawn[0].Activities {
		assert.Equal(t, resolvedKind, act.Kind, "every activity was resolved")
	}
	assert.Equal(t, 1, lessons.batches, "starting a sitting resolves its activities once")

	sections := []domain.SectionOutcome{{Items: []domain.ItemOutcome{
		{ActivityID: uuid.New()}, {ActivityID: uuid.New()}, {ActivityID: uuid.New()},
	}}}
	svc.attachReview(context.Background(), &sqlc.AssessExamAttempt{ID: uuid.New(), Status: "submitted"}, sections)
	assert.Equal(t, 2, lessons.batches, "opening a report resolves its items once")
	for _, item := range sections[0].Items {
		assert.NotNil(t, item.Content, "every reviewed item carries its content")
	}
}
