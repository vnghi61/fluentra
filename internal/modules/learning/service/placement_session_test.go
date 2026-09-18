package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// --------------------------------------------------------------------------
// Fixture
// --------------------------------------------------------------------------

const (
	kindChoice     = "grammar_tense_choice"
	kindReading    = "reading_comprehension"
	kindListening  = "listening_comprehension"
	kindWriting    = "writing_prompt"
	kindSpeaking   = "speaking_task"
	eventPlacement = "placement.completed"
)

type fakeFlags struct{ on bool }

func (f *fakeFlags) IsEnabled(_ context.Context, key string, _ uuid.UUID) (bool, error) {
	return f.on && key == service.PlacementInviteFlag, nil
}

type fakeProfiles struct {
	profile usercontract.LearningProfileDTO
	found   bool
}

func (f *fakeProfiles) GetLearningProfile(context.Context, uuid.UUID) (usercontract.LearningProfileDTO, bool, error) {
	return f.profile, f.found, nil
}

// placementGrader grades the way a scripted learner answers: correct when the
// response says so, with one result per question for a passage or a clip.
// Writing and speaking are graded later, as they are in production.
type placementGrader struct{ async bool }

func (g placementGrader) Grade(
	_ context.Context, req learningcontract.GradeRequest,
) (learningcontract.GradeResult, error) {
	if g.async {
		return learningcontract.GradeResult{Async: true}, nil
	}
	var response struct {
		Correct   bool `json:"correct"`
		Questions int  `json:"questions"`
	}
	_ = json.Unmarshal(req.Response, &response)
	if response.Questions == 0 {
		score := 0
		if response.Correct {
			score = 1
		}
		return learningcontract.GradeResult{Score: score, MaxScore: 1, Correct: response.Correct}, nil
	}
	results := make([]learningcontract.ItemResult, response.Questions)
	score := 0
	for i := range results {
		results[i] = learningcontract.ItemResult{ID: fmt.Sprintf("q%d", i+1), Correct: response.Correct}
		if response.Correct {
			score++
		}
	}
	return learningcontract.GradeResult{
		Score: score, MaxScore: response.Questions, Correct: response.Correct, ItemResults: results,
	}, nil
}

type placementFixture struct {
	svc      *service.Service
	repo     *fakePoolRepo
	lessons  *fakePoolLessons
	clock    *clock.Fake
	flags    *fakeFlags
	profiles *fakeProfiles
	events   *fakeEventWriter
	user     uuid.UUID
}

func newPlacementFixture(t *testing.T) *placementFixture {
	t.Helper()
	f := &placementFixture{
		repo:     newFakePoolRepo(),
		lessons:  newFakePoolLessons(),
		clock:    clock.NewFake(time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)),
		flags:    &fakeFlags{},
		profiles: &fakeProfiles{},
		events:   &fakeEventWriter{},
		user:     uuid.New(),
	}
	graders := domain.NewGraderRegistry()
	for _, kind := range []string{kindChoice, kindReading, kindListening} {
		require.NoError(t, graders.Register(kind, placementGrader{}))
	}
	for _, kind := range []string{kindWriting, kindSpeaking} {
		require.NoError(t, graders.Register(kind, placementGrader{async: true}))
	}
	f.svc = service.New(service.Deps{
		Repo:          f.repo,
		Lesson:        f.lessons,
		LessonAuthor:  f.lessons,
		Content:       newFakeContentReader(),
		ContentAuthor: &fakeContentAuthor{},
		Graders:       graders,
		Events:        f.events,
		Clock:         f.clock,
		User:          f.profiles,
		Flags:         f.flags,
	})
	require.NoError(t, f.svc.EnsurePlacementPoolStructure(context.Background()))
	return f
}

type placementSeedSlot struct {
	title string
	kind  string
	body  func(level string, i int) string
}

func choiceBody(level string, i int) string {
	return fmt.Sprintf(`{"prompt": "%s question %d", "options": [
		{"id": "A", "text": "one"}, {"id": "B", "text": "two"}, {"id": "C", "text": "three"}, {"id": "D", "text": "four"}
	], "correct_option_id": "A"}`, level, i)
}

