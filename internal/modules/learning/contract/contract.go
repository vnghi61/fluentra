package contract

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Aggregate is the outbox aggregate name every event below is written under.
const Aggregate = "learning"

// Event topics published by the learning module.
const (
	EventActivityCompleted        = "activity.completed"
	EventLessonCompleted          = "lesson.completed"
	EventCourseCompleted          = "course.completed"
	EventPlacementCompleted       = "placement.completed"
	EventLearningSessionCompleted = "learning.session_completed"
)

// ActivityCompleted is published when a learner completes an activity attempt.
type ActivityCompleted struct {
	UserID     uuid.UUID `json:"user_id"`
	ActivityID uuid.UUID `json:"activity_id"`
	Score      int       `json:"score"`
	Skill      string    `json:"skill"`
	DurationMs int       `json:"duration_ms"`
	OccurredAt time.Time `json:"occurred_at"`
}

// LessonCompleted is published when all required activities in a lesson are completed.
type LessonCompleted struct {
	UserID     uuid.UUID `json:"user_id"`
	LessonID   uuid.UUID `json:"lesson_id"`
	Score      int       `json:"score"`
	SkillFocus string    `json:"skill_focus"`
	OccurredAt time.Time `json:"occurred_at"`
}

// CourseCompleted is published when all required units and lessons in a course are completed.
type CourseCompleted struct {
	UserID     uuid.UUID `json:"user_id"`
	CourseID   uuid.UUID `json:"course_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// PlacementCompleted is published when a learner finishes an adaptive placement test.
type PlacementCompleted struct {
	UserID     uuid.UUID       `json:"user_id"`
	Level      string          `json:"level"`
	PerSkill   json.RawMessage `json:"per_skill"`
	OccurredAt time.Time       `json:"occurred_at"`
}

// SessionCompleted is published when a study session is ended.
type SessionCompleted struct {
	UserID     uuid.UUID `json:"user_id"`
	SessionID  uuid.UUID `json:"session_id"`
	Minutes    int       `json:"minutes"`
	Activities int       `json:"activities"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ReviewItem models an item worthy of spaced repetition produced by grading.
// Owned by learning to allow all skill modules to implement ExerciseGrader without depending on srs (Trap 1).
type ReviewItem struct {
	ContentVersionID uuid.UUID `json:"content_version_id"`
	Skill            string    `json:"skill"`
	InitialGrade     string    `json:"initial_grade"`
}

// GradeRequest contains the inputs required for a skill grader to evaluate an attempt.
// Carries IDs rather than full content structs to avoid tight coupling with content representations (P8.3 trap).
type GradeRequest struct {
	AttemptID        uuid.UUID       `json:"attempt_id"`
	ActivityID       uuid.UUID       `json:"activity_id"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	UserID           uuid.UUID       `json:"user_id"`
	Response         json.RawMessage `json:"response"`
}

// GradeResult contains the outcome of grading an exercise.
type GradeResult struct {
	Score       int          `json:"score"`
	MaxScore    int          `json:"max_score"`
	Correct     bool         `json:"correct"`
	Feedback    string       `json:"feedback"`
	Async       bool         `json:"async"`
	ReviewItems []ReviewItem `json:"review_items,omitempty"`
}

// ExerciseGrader is implemented by every skill module to grade domain-specific exercises.
type ExerciseGrader interface {
	Grade(ctx context.Context, req GradeRequest) (GradeResult, error)
}

// ProgressScope represents the aggregation level of learner progress.
type ProgressScope string

// ProgressScope enumeration values.
const (
	ScopeActivity ProgressScope = "activity"
	ScopeLesson   ProgressScope = "lesson"
	ScopeUnit     ProgressScope = "unit"
	ScopeCourse   ProgressScope = "course"
)

// Progress represents a rolled-up progress record.
type Progress struct {
	UserID      uuid.UUID     `json:"user_id"`
	Scope       ProgressScope `json:"scope"`
	ScopeID     uuid.UUID     `json:"scope_id"`
	Status      string        `json:"status"`
	Score       *int          `json:"score,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
}

// ProgressReader provides read access to rolled-up progress for other modules.
//
// One call answers for a whole scope rather than one row. `gamification`,
// `admin` and `analytics` each render a learner's progress across every course
// or lesson at once, so a reader keyed on a single scope id would put the same
// N+1 into three Phase 3 modules that UnlockChecker below was batched to avoid.
// This is the signature AGENT.md §4 documents.
type ProgressReader interface {
	ProgressOf(ctx context.Context, userID uuid.UUID, scope ProgressScope) ([]Progress, error)
}

// UnlockChecker answers whether a learner has met prerequisites for lessons.
// Batched to avoid N+1 queries when evaluating an entire course tree (Trap 3).
type UnlockChecker interface {
	IsUnlocked(ctx context.Context, userID uuid.UUID, lessonIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}
