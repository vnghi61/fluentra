package contract

import (
	"context"

	"github.com/google/uuid"
)

// FlagReader is the interface consumed by other modules to evaluate feature flags.
type FlagReader interface {
	// IsEnabled evaluates whether a feature flag is enabled for the specified user.
	IsEnabled(ctx context.Context, key string, userID uuid.UUID) (bool, error)
}

// FeatureFlag DTO returned by FlagReader or Admin APIs.
type FeatureFlag struct {
	Key            string `json:"key"`
	Enabled        bool   `json:"enabled"`
	RolloutPercent int    `json:"rollout_percent"`
	Owner          string `json:"owner"`
	ExpiresOn      string `json:"expires_on"`
	Description    string `json:"description"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}
