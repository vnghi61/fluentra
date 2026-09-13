package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const (
	examKindListeningComprehension = "listening_comprehension"
	examKindReadingComprehension   = "reading_comprehension"
	examKindGrammarSentenceTransform = "grammar_sentence_transform"
	examKindWritingPrompt          = "writing_prompt"
	examKindSpeakingTask           = "speaking_task"
)

type examPoolFixture struct {
	svc     *service.Service
	repo    *fakePoolRepo
	lessons *fakePoolLessons
	content *fakeContentReader
	authors *fakeContentAuthor
}

type fakeAudioSynthesiser struct {
	key string
	err error
}

func (f *fakeAudioSynthesiser) Synthesise(_ context.Context, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if f.key != "" {
		return f.key, nil
	}
	return "audio/listening/" + uuid.New().String() + ".mp3", nil
}

type fakeAuthorResolver struct {
	authorID uuid.UUID
}

func (f *fakeAuthorResolver) FirstHolderOf(_ context.Context, _ string) (uuid.UUID, error) {
	return f.authorID, nil
}

func examPassingGraders() *domain.GraderRegistry {
	graders := domain.NewGraderRegistry()
	for _, k := range []string{
		examKindListeningComprehension,
		examKindReadingComprehension,
		examKindGrammarSentenceTransform,
		examKindWritingPrompt,
		examKindSpeakingTask,
	} {
		_ = graders.Register(k, &testPracticeGrader{shouldPass: true})
	}
	return graders
}

func newExamPoolFixture(
	t *testing.T, clk clock.Clock, author uuid.UUID, graders *domain.GraderRegistry, aiClient ai.Client,
) examPoolFixture {
	t.Helper()
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
		Clock:             clk,
		GeneratorAuthorID: author,
		Synthesiser:       &fakeAudioSynthesiser{},
	})
	if err := f.svc.EnsureExamPoolStructure(context.Background()); err != nil {
		t.Fatalf("resolve exam pool: %v", err)
	}
	return f
}

// seedExamSlot seeds activities into an exam pool slot.
func (f examPoolFixture) seedExamSlot(
	t *testing.T, level, slotTitle, kind string, count int, configGen func(i int) json.RawMessage,
) []uuid.UUID {
	t.Helper()
	var ids []uuid.UUID
	for i := 0; i < count; i++ {
		versionID := uuid.New()
		body := configGen(i)
		f.content.versions[versionID] = &contentcontract.Version{ID: versionID, Kind: kind, Body: body}
		actID := f.lessons.seedCourse(t, "pool-exam", level, slotTitle, kind, versionID, body)
		ids = append(ids, actID)
	}
	return ids
}

// seedFullExamPool seeds sufficient items for all 6 slots at a level.
func (f examPoolFixture) seedFullExamPool(t *testing.T, level string, multiplier int) {
	t.Helper()
	// Listening: 3 * multiplier
	f.seedExamSlot(t, level, "Listening Comprehension", examKindListeningComprehension, 3*multiplier, func(i int) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{
			"title": "Clip %d",
			"script": "Script content %d",
			"audio_object_key": "audio/listening/clip-%d.mp3",
			"questions": [{"id": "q1", "type": "multiple_choice", "prompt": "P1", "options": [{"id":"A","text":"1"}], "correct_option_id":"A"}]
		}`, i, i, i))
	})

	// Reading: 2 * multiplier
	f.seedExamSlot(t, level, "Reading Comprehension", examKindReadingComprehension, 2*multiplier, func(i int) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"passage": "Passage %d", "questions": []}`, i))
	})

	// Writing prompt: 1 * multiplier
	f.seedExamSlot(t, level, "Writing Prompt", examKindWritingPrompt, 1*multiplier, func(i int) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"prompt": "Essay prompt %d", "model_answer": "Model answer %d"}`, i, i))
	})

	// Sentence transform: 3 * multiplier
	f.seedExamSlot(t, level, "Grammar Sentence Transform", examKindGrammarSentenceTransform, 3*multiplier, func(i int) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"prompt": "Transform %d", "correct_answer": "Answer %d"}`, i, i))
	})

	// Speaking read aloud: 2 * multiplier
	f.seedExamSlot(t, level, "Speaking Read Aloud", examKindSpeakingTask, 2*multiplier, func(i int) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"task_type": "read_aloud", "reference_text": "Read this text aloud %d"}`, i))
	})

	// Speaking respond: 2 * multiplier
	f.seedExamSlot(t, level, "Speaking Respond", examKindSpeakingTask, 2*multiplier, func(i int) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"task_type": "respond", "prompt": "Describe your favourite place %d"}`, i))
	})
}

