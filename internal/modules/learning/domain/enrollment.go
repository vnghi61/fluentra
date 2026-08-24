package domain

import (
	"time"

	"github.com/google/uuid"
)

// Enrollment status constants.
//
// These match both the DB CHECK constraint and the OpenAPI enum.
const (
	StatusEnrollmentActive    = "active"
	StatusEnrollmentCompleted = "completed"
	StatusEnrollmentDropped   = "dropped"
)

// Enrollment models a learner's enrollment in a course.
type Enrollment struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	CourseID    uuid.UUID  `json:"course_id"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsActive returns true if the enrollment is currently active.
func (e *Enrollment) IsActive() bool {
	return e.Status == StatusEnrollmentActive
}

// IsCompleted returns true if the enrollment is completed.
func (e *Enrollment) IsCompleted() bool {
	return e.Status == StatusEnrollmentCompleted
}
