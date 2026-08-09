package contract

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Summary is the shape other modules render when they need to show a person.
// It is deliberately not the full profile: a module that needs more than this
// is usually reaching for data it should not have.
type Summary struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Locale      string    `json:"locale"`
	Timezone    string    `json:"timezone"`
	Status      string    `json:"status"`
}

// Reader reads user summaries.
//
// GetManyByIDs exists from the first version of this interface even though
// nothing batches yet. The alternative is that five modules each write a loop
// around GetByID first and get refactored later — the loop is the natural way
// to write it when the batched call is missing, and an N+1 that works in
// development is invisible until it is in production.
type Reader interface {
	// GetByID returns one summary, or an error that apperr.Is reports as
	// apperr.NotFound when the account does not exist.
	GetByID(ctx context.Context, id uuid.UUID) (Summary, error)

	// GetManyByIDs returns the summaries that exist, keyed by id. Ids that do
	// not exist are absent from the map rather than an error: a caller
	// rendering a list wants the rows it can render, not a failure because one
	// account was deleted.
	GetManyByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Summary, error)

	// Exists reports whether the account exists, without reading it.
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// NewUser is the minimum needed to create an account. Only `auth` calls this,
// during registration: a user record without credentials is not something any
// other module has a reason to make.
type NewUser struct {
	Email       string
	DisplayName string
	Locale      string
	Timezone    string
}

// Creator creates accounts.
type Creator interface {
	// CreateUser writes the user, profile and preference rows together and
	// returns the new id. It is one call rather than three because an account
	// with no preferences is not a state any reader is prepared for.
	CreateUser(ctx context.Context, newUser NewUser) (uuid.UUID, error)
}

// Event names published by this module. They are strings rather than a Go type
// because a consumer in another module matches on the wire value, and a typed
// constant it cannot import would not help it.
const (
	EventProfileUpdated     = "user.profile_updated"
	EventPreferencesUpdated = "user.preferences_updated"
	EventDeletionRequested  = "user.deletion_requested"
	EventDeleted            = "user.deleted"
	EventSuspended          = "user.suspended"
)

// Aggregate is the outbox aggregate name every event above is written under.
const Aggregate = "user"

// ProfileUpdated is published when a learner changes their own profile.
//
// It carries the names of the fields that changed and not their values. The
// audit consumer writes this straight into audit_logs, and an audit log that
// records the old and new display name is a second copy of personal data with
// a longer retention period than the first.
type ProfileUpdated struct {
	UserID        uuid.UUID `json:"user_id"`
	ChangedFields []string  `json:"changed_fields"`
	ActorID       uuid.UUID `json:"actor_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// PreferencesUpdated is published when the preference set is replaced.
type PreferencesUpdated struct {
	UserID     uuid.UUID `json:"user_id"`
	ActorID    uuid.UUID `json:"actor_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// DeletionRequested is the event every module holding personal data reacts to.
// The type is declared here so those modules can be written against it; the
// flow that publishes it arrives with P3.3.
type DeletionRequested struct {
	UserID       uuid.UUID `json:"user_id"`
	ExecuteAfter time.Time `json:"execute_after"`
	OccurredAt   time.Time `json:"occurred_at"`
}
