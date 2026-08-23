package service_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

const (
	kindQuiz = "quiz"
)

type staticUnlocker struct {
	unlocked bool
}

func (s staticUnlocker) IsUnlocked(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return s.unlocked, nil
}

func lockFixtureIDs() (lessonID, reqLessonID, courseID, unitID, userID uuid.UUID) {
	return uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
}

func lockFixturePrereqs(lessonID, reqLessonID uuid.UUID) []service.PrerequisiteItem {
	return []service.PrerequisiteItem{{
		LessonID:            lessonID,
		RequiresLessonID:    reqLessonID,
		MinScore:            70,
		RequiresLessonTitle: "Unit 1 Lesson 1",
	}}
}

func lockFixture(lessonID, unitID, courseID uuid.UUID) (*contract.Lesson, *contract.Unit, *contract.Course) {
	lesson := &contract.Lesson{
		ID:               lessonID,
		UnitID:           unitID,
		Position:         2,
		Title:            "Unit 1 Lesson 2",
		SkillFocus:       "grammar",
		EstimatedMinutes: 20,
		Status:           statusPublished,
	}

	unit := &contract.Unit{
		ID:       unitID,
		CourseID: courseID,
		Position: 1,
		Title:    "Unit 1",
	}

	course := &contract.Course{
		ID:             courseID,
		Slug:           slugIELTSCore,
		Title:          "IELTS Core",
		CEFRFrom:       "B1",
		CEFRTo:         "B2",
		Status:         statusPublished,
		EstimatedHours: 20,
	}

	return lesson, unit, course
}

func TestService_LockReasonAndEvaluation(t *testing.T) {
	lessonID, reqLessonID, courseID, unitID, userID := lockFixtureIDs()
	lesson, unit, course := lockFixture(lessonID, unitID, courseID)
	prereqs := lockFixturePrereqs(lessonID, reqLessonID)

	t.Run("locked lesson generates lock_reason and returns 403", func(t *testing.T) {
		repo := &fakeLessonRepo{
			lesson:  lesson,
			units:   []*contract.Unit{unit},
			courses: []*contract.Course{course},
			prereqs: prereqs,
		}

		svc := service.New(service.Deps{
			Repo:     repo,
			Unlocker: staticUnlocker{unlocked: false},
		})

		ctx := context.Background()

		// In course detail, lesson summary carries locked = true and the formatted lock_reason (Trap 3)
		detail, err := svc.GetCourseDetail(ctx, slugIELTSCore, userID)
		if err != nil {
			t.Fatalf("GetCourseDetail: %v", err)
		}
		if len(detail.Units) != 1 || len(detail.Units[0].Lessons) != 1 {
			t.Fatalf("unexpected course structure: %+v", detail)
		}
		lSummary := detail.Units[0].Lessons[0]
		if !lSummary.Locked {
			t.Errorf("expected lesson to be locked")
		}
		if lSummary.LockReason == nil || *lSummary.LockReason != reasonLesson1 {
			t.Errorf("expected lock_reason 'Complete Unit 1 Lesson 1 first', got %v", lSummary.LockReason)
		}

		// Directly getting lesson detail is a 403 LESSON_LOCKED whose body carries
		// the same sentence. WithMeta returns a clone, so errors.Is against the
		// sentinel would not match — assert the code, the status and the reason,
		// which is what the acceptance criterion actually names.
		_, err = svc.GetLessonDetail(ctx, lessonID, userID)
		var appErr *apperr.Error
		if !errors.As(err, &appErr) {
			t.Fatalf("error %v is not an *apperr.Error", err)
		}
		if appErr.Code != "LESSON_LOCKED" {
			t.Errorf("error code = %q, want LESSON_LOCKED", appErr.Code)
		}
		if appErr.Status() != http.StatusForbidden {
			t.Errorf("status = %d, want 403", appErr.Status())
		}
		if got := appErr.Meta["lock_reason"]; got != reasonLesson1 {
			t.Errorf("meta lock_reason = %v, want the specific prerequisite sentence", got)
		}
	})

}