func passageBody(extra string) func(level string, i int) string {
	return func(level string, i int) string {
		question := `{"id": "q%d", "prompt": "Question?", "options": [
			{"id": "A", "text": "one"}, {"id": "B", "text": "two"}, {"id": "C", "text": "three"}, {"id": "D", "text": "four"}
		], "correct_option_id": "A"}`
		return fmt.Sprintf(`{"passage": "%s passage %d", "script": "%s script %d", %s "questions": [%s, %s, %s]}`,
			level, i, level, i, extra, fmt.Sprintf(question, 1), fmt.Sprintf(question, 2), fmt.Sprintf(question, 3))
	}
}

var placementSeedSlots = []placementSeedSlot{
	{title: "Vocabulary Multiple Choice", kind: kindChoice, body: choiceBody},
	{title: "Grammar Tense Choice", kind: kindChoice, body: choiceBody},
	{title: "Reading Comprehension", kind: kindReading, body: passageBody("")},
	{title: "Listening Comprehension", kind: kindListening, body: passageBody(`"audio_object_key": "audio/clip.mp3",`)},
	{title: "Writing Prompt", kind: kindWriting, body: func(level string, i int) string {
		return fmt.Sprintf(`{"prompt": "%s task %d", "model_answer": "A model answer.", "min_words": 60}`, level, i)
	}},
	{title: "Speaking Task", kind: kindSpeaking, body: func(level string, i int) string {
		return fmt.Sprintf(`{"task_type": "respond", "prompt": "%s talk %d", "speaking_time_seconds": 45}`, level, i)
	}},
}

// seedPool fills every slot at every band, except what skip leaves out.
func (f *placementFixture) seedPool(t *testing.T, perSlot int, skip func(level, title string) bool) {
	t.Helper()
	for _, level := range domain.PlacementBands {
		for _, slot := range placementSeedSlots {
			if skip != nil && skip(level, slot.title) {
				continue
			}
			for i := 0; i < perSlot; i++ {
				f.lessons.seedCourse(t, service.PlacementPoolCourseSlug, level, slot.title, slot.kind, uuid.New(),
					json.RawMessage(slot.body(level, i)))
			}
		}
	}
}

func (f *placementFixture) start(t *testing.T) *service.PlacementSessionDTO {
	t.Helper()
	session, err := f.svc.StartPlacement(context.Background(), f.user)
	require.NoError(t, err)
	require.NotNil(t, session.CurrentItem)
	return session
}

func responseFor(item *service.PlacementItemDTO, correct bool) json.RawMessage {
	questions := 0
	if item.Skill == domain.SkillReading || item.Skill == domain.SkillListening {
		questions = 3
	}
	return json.RawMessage(fmt.Sprintf(`{"correct": %t, "questions": %d}`, correct, questions))
}

func (f *placementFixture) answer(
	session *service.PlacementSessionDTO, correct bool,
) (*service.PlacementSessionDTO, error) {
	item := session.CurrentItem
	return f.svc.SubmitPlacementAnswer(context.Background(), f.user, session.ID, item.ActivityID, uuid.New(),
		responseFor(item, correct))
}

// finish answers until the adaptive part is over.
func (f *placementFixture) finish(
	t *testing.T, session *service.PlacementSessionDTO, correct bool,
) *service.PlacementSessionDTO {
	t.Helper()
	for turn := 0; session.CurrentItem != nil; turn++ {
		require.Less(t, turn, 40, "the test never finished")
		var err error
		session, err = f.answer(session, correct)
		require.NoError(t, err)
	}
	return session
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr), "want %s, got %v", code, err)
	require.Equal(t, code, appErr.Code)
}

// --------------------------------------------------------------------------
// Starting
// --------------------------------------------------------------------------

