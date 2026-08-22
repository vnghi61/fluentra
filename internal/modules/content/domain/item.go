package domain

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Item represents a persistent learning material identity, independent of revisions.
type Item struct {
	ID               uuid.UUID
	Kind             string
	Slug             string
	CurrentVersionID *uuid.UUID
	Status           AuthoringStatus
	OwnerID          uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ValidateSlug checks if the slug conforms to kebab-case format.
func ValidateSlug(slug string) error {
	if slug == "" || !slugRegex.MatchString(slug) {
		return ErrInvalidSlug.WithInternal("slug must contain only lowercase alphanumeric characters and hyphens")
	}
	return nil
}

// ValidateKind checks if kind is well-formed.
func ValidateKind(kind string) error {
	if kind == "" || len(kind) > 50 {
		return ErrInvalidKind.WithInternal("kind must be between 1 and 50 characters")
	}
	return nil
}