func TestService_UnlockedLessonAllowsAccess(t *testing.T) {
	lessonID, reqLessonID, courseID, unitID, userID := lockFixtureIDs()
	lesson, unit, course := lockFixture(lessonID, unitID, courseID)
	prereqs := lockFixturePrereqs(lessonID, reqLessonID)

	t.Run("unlocked lesson allows access", func(t *testing.T) {
		repo := &fakeLessonRepo{
			lesson:  lesson,
			units:   []*contract.Unit{unit},
			courses: []*contract.Course{course},
			prereqs: prereqs,
		}

		svc := service.New(service.Deps{
			Repo:     repo,
			Unlocker: staticUnlocker{unlocked: true},
		})

		ctx := context.Background()
		detail, err := svc.GetCourseDetail(ctx, slugIELTSCore, userID)
		if err != nil {
			t.Fatalf("GetCourseDetail: %v", err)
		}
		lSummary := detail.Units[0].Lessons[0]
		if lSummary.Locked {
			t.Errorf("expected lesson to be unlocked")
		}
		if lSummary.LockReason != nil {
			t.Errorf("expected nil lock_reason, got %v", *lSummary.LockReason)
		}

		res, err := svc.GetLessonDetail(ctx, lessonID, userID)
		if err != nil {
			t.Fatalf("unexpected error getting unlocked lesson: %v", err)
		}
		if res.ID != lessonID {
			t.Errorf("expected lesson ID %v, got %v", lessonID, res.ID)
		}
	})
}

func TestService_PublishLesson_BR_LESSON_02(t *testing.T) {
	lessonID := uuid.New()
	unitID := uuid.New()
	vPublishedID := uuid.New()
	vDraftID := uuid.New()
	vArchivedID := uuid.New()

	ctx := context.Background()

	t.Run("refuses publish when activity content is draft", func(t *testing.T) {
		acts := []contract.Activity{
			{ID: uuid.New(), LessonID: lessonID, Position: 1, Kind: kindQuiz, ContentVersionID: vDraftID, Weight: 1},
		}
		repo := &fakeLessonRepo{
			lesson:     &contract.Lesson{ID: lessonID, UnitID: unitID, Status: statusDraft},
			activities: acts,
		}
		contentReader := &countingContentReader{
			versions: map[uuid.UUID]*contentcontract.Version{
				vDraftID: {ID: vDraftID, Status: statusDraft},
			},
		}

		svc := service.New(service.Deps{
			Repo:    repo,
			Content: contentReader,
		})

		_, err := svc.PublishLesson(ctx, uuid.New(), lessonID)
		if !errors.Is(err, domain.ErrActivityContentUnpublished) {
			t.Fatalf("expected ErrActivityContentUnpublished, got %v", err)
		}
	})

	t.Run("refuses publish when activity content is archived", func(t *testing.T) {
		acts := []contract.Activity{
			{ID: uuid.New(), LessonID: lessonID, Position: 1, Kind: kindQuiz, ContentVersionID: vArchivedID, Weight: 1},
		}
		repo := &fakeLessonRepo{
			lesson:     &contract.Lesson{ID: lessonID, UnitID: unitID, Status: statusDraft},
			activities: acts,
		}
		contentReader := &countingContentReader{
			versions: map[uuid.UUID]*contentcontract.Version{
				vArchivedID: {ID: vArchivedID, Status: "archived"},
			},
		}

		svc := service.New(service.Deps{
			Repo:    repo,
			Content: contentReader,
		})

		_, err := svc.PublishLesson(ctx, uuid.New(), lessonID)
		if !errors.Is(err, domain.ErrActivityContentUnpublished) {
			t.Fatalf("expected ErrActivityContentUnpublished, got %v", err)
		}
	})

	t.Run("succeeds when all activity content is published", func(t *testing.T) {
		acts := []contract.Activity{
			{ID: uuid.New(), LessonID: lessonID, Position: 1, Kind: kindQuiz, ContentVersionID: vPublishedID, Weight: 1},
		}
		repo := &fakeLessonRepo{
			lesson:     &contract.Lesson{ID: lessonID, UnitID: unitID, Status: statusDraft},
			activities: acts,
		}
		contentReader := &countingContentReader{
			versions: map[uuid.UUID]*contentcontract.Version{
				vPublishedID: {ID: vPublishedID, Status: "published"},
			},
		}

		svc := service.New(service.Deps{
			Repo:    repo,
			Content: contentReader,
		})

		res, err := svc.PublishLesson(ctx, uuid.New(), lessonID)
		if err != nil {
			t.Fatalf("PublishLesson: %v", err)
		}
		if res.Status != "published" {
			t.Errorf("expected published status, got %s", res.Status)
		}
	})
}