func TestPlacement_StartServesARedactedItemAndRecordsItsExposure(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 8, nil)

	session := f.start(t)

	assert.Equal(t, domain.PlacementInProgress, session.Status)
	assert.Equal(t, f.clock.Now().Add(20*time.Minute), session.DeadlineAt, "the server's clock, 20 minutes")
	assert.Equal(t, domain.SkillVocabulary, session.CurrentItem.Skill)
	assert.NotContains(t, string(session.CurrentItem.Config), "correct_option_id",
		"nothing answer-bearing reaches the learner before they answer (ADR-0025)")
	assert.Equal(t, 1, f.repo.exposureCount(), "the served item is marked seen")
}

func TestPlacement_OneSessionAtATime(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 8, nil)
	first := f.start(t)

	_, err := f.svc.StartPlacement(context.Background(), f.user)
	requireCode(t, err, "PLACEMENT_IN_PROGRESS")
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, first.ID.String(), appErr.Meta["session_id"], "the session in progress is named")
}

func TestPlacement_AShortBandRefusesToStart(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 8, func(level, title string) bool {
		return level == domain.LevelC1 && title == "Listening Comprehension"
	})

	_, err := f.svc.StartPlacement(context.Background(), f.user)
	requireCode(t, err, "PLACEMENT_UNAVAILABLE")
	assert.Zero(t, f.repo.exposureCount(), "a refused test marks nothing as seen")
}

func TestPlacement_InvitationFollowsTheFlagAndTheLearnerState(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 8, nil)
	ctx := context.Background()

	overview, err := f.svc.GetPlacementOverview(ctx, f.user)
	require.NoError(t, err)
	assert.False(t, overview.InviteAvailable, "the invitation is off until the flag is on")

	f.flags.on = true
	overview, err = f.svc.GetPlacementOverview(ctx, f.user)
	require.NoError(t, err)
	assert.True(t, overview.InviteAvailable)

	session := f.start(t)
	overview, err = f.svc.GetPlacementOverview(ctx, f.user)
	require.NoError(t, err)
	assert.False(t, overview.InviteAvailable, "not while a session is in progress")
	require.NotNil(t, overview.ActiveSession)
	assert.Equal(t, session.ID, overview.ActiveSession.ID)
}

// --------------------------------------------------------------------------
// The clock
// --------------------------------------------------------------------------

func TestPlacement_AnAnswerOneSecondAfterTheDeadlineIsRefusedAndOneBeforeCounts(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 8, nil)
	ctx := context.Background()

	before := f.start(t)
	f.clock.Set(before.DeadlineAt.Add(domain.PlacementAnswerGrace - time.Second))
	counted, err := f.answer(before, true)
	require.NoError(t, err)
	assert.Equal(t, 1, counted.Responses, "an answer one second before the deadline counts")

	f.user = uuid.New()
	after := f.start(t)
	f.clock.Set(after.DeadlineAt.Add(domain.PlacementAnswerGrace + time.Second))
	_, err = f.answer(after, true)
	requireCode(t, err, "PLACEMENT_SESSION_EXPIRED")

	stored, err := f.svc.GetPlacementSession(ctx, f.user, after.ID)
	require.NoError(t, err)
	assert.Zero(t, stored.Responses, "the late answer was not recorded")
}

func TestPlacement_AnAbandonedSessionIsExpiredBySweepAndByRead(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 8, nil)
	ctx := context.Background()

	bySweep := f.start(t)
	f.clock.Advance(domain.PlacementTimeLimit + time.Minute)
	finished, err := f.svc.SweepExpiredPlacementSessions(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, finished)
	stored, err := f.repo.GetPlacementSession(ctx, bySweep.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PlacementExpired, stored.Status, "the sweep finished it")

	// With the sweep removed, the next read finishes it.
	f.user = uuid.New()
	byRead := f.start(t)
	f.clock.Advance(domain.PlacementTimeLimit + time.Minute)
	read, err := f.svc.GetPlacementSession(ctx, f.user, byRead.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PlacementExpired, read.Status)
	assert.Nil(t, read.CurrentItem)
}

