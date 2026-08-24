package domain

import (
	"time"

	"github.com/google/uuid"
)

// Dashboard state constants matching OpenAPI enum.
const (
	DashboardStateNotStarted = StateNotStarted
	DashboardStateInProgress = StateInProgress
	DashboardStateCompleted  = StateCompleted
)

// Course progress status constants matching OpenAPI enum.
const (
	CourseProgressNotStarted = "not_started"
	CourseProgressInProgress = "in_progress"
	CourseProgressCompleted  = "completed"
)

// DashboardData represents aggregated dashboard state for a learner.
type DashboardData struct {
	State           string         `json:"state"`
	NextActivity    *NextActivity  `json:"next_activity,omitempty"`
	DueReviewsCount int            `json:"due_reviews_count"`
	SkillMastery    []SkillMastery `json:"skill_mastery"`
}

// CourseProgressData represents progress within a single enrolled course.
type CourseProgressData struct {
	CourseID            uuid.UUID  `json:"course_id"`
	Status              string     `json:"status"`
	CompletedActivities int        `json:"completed_activities"`
	TotalActivities     int        `json:"total_activities"`
	Percentage          int        `json:"percentage"`
	Score               *int       `json:"score,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

// ProgressData represents overall learner progress across all courses and skills.
type ProgressData struct {
	Courses []CourseProgressData `json:"courses"`
	Skills  []SkillMastery       `json:"skills"`
}

// CalculatePercentage computes the integer progress percentage for a course.
// Returns 0 if total is 0 to avoid division by zero.
func CalculatePercentage(completed, total int) int {
	if total <= 0 || completed <= 0 {
		return 0
	}
	if completed >= total {
		return 100
	}
	return (completed * 100) / total
}

// DeriveCourseProgressStatus calculates the CourseProgress status string.
//
// Rules:
// - If all activities are completed (and total > 0) or enrollment is completed -> "completed"
// - If at least one activity is completed -> "in_progress"
// - Otherwise -> "not_started"
func DeriveCourseProgressStatus(completed, total int, isEnrollmentCompleted bool) string {
	if isEnrollmentCompleted || (total > 0 && completed >= total) {
		return CourseProgressCompleted
	}
	if completed > 0 {
		return CourseProgressInProgress
	}
	return CourseProgressNotStarted
}
