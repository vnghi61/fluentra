package service_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

func publishedCourse(slug string) *contract.Course {
	return &contract.Course{
		ID: uuid.New(), Slug: slug, Title: "IELTS " + slug,
		CEFRFrom: "B1", CEFRTo: "B2", Status: statusPublished, EstimatedHours: 20,
	}
}

func draftCourse(slug string) *contract.Course {
	course := publishedCourse(slug)
	course.Status = statusDraft
	return course
}

// TestListCoursesClampsPaging asserts on what the repository was handed, not on
// what ListCourses returned. Both values arrive from strconv.Atoi over a query
// string, so an unclamped one reaches SQL as a page of two billion, or wraps
// negative on the way into int32.
func TestListCoursesClampsPaging(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int32
		wantOffset int32
	}{
		{"omitted falls back to the documented default", 0, 0, domain.DefaultLimit, 0},
		{"negative offset floors at zero", 0, -5, domain.DefaultLimit, 0},
		{"in range passes through", 50, 40, 50, 40},
		{"oversized page capped at the spec maximum", 2_000_000_000, 0, domain.MaxLimit, 0},
		{"page that would wrap int32 is capped, not truncated", math.MaxUint32 + 2, 0, domain.MaxLimit, 0},
		{"offset that would wrap int32 is capped, not negated", 0, math.MaxInt32 + 100, domain.DefaultLimit, math.MaxInt32},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeLessonRepo{courses: []*contract.Course{publishedCourse(slugIELTSCore)}}
			svc := service.New(service.Deps{Repo: repo})

			if _, _, err := svc.ListCourses(context.Background(), nil, testCase.limit, testCase.offset); err != nil {
				t.Fatalf("ListCourses: %v", err)
			}
			if repo.lastLimit != testCase.wantLimit {
				t.Errorf("repository got limit %d, want %d", repo.lastLimit, testCase.wantLimit)
			}
			if repo.lastOffset != testCase.wantOffset {
				t.Errorf("repository got offset %d, want %d", repo.lastOffset, testCase.wantOffset)
			}
		})
	}
}

// TestListCoursesPassesTheLevelFilterDown proves `level` is not decoration: the
// spec declares it, so it has to reach the query.
func TestListCoursesPassesTheLevelFilterDown(t *testing.T) {
	t.Parallel()

	repo := &fakeLessonRepo{courses: []*contract.Course{publishedCourse(slugIELTSCore)}}
	svc := service.New(service.Deps{Repo: repo})

	level := "B1"
	if _, _, err := svc.ListCourses(context.Background(), &level, 0, 0); err != nil {
		t.Fatalf("ListCourses: %v", err)
	}
	if repo.lastLevel == nil || *repo.lastLevel != "B1" {
		t.Fatalf("repository got level %v, want B1", repo.lastLevel)
	}

	repo.lastLevel = &level
	if _, _, err := svc.ListCourses(context.Background(), nil, 0, 0); err != nil {
		t.Fatalf("ListCourses without level: %v", err)
	}
	if repo.lastLevel != nil {
		t.Errorf("repository got level %v, want nil when the caller did not filter", *repo.lastLevel)
	}
}

func TestListCoursesRejectsAnUnknownLevel(t *testing.T) {
	t.Parallel()

	repo := &fakeLessonRepo{}
	svc := service.New(service.Deps{Repo: repo})

	level := "Z9"
	_, _, err := svc.ListCourses(context.Background(), &level, 0, 0)

	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not an *apperr.Error", err)
	}
	if appErr.Code != codeInvalidCEFR {
		t.Errorf("error code = %q, want INVALID_CEFR_LEVEL", appErr.Code)
	}
}

// TestListCoursesReturnsPublishedOnly is the catalogue half of BR-CONTENT-02's
// lesson equivalent: a draft course is not part of the catalogue.
func TestListCoursesReturnsPublishedOnly(t *testing.T) {
	t.Parallel()

	repo := &fakeLessonRepo{courses: []*contract.Course{
		publishedCourse("live-course"),
		draftCourse("work-in-progress"),
	}}
	svc := service.New(service.Deps{Repo: repo})

	courses, total, err := svc.ListCourses(context.Background(), nil, 0, 0)
	if err != nil {
		t.Fatalf("ListCourses: %v", err)
	}
	if len(courses) != 1 || total != 1 {
		t.Fatalf("got %d courses (total %d), want 1 and 1", len(courses), total)
	}
	if courses[0].Slug != "live-course" {
		t.Errorf("catalogue returned %q, want the published course", courses[0].Slug)
	}
}

