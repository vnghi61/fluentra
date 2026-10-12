package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// placementWordBounds mirrors the passage and script lengths the pool accepts.
var placementWordBounds = map[string][4]int{
	"A1": {60, 90, 45, 70},
	"A2": {80, 110, 55, 85},
	"B1": {100, 140, 70, 105},
	"B2": {120, 160, 85, 125},
	"C1": {140, 180, 100, 140},
}

func words(prefix string, n int) string {
	return prefix + strings.Repeat(" word", n-len(strings.Fields(prefix)))
}

const explanation = `{"explanation_en": "Because.", "explanation_vi": "Vì vậy."}`

func questionsJSON() string {
	var qs []string
	for i := 1; i <= 3; i++ {
		qs = append(qs, fmt.Sprintf(`{"id": "q%d", "type": "multiple_choice", "prompt": "Question %d?",
			"options": [{"id": "A", "text": "yes"}, {"id": "B", "text": "no"}, {"id": "C", "text": "maybe"}],
			"correct_option_id": "A", "explanation": %s}`, i, i, explanation))
	}
	return strings.Join(qs, ",")
}

// placementAI writes a valid, distinct placement item for every kind and level,
// unless broken is set, in which case every item has the wrong shape.
type placementAI struct {
	mu        sync.Mutex
	generated int
	broken    bool
}

func (p *placementAI) Complete(_ context.Context, req ai.Request) (ai.Response, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	kind, _ := req.Vars["Kind"].(string)
	if req.Task == ai.TaskPlacementSolve {
		if kind == kindChoice {
			return ai.Response{Text: solveOptionA}, nil
		}
		return ai.Response{Text: `{"answers": {"q1": "A", "q2": "A", "q3": "A"}}`}, nil
	}
	p.generated++
	level, _ := req.Vars["CEFRLevel"].(string)
	return ai.Response{Text: p.item(kind, level, p.generated)}, nil
}

func (p *placementAI) item(kind, level string, n int) string {
	bounds := placementWordBounds[level]
	options := `[{"id": "A", "text": "one"}, {"id": "B", "text": "two"}, ` +
		`{"id": "C", "text": "three"}, {"id": "D", "text": "four"}]`
	passage, script, minWords, seconds := bounds[0]+5, bounds[2]+5, 60, 45
	if p.broken {
		options = `[{"id": "A", "text": "one"}, {"id": "B", "text": "two"}, {"id": "C", "text": "three"}]`
		passage, script, minWords, seconds = bounds[0]-10, bounds[3]+10, 40, 30
	}
	switch kind {
	case "vocabulary", kindChoice:
		return fmt.Sprintf(`{"prompt": "Item %d: choose the word.", "options": %s, "correct_option_id": "A",
			"explanation": %s}`, n, options, explanation)
	case kindReading:
		return fmt.Sprintf(`{"passage_title": "T", "passage": %q, "questions": [%s]}`,
			words(fmt.Sprintf("Passage %d", n), passage), questionsJSON())
	case kindListening:
		return fmt.Sprintf(`{"title": "T", "script": %q, "questions": [%s]}`,
			words(fmt.Sprintf("Script %d", n), script), questionsJSON())
	case "writing_prompt":
		return fmt.Sprintf(`{"prompt": %q, "model_answer": %q, "min_words": %d}`,
			words(fmt.Sprintf("Task %d: write to a friend about", n), 20), words("Dear friend", 90), minWords)
	case "speaking_task":
		return fmt.Sprintf(`{"task_type": "respond", "prompt": %q, "speaking_time_seconds": %d}`,
			words(fmt.Sprintf("Talk %d about a place you like", n), 12), seconds)
	}
	return `{}`
}

