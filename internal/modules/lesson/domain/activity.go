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

// IsValidPosition checks if position is a positive integer.
func IsValidPosition(pos int) bool {
	return pos >= 1
}

// IsValidActivityKind checks if activity kind is non-empty and bounded.
func IsValidActivityKind(kind string) bool {
	return len(kind) >= 1 && len(kind) <= 50
}