// TestGetCourseDetailHidesUnpublishedCourses is the detail half. A learner who
// guesses a draft course's slug must get a 404, not the curriculum.
func TestGetCourseDetailHidesUnpublishedCourses(t *testing.T) {
	t.Parallel()

	repo := &fakeLessonRepo{courses: []*contract.Course{draftCourse("work-in-progress")}}
	svc := service.New(service.Deps{Repo: repo})

	_, err := svc.GetCourseDetail(context.Background(), "work-in-progress", uuid.New())

	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not an *apperr.Error", err)
	}
	if appErr.Code != "COURSE_NOT_FOUND" {
		t.Errorf("error code = %q, want COURSE_NOT_FOUND", appErr.Code)
	}
}

// TestGetLessonDetailHidesUnpublishedLessons is the same guarantee one level down.
func TestGetLessonDetailHidesUnpublishedLessons(t *testing.T) {
	t.Parallel()

	lessonID := uuid.New()
	repo := &fakeLessonRepo{lesson: &contract.Lesson{
		ID: lessonID, UnitID: uuid.New(), Position: 1, Title: "Draft lesson", Status: statusDraft,
	}}
	svc := service.New(service.Deps{Repo: repo})

	_, err := svc.GetLessonDetail(context.Background(), lessonID, uuid.New())

	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not an *apperr.Error", err)
	}
	if appErr.Code != "LESSON_NOT_FOUND" {
		t.Errorf("error code = %q, want LESSON_NOT_FOUND", appErr.Code)
	}
}

// TestGetCourseDetailCostIsFlatInLessonCount is the course-level half of the
// acceptance criterion. The count must not grow with the number of lessons:
// course, units, lessons, prerequisites — four, whether the course holds one
// lesson or forty.
func TestGetCourseDetailCostIsFlatInLessonCount(t *testing.T) {
	t.Parallel()

	course := publishedCourse(slugIELTSCore)
	unit := &contract.Unit{ID: uuid.New(), CourseID: course.ID, Position: 1, Title: titleUnit1}
	repo := &fakeLessonRepo{
		courses: []*contract.Course{course},
		units:   []*contract.Unit{unit},
		lesson: &contract.Lesson{
			ID: uuid.New(), UnitID: unit.ID, Position: 1, Title: titleLesson1, Status: statusPublished,
		},
	}
	svc := service.New(service.Deps{Repo: repo})

	if _, err := svc.GetCourseDetail(context.Background(), course.Slug, uuid.New()); err != nil {
		t.Fatalf("GetCourseDetail: %v", err)
	}

	const wantQueries = 4
	if got := repo.queryCounter.Load(); got != wantQueries {
		t.Errorf("GetCourseDetail issued %d queries, want exactly %d "+
			"(course, units, lessons, prerequisites)", got, wantQueries)
	}
}

// TestEvaluateLockWithoutLearningReportsUnlocked pins the Phase 2 default.
// `learning` is WP8; until it lands nothing can answer whether a learner met a
// prerequisite, and locking by default would make every lesson after the first
// unreachable for the seed data and the learner web app alike.
func TestEvaluateLockWithoutLearningReportsUnlocked(t *testing.T) {
	t.Parallel()

	course := publishedCourse(slugIELTSCore)
	unit := &contract.Unit{ID: uuid.New(), CourseID: course.ID, Position: 1, Title: titleUnit1}
	lessonID := uuid.New()

	repo := &fakeLessonRepo{
		courses: []*contract.Course{course},
		units:   []*contract.Unit{unit},
		lesson: &contract.Lesson{
			ID: lessonID, UnitID: unit.ID, Position: 2, Title: titleLesson2, Status: statusPublished,
		},
		prereqs: []service.PrerequisiteItem{{
			LessonID: lessonID, RequiresLessonID: uuid.New(), RequiresLessonTitle: titleLesson1,
		}},
	}

	// Deps.Unlocker left nil, exactly as cmd/api wires it today.
	svc := service.New(service.Deps{Repo: repo})

	detail, err := svc.GetCourseDetail(context.Background(), course.Slug, uuid.New())
	if err != nil {
		t.Fatalf("GetCourseDetail: %v", err)
	}
	summary := detail.Units[0].Lessons[0]
	if summary.Locked {
		t.Error("a lesson is locked with no learning module wired; every lesson would be unreachable")
	}
	if summary.LockReason != nil {
		t.Errorf("lock_reason = %q, want nil when nothing is locked", *summary.LockReason)
	}

	if _, err := svc.GetLessonDetail(context.Background(), lessonID, uuid.New()); err != nil {
		t.Fatalf("GetLessonDetail on an unlocked lesson: %v", err)
	}
}