func extractAllActivityIDs(sections []learningcontract.ExamSectionActivities) []uuid.UUID {
	var ids []uuid.UUID
	for _, sec := range sections {
		for _, act := range sec.Activities {
			ids = append(ids, act.ID)
		}
	}
	return ids
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

func TestDrawExamSitting_CompositionAndSectionOrdering(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC))
	f := newExamPoolFixture(t, clk, uuid.New(), passingGraders(), nil)
	f.seedFullExamPool(t, "B1", 2)

	userID := uuid.New()
	sections, err := f.svc.DrawExamSitting(context.Background(), userID, "B1")
	require.NoError(t, err)
	require.Len(t, sections, 4)

	// Section 1: Listening (3 clips)
	assert.Equal(t, 1, sections[0].SectionPosition)
	assert.Equal(t, "listening", sections[0].Skill)
	assert.Len(t, sections[0].Activities, 3)
	for _, act := range sections[0].Activities {
		assert.Equal(t, examKindListeningComprehension, act.Kind)
	}

	// Section 2: Reading (2 passages)
	assert.Equal(t, 2, sections[1].SectionPosition)
	assert.Equal(t, "reading", sections[1].Skill)
	assert.Len(t, sections[1].Activities, 2)
	for _, act := range sections[1].Activities {
		assert.Equal(t, examKindReadingComprehension, act.Kind)
	}

	// Section 3: Writing (1 essay prompt + 3 sentence transforms = 4)
	assert.Equal(t, 3, sections[2].SectionPosition)
	assert.Equal(t, "writing", sections[2].Skill)
	assert.Len(t, sections[2].Activities, 4)
	essayCount, transformCount := 0, 0
	for _, act := range sections[2].Activities {
		if act.Kind == examKindWritingPrompt {
			essayCount++
		} else if act.Kind == examKindGrammarSentenceTransform {
			transformCount++
		}
	}
	assert.Equal(t, 1, essayCount)
	assert.Equal(t, 3, transformCount)

	// Section 4: Speaking (2 read-aloud + 2 respond = 4)
	assert.Equal(t, 4, sections[3].SectionPosition)
	assert.Equal(t, "speaking", sections[3].Skill)
	assert.Len(t, sections[3].Activities, 4)
	for _, act := range sections[3].Activities {
		assert.Equal(t, examKindSpeakingTask, act.Kind)
	}
}

// TestDrawExamSitting_TwoSittingsShareNoItemsWhileSlotsHaveUnseenItems asserts requirement:
// "Two sittings by one learner share no item while the slots have unseen items."
func TestDrawExamSitting_TwoSittingsShareNoItemsWhileSlotsHaveUnseenItems(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC))
	f := newExamPoolFixture(t, clk, uuid.New(), passingGraders(), nil)
	// Seed 2x sitting capacity (6 listening, 4 reading, 2 essay, 6 transforms, 4 read-aloud, 4 respond)
	f.seedFullExamPool(t, "B1", 2)

	userID := uuid.New()
	sitting1, err := f.svc.DrawExamSitting(context.Background(), userID, "B1")
	require.NoError(t, err)

	sitting1IDs := extractAllActivityIDs(sitting1)
	// Mark sitting 1 items as exposed
	for _, id := range sitting1IDs {
		err := f.repo.RecordItemExposure(context.Background(), userID, id)
		require.NoError(t, err)
	}

	sitting2, err := f.svc.DrawExamSitting(context.Background(), userID, "B1")
	require.NoError(t, err)
	sitting2IDs := extractAllActivityIDs(sitting2)

	// Verify no overlap
	sitting1Set := make(map[uuid.UUID]bool, len(sitting1IDs))
	for _, id := range sitting1IDs {
		sitting1Set[id] = true
	}

	for _, id := range sitting2IDs {
		assert.False(t, sitting1Set[id], "sitting 2 must not repeat item %s while unseen items exist in slot", id)
	}
}

