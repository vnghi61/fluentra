// Package domain holds the user module's entities, value objects and
// invariants. It imports no infrastructure: everything here is testable with
// zero setup, which is what makes the rules cheap enough to state precisely.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Status is the account lifecycle state stored in core.users.status.
type Status string

// The complete set of account states. They mirror the core.user_status enum;
// a value the database can hold that this list does not is a bug in one of the
// two, which ParseStatus turns into an error rather than a silent zero value.
const (
	StatusActive          Status = "active"
	StatusSuspended       Status = "suspended"
	StatusPendingDeletion Status = "pending_deletion"
	StatusDeleted         Status = "deleted"
)

// ParseStatus converts a stored or submitted value into a Status.
func ParseStatus(value string) (Status, error) {
	switch Status(value) {
	case StatusActive, StatusSuspended, StatusPendingDeletion, StatusDeleted:
		return Status(value), nil
	default:
		return "", ErrUnknownStatus
	}
}

// Usable reports whether the account may act. Suspension and deletion both
// remove that right, and the difference between them matters to the owner, not
// to the authorisation decision.
func (s Status) Usable() bool { return s == StatusActive }

// User is the identity record. It deliberately carries nothing descriptive:
// every schema in the system foreign-keys to this table, and it stays
// referenceable only while it stays narrow.
type User struct {
	ID              uuid.UUID
	Email           string
	Status          Status
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// EmailVerified reports whether the address has been confirmed.
func (u User) EmailVerified() bool { return u.EmailVerifiedAt != nil }
