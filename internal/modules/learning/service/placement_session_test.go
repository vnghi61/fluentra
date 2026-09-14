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

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type fakeUserProfileReader struct {
	profile usercontract.LearningProfileDTO
	found   bool
	err     error
}

func (f *fakeUserProfileReader) GetLearningProfile(_ context.Context, _ uuid.UUID) (usercontract.LearningProfileDTO, bool, error) {
	return f.profile, f.found, f.err
}

type fakePlacementRepo struct {
	*fakePoolRepo
	activeSession    *domain.PlacementSession
	latestCompleted  *domain.PlacementSession
	sessions         map[uuid.UUID]*domain.PlacementSession
	results          map[uuid.UUID]*domain.PlacementResult
	weeklyPlans      map[string]*domain.WeeklyPlan
	expiredSweptCount int64
}

func newFakePlacementRepo() *fakePlacementRepo {
	return &fakePlacementRepo{
		fakePoolRepo: newFakePoolRepo(),
		sessions:     make(map[uuid.UUID]*domain.PlacementSession),
		results:      make(map[uuid.UUID]*domain.PlacementResult),
		weeklyPlans:  make(map[string]*domain.WeeklyPlan),
	}
}

func (m *fakePlacementRepo) GetActivePlacementSessionByUser(_ context.Context, _ uuid.UUID) (*domain.PlacementSession, error) {
	return m.activeSession, nil
}

func (m *fakePlacementRepo) GetPlacementSessionByID(_ context.Context, id uuid.UUID) (*domain.PlacementSession, error) {
	if s, ok := m.sessions[id]; ok {
		return s, nil
	}
	return nil, domain.ErrPlacementSessionNotFound
}

func (m *fakePlacementRepo) GetLatestCompletedPlacementSession(_ context.Context, _ uuid.UUID) (*domain.PlacementSession, error) {
	return m.latestCompleted, nil
}

func (m *fakePlacementRepo) CreatePlacementSession(_ context.Context, session *domain.PlacementSession) (*domain.PlacementSession, error) {
	m.sessions[session.ID] = session
	m.activeSession = session
	return session, nil
}

func (m *fakePlacementRepo) UpdatePlacementSessionProgress(_ context.Context, session *domain.PlacementSession) (*domain.PlacementSession, error) {
	m.sessions[session.ID] = session
	if m.activeSession != nil && m.activeSession.ID == session.ID {
		m.activeSession = session
	}
	return session, nil
}

func (m *fakePlacementRepo) CompletePlacementSession(_ context.Context, session *domain.PlacementSession) (*domain.PlacementSession, error) {
	m.sessions[session.ID] = session
	if m.activeSession != nil && m.activeSession.ID == session.ID {
		m.activeSession = nil
	}
	m.latestCompleted = session
	return session, nil
}

func (m *fakePlacementRepo) ExpireStalePlacementSessions(_ context.Context) (int64, error) {
	m.expiredSweptCount = 2
	return 2, nil
}

func (m *fakePlacementRepo) CreatePlacementResult(_ context.Context, result *domain.PlacementResult) (*domain.PlacementResult, error) {
	m.results[result.ID] = result
	return result, nil
}

func (m *fakePlacementRepo) GetWeeklyPlanByUserAndDate(_ context.Context, userID uuid.UUID, weekStartDate time.Time) (*domain.WeeklyPlan, error) {
	k := userID.String() + ":" + weekStartDate.Format("2006-01-02")
	if p, ok := m.weeklyPlans[k]; ok {
		return p, nil
	}
	return nil, nil
}

func (m *fakePlacementRepo) UpsertWeeklyPlan(_ context.Context, plan *domain.WeeklyPlan) (*domain.WeeklyPlan, error) {
	k := plan.UserID.String() + ":" + plan.WeekStartDate.Format("2006-01-02")
	m.weeklyPlans[k] = plan
	return plan, nil
}