func newPlacementPoolFixture(t *testing.T, aiClient ai.Client) examPoolFixture {
	t.Helper()
	graders := domain.NewGraderRegistry()
	for _, kind := range []string{kindChoice, kindReading, kindListening, kindWriting, kindSpeaking} {
		require.NoError(t, graders.Register(kind, placementGrader{}))
	}
	f := examPoolFixture{
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
		Clock:             clock.Real{},
		GeneratorAuthorID: uuid.New(),
	})
	require.NoError(t, f.svc.EnsurePlacementPoolStructure(context.Background()))
	return f
}

func TestPlacementPool_HasAUnitPerLevelAndALessonPerSlot(t *testing.T) {
	f := newPlacementPoolFixture(t, &placementAI{})
	ctx := context.Background()

	courseID := f.lessons.ids["course/"+service.PlacementPoolCourseSlug]
	require.NotEqual(t, uuid.Nil, courseID)
	units, err := f.lessons.ListUnitsByCourseID(ctx, courseID)
	require.NoError(t, err)
	assert.Len(t, units, 5, "A1 to C1")
	for _, unit := range units {
		lessons, err := f.lessons.ListLessons(ctx, unit.ID)
		require.NoError(t, err)
		assert.Len(t, lessons, 6, "vocabulary, grammar, reading, listening, writing and speaking")
	}
}

func TestPlacementPool_TopUpAddsFivePerSlotAndPublishesVocabularyAsATenseChoice(t *testing.T) {
	f := newPlacementPoolFixture(t, &placementAI{})
	require.NoError(t, f.svc.TopUpPlacementPool(context.Background()))

	published := f.authors.published()
	assert.Len(t, published, 5*30, "five items for each of the 30 slots")
	kinds := map[string]int{}
	for _, spec := range published {
		kinds[spec.Kind]++
		assert.True(t, strings.HasPrefix(spec.Slug, "pool-placement-"), spec.Slug)
		assert.Regexp(t, kebabSlug, spec.Slug)
		if strings.Contains(spec.Slug, "-vocabulary-") {
			assert.Equal(t, kindChoice, spec.Kind,
				"vocabulary questions use the self-contained tense-choice shape and its grader")
		}
	}
	assert.Equal(t, 2*25, kinds[kindChoice], "vocabulary and grammar")
	assert.Equal(t, 25, kinds["reading_comprehension"])
	assert.Equal(t, 25, kinds["listening_comprehension"])
	assert.Equal(t, 25, kinds["writing_prompt"])
	assert.Equal(t, 25, kinds["speaking_task"])
	assert.NotContains(t, kinds, "vocabulary", "no activity kind without a grader is published")
}

func TestPlacementPool_ItemsOfTheWrongShapeAreRejected(t *testing.T) {
	f := newPlacementPoolFixture(t, &placementAI{broken: true})
	require.NoError(t, f.svc.TopUpPlacementPool(context.Background()))
	assert.Zero(t, f.lessons.appendedCount(),
		"three options, a short passage, a long script, a 40-word task and a 30-second answer all fail the checks")
}

