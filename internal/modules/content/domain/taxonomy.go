package domain

import (
	"time"

	"github.com/google/uuid"
)

// Taxonomy represents a controlled classification entry (topic, skill, exam, etc.).
type Taxonomy struct {
	ID        uuid.UUID
	Namespace string
	Code      string
	Label     string
	ParentID  *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TaxonomyTag is the lightweight representation used in content responses.
type TaxonomyTag struct {
	Namespace string
	Code      string
	Label     string
}
