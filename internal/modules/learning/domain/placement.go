package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PlacementSessionStatus values
const (
	PlacementSessionStatusInProgress = "in_progress"
	PlacementSessionStatusCompleted  = "completed"
	PlacementSessionStatusExpired    = "expired"
)

// PlacementRetakeCooldown is 30 days between completed placement tests (WO13 §3.5).
const PlacementRetakeCooldown = 30 * 24 * time.Hour

// PlacementSessionDuration is 60 minutes expiry for an adaptive test session.
const PlacementSessionDuration = 60 * time.Minute

// PlacementResponseRecord records one item response during the placement test.
type PlacementResponseRecord struct {
	ActivityID uuid.UUID       `json:"activity_id"`
	Kind       string          `json:"kind"`
	Level      string          `json:"level"`
	Score      float64         `json:"score"`
	Response   json.RawMessage `json:"response"`
	AnsweredAt time.Time       `json:"answered_at"`
}

// PlacementSession represents an adaptive placement test session.
type PlacementSession struct {
	ID                uuid.UUID                 `json:"id"`
	UserID            uuid.UUID                 `json:"user_id"`
	Status            string                    `json:"status"`
	Stage             string                    `json:"stage"`
	ThetaEstimate     float64                   `json:"theta_estimate"`
	PlacedLevel       *string                   `json:"placed_level,omitempty"`
	Confidence        float64                   `json:"confidence"`
	CurrentActivityID *uuid.UUID                `json:"current_activity_id,omitempty"`
	CurrentItemKind   *string                   `json:"current_item_kind,omitempty"`
	CurrentItemLevel  *string                   `json:"current_item_level,omitempty"`
	Responses         []PlacementResponseRecord `json:"responses"`
	AdaptiveState     AdaptiveState             `json:"adaptive_state"`
	StartedAt         time.Time                 `json:"started_at"`
	CompletedAt       *time.Time                `json:"completed_at,omitempty"`
	ExpiresAt         time.Time                 `json:"expires_at"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
}

// IsInProgress reports whether the session is currently active.
func (s *PlacementSession) IsInProgress() bool {
	return s.Status == PlacementSessionStatusInProgress
}

// IsCompleted reports whether the session has finished.
func (s *PlacementSession) IsCompleted() bool {
	return s.Status == PlacementSessionStatusCompleted
}

// IsExpired reports whether the session has expired.
func (s *PlacementSession) IsExpired(now time.Time) bool {
	return s.Status == PlacementSessionStatusExpired || (s.Status == PlacementSessionStatusInProgress && now.After(s.ExpiresAt))
}

// PlacementInvitationDTO summarizes a learner's placement status and eligibility to test.
type PlacementInvitationDTO struct {
	Eligible        bool             `json:"eligible"`
	HasActiveTest   bool             `json:"has_active_test"`
	ActiveSessionID *uuid.UUID       `json:"active_session_id,omitempty"`
	CooldownUntil   *time.Time       `json:"cooldown_until,omitempty"`
	LastResult      *PlacementResult `json:"last_result,omitempty"`
	PoolSufficient  bool             `json:"pool_sufficient"`
}

// PlacementResult models the outcome of a completed placement test.
type PlacementResult struct {
	ID             uuid.UUID          `json:"id"`
	UserID         uuid.UUID          `json:"user_id"`
	EstimatedLevel string             `json:"estimated_level"`
	PerSkill       map[string]float64 `json:"per_skill"`
	SessionID      *uuid.UUID         `json:"session_id,omitempty"`
	TakenAt        time.Time          `json:"taken_at"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// WeeklyPlan models a personalized weekly study plan.
type WeeklyPlan struct {
	ID               uuid.UUID          `json:"id"`
	UserID           uuid.UUID          `json:"user_id"`
	WeekStartDate    time.Time          `json:"week_start_date"`
	PlacedLevel      string             `json:"placed_level"`
	WeakestSkill     string             `json:"weakest_skill"`
	TimeDistribution map[string]float64 `json:"time_distribution"`
	DailyTargets     []DailyPlanTarget  `json:"daily_targets"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

// DailyPlanTarget models recommended daily study objectives.
type DailyPlanTarget struct {
	DayOfWeek       string     `json:"day_of_week"` // e.g. "Monday", "Tuesday"
	TargetMinutes   int        `json:"target_minutes"`
	PrimarySkill    string     `json:"primary_skill"`
	RecommendedKind string     `json:"recommended_kind"`
	LessonID        *uuid.UUID `json:"lesson_id,omitempty"`
	LessonTitle     string     `json:"lesson_title,omitempty"`
}

// StartingPathDTO models the recommended course and starting lesson for a learner.
type StartingPathDTO struct {
	PlacedLevel              string     `json:"placed_level"`
	DeclaredLevel            string     `json:"declared_level,omitempty"`
	RecommendedCourseID      uuid.UUID  `json:"recommended_course_id"`
	RecommendedCourseSlug    string     `json:"recommended_course_slug"`
	RecommendedCourseTitle   string     `json:"recommended_course_title"`
	FirstUnlockedLessonID    *uuid.UUID `json:"first_unlocked_lesson_id,omitempty"`
	FirstUnlockedLessonTitle string     `json:"first_unlocked_lesson_title,omitempty"`
}
