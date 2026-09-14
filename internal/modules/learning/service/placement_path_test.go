package service_test

import (
	"context"
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

type fakeCatalog struct{ courses []*lessoncontract.Course }

func (c *fakeCatalog) ListCurriculumCourses(_ context.Context, level *string) ([]*lessoncontract.Course, error) {
	if level == nil {
		return c.courses, nil
	}
	var out []*lessoncontract.Course
	for _, course := range c.courses {
		at := domain.CEFROrdinal(*level)
		if domain.CEFROrdinal(course.CEFRFrom) <= at && at <= domain.CEFROrdinal(course.CEFRTo) {
			out = append(out, course)
		}
	}
	return out, nil
}

// pathFixture is an A2–B1 course of four lessons, each requiring the one before:
// two A2 lessons, then two B1 lessons.
type pathFixture struct {
	svc      *service.Service
	repo     *fakeLearningRepo
	reader   *fakeLessonReader
	profiles *fakeProfiles
	clock    *clock.Fake
	user     uuid.UUID
	courseID uuid.UUID
	lessons  []*lessoncontract.Lesson
}

func newPathFixture(t *testing.T) *pathFixture {
	t.Helper()
	f := &pathFixture{
		repo: newFakeRepo(),
		reader: &fakeLessonReader{
			calls:         map[string]int{},
			hierarchy:     map[uuid.UUID]*lessoncontract.ActivityHierarchy{},
			lessons:       map[uuid.UUID]*lessoncontract.Lesson{},
			unitLesson:    map[uuid.UUID][]*lessoncontract.Lesson{},
			courseUnits:   map[uuid.UUID][]*lessoncontract.Unit{},
			courseLessons: map[uuid.UUID][]*lessoncontract.Lesson{},
			courseActs:    map[uuid.UUID][]uuid.UUID{},
			prereqs:       map[uuid.UUID][]lessoncontract.PrerequisiteItem{},
		},
		profiles: &fakeProfiles{},
		// Monday 14 September 2026, 10:00 in Asia/Ho_Chi_Minh.
		clock:    clock.NewFake(time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)),
		user:     uuid.New(),
		courseID: uuid.New(),
	}

	levels := []string{domain.LevelA2, domain.LevelA2, domain.LevelB1, domain.LevelB1}
	units := []*lessoncontract.Unit{
		{ID: uuid.New(), CourseID: f.courseID, Position: 1},
		{ID: uuid.New(), CourseID: f.courseID, Position: 2},
	}
	f.reader.courseUnits[f.courseID] = units
	for i, level := range levels {
		lvl := level
		unit := units[i/2]
		lesson := &lessoncontract.Lesson{
			ID: uuid.New(), UnitID: unit.ID, Position: i%2 + 1, Title: "Lesson " + level,
			SkillFocus: domain.SkillGrammar, EstimatedMinutes: 10, CEFRLevel: &lvl,
			Activities: []lessoncontract.Activity{{ID: uuid.New(), Kind: testKindQuiz}},
		}
		f.lessons = append(f.lessons, lesson)
		f.reader.lessons[lesson.ID] = lesson
		f.reader.unitLesson[unit.ID] = append(f.reader.unitLesson[unit.ID], lesson)
		if i > 0 {
			previous := f.lessons[i-1]
			f.reader.prereqs[lesson.ID] = []lessoncontract.PrerequisiteItem{{
				LessonID: lesson.ID, RequiresLessonID: previous.ID, RequiresLessonLevel: previous.CEFRLevel,
			}}
		}
	}

	catalog := &fakeCatalog{courses: []*lessoncontract.Course{{
		ID: f.courseID, Slug: "everyday-english-a2-b1", Title: "Everyday English",
		CEFRFrom: domain.LevelA2, CEFRTo: domain.LevelB1,
	}}}
	f.svc = service.New(service.Deps{
		Repo:    f.repo,
		Lesson:  f.reader,
		Graders: domain.NewGraderRegistry(),
		Clock:   f.clock,
		User:    f.profiles,
		Courses: catalog,
	})
	return f
}

func (f *pathFixture) place(t *testing.T, level string) {
	t.Helper()
	_, err := f.repo.CreatePlacementResult(context.Background(), &domain.PlacementResult{
		UserID: f.user, Level: level, PerSkill: map[string]domain.SkillEstimate{}, TakenAt: f.clock.Now(),
	})
	require.NoError(t, err)
}

func (f *pathFixture) lessonIDs() []uuid.UUID {
	ids := make([]uuid.UUID, len(f.lessons))
	for i, lesson := range f.lessons {
		ids[i] = lesson.ID
	}
	return ids
}