func seedPlacementItems(t *testing.T, lessons *fakePoolLessons, content *fakeContentReader) {
	t.Helper()
	levels := []string{"A1", "A2", "B1", "B2", "C1"}
	slots := []struct {
		title string
		kind  string
		count int
	}{
		{title: "Vocabulary Multiple Choice", kind: "vocabulary", count: 15},
		{title: "Grammar Tense Choice", kind: "grammar_tense_choice", count: 15},
		{title: "Reading Comprehension", kind: "reading_comprehension", count: 3},
		{title: "Listening Comprehension", kind: "listening_comprehension", count: 3},
		{title: "Writing Prompt", kind: "writing_prompt", count: 3},
		{title: "Speaking Task", kind: "speaking_task", count: 3},
	}

	for _, lvl := range levels {
		for _, s := range slots {
			for i := 0; i < s.count; i++ {
				vID := uuid.New()
				body := json.RawMessage(fmt.Sprintf(`{"prompt":"test %s %s %d"}`, lvl, s.title, i))
				lessons.seedCourse(t, service.PlacementPoolCourseSlug, lvl, s.title, s.kind, vID, body)
			}
		}
	}
}

func TestPlacementSession_Invitation(t *testing.T) {
	ctx := context.Background()
	adminID := uuid.New()
	clk := clock.Real{}
	graders := placementPassingGraders()

	f := placementPoolFixture{
		repo:    newFakePoolRepo(),
		lessons: newFakePoolLessons(),
		content: newFakeContentReader(),
		authors: &fakeContentAuthor{},
	}
	pRepo := newFakePlacementRepo()

	userReader := &fakeUserProfileReader{
		found: true,
		profile: usercontract.LearningProfileDTO{
			TargetExam: "general",
		},
	}

	svc := service.New(service.Deps{
		Repo:              pRepo,
		Lesson:            f.lessons,
		LessonAuthor:      f.lessons,
		Content:           f.content,
		ContentAuthor:     f.authors,
		Graders:           graders,
		Clock:             clk,
		GeneratorAuthorID: adminID,
		User:              userReader,
	})

	require.NoError(t, svc.EnsurePlacementPoolStructure(ctx))

	// 1. Pool insufficient initially
	userID := uuid.New()
	inv, err := svc.GetPlacementInvitation(ctx, userID)
	require.NoError(t, err)
	assert.False(t, inv.Eligible)
	assert.False(t, inv.PoolSufficient)

	// Seed full placement pool
	seedPlacementItems(t, f.lessons, f.content)

	// 2. Now eligible!
	inv, err = svc.GetPlacementInvitation(ctx, userID)
	require.NoError(t, err)
	assert.True(t, inv.Eligible)
	assert.True(t, inv.PoolSufficient)

	// 3. Active session exists
	sessID := uuid.New()
	pRepo.activeSession = &domain.PlacementSession{
		ID:        sessID,
		UserID:    userID,
		Status:    domain.PlacementSessionStatusInProgress,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	inv, err = svc.GetPlacementInvitation(ctx, userID)
	require.NoError(t, err)
	assert.False(t, inv.Eligible)
	assert.True(t, inv.HasActiveTest)
	require.NotNil(t, inv.ActiveSessionID)
	assert.Equal(t, sessID, *inv.ActiveSessionID)

	// 4. Cooldown (completed test 10 days ago)
	pRepo.activeSession = nil
	completedAt := time.Now().Add(-10 * 24 * time.Hour)
	pRepo.latestCompleted = &domain.PlacementSession{
		ID:          uuid.New(),
		UserID:      userID,
		Status:      domain.PlacementSessionStatusCompleted,
		CompletedAt: &completedAt,
	}
	inv, err = svc.GetPlacementInvitation(ctx, userID)
	require.NoError(t, err)
	assert.False(t, inv.Eligible)
	assert.NotNil(t, inv.CooldownUntil)
}

func TestPlacementSession_FullFlow(t *testing.T) {
	ctx := context.Background()
	adminID := uuid.New()
	clk := clock.Real{}
	graders := placementPassingGraders()

	f := placementPoolFixture{
		repo:    newFakePoolRepo(),
		lessons: newFakePoolLessons(),
		content: newFakeContentReader(),
		authors: &fakeContentAuthor{},
	}
	pRepo := newFakePlacementRepo()

	goalMinutes := 180
	userReader := &fakeUserProfileReader{
		found: true,
		profile: usercontract.LearningProfileDTO{
			TargetExam:        "ielts",
			WeeklyMinutesGoal: &goalMinutes,
		},
	}

	svc := service.New(service.Deps{
		Repo:              pRepo,
		Lesson:            f.lessons,
		LessonAuthor:      f.lessons,
		Content:           f.content,
		ContentAuthor:     f.authors,
		Graders:           graders,
		Clock:             clk,
		GeneratorAuthorID: adminID,
		User:              userReader,
	})

	require.NoError(t, svc.EnsurePlacementPoolStructure(ctx))
	seedPlacementItems(t, f.lessons, f.content)

	userID := uuid.New()

	// Start placement session
	sess, firstAct, err := svc.StartPlacementSession(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, sess)
	require.NotNil(t, firstAct)
	assert.Equal(t, domain.PlacementSessionStatusInProgress, sess.Status)
	assert.Equal(t, domain.StageFastConvergence, sess.Stage)
	assert.NotNil(t, sess.CurrentActivityID)

	// Submitting answers iteratively until test completes
	iterations := 0
	for sess.Status == domain.PlacementSessionStatusInProgress && iterations < 40 {
		iterations++
		var completed bool
		sess, _, completed, err = svc.SubmitPlacementAnswer(ctx, userID, sess.ID, json.RawMessage(`{"answer":"A"}`))
		require.NoError(t, err)
		if completed {
			break
		}
	}

	assert.Equal(t, domain.PlacementSessionStatusCompleted, sess.Status)
	require.NotNil(t, sess.PlacedLevel)
	t.Logf("Placement test finished after %d items. Placed level: %s, confidence: %.2f",
		len(sess.Responses), *sess.PlacedLevel, sess.Confidence)

	// Check weekly plan was created
	plan, err := svc.GetWeeklyPlan(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.NotEmpty(t, plan.TimeDistribution)
	assert.NotEmpty(t, plan.DailyTargets)

	// Check sweep
	swept, err := svc.SweepExpiredPlacementSessions(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), swept)
}

func TestPlacementSession_IsUnlocked_CEFRBypass(t *testing.T) {
	ctx := context.Background()
	pRepo := newFakePlacementRepo()

	b1Level := "B1"
	pRepo.latestCompleted = &domain.PlacementSession{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		Status:      domain.PlacementSessionStatusCompleted,
		PlacedLevel: &b1Level,
	}

	a1ID := uuid.New()
	a2ID := uuid.New()
	b1ID := uuid.New()
	b2ID := uuid.New()
	noCEFRID := uuid.New()

	a1Level := "A1"
	a2Level := "A2"
	b2Level := "B2"

	fakeReader := &fakeLessonReader{
		lessons: map[uuid.UUID]*lessoncontract.Lesson{
			a1ID:     {ID: a1ID, CEFRLevel: &a1Level},
			a2ID:     {ID: a2ID, CEFRLevel: &a2Level},
			b1ID:     {ID: b1ID, CEFRLevel: &b1Level},
			b2ID:     {ID: b2ID, CEFRLevel: &b2Level},
			noCEFRID: {ID: noCEFRID},
		},
		prereqs: map[uuid.UUID][]lessoncontract.PrerequisiteItem{
			a1ID:     {{LessonID: a1ID, RequiresLessonID: uuid.New()}},
			a2ID:     {{LessonID: a2ID, RequiresLessonID: uuid.New()}},
			b1ID:     {{LessonID: b1ID, RequiresLessonID: uuid.New()}},
			b2ID:     {{LessonID: b2ID, RequiresLessonID: uuid.New()}},
			noCEFRID: {{LessonID: noCEFRID, RequiresLessonID: uuid.New()}},
		},
	}

	svc := service.New(service.Deps{
		Repo:   pRepo,
		Lesson: fakeReader,
		Clock:  clock.Real{},
	})

	userID := pRepo.latestCompleted.UserID
	ids := []uuid.UUID{a1ID, a2ID, b1ID, b2ID, noCEFRID}

	unlocked, err := svc.IsUnlocked(ctx, userID, ids)
	require.NoError(t, err)

	// A1, A2, B1 should be unlocked because CEFRLevel <= B1 (bypassing unsatisfied prereqs without progress rows or XP)
	assert.True(t, unlocked[a1ID], "A1 should be unlocked")
	assert.True(t, unlocked[a2ID], "A2 should be unlocked")
	assert.True(t, unlocked[b1ID], "B1 should be unlocked")

	// B2 and noCEFR should be locked because prerequisites not satisfied and level > B1
	assert.False(t, unlocked[b2ID], "B2 should be locked")
	assert.False(t, unlocked[noCEFRID], "no-CEFR should be locked")
}
