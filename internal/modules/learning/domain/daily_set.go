package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// DailySet represents a learner's cached practice set for one local date.
type DailySet struct {
	ID          uuid.UUID   `json:"id"`
	UserID      uuid.UUID   `json:"user_id"`
	LocalDate   time.Time   `json:"local_date"`
	ActivityIDs []uuid.UUID `json:"activity_ids"`
	CreatedAt   time.Time   `json:"created_at"`
}

// PoolActivity represents an activity in the practice pool.
type PoolActivity struct {
	ID               uuid.UUID       `json:"id"`
	LessonID         uuid.UUID       `json:"lesson_id"`
	Position         int             `json:"position"`
	Kind             string          `json:"kind"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	Config           json.RawMessage `json:"config"`
	Weight           int             `json:"weight"`
}

// DailySetActivityDTO models an activity within a daily practice set with redacted content.
type DailySetActivityDTO struct {
	ID               uuid.UUID              `json:"id"`
	LessonID         uuid.UUID              `json:"lesson_id"`
	Position         int                    `json:"position"`
	Kind             string                 `json:"kind"`
	ContentVersionID uuid.UUID              `json:"content_version_id"`
	Config           map[string]interface{} `json:"config,omitempty"`
	Content          map[string]interface{} `json:"content,omitempty"`
	Weight           int                    `json:"weight"`
}

// DailySetDTO models the learner-facing daily practice set for today.
type DailySetDTO struct {
	ID         uuid.UUID             `json:"id"`
	LocalDate  time.Time             `json:"local_date"`
	Level      string                `json:"level"`
	Activities []DailySetActivityDTO `json:"activities"`
}