func TestPath_APlacedB2LearnerStartsAtTheFirstB1LessonAndOpensEveryLessonBelow(t *testing.T) {
	f := newPathFixture(t)
	f.place(t, domain.LevelB2)
	ctx := context.Background()

	path, err := f.svc.GetStartingPath(ctx, f.user)
	require.NoError(t, err)
	assert.Equal(t, domain.LevelB2, path.Level)
	assert.Equal(t, service.LevelSourcePlacement, path.LevelSource)
	require.Len(t, path.Courses, 1, "the nearest course below B2")
	require.NotNil(t, path.Courses[0].StartLesson)
	assert.Equal(t, f.lessons[2].ID, path.Courses[0].StartLesson.ID, "the first B1 lesson")

	unlocked, err := f.svc.IsUnlocked(ctx, f.user, f.lessonIDs())
	require.NoError(t, err)
	for _, lesson := range f.lessons {
		assert.True(t, unlocked[lesson.ID], "lesson %s opens", *lesson.CEFRLevel)
	}
	assert.Empty(t, f.repo.progress, "opening writes nothing, so the course still reads 0%")
}

func TestPath_ADeclaredLevelPicksACourseButOpensNothingEarly(t *testing.T) {
	f := newPathFixture(t)
	declared := domain.LevelB1
	f.profiles.profile = usercontract.LearningProfileDTO{DeclaredLevel: &declared}
	f.profiles.found = true
	ctx := context.Background()

	path, err := f.svc.GetStartingPath(ctx, f.user)
	require.NoError(t, err)
	assert.Equal(t, service.LevelSourceDeclared, path.LevelSource)
	require.Len(t, path.Courses, 1)
	assert.Equal(t, f.lessons[2].ID, path.Courses[0].StartLesson.ID)

	unlocked, err := f.svc.IsUnlocked(ctx, f.user, f.lessonIDs())
	require.NoError(t, err)
	assert.True(t, unlocked[f.lessons[0].ID], "the first lesson has no prerequisite")
	for _, lesson := range f.lessons[1:] {
		assert.False(t, unlocked[lesson.ID], "a claim is not a measurement")
	}
}

func TestPath_WithNothingKnownTheLevelIsA2(t *testing.T) {
	f := newPathFixture(t)
	path, err := f.svc.GetStartingPath(context.Background(), f.user)
	require.NoError(t, err)
	assert.Equal(t, domain.LevelA2, path.Level)
	assert.Equal(t, service.LevelSourceDefault, path.LevelSource)
	assert.Equal(t, f.lessons[0].ID, path.Courses[0].StartLesson.ID)
}

func TestDashboard_APlacedLearnerWhoHasNotBegunStartsAtTheStartLesson(t *testing.T) {
	f := newPathFixture(t)
	f.place(t, domain.LevelB1)
	ctx := context.Background()
	_, err := f.repo.CreateEnrollment(ctx, f.user, f.courseID, "active", f.clock.Now())
	require.NoError(t, err)

	next, err := f.svc.NextActivity(ctx, f.user)
	require.NoError(t, err)
	require.NotNil(t, next.NextActivity)
	assert.Equal(t, f.lessons[2].ID, next.NextActivity.LessonID, "not the course's first lesson")
}

func TestWeeklyPlan_IsFixedForTheWeekAndNewOnMonday(t *testing.T) {
	f := newPathFixture(t)
	ctx := context.Background()

	monday, err := f.svc.GetWeeklyPlan(ctx, f.user)
	require.NoError(t, err)
	assert.Equal(t, "2026-09-14", monday.WeekStart)
	assert.Equal(t, domain.DefaultWeeklyMinutes, monday.MinutesGoal, "90 minutes with no goal")
	assert.NotEmpty(t, monday.Items, "a learner with no mastery and no placement still gets a plan")

	// Sunday 23:59 in Asia/Ho_Chi_Minh is still the same week.
	f.clock.Set(time.Date(2026, 9, 20, 16, 59, 0, 0, time.UTC))
	sunday, err := f.svc.GetWeeklyPlan(ctx, f.user)
	require.NoError(t, err)
	assert.Equal(t, monday.WeekStart, sunday.WeekStart)
	assert.Equal(t, monday.Items, sunday.Items)

	// Monday 00:00 there is a new week.
	f.clock.Set(time.Date(2026, 9, 20, 17, 0, 0, 0, time.UTC))
	next, err := f.svc.GetWeeklyPlan(ctx, f.user)
	require.NoError(t, err)
	assert.Equal(t, "2026-09-21", next.WeekStart)
}

func TestWeeklyPlan_UsesTheGoalAndReadsProgressAtRequestTime(t *testing.T) {
	f := newPathFixture(t)
	goal := 150
	f.profiles.profile = usercontract.LearningProfileDTO{WeeklyMinutesGoal: &goal}
	f.profiles.found = true
	f.repo.store().minutes = 35
	f.repo.store().practiced = 1

	plan, err := f.svc.GetWeeklyPlan(context.Background(), f.user)
	require.NoError(t, err)
	assert.Equal(t, 150, plan.MinutesGoal)
	assert.Equal(t, 35, plan.Progress.Minutes)
	assert.Equal(t, len(plan.Items), plan.Progress.ItemsTotal)
	assert.Positive(t, plan.Progress.ItemsDone, "a practiced set marks a practice item done")
}