func TestPlacement_SevenResponsesAtExpiryRecordNoResult(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 8, nil)
	ctx := context.Background()

	session := f.start(t)
	for i := 0; i < 7; i++ {
		var err error
		session, err = f.answer(session, i%2 == 0)
		require.NoError(t, err)
	}
	require.Equal(t, 7, session.Responses)

	f.clock.Advance(domain.PlacementTimeLimit + time.Minute)
	_, err := f.svc.SweepExpiredPlacementSessions(ctx)
	require.NoError(t, err)

	overview, err := f.svc.GetPlacementOverview(ctx, f.user)
	require.NoError(t, err)
	assert.Nil(t, overview.Result, "fewer than eight responses place nobody")
	assert.Nil(t, overview.RetakeAvailableAt)
	_, err = f.svc.StartPlacement(ctx, f.user)
	assert.NoError(t, err, "the learner may start again at once")
}

func TestPlacement_TheSameKeyGradesOnce(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 8, nil)
	ctx := context.Background()

	session := f.start(t)
	item := session.CurrentItem
	key := uuid.New()
	first, err := f.svc.SubmitPlacementAnswer(ctx, f.user, session.ID, item.ActivityID, key, responseFor(item, true))
	require.NoError(t, err)
	again, err := f.svc.SubmitPlacementAnswer(ctx, f.user, session.ID, item.ActivityID, key, responseFor(item, true))
	require.NoError(t, err)

	assert.Equal(t, 1, first.Responses)
	assert.Equal(t, 1, again.Responses, "the replay changed nothing")
	assert.Len(t, f.repo.attempts, 1, "one attempt row")
}

func TestPlacement_AnotherLearnersSessionIsNotFound(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 8, nil)
	session := f.start(t)

	_, err := f.svc.GetPlacementSession(context.Background(), uuid.New(), session.ID)
	requireCode(t, err, "PLACEMENT_SESSION_NOT_FOUND")
}

// --------------------------------------------------------------------------
// Finishing
// --------------------------------------------------------------------------

func TestPlacement_FinishingRecordsTheResultMasteryAndOneEvent(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 10, nil)
	ctx := context.Background()
	_, err := f.repo.UpsertSkillMastery(ctx, f.user, domain.SkillVocabulary, domain.LevelA2, 0.7)
	require.NoError(t, err)

	session := f.finish(t, f.start(t), true)

	assert.Equal(t, domain.PlacementCompleted, session.Status)
	require.NotNil(t, session.Result)
	assert.Equal(t, domain.LevelC1, session.Result.Level)
	measured := []string{domain.SkillVocabulary, domain.SkillGrammar, domain.SkillReading, domain.SkillListening}
	for _, skill := range measured {
		assert.Contains(t, session.Result.PerSkill, skill)
	}
	assert.NotContains(t, session.Result.PerSkill, domain.SkillWriting, "writing is not measured yet")
	assert.LessOrEqual(t, session.Responses, domain.PlacementMaxResponses)

	grammar, err := f.repo.GetSkillMastery(ctx, f.user, domain.SkillGrammar)
	require.NoError(t, err)
	require.NotNil(t, grammar)
	assert.InDelta(t, 0.40, grammar.Confidence, 1e-9)
	vocabulary, err := f.repo.GetSkillMastery(ctx, f.user, domain.SkillVocabulary)
	require.NoError(t, err)
	assert.InDelta(t, 0.7, vocabulary.Confidence, 1e-9, "a higher confidence is never lowered")
	assert.Equal(t, domain.LevelA2, vocabulary.Level)

	assert.Equal(t, []string{eventPlacement}, f.events.recorded(), "placement.completed, once, and nothing else")
	assert.Empty(t, f.repo.progress, "placement completes nothing: no progress row")
	assert.Equal(t, domain.ProductiveOffered, session.ProductiveStatus,
		"writing and speaking are offered after the result")
}

