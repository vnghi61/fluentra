package domain

import (
	"encoding/json"

	"github.com/google/uuid"
)

// ActivityInput represents an activity to be attached or updated on a lesson.
type ActivityInput struct {
	Position         int             `json:"position"`
	Kind             string          `json:"kind"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	Config           json.RawMessage `json:"config"`
	Weight           int             `json:"weight"`
}

// The bounds `openapi.yaml` declares for an activity, restated here because the
// handlers are hand-written: a schema the router does not enforce is a comment.
// They are also what makes the int32 in CalculateLessonDuration safe.
const (
	// MaxActivitiesPerLesson bounds the activity list a single write may carry.
	MaxActivitiesPerLesson = 100
	// MaxActivityKindLength matches the column the migration declares.
	MaxActivityKindLength = 50
	// MaxActivityWeight bounds one activity's contribution to the lesson duration.
	MaxActivityWeight = 100
)

// IsValidPosition reports whether a position is within the bounded activity list.
func IsValidPosition(pos int) bool {
	return pos >= 1 && pos <= MaxActivitiesPerLesson
}

// IsValidActivityKind checks if activity kind is non-empty and bounded.
func IsValidActivityKind(kind string) bool {
	return len(kind) >= 1 && len(kind) <= MaxActivityKindLength
}

// IsValidWeight reports whether an activity weight is within the range the spec
// declares. Unbounded, it would overflow the int32 duration it feeds.
func IsValidWeight(weight int) bool {
	return weight >= 0 && weight <= MaxActivityWeight
}
