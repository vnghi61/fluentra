package contract

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Aggregate is the outbox aggregate name every event below is written under.
const Aggregate = "lesson"

// Event topics for the lesson module.
const (
	EventLessonPublished = "lesson.published"
)

// Course represents a top-level curriculum container (e.g., IELTS Foundation).
type Course struct {
	ID             uuid.UUID `json:"id"`
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	CEFRFrom       string    `json:"cefr_from"`
	CEFRTo         string    `json:"cefr_to"`
	Status         string    `json:"status"`
	EstimatedHours int       `json:"estimated_hours"`
}

// Unit represents a thematic group of lessons within a course.
type Unit struct {
	ID          uuid.UUID `json:"id"`
	CourseID    uuid.UUID `json:"course_id"`
	Position    int       `json:"position"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
}

// Activity represents a discrete interactive element in a lesson (e.g. multiple choice, gap-fill, flashcard).
type Activity struct {
	ID               uuid.UUID       `json:"id"`
	LessonID         uuid.UUID       `json:"lesson_id"`
	Position         int             `json:"position"`
	Kind             string          `json:"kind"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	Config           json.RawMessage `json:"config"`
	Weight           int             `json:"weight"`
}

// Lesson represents a schedulable learning unit containing activities.
type Lesson struct {
	ID               uuid.UUID  `json:"id"`
	UnitID           uuid.UUID  `json:"unit_id"`
	Position         int        `json:"position"`
	Title            string     `json:"title"`
	SkillFocus       string     `json:"skill_focus"`
	EstimatedMinutes int        `json:"estimated_minutes"`
	Status           string     `json:"status"`
	Activities       []Activity `json:"activities,omitempty"`
}

// ActivityHierarchy contains the structural path and metadata of an activity.
type ActivityHierarchy struct {
	ActivityID       uuid.UUID       `json:"activity_id"`
	LessonID         uuid.UUID       `json:"lesson_id"`
	UnitID           uuid.UUID       `json:"unit_id"`
	CourseID         uuid.UUID       `json:"course_id"`
	Kind             string          `json:"kind"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	Config           json.RawMessage `json:"config"`
	Weight           int             `json:"weight"`
	LessonSkillFocus string          `json:"lesson_skill_focus"`
}

// Reader provides access to course hierarchy and lesson activities.
type Reader interface {
	GetLesson(ctx context.Context, id uuid.UUID) (*Lesson, error)
	ListLessons(ctx context.Context, unitID uuid.UUID) ([]*Lesson, error)
	NextLesson(ctx context.Context, courseID uuid.UUID, currentLessonID *uuid.UUID) (*Lesson, error)
	ResolveActivity(ctx context.Context, activityID uuid.UUID) (*ActivityHierarchy, error)
}

// Published is emitted when a lesson is published.
type Published struct {
	LessonID   uuid.UUID `json:"lesson_id"`
	CourseID   uuid.UUID `json:"course_id"`
	SkillFocus string    `json:"skill_focus"`
	OccurredAt time.Time `json:"occurred_at"`
}
