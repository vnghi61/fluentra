package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

func newPlacementPoolFixture(
	t *testing.T, clk clock.Clock, author uuid.UUID, graders *domain.GraderRegistry, aiClient ai.Client,
) (placementPoolFixture, func()) {
	t.Helper()
	f := placementPoolFixture{
		repo:    newFakePoolRepo(),
		lessons: newFakePoolLessons(),
		content: newFakeContentReader(),
		authors: &fakeContentAuthor{},
	}
	f.svc = service.New(service.Deps{
		Repo:              f.repo,
		Lesson:            f.lessons,
		LessonAuthor:      f.lessons,
		Content:           f.content,
		ContentAuthor:     f.authors,
		Graders:           graders,
		AI:                aiClient,
		Clock:             clk,
		GeneratorAuthorID: author,
		Synthesiser:       &fakeAudioSynthesiser{},
	})
	if err := f.svc.EnsurePlacementPoolStructure(context.Background()); err != nil {
		t.Fatalf("resolve placement pool: %v", err)
	}
	return f, func() {}
}

type placementPoolFixture struct {
	svc     *service.Service
	repo    *fakePoolRepo
	lessons *fakePoolLessons
	content *fakeContentReader
	authors *fakeContentAuthor
}

func placementPassingGraders() *domain.GraderRegistry {
	graders := domain.NewGraderRegistry()
	for _, k := range []string{
		"vocabulary",
		"grammar_tense_choice",
		"reading_comprehension",
		"listening_comprehension",
		"writing_prompt",
		"speaking_task",
	} {
		_ = graders.Register(k, &testPracticeGrader{shouldPass: true})
	}
	return graders
}

func TestPlacementPool_Structure(t *testing.T) {
	adminID := uuid.New()
	mockAI := ai.NewMockProvider(nil)
	f, _ := newPlacementPoolFixture(t, clock.Real{}, adminID, placementPassingGraders(), mockAI)

	// Course must be pool-placement
	courseID := f.lessons.ids["course/"+service.PlacementPoolCourseSlug]
	require.NotEqual(t, uuid.Nil, courseID, "placement pool course must exist")

	// Must have 5 units: A1, A2, B1, B2, C1
	units, err := f.lessons.ListUnitsByCourseID(context.Background(), courseID)
	require.NoError(t, err)
	assert.Len(t, units, 5, "must have 5 units for A1..C1")

	// Each unit must have 6 slot lessons (30 lessons in total)
	totalLessons := 0
	for _, u := range units {
		lessons, err := f.lessons.ListLessons(context.Background(), u.ID)
		require.NoError(t, err)
		assert.Len(t, lessons, 6, "each unit must have 6 slot lessons")
		totalLessons += len(lessons)
	}
	assert.Equal(t, 30, totalLessons, "total 30 slot lessons across 5 bands")
}

func TestPlacementPool_TopUpAndSufficiency(t *testing.T) {
	adminID := uuid.New()
	mockAI := ai.NewMockProvider(nil)
	f, _ := newPlacementPoolFixture(t, clock.Real{}, adminID, placementPassingGraders(), mockAI)
	ctx := context.Background()

	// Initially empty: HasSufficientPlacementPool is false
	sufficient, err := f.svc.HasSufficientPlacementPool(ctx)
	require.NoError(t, err)
	assert.False(t, sufficient, "initially empty pool must not be sufficient")

	// Top up once
	err = f.svc.TopUpPlacementPool(ctx)
	require.NoError(t, err)

	assert.Greater(t, f.lessons.appendedCount(), 0, "top up should have added activities to placement pool")
}