func TestPlacementPool_NoItemIsSharedAcrossThePools(t *testing.T) {
	aiClient := &placementAI{}
	f := newPlacementPoolFixture(t, aiClient)
	f.svc = service.New(service.Deps{
		Repo: f.repo, Lesson: f.lessons, LessonAuthor: f.lessons, Content: f.content, ContentAuthor: f.authors,
		Graders: examPassingGradersWith(t), Clock: clock.Real{}, GeneratorAuthorID: uuid.New(),
	})
	ctx := context.Background()
	require.NoError(t, f.svc.EnsurePracticePoolStructure(ctx))
	require.NoError(t, f.svc.EnsureExamPoolStructure(ctx))
	require.NoError(t, f.svc.EnsurePlacementPoolStructure(ctx))

	placement := map[uuid.UUID]bool{}
	practice := map[uuid.UUID]bool{}
	for i := 0; i < 8; i++ {
		body := json.RawMessage(choiceBody("B1", i))
		placement[f.lessons.seedCourse(t, service.PlacementPoolCourseSlug, "B1", "Grammar Tense Choice",
			kindChoice, uuid.New(), body)] = true
		for _, slot := range poolSlots {
			practice[f.lessons.seed(t, "B1", slot.title, slot.kind, uuid.New(), body)] = true
		}
	}
	f.seedFullExamPool(t, "B1", 1)
	exam := map[uuid.UUID]bool{}
	for id, activity := range f.lessons.byID {
		slug := f.lessons.courseSlug[f.lessons.unitCourse[f.lessons.lessonUnit[activity.LessonID]]]
		if !placement[id] && !practice[id] && slug == service.ExamPoolCourseSlug {
			exam[id] = true
		}
	}

	user := uuid.New()
	daily, err := f.svc.GetDailySet(ctx, user, "B1")
	require.NoError(t, err)
	for _, activity := range daily.Activities {
		assert.False(t, placement[activity.ID] || exam[activity.ID], "a daily set holds only practice items")
	}

	sitting, err := f.svc.DrawExamSitting(ctx, user, "B1")
	require.NoError(t, err)
	for _, section := range sitting {
		for _, activity := range section.Activities {
			assert.False(t, placement[activity.ID] || practice[activity.ID], "a sitting holds only exam items")
		}
	}

	for id := range practice {
		assert.False(t, placement[id] || exam[id])
	}
}

func TestPlacementPool_APoolItemIsRefusedByTheLessonRoutes(t *testing.T) {
	f := newPlacementPoolFixture(t, &placementAI{})
	ctx := context.Background()
	id := f.lessons.seedCourse(t, service.PlacementPoolCourseSlug, "B1", "Vocabulary Multiple Choice",
		kindChoice, uuid.New(), json.RawMessage(choiceBody("B1", 1)))

	_, err := f.svc.StartAttempt(ctx, uuid.New(), id)
	assert.ErrorIs(t, err, domain.ErrUnauthorizedAttemptAccess, "no standalone attempt on a placement item")

	_, err = f.svc.GradePreview(ctx, id, json.RawMessage(`{"selected_option_id": "A"}`))
	assert.ErrorIs(t, err, domain.ErrActivityNotFound, "the preview grade does not reveal a placement answer")
}

func TestDailySet_UsesThePlacedLevelHeldToA2B2UnlessALevelIsChosen(t *testing.T) {
	f := newPlacementPoolFixture(t, &placementAI{})
	ctx := context.Background()
	require.NoError(t, f.svc.EnsurePracticePoolStructure(ctx))
	for _, level := range []string{"A2", "B1", "B2"} {
		for i := 0; i < 6; i++ {
			for _, slot := range poolSlots {
				f.lessons.seed(t, level, slot.title, slot.kind, uuid.New(), json.RawMessage(choiceBody(level, i)))
			}
		}
	}

	placed := uuid.New()
	_, err := f.repo.CreatePlacementResult(ctx, &domain.PlacementResult{
		UserID: placed, Level: domain.LevelC1, PerSkill: map[string]domain.SkillEstimate{},
	})
	require.NoError(t, err)

	fromPlacement, err := f.svc.GetDailySet(ctx, placed, "")
	require.NoError(t, err)
	assert.Equal(t, domain.LevelB2, fromPlacement.Level, "C1 is held to B2")

	chosen, err := f.svc.GetDailySet(ctx, uuid.New(), "A2")
	require.NoError(t, err)
	assert.Equal(t, domain.LevelA2, chosen.Level, "a chosen practice level still wins")

	neither, err := f.svc.GetDailySet(ctx, uuid.New(), "")
	require.NoError(t, err)
	assert.Equal(t, domain.LevelB1, neither.Level)
}

// examPassingGradersWith registers a grader for every kind the three pools draw.
func examPassingGradersWith(t *testing.T) *domain.GraderRegistry {
	t.Helper()
	graders := examPassingGraders()
	require.NoError(t, graders.Register(kindChoice, &testPracticeGrader{shouldPass: true}))
	return graders
}
