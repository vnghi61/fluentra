package domain

import (
	"github.com/google/uuid"
)

// Learner state constants for dashboard and progress tracking.
const (
	StateNotStarted = "not_started"
	StateInProgress = "in_progress"
	StateCompleted  = "completed"
)

// NextActivity models the next interactive activity recommended for the learner.
type NextActivity struct {
	ActivityID       uuid.UUID `json:"activity_id"`
	LessonID         uuid.UUID `json:"lesson_id"`
	UnitID           uuid.UUID `json:"unit_id"`
	CourseID         uuid.UUID `json:"course_id"`
	Title            string    `json:"title"`
	Kind             string    `json:"kind"`
	Skill            string    `json:"skill"`
	EstimatedMinutes *int      `json:"estimated_minutes,omitempty"`
}

// NextActivityResolution represents the resolved state and optional next activity for a learner.
type NextActivityResolution struct {
	State        string        `json:"state"`
	NextActivity *NextActivity `json:"next_activity,omitempty"`
}