// TestDrawExamSitting_SeenExhaustion_RepeatsOldest asserts requirement:
// "A learner who has seen every item still gets a full sitting."
func TestDrawExamSitting_SeenExhaustion_RepeatsOldest(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC))
	f := newExamPoolFixture(t, clk, uuid.New(), passingGraders(), nil)
	// Exactly 1x capacity
	f.seedFullExamPool(t, "B1", 1)

	userID := uuid.New()
	sitting1, err := f.svc.DrawExamSitting(context.Background(), userID, "B1")
	require.NoError(t, err)
	sitting1IDs := extractAllActivityIDs(sitting1)

	// Mark all as exposed
	for _, id := range sitting1IDs {
		err := f.repo.RecordItemExposure(context.Background(), userID, id)
		require.NoError(t, err)
	}

	// Draw second sitting: learner has seen every item, but must still receive a full sitting!
	sitting2, err := f.svc.DrawExamSitting(context.Background(), userID, "B1")
	require.NoError(t, err)
	sitting2IDs := extractAllActivityIDs(sitting2)

	assert.Equal(t, len(sitting1IDs), len(sitting2IDs), "sitting must not be short when learner has seen every item")
}

// TestDrawExamSitting_ListeningWithoutAudioNeverDrawn asserts requirement:
// "A listening item without audio is never drawn."
func TestDrawExamSitting_ListeningWithoutAudioNeverDrawn(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC))
	f := newExamPoolFixture(t, clk, uuid.New(), passingGraders(), nil)

	// Seed reading, writing, speaking
	f.seedExamSlot(t, "B1", "Reading Comprehension", examKindReadingComprehension, 2, func(i int) json.RawMessage {
		return json.RawMessage(`{"passage": "text"}`)
	})
	f.seedExamSlot(t, "B1", "Writing Prompt", examKindWritingPrompt, 1, func(i int) json.RawMessage {
		return json.RawMessage(`{"prompt": "essay", "model_answer": "answer"}`)
	})
	f.seedExamSlot(t, "B1", "Grammar Sentence Transform", examKindGrammarSentenceTransform, 3, func(i int) json.RawMessage {
		return json.RawMessage(`{"prompt": "transform", "correct_answer": "answer"}`)
	})
	f.seedExamSlot(t, "B1", "Speaking Read Aloud", examKindSpeakingTask, 2, func(i int) json.RawMessage {
		return json.RawMessage(`{"task_type": "read_aloud", "reference_text": "read"}`)
	})
	f.seedExamSlot(t, "B1", "Speaking Respond", examKindSpeakingTask, 2, func(i int) json.RawMessage {
		return json.RawMessage(`{"task_type": "respond", "prompt": "respond"}`)
	})

	// Seed 2 listening items with audio, and 5 listening items WITHOUT audio
	f.seedExamSlot(t, "B1", "Listening Comprehension", examKindListeningComprehension, 2, func(i int) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"script": "script %d", "audio_object_key": "audio/%d.mp3"}`, i, i))
	})
	f.seedExamSlot(t, "B1", "Listening Comprehension", examKindListeningComprehension, 5, func(i int) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"script": "script %d", "audio_object_key": ""}`, i))
	})

	userID := uuid.New()
	// Needs 3 with audio; only 2 exist -> must fail with pool empty/insufficient
	_, err := f.svc.DrawExamSitting(context.Background(), userID, "B1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient listening items with audio")

	// Now add a 3rd with audio
	f.seedExamSlot(t, "B1", "Listening Comprehension", examKindListeningComprehension, 1, func(i int) json.RawMessage {
		return json.RawMessage(`{"script": "script 3", "audio_object_key": "audio/3.mp3"}`)
	})

	// Now draw succeeds
	sections, err := f.svc.DrawExamSitting(context.Background(), userID, "B1")
	require.NoError(t, err)
	require.Len(t, sections, 4)
	listeningActs := sections[0].Activities
	require.Len(t, listeningActs, 3)
	// Assert none of the 5 without audio were drawn
	for _, act := range listeningActs {
		var cand struct {
			AudioObjectKey string `json:"audio_object_key"`
		}
		err := json.Unmarshal(act.Config, &cand)
		require.NoError(t, err)
		assert.NotEmpty(t, cand.AudioObjectKey, "listening item without audio must never be drawn")
	}
}

