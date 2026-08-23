package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

func publishTestFixture() (*contract.Course, *contract.Unit, uuid.UUID) {
	courseID := uuid.New()
	unitID := uuid.New()
	lessonID := uuid.New()

	course := &contract.Course{
		ID:             courseID,
		Slug:           "publish-course",
		Title:          "Publish Course",
		CEFRFrom:       "B1",
		CEFRTo:         "B2",
		Status:         statusPublished,
		EstimatedHours: 10,
	}
	unit := &contract.Unit{
		ID:       unitID,
		CourseID: courseID,
		Position: 1,
		Title:    titleUnit1,
	}

	return course, unit, lessonID
}

func TestService_PublishLesson_EmptyActivities(t *testing.T) {
	t.Parallel()

	course, unit, lessonID := publishTestFixture()
	lesson := &contract.Lesson{
		ID:               lessonID,
		UnitID:           unit.ID,
		Position:         1,
		Title:            titleLesson1,
		Status:           statusDraft,
		EstimatedMinutes: 10,
	}
	repo := &fakeLessonRepo{
		courses:    []*contract.Course{course},
		units:      []*contract.Unit{unit},
		lesson:     lesson,
		activities: []contract.Activity{},
	}

	svc := service.New(service.Deps{Repo: repo})
	_, err := svc.PublishLesson(context.Background(), uuid.New(), lessonID)
	if err == nil {
		t.Fatal("expected error for empty activities")
	}

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "EMPTY_ACTIVITIES" {
		t.Errorf("got error %v, want EMPTY_ACTIVITIES (422)", err)
	}
}

func TestService_PublishLesson_DraftContent(t *testing.T) {
	t.Parallel()

	course, unit, lessonID := publishTestFixture()
	contentID := uuid.New()
	lesson := &contract.Lesson{
		ID:               lessonID,
		UnitID:           unit.ID,
		Position:         1,
		Title:            titleLesson1,
		Status:           statusDraft,
		EstimatedMinutes: 10,
	}
	acts := []contract.Activity{
		{
			ID:               uuid.New(),
			LessonID:         lessonID,
			Position:         1,
			Kind:             kindQuiz,
			ContentVersionID: contentID,
		},
	}
	repo := &fakeLessonRepo{
		courses:    []*contract.Course{course},
		units:      []*contract.Unit{unit},
		lesson:     lesson,
		activities: acts,
	}
	contentReader := &countingContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			contentID: {ID: contentID, Status: statusDraft, Kind: kindQuiz, CEFRLevel: "B1"},
		},
	}

	svc := service.New(service.Deps{
		Repo:    repo,
		Content: contentReader,
	})

	_, err := svc.PublishLesson(context.Background(), uuid.New(), lessonID)
	if err == nil {
		t.Fatal("expected error when activity content is draft")
	}

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "ACTIVITY_CONTENT_UNPUBLISHED" {
		t.Errorf("got error %v, want ACTIVITY_CONTENT_UNPUBLISHED (409)", err)
	}
}

func TestService_PublishLesson_ArchivedContent(t *testing.T) {
	t.Parallel()

	course, unit, lessonID := publishTestFixture()
	contentID := uuid.New()
	lesson := &contract.Lesson{
		ID:               lessonID,
		UnitID:           unit.ID,
		Position:         1,
		Title:            titleLesson1,
		Status:           statusDraft,
		EstimatedMinutes: 10,
	}
	acts := []contract.Activity{
		{
			ID:               uuid.New(),
			LessonID:         lessonID,
			Position:         1,
			Kind:             kindVocab,
			ContentVersionID: contentID,
		},
	}
	repo := &fakeLessonRepo{
		courses:    []*contract.Course{course},
		units:      []*contract.Unit{unit},
		lesson:     lesson,
		activities: acts,
	}
	contentReader := &countingContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			contentID: {ID: contentID, Status: statusArchived, Kind: kindVocab, CEFRLevel: "B1"},
		},
	}

	svc := service.New(service.Deps{
		Repo:    repo,
		Content: contentReader,
	})

	_, err := svc.PublishLesson(context.Background(), uuid.New(), lessonID)
	if err == nil {
		t.Fatal("expected error when activity content is archived")
	}

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "ACTIVITY_CONTENT_UNPUBLISHED" {
		t.Errorf("got error %v, want ACTIVITY_CONTENT_UNPUBLISHED (409)", err)
	}
}

func TestService_PublishLesson_Success(t *testing.T) {
	t.Parallel()

	course, unit, lessonID := publishTestFixture()
	contentID := uuid.New()
	lesson := &contract.Lesson{
		ID:               lessonID,
		UnitID:           unit.ID,
		Position:         1,
		Title:            titleLesson1,
		SkillFocus:       "listening",
		Status:           statusDraft,
		EstimatedMinutes: 15,
	}
	acts := []contract.Activity{
		{
			ID:               uuid.New(),
			LessonID:         lessonID,
			Position:         1,
			Kind:             kindVocab,
			ContentVersionID: contentID,
		},
	}
	repo := &fakeLessonRepo{
		courses:    []*contract.Course{course},
		units:      []*contract.Unit{unit},
		lesson:     lesson,
		activities: acts,
	}
	contentReader := &countingContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			contentID: {ID: contentID, Status: statusPublished, Kind: kindVocab, CEFRLevel: "B1"},
		},
	}
	events := &fakeEvents{}

	svc := service.New(service.Deps{
		Repo:    repo,
		Content: contentReader,
		Events:  events,
	})

	pub, err := svc.PublishLesson(context.Background(), uuid.New(), lessonID)
	if err != nil {
		t.Fatalf("PublishLesson failed: %v", err)
	}

	if pub.Status != statusPublished {
		t.Errorf("published status = %q, want published", pub.Status)
	}
	if len(events.events) != 1 || events.events[0] != "lesson.published" {
		t.Errorf("expected outbox event lesson.published, got: %v", events.events)
	}
}

func TestService_PublishLesson_Idempotent(t *testing.T) {
	t.Parallel()

	course, unit, lessonID := publishTestFixture()
	contentID := uuid.New()
	lesson := &contract.Lesson{
		ID:               lessonID,
		UnitID:           unit.ID,
		Position:         1,
		Title:            titleLesson1,
		Status:           statusPublished,
		EstimatedMinutes: 15,
	}
	acts := []contract.Activity{
		{
			ID:               uuid.New(),
			LessonID:         lessonID,
			Position:         1,
			Kind:             kindVocab,
			ContentVersionID: contentID,
		},
	}
	repo := &fakeLessonRepo{
		courses:    []*contract.Course{course},
		units:      []*contract.Unit{unit},
		lesson:     lesson,
		activities: acts,
	}

	svc := service.New(service.Deps{Repo: repo})
	pub, err := svc.PublishLesson(context.Background(), uuid.New(), lessonID)
	if err != nil {
		t.Fatalf("PublishLesson on already published: %v", err)
	}
	if pub.Status != statusPublished {
		t.Errorf("pub.Status = %q, want published", pub.Status)
	}
}
