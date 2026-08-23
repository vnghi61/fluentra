package repository_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/generated/lesson/sqlc"
	"github.com/fluentra/fluentra/internal/modules/lesson/repository"
)

func TestMappingFunctions(t *testing.T) {
	courseID := uuid.New()
	unitID := uuid.New()
	lessonID := uuid.New()
	actID := uuid.New()
	contentVerID := uuid.New()

	t.Run("ToContractCourse", func(t *testing.T) {
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
	})

	t.Run("ToContractUnit", func(t *testing.T) {
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
	})

	t.Run("ToContractActivity", func(t *testing.T) {
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

		// Empty config should fall back to "{}"
		a.Config = nil
		dtoEmpty := repository.ToContractActivity(a)
		if string(dtoEmpty.Config) != "{}" {
			t.Errorf("expected {}, got %s", string(dtoEmpty.Config))
		}
	})

	t.Run("ToContractLesson", func(t *testing.T) {
		l := sqlc.LearnLesson{
			ID:               lessonID,
			UnitID:           unitID,
			Position:         2,
			Title:            "Lesson 2",
			SkillFocus:       "vocabulary",
			EstimatedMinutes: 15,
			Status:           "published",
		}
		acts := []repository.ActivityInputDTO{}
		dto := repository.ToContractLesson(l, nil)
		if dto.ID != lessonID || dto.Position != 2 || dto.EstimatedMinutes != 15 {
			t.Errorf("unexpected mapping: %+v", dto)
		}
		_ = acts
	})
}
