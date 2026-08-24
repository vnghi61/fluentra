package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LearningSession models a study session tracked on the server.
type LearningSession struct {
	ID                  uuid.UUID       `json:"id"`
	UserID              uuid.UUID       `json:"user_id"`
	StartedAt           time.Time       `json:"started_at"`
	EndedAt             *time.Time      `json:"ended_at,omitempty"`
	ActivitiesCompleted int             `json:"activities_completed"`
	Minutes             int             `json:"minutes"`
	Metadata            json.RawMessage `json:"metadata,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// IsCompleted reports whether the session has ended.
func (s *LearningSession) IsCompleted() bool {
	return s.EndedAt != nil
}