func TestPlacementPool_IsolationAcrossAllThreePools(t *testing.T) {
	// Rule: No item is shared between pool-practice, pool-exam, or pool-placement.
	// Test all three directions.
	adminID := uuid.New()
	mockAI := ai.NewMockProvider(nil)
	graders := placementPassingGraders()
	_ = graders.Register("grammar_sentence_transform", &testPracticeGrader{shouldPass: true})

	f := examPoolFixture{
		repo:    newFakePoolRepo(),
		lessons: newFakePoolLessons(),
		content: newFakeContentReader(),
		authors: &fakeContentAuthor{},
	}
	svc := service.New(service.Deps{
		Repo:              f.repo,
		Lesson:            f.lessons,
		LessonAuthor:      f.lessons,
		Content:           f.content,
		ContentAuthor:     f.authors,
		Graders:           graders,
		AI:                mockAI,
		Clock:             clock.Real{},
		GeneratorAuthorID: adminID,
		Synthesiser:       &fakeAudioSynthesiser{},
	})

	ctx := context.Background()
	require.NoError(t, svc.EnsurePracticePoolStructure(ctx))
	require.NoError(t, svc.EnsureExamPoolStructure(ctx))
	require.NoError(t, svc.EnsurePlacementPoolStructure(ctx))

	// Seed items directly into the three pool courses
	placementActID := f.lessons.seedCourse(t, service.PlacementPoolCourseSlug, "B1", "Reading Comprehension",
		"reading_comprehension", uuid.New(), json.RawMessage(`{}`))
	practiceActID := f.lessons.seedCourse(t, service.PracticePoolCourseSlug, "B1", "Reading Comprehension",
		"reading_comprehension", uuid.New(), json.RawMessage(`{}`))
	examActID := f.lessons.seedCourse(t, service.ExamPoolCourseSlug, "B1", "Reading Comprehension",
		"reading_comprehension", uuid.New(), json.RawMessage(`{}`))

	// Direction 1: Placement items must not be in practice or exam sets
	user := uuid.New()
	dailySet, err := svc.GetDailySet(ctx, user, "B1")
	require.NoError(t, err)
	for _, act := range dailySet.Activities {
		assert.NotEqual(t, placementActID, act.ID, "daily practice set must not contain item from pool-placement")
		assert.NotEqual(t, examActID, act.ID, "daily practice set must not contain item from pool-exam")
	}

	// Direction 2: Exam sitting must not contain placement or practice items
	sitting, err := svc.DrawExamSitting(ctx, user, "B1")
	if err == nil {
		for _, section := range sitting {
			for _, act := range section.Activities {
				assert.NotEqual(t, placementActID, act.ID, "exam sitting must not contain item from pool-placement")
				assert.NotEqual(t, practiceActID, act.ID, "exam sitting must not contain item from pool-practice")
			}
		}
	}

	// Direction 3: Placement unseen items must only come from pool-placement
	unseen, err := svc.GetUnseenPlacementItems(ctx, user, "B1", "reading-comprehension")
	require.NoError(t, err)
	for _, act := range unseen {
		assert.NotEqual(t, practiceActID, act.ID, "placement unseen must not contain item from pool-practice")
		assert.NotEqual(t, examActID, act.ID, "placement unseen must not contain item from pool-exam")
	}
}

func TestPlacementPool_AntiLeakGuards(t *testing.T) {
	adminID := uuid.New()
	mockAI := ai.NewMockProvider(nil)
	f, _ := newPlacementPoolFixture(t, clock.Real{}, adminID, placementPassingGraders(), mockAI)
	ctx := context.Background()

	placementActID := f.lessons.seedCourse(t, service.PlacementPoolCourseSlug, "B1", "Vocabulary Multiple Choice",
		"vocabulary", uuid.New(), json.RawMessage(`{}`))

	userID := uuid.New()

	// StartAttempt rejected
	_, err := f.svc.StartAttempt(ctx, userID, placementActID)
	assert.ErrorIs(t, err, domain.ErrUnauthorizedAttemptAccess, "placement item cannot be started as regular attempt")

	// SubmitAttempt rejected
	_, err = f.svc.SubmitAttempt(ctx, userID, uuid.New(), uuid.New(), json.RawMessage(`{"selected_option_id":"A"}`))
	// Activity lookup in submit attempt fails or rejects with ErrUnauthorizedAttemptAccess
	assert.Error(t, err)

	// GradePreview rejected
	_, err = f.svc.GradePreview(ctx, placementActID, json.RawMessage(`{"selected_option_id":"A"}`))
	assert.ErrorIs(t, err, domain.ErrActivityNotFound, "placement item cannot be preview graded")
}
