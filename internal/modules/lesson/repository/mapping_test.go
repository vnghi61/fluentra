package repository_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/generated/lesson/sqlc"
	"github.com/fluentra/fluentra/internal/modules/lesson/repository"
)

func TestMapping_Courses(t *testing.T) {
	t.Parallel()

	courseID := uuid.New()
	c := sqlc.LearnCourse{
		ID:             courseID,
		Slug:           "ielts-foundation",
		Title:          "IELTS Foundation",
		Description:    "Prep course",
		CefrFrom:       "B1",
		CefrTo:         "B2",
		Status:         "published",
		EstimatedHours: 40,
	}
	dto := repository.ToContractCourse(c)
	if dto.ID != courseID || dto.Slug != "ielts-foundation" || dto.EstimatedHours != 40 {
		t.Errorf("unexpected mapping: %+v", dto)
	}

	courses := repository.ToContractCourses([]sqlc.LearnCourse{c})
	if len(courses) != 1 || courses[0].ID != courseID {
		t.Errorf("unexpected courses: %+v", courses)
	}
}

func TestMapping_Units(t *testing.T) {
	t.Parallel()

	courseID := uuid.New()
	unitID := uuid.New()
	u := sqlc.LearnCourseUnit{
		ID:          unitID,
		CourseID:    courseID,
		Position:    1,
		Title:       "Unit 1",
		Description: "Campus life",
	}
	dto := repository.ToContractUnit(u)
	if dto.ID != unitID || dto.Position != 1 || dto.Title != "Unit 1" {
		t.Errorf("unexpected mapping: %+v", dto)
	}

	units := repository.ToContractUnits([]sqlc.LearnCourseUnit{u})
	if len(units) != 1 || units[0].ID != unitID {
		t.Errorf("unexpected units: %+v", units)
	}
}

func TestMapping_Activities(t *testing.T) {
	t.Parallel()

	lessonID := uuid.New()
	actID := uuid.New()
	contentVerID := uuid.New()
	a := sqlc.LearnActivity{
		ID:               actID,
		LessonID:         lessonID,
		Position:         1,
		Kind:             "vocab_mc",
		ContentVersionID: contentVerID,
		Config:           []byte(`{"prompt":"hello"}`),
		Weight:           5,
	}
	dto := repository.ToContractActivity(a)
	if dto.ID != actID || dto.Kind != "vocab_mc" || dto.Weight != 5 || string(dto.Config) != `{"prompt":"hello"}` {
		t.Errorf("unexpected mapping: %+v", dto)
	}

	acts := repository.ToContractActivities([]sqlc.LearnActivity{a})
	if len(acts) != 1 || acts[0].ID != actID {
		t.Errorf("unexpected activities: %+v", acts)
	}

	// Empty config should fall back to "{}"
	a.Config = nil
	dtoEmpty := repository.ToContractActivity(a)
	if string(dtoEmpty.Config) != "{}" {
		t.Errorf("expected {}, got %s", string(dtoEmpty.Config))
	}
}

func TestMapping_LessonsAndEdges(t *testing.T) {
	t.Parallel()

	unitID := uuid.New()
	lessonID := uuid.New()
	actID := uuid.New()
	contentVerID := uuid.New()
	l := sqlc.LearnLesson{
		ID:               lessonID,
		UnitID:           unitID,
		Position:         2,
		Title:            "Lesson 2",
		SkillFocus:       "vocabulary",
		EstimatedMinutes: 15,
		Status:           "published",
	}
	acts := repository.ToContractActivities([]sqlc.LearnActivity{
		{ID: actID, LessonID: lessonID, Position: 1, Kind: "quiz", ContentVersionID: contentVerID, Weight: 1},
	})
	dto := repository.ToContractLesson(l, acts)
	if dto.ID != lessonID || dto.Position != 2 || dto.EstimatedMinutes != 15 || len(dto.Activities) != 1 {
		t.Errorf("unexpected mapping: %+v", dto)
	}

	edges := repository.ToPrerequisiteEdges([]sqlc.ListAllPrerequisitesInCourseRow{
		{LessonID: lessonID, RequiresLessonID: uuid.New()},
	})
	if len(edges) != 1 || edges[0].LessonID != lessonID {
		t.Errorf("unexpected edges: %+v", edges)
	}
}
