package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// The authoring surface a curriculum generator uses.
//
// Every method is an upsert on something stable — a slug, or a position within
// a parent — so the job that calls them can run every hour and converge on the
// same curriculum rather than appending a copy of it.
//
// Nothing here can unpublish or delete a lesson somebody wrote by hand. The only
// destructive operation is ReplaceActivities, and it is scoped to one lesson the
// caller has just created or matched.

// EnsureCourse implements contract.Author.
func (s *Service) EnsureCourse(ctx context.Context, spec contract.CourseSpec) (uuid.UUID, error) {
	if strings.TrimSpace(spec.Slug) == "" || strings.TrimSpace(spec.Title) == "" {
		return uuid.Nil, authorInvalid("A generated course needs a slug and a title.")
	}

	course, err := s.repo.UpsertCourse(ctx, spec)
	if err != nil {
		return uuid.Nil, err
	}
	return course.ID, nil
}

// EnsureUnit implements contract.Author.
func (s *Service) EnsureUnit(ctx context.Context, spec contract.UnitSpec) (uuid.UUID, error) {
	if spec.CourseID == uuid.Nil || spec.Position <= 0 {
		return uuid.Nil, authorInvalid("A generated unit needs a course and a position above zero.")
	}

	unit, err := s.repo.UpsertUnit(ctx, spec)
	if err != nil {
		return uuid.Nil, err
	}
	return unit.ID, nil
}

// EnsureLesson implements contract.Author.
func (s *Service) EnsureLesson(ctx context.Context, spec contract.LessonSpec) (uuid.UUID, error) {
	if spec.UnitID == uuid.Nil || spec.Position <= 0 {
		return uuid.Nil, authorInvalid("A generated lesson needs a unit and a position above zero.")
	}

	// A zero duration fails the schema's check constraint, and "unknown" is the
	// honest value for a generated lesson nobody has timed.
	if spec.EstimatedMinutes <= 0 {
		spec.EstimatedMinutes = 1
	}
	lesson, err := s.repo.UpsertLesson(ctx, spec)
	if err != nil {
		return uuid.Nil, err
	}
	return lesson.ID, nil
}

// ReplaceActivities implements contract.Author.
//
// Wholesale, not item by item. A generated lesson is a rendering of its source
// data: if a word gained a fifth example and its gap-fill moved position, the
// lesson should look like the new rendering and not like the two interleaved.
//
// The lesson's cached detail is dropped afterwards for the same reason
// UpdateActivities drops it — the cache holds the activity list, and leaving it
// means a learner opens the lesson and gets the exercises that were there an
// hour ago.
func (s *Service) ReplaceActivities(
	ctx context.Context, lessonID uuid.UUID, activities []contract.ActivitySpec,
) error {
	if lessonID == uuid.Nil {
		return authorInvalid("Replacing activities needs a lesson.")
	}

	inputs := make([]domain.ActivityInput, 0, len(activities))
	for _, activity := range activities {
		if activity.Kind == "" || activity.ContentVersionID == uuid.Nil {
			return authorInvalid("Every generated activity needs a kind and a content version.")
		}
		config := activity.Config
		if len(config) == 0 {
			config = json.RawMessage("{}")
		}
		weight := activity.Weight
		if weight <= 0 {
			weight = 1
		}
		inputs = append(inputs, domain.ActivityInput{
			Position:         activity.Position,
			Kind:             activity.Kind,
			ContentVersionID: activity.ContentVersionID,
			Config:           config,
			Weight:           weight,
		})
	}

	if _, err := s.repo.ReplaceActivities(ctx, lessonID, inputs); err != nil {
		return err
	}
	// courseID is not resolved here: the tree cache is keyed on the course slug
	// and a generated lesson's course is the generator's own, which no learner
	// browses as a tree. The detail cache is the one that would serve stale
	// exercises, and invalidateLessonCaches drops it with a nil course.
	s.invalidateLessonCaches(ctx, lessonID, uuid.Nil)
	return nil
}

func authorInvalid(message string) error {
	return apperr.New(apperr.Validation, "LESSON_AUTHOR_SPEC_INVALID", message)
}

var _ contract.Author = (*Service)(nil)