func TestService_PrerequisitesCycleDetection_BR_LESSON_03(t *testing.T) {
	nodeA := uuid.New()
	nodeB := uuid.New()
	nodeC := uuid.New()
	courseID := uuid.New()
	unitID := uuid.New()

	ctx := context.Background()

	repo := &fakeLessonRepo{
		lesson: &contract.Lesson{ID: nodeC, UnitID: unitID},
		units:  []*contract.Unit{{ID: unitID, CourseID: courseID}},
		edges: []domain.PrerequisiteEdge{
			{LessonID: nodeA, RequiresLessonID: nodeB},
			{LessonID: nodeB, RequiresLessonID: nodeC},
		},
	}

	svc := service.New(service.Deps{
		Repo: repo,
	})

	// Attempting to add edge nodeC -> nodeA would create cycle nodeA -> nodeB -> nodeC -> nodeA
	err := svc.AddPrerequisite(ctx, uuid.New(), nodeC, nodeA, 50)
	if !errors.Is(err, domain.ErrPrerequisiteCycle) {
		t.Fatalf("expected ErrPrerequisiteCycle, got %v", err)
	}

	// Self-prerequisite attempt
	err = svc.AddPrerequisite(ctx, uuid.New(), nodeC, nodeC, 50)
	if !errors.Is(err, domain.ErrPrerequisiteCycle) {
		t.Fatalf("expected ErrPrerequisiteCycle, got %v", err)
	}
}

func TestService_UpdateActivitiesAndRecalculateDuration_BR_LESSON_06(t *testing.T) {
	lessonID := uuid.New()
	ctx := context.Background()

	repo := &fakeLessonRepo{
		lesson: &contract.Lesson{ID: lessonID, EstimatedMinutes: 0},
	}

	svc := service.New(service.Deps{
		Repo: repo,
	})

	inputs := []domain.ActivityInput{
		{Position: 1, Kind: "quiz", ContentVersionID: uuid.New(), Weight: 2},
		{Position: 2, Kind: "gap_fill", ContentVersionID: uuid.New(), Weight: 3},
	}

	acts, err := svc.UpdateActivities(ctx, uuid.New(), lessonID, inputs)
	if err != nil {
		t.Fatalf("UpdateActivities: %v", err)
	}
	if len(acts) != 2 {
		t.Fatalf("got %d activities, want 2", len(acts))
	}
	// Total duration = (2*2) + (3*2) = 10 minutes
	if repo.lesson.EstimatedMinutes != 10 {
		t.Errorf("expected estimated duration 10, got %d", repo.lesson.EstimatedMinutes)
	}
}

func TestService_NextLesson(t *testing.T) {
	courseID := uuid.New()
	l1 := &contract.Lesson{ID: uuid.New(), Position: 1, Title: "L1"}
	l2 := &contract.Lesson{ID: uuid.New(), Position: 2, Title: "L2"}
	l3 := &contract.Lesson{ID: uuid.New(), Position: 3, Title: "L3"}

	repo := &fakeLessonRepo{
		lesson: l1,
	}
	// override ListLessonsByCourseID
	_ = l2
	_ = l3

	svc := service.New(service.Deps{Repo: repo})
	ctx := context.Background()

	// When currentLessonID is nil, returns first lesson
	next, err := svc.NextLesson(ctx, courseID, nil)
	if err != nil {
		t.Fatalf("NextLesson: %v", err)
	}
	if next.ID != l1.ID {
		t.Errorf("expected first lesson %v, got %v", l1.ID, next.ID)
	}
}