// TestPoolIsolation asserts requirement:
// "A practice daily set never contains an exam pool item, and a sitting never contains a practice pool item."
func TestPoolIsolation_PracticeNeverContainsExam_ExamNeverContainsPractice(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC))
	f := newPoolFixture(t, clk, uuid.New(), passingGraders(), nil)

	// Ensure both structures
	require.NoError(t, f.svc.EnsurePracticePoolStructure(context.Background()))
	require.NoError(t, f.svc.EnsureExamPoolStructure(context.Background()))

	// Seed practice pool
	practiceIDsMap := f.seed(t, "B1", map[string]int{
		poolKindReading:   3,
		poolKindTense:     10,
		poolKindTransform: 6,
	})
	var allPracticeIDs []uuid.UUID
	practiceSet := make(map[uuid.UUID]bool)
	for _, ids := range practiceIDsMap {
		for _, id := range ids {
			allPracticeIDs = append(allPracticeIDs, id)
			practiceSet[id] = true
		}
	}

	// Seed exam pool
	examFixture := examPoolFixture{
		svc:     f.svc,
		repo:    f.repo,
		lessons: f.lessons,
		content: f.content,
		authors: f.authors,
	}
	examFixture.seedFullExamPool(t, "B1", 2)

	// 1. Draw practice daily set -> must contain NO exam pool activity
	userID := uuid.New()
	dailySet, err := f.svc.GetDailySet(context.Background(), userID, "B1")
	require.NoError(t, err)
	for _, act := range dailySet.Activities {
		assert.True(t, practiceSet[act.ID], "practice daily set must only contain items from pool-practice")
	}

	// 2. Draw exam sitting -> must contain NO practice pool activity
	examSections, err := f.svc.DrawExamSitting(context.Background(), userID, "B1")
	require.NoError(t, err)
	for _, sec := range examSections {
		for _, act := range sec.Activities {
			assert.False(t, practiceSet[act.ID], "exam sitting must never contain an item from pool-practice")
		}
	}
}

// TestTopUpExamPool_AuthorResolutionAndKebabSlugs verifies author resolution and slug formatting.
func TestTopUpExamPool_AuthorResolutionAndKebabSlugs(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC))
	mockAI := ai.NewMockProvider(nil)

	adminID := uuid.New()
	repo := newFakePoolRepo()
	lessons := newFakePoolLessons()
	authors := &fakeContentAuthor{}

	svc := service.New(service.Deps{
		Repo:              repo,
		Lesson:            lessons,
		LessonAuthor:      lessons,
		Content:           newFakeContentReader(),
		ContentAuthor:     authors,
		Graders:           examPassingGraders(),
		AI:                mockAI,
		Clock:             clk,
		GeneratorAuthorID: uuid.Nil, // nil initially!
		AuthorResolver:    &fakeAuthorResolver{authorID: adminID},
		Synthesiser:       &fakeAudioSynthesiser{},
	})

	err := svc.TopUpExamPool(context.Background())
	require.NoError(t, err)

	published := authors.published()
	require.NotEmpty(t, published)

	for _, spec := range published {
		assert.Equal(t, adminID, spec.AuthorID, "author must be resolved dynamically")
		assert.True(t, kebabSlug.MatchString(spec.Slug), "slug %q must be strictly kebab-case", spec.Slug)
	}
}
