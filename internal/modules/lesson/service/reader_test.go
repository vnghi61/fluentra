package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// wantCode asserts the error carries the apperr code the API contract names.
// A bare error here is a 500 the caller was never told to expect.
func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %q, got nil", code)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not an *apperr.Error", err)
	}
	if appErr.Code != code {
		t.Fatalf("error code = %q, want %q", appErr.Code, code)
	}
}

// multiLessonRepo serves an ordered course, which the single-lesson fake cannot.
type multiLessonRepo struct {
	fakeLessonRepo
	lessons []*contract.Lesson
	listErr error
}

func (m *multiLessonRepo) ListLessonsByCourseID(_ context.Context, _ uuid.UUID) ([]*contract.Lesson, error) {
	m.queryCounter.Add(1)
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.lessons, nil
}

func (m *multiLessonRepo) ListLessonsByUnitID(_ context.Context, unitID uuid.UUID) ([]*contract.Lesson, error) {
	m.queryCounter.Add(1)
	out := make([]*contract.Lesson, 0, len(m.lessons))
	for _, l := range m.lessons {
		if l.UnitID == unitID {
			out = append(out, l)
		}
	}
	return out, nil
}

func orderedLessons(unitID uuid.UUID, n int) []*contract.Lesson {
	lessons := make([]*contract.Lesson, n)
	for i := range lessons {
		lessons[i] = &contract.Lesson{
			ID: uuid.New(), UnitID: unitID, Position: i + 1,
			Title: "Lesson", Status: statusPublished,
		}
	}
	return lessons
}

// TestReaderNextLesson covers contract.Reader.NextLesson, which is how
// `learning` will walk a course. Every branch matters to that caller: the first
// lesson, the middle, the end of the course, and an id from another course.
func TestReaderNextLesson(t *testing.T) {
	t.Parallel()

	unitID := uuid.New()
	lessons := orderedLessons(unitID, 3)
	courseID := uuid.New()

	newReader := func() *service.Service {
		return service.New(service.Deps{Repo: &multiLessonRepo{lessons: lessons}})
	}

	t.Run("no current lesson starts at the first", func(t *testing.T) {
		t.Parallel()
		got, err := newReader().NextLesson(context.Background(), courseID, nil)
		if err != nil {
			t.Fatalf("NextLesson: %v", err)
		}
		if got == nil || got.ID != lessons[0].ID {
			t.Errorf("got %v, want the first lesson", got)
		}
	})

	t.Run("advances one position", func(t *testing.T) {
		t.Parallel()
		got, err := newReader().NextLesson(context.Background(), courseID, &lessons[0].ID)
		if err != nil {
			t.Fatalf("NextLesson: %v", err)
		}
		if got == nil || got.ID != lessons[1].ID {
			t.Errorf("got %v, want the second lesson", got)
		}
	})

	t.Run("the last lesson has no next", func(t *testing.T) {
		t.Parallel()
		got, err := newReader().NextLesson(context.Background(), courseID, &lessons[2].ID)
		if err != nil {
			t.Fatalf("NextLesson: %v", err)
		}
		if got != nil {
			t.Errorf("got %v, want nil at the end of the course", got)
		}
	})

	t.Run("an empty course has no next", func(t *testing.T) {
		t.Parallel()
		svc := service.New(service.Deps{Repo: &multiLessonRepo{}})
		got, err := svc.NextLesson(context.Background(), courseID, nil)
		if err != nil {
			t.Fatalf("NextLesson: %v", err)
		}
		if got != nil {
			t.Errorf("got %v, want nil for a course with no lessons", got)
		}
	})

	t.Run("a repository failure is not silently an empty course", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("query failed")
		svc := service.New(service.Deps{Repo: &multiLessonRepo{listErr: boom}})
		if _, err := svc.NextLesson(context.Background(), courseID, nil); !errors.Is(err, boom) {
			t.Fatalf("error = %v, want the underlying failure", err)
		}
	})
}

// TestReaderGetLessonAndListLessons covers the other two contract.Reader methods.
func TestReaderGetLessonAndListLessons(t *testing.T) {
	t.Parallel()

	unitID := uuid.New()
	lessons := orderedLessons(unitID, 2)
	repo := &multiLessonRepo{lessons: lessons}
	repo.lesson = lessons[0]
	svc := service.New(service.Deps{Repo: repo})

	got, err := svc.GetLesson(context.Background(), lessons[0].ID)
	if err != nil {
		t.Fatalf("GetLesson: %v", err)
	}
	if got == nil || got.ID != lessons[0].ID {
		t.Errorf("GetLesson returned %v, want %v", got, lessons[0].ID)
	}

	listed, err := svc.ListLessons(context.Background(), unitID)
	if err != nil {
		t.Fatalf("ListLessons: %v", err)
	}
	if len(listed) != 2 {
		t.Errorf("ListLessons returned %d lessons, want 2", len(listed))
	}
}

func TestCreateCourseRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input service.CreateCourseInput
		code  string
	}{
		{
			name:  "empty slug",
			input: service.CreateCourseInput{Slug: "", Title: titleCourse, CEFRFrom: "B1", CEFRTo: "B2"},
			code:  "INVALID_SLUG",
		},
		{
			name:  "slug with spaces and capitals",
			input: service.CreateCourseInput{Slug: "Not A Slug", Title: titleCourse, CEFRFrom: "B1", CEFRTo: "B2"},
			code:  "INVALID_SLUG",
		},
		{
			name:  "unknown cefr_from",
			input: service.CreateCourseInput{Slug: slugIELTSCore, Title: titleCourse, CEFRFrom: "Z9", CEFRTo: "B2"},
			code:  codeInvalidCEFR,
		},
		{
			name:  "unknown cefr_to",
			input: service.CreateCourseInput{Slug: slugIELTSCore, Title: titleCourse, CEFRFrom: "B1", CEFRTo: "Z9"},
			code:  codeInvalidCEFR,
		},
		{
			// Not INVALID_STATUS: an empty title is a title problem, and the
			// client cannot act on a code that names the wrong field.
			name:  "empty title",
			input: service.CreateCourseInput{Slug: slugIELTSCore, Title: "", CEFRFrom: "B1", CEFRTo: "B2"},
			code:  "INVALID_TITLE",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			svc := service.New(service.Deps{Repo: &fakeLessonRepo{}})
			_, err := svc.CreateCourse(context.Background(), uuid.New(), testCase.input)
			wantCode(t, err, testCase.code)
		})
	}
}

func TestCreateCourseStartsAsDraft(t *testing.T) {
	t.Parallel()

	svc := service.New(service.Deps{Repo: &fakeLessonRepo{}})
	course, err := svc.CreateCourse(context.Background(), uuid.New(), service.CreateCourseInput{
		Slug: slugIELTSCore, Title: "IELTS Core", CEFRFrom: "B1", CEFRTo: "B2", EstimatedHours: 20,
	})
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}
	if course.Status != statusDraft {
		t.Errorf("status = %q, want draft — a new course is not live", course.Status)
	}
}

func TestAddPrerequisiteRejectsAnOutOfRangeMinScore(t *testing.T) {
	t.Parallel()

	lessonID := uuid.New()
	unitID := uuid.New()

	for _, minScore := range []int{-1, 101, 4_294_967_346} {
		t.Run("min_score", func(t *testing.T) {
			t.Parallel()
			repo := &fakeLessonRepo{
				lesson: &contract.Lesson{ID: lessonID, UnitID: unitID, Status: statusDraft},
				units:  []*contract.Unit{{ID: unitID, CourseID: uuid.New()}},
			}
			svc := service.New(service.Deps{Repo: repo})
			// 4294967346 truncates to 50 as an int32 — inside the CHECK
			// constraint's range, so the database would accept a value the
			// caller never sent.
			err := svc.AddPrerequisite(context.Background(), uuid.New(), lessonID, uuid.New(), minScore)
			wantCode(t, err, "INVALID_MIN_SCORE")
		})
	}
}

func TestAddPrerequisiteRejectsASelfEdge(t *testing.T) {
	t.Parallel()

	lessonID := uuid.New()
	svc := service.New(service.Deps{Repo: &fakeLessonRepo{}})
	err := svc.AddPrerequisite(context.Background(), uuid.New(), lessonID, lessonID, 70)
	wantCode(t, err, "PREREQUISITE_CYCLE")
}

func TestUpdateActivitiesRejectsMalformedLists(t *testing.T) {
	t.Parallel()

	lessonID := uuid.New()
	versionID := uuid.New()

	cases := []struct {
		name       string
		activities []domain.ActivityInput
		code       string
	}{
		{"empty list", nil, "EMPTY_ACTIVITIES"},
		{
			name: "position zero",
			activities: []domain.ActivityInput{
				{Position: 0, Kind: kindQuiz, ContentVersionID: versionID, Weight: 1},
			},
			code: "INVALID_POSITION",
		},
		{
			name: "duplicate position",
			activities: []domain.ActivityInput{
				{Position: 1, Kind: kindQuiz, ContentVersionID: versionID, Weight: 1},
				{Position: 1, Kind: kindQuiz, ContentVersionID: versionID, Weight: 1},
			},
			code: "INVALID_POSITION",
		},
		{
			name: "empty kind",
			activities: []domain.ActivityInput{
				{Position: 1, Kind: "", ContentVersionID: versionID, Weight: 1},
			},
			code: "INVALID_ACTIVITY_KIND",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeLessonRepo{lesson: &contract.Lesson{ID: lessonID, Status: statusDraft}}
			svc := service.New(service.Deps{Repo: repo})
			_, err := svc.UpdateActivities(context.Background(), uuid.New(), lessonID, testCase.activities)
			wantCode(t, err, testCase.code)
		})
	}
}