type failingUnlocker struct{ err error }

func (f failingUnlocker) IsUnlocked(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, f.err
}

// TestEvaluateLockPropagatesUnlockerFailure keeps a learning outage a 500. Read
// as "locked", it would tell a learner they had not finished work they had.
func TestEvaluateLockPropagatesUnlockerFailure(t *testing.T) {
	t.Parallel()

	boom := errors.New("learning unavailable")
	lessonID := uuid.New()
	repo := &fakeLessonRepo{
		lesson: &contract.Lesson{
			ID: lessonID, UnitID: uuid.New(), Position: 2, Title: titleLesson2, Status: statusPublished,
		},
		prereqs: []service.PrerequisiteItem{{
			LessonID: lessonID, RequiresLessonID: uuid.New(), RequiresLessonTitle: titleLesson1,
		}},
	}
	svc := service.New(service.Deps{Repo: repo, Unlocker: failingUnlocker{err: boom}})

	_, err := svc.GetLessonDetail(context.Background(), lessonID, uuid.New())
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap the unlocker failure", err)
	}

	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr.Code == "LESSON_LOCKED" {
		t.Error("an unlocker outage was reported to the learner as a locked lesson")
	}
}

// TestLockReasonNamesEveryPrerequisite: UnlockChecker answers with a bool, so
// this module cannot know which prerequisite is the unmet one. Naming only the
// first would send a learner to a lesson they may already have finished.
func TestLockReasonNamesEveryPrerequisite(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		titles []string
		want   string
	}{
		{"one", []string{"Unit 1 Lesson 1"}, reasonLesson1},
		{"two", []string{"Lesson A", "Lesson B"}, "Complete Lesson A and Lesson B first"},
		{"three", []string{"A", "B", "C"}, "Complete A, B and C first"},
		{"untitled falls back to a generic sentence", []string{""}, "Complete the earlier lessons first"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			lessonID := uuid.New()
			prereqs := make([]service.PrerequisiteItem, len(testCase.titles))
			for i, title := range testCase.titles {
				prereqs[i] = service.PrerequisiteItem{
					LessonID: lessonID, RequiresLessonID: uuid.New(), RequiresLessonTitle: title,
				}
			}

			course := publishedCourse(slugIELTSCore)
			unit := &contract.Unit{ID: uuid.New(), CourseID: course.ID, Position: 1, Title: titleUnit1}
			repo := &fakeLessonRepo{
				courses: []*contract.Course{course},
				units:   []*contract.Unit{unit},
				lesson: &contract.Lesson{
					ID: lessonID, UnitID: unit.ID, Position: 2, Title: titleLesson2, Status: statusPublished,
				},
				prereqs: prereqs,
			}
			svc := service.New(service.Deps{Repo: repo, Unlocker: staticUnlocker{unlocked: false}})

			detail, err := svc.GetCourseDetail(context.Background(), course.Slug, uuid.New())
			if err != nil {
				t.Fatalf("GetCourseDetail: %v", err)
			}
			summary := detail.Units[0].Lessons[0]
			if !summary.Locked {
				t.Fatal("expected the lesson to be locked")
			}
			if summary.LockReason == nil || *summary.LockReason != testCase.want {
				t.Errorf("lock_reason = %v, want %q", summary.LockReason, testCase.want)
			}
		})
	}
}