func TestPlacement_ARetakeOnDay29IsRefusedAndOnDay30Starts(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 14, nil)
	ctx := context.Background()
	f.finish(t, f.start(t), true)

	f.clock.Advance(29 * 24 * time.Hour)
	_, err := f.svc.StartPlacement(ctx, f.user)
	requireCode(t, err, "PLACEMENT_RETAKE_TOO_SOON")

	f.clock.Advance(24 * time.Hour)
	retake, err := f.svc.StartPlacement(ctx, f.user)
	require.NoError(t, err)
	assert.Equal(t, domain.PlacementInProgress, retake.Status)
}

// --------------------------------------------------------------------------
// Writing and speaking
// --------------------------------------------------------------------------

func TestPlacement_AGradedWritingTaskAddsWritingAndPublishesNothing(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 10, nil)
	ctx := context.Background()
	finished := f.finish(t, f.start(t), false)
	require.NotNil(t, finished.Result)

	productive, err := f.svc.StartPlacementProductive(ctx, f.user, finished.ID, false)
	require.NoError(t, err)
	assert.Equal(t, domain.ProductiveInProgress, productive.ProductiveStatus)
	require.Len(t, productive.ProductiveItems, 2)
	var writing service.PlacementProductiveItemDTO
	for _, item := range productive.ProductiveItems {
		if item.Skill == domain.SkillWriting {
			writing = item
		}
	}
	require.NotEqual(t, uuid.Nil, writing.ActivityID)

	answered, err := f.svc.SubmitPlacementAnswer(ctx, f.user, finished.ID, writing.ActivityID, uuid.New(),
		json.RawMessage(`{"text_answer": "An essay."}`))
	require.NoError(t, err)
	var attemptID uuid.UUID
	for id := range f.repo.attempts {
		if f.repo.attempts[id].ActivityID == writing.ActivityID {
			attemptID = id
		}
	}
	require.NotEqual(t, uuid.Nil, attemptID)
	assert.Equal(t, domain.ProductiveInProgress, answered.ProductiveStatus, "speaking is still to answer")

	updated, err := f.svc.CompleteAsyncGrading(ctx, attemptID, learningcontract.GradeResult{Score: 72, MaxScore: 100})
	require.NoError(t, err)
	require.True(t, updated)

	after, err := f.svc.GetPlacementSession(ctx, f.user, finished.ID)
	require.NoError(t, err)
	require.NotNil(t, after.Result)
	assert.Equal(t, domain.LevelB2, after.Result.PerSkill[domain.SkillWriting].Band)
	assert.Equal(t, []string{eventPlacement}, f.events.recorded(), "placement.completed is not published again")
	mastery, err := f.repo.GetSkillMastery(ctx, f.user, domain.SkillWriting)
	require.NoError(t, err)
	require.NotNil(t, mastery)
	assert.Equal(t, domain.LevelB2, mastery.Level)
}

func TestPlacement_WritingAndSpeakingCanBeSkippedAndTakenLater(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 10, nil)
	ctx := context.Background()
	finished := f.finish(t, f.start(t), true)

	skipped, err := f.svc.StartPlacementProductive(ctx, f.user, finished.ID, true)
	require.NoError(t, err)
	assert.Equal(t, domain.ProductiveSkipped, skipped.ProductiveStatus)

	later, err := f.svc.StartPlacementProductive(ctx, f.user, finished.ID, false)
	require.NoError(t, err)
	assert.Equal(t, domain.ProductiveInProgress, later.ProductiveStatus)
	for _, item := range later.ProductiveItems {
		assert.False(t, strings.Contains(string(item.Config), "model_answer"), "the model answer is not served")
	}
}

func TestPlacement_WritingAndSpeakingAreNotOfferedBeforeTheResult(t *testing.T) {
	f := newPlacementFixture(t)
	f.seedPool(t, 8, nil)
	session := f.start(t)

	_, err := f.svc.StartPlacementProductive(context.Background(), f.user, session.ID, false)
	requireCode(t, err, "PLACEMENT_PRODUCTIVE_UNAVAILABLE")
}