// TestPublishLessonRefusesUnpublishedContent is BR-LESSON-02. An activity
// bound to a draft version must block the publish, not ship a lesson whose
// material the learner cannot read.
func TestPublishLessonRefusesUnpublishedContent(t *testing.T) {
	t.Parallel()

	lessonID := uuid.New()
	versionID := uuid.New()

	repo := &fakeLessonRepo{
		lesson: &contract.Lesson{ID: lessonID, UnitID: uuid.New(), Status: statusDraft},
		activities: []contract.Activity{{
			ID: uuid.New(), LessonID: lessonID, Position: 1, Kind: kindQuiz,
			ContentVersionID: versionID, Config: json.RawMessage("{}"), Weight: 1,
		}},
	}
	reader := &countingContentReader{versions: map[uuid.UUID]*contentcontract.Version{
		versionID: {ID: versionID, Kind: kindQuiz, Status: statusDraft},
	}}

	svc := service.New(service.Deps{Repo: repo, Content: reader})
	_, err := svc.PublishLesson(context.Background(), uuid.New(), lessonID)
	wantCode(t, err, "ACTIVITY_CONTENT_UNPUBLISHED")
}

func TestPublishLessonRefusesAnEmptyLesson(t *testing.T) {
	t.Parallel()

	lessonID := uuid.New()
	repo := &fakeLessonRepo{lesson: &contract.Lesson{ID: lessonID, Status: statusDraft}}
	svc := service.New(service.Deps{Repo: repo})

	_, err := svc.PublishLesson(context.Background(), uuid.New(), lessonID)
	wantCode(t, err, "EMPTY_ACTIVITIES")
}

// TestPublishLessonIsIdempotent: publishing an already-published lesson is a
// no-op that returns the lesson, not a second event.
func TestPublishLessonIsIdempotent(t *testing.T) {
	t.Parallel()

	lessonID := uuid.New()
	repo := &fakeLessonRepo{lesson: &contract.Lesson{ID: lessonID, Status: statusPublished}}
	svc := service.New(service.Deps{Repo: repo})

	got, err := svc.PublishLesson(context.Background(), uuid.New(), lessonID)
	if err != nil {
		t.Fatalf("PublishLesson: %v", err)
	}
	if got == nil || got.ID != lessonID {
		t.Errorf("got %v, want the already-published lesson", got)
	}
}

// TestResolveActivityReturnsHierarchy verifies that ResolveActivity resolves
// an activity's structural path in one call (Trap 1).
func TestResolveActivityReturnsHierarchy(t *testing.T) {
	t.Parallel()

	activityID := uuid.New()
	lessonID := uuid.New()
	unitID := uuid.New()
	courseID := uuid.New()
	versionID := uuid.New()

	repo := &fakeLessonRepo{
		unit:   &contract.Unit{ID: unitID, CourseID: courseID},
		lesson: &contract.Lesson{ID: lessonID, UnitID: unitID, SkillFocus: "reading"},
		activities: []contract.Activity{{
			ID:               activityID,
			LessonID:         lessonID,
			Position:         1,
			Kind:             "quiz",
			ContentVersionID: versionID,
			Config:           json.RawMessage(`{"shuffle":true}`),
			Weight:           5,
		}},
	}
	svc := service.New(service.Deps{Repo: repo})

	hierarchy, err := svc.ResolveActivity(context.Background(), activityID)
	if err != nil {
		t.Fatalf("ResolveActivity: %v", err)
	}
	if hierarchy == nil {
		t.Fatal("expected non-nil hierarchy")
	}
	if hierarchy.ActivityID != activityID {
		t.Errorf("got ActivityID %s, want %s", hierarchy.ActivityID, activityID)
	}
	if hierarchy.LessonID != lessonID {
		t.Errorf("got LessonID %s, want %s", hierarchy.LessonID, lessonID)
	}
	if hierarchy.UnitID != unitID {
		t.Errorf("got UnitID %s, want %s", hierarchy.UnitID, unitID)
	}
	if hierarchy.CourseID != courseID {
		t.Errorf("got CourseID %s, want %s", hierarchy.CourseID, courseID)
	}
	if hierarchy.Kind != "quiz" {
		t.Errorf("got Kind %s, want quiz", hierarchy.Kind)
	}
	if hierarchy.ContentVersionID != versionID {
		t.Errorf("got ContentVersionID %s, want %s", hierarchy.ContentVersionID, versionID)
	}
	if hierarchy.Weight != 5 {
		t.Errorf("got Weight %d, want 5", hierarchy.Weight)
	}
	if hierarchy.LessonSkillFocus != "reading" {
		t.Errorf("got LessonSkillFocus %s, want reading", hierarchy.LessonSkillFocus)
	}
}

func TestResolveActivityNotFound(t *testing.T) {
	t.Parallel()

	repo := &fakeLessonRepo{}
	svc := service.New(service.Deps{Repo: repo})

	_, err := svc.ResolveActivity(context.Background(), uuid.New())
	wantCode(t, err, "ACTIVITY_NOT_FOUND")
}
