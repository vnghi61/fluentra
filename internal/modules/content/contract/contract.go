package contract

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Aggregate is the outbox aggregate name every event below is written under.
const Aggregate = "content"

// Event topics for the content module.
const (
	EventContentPublished = "content.published"
	EventContentArchived  = "content.archived"
)

// Version represents an immutable snapshot of a learning material item.
type Version struct {
	ID          uuid.UUID       `json:"id"`
	ItemID      uuid.UUID       `json:"item_id"`
	Version     int             `json:"version"`
	Kind        string          `json:"kind"`
	Body        json.RawMessage `json:"body"`
	CEFRLevel   string          `json:"cefr_level"`
	MediaRefs   []string        `json:"media_refs"`
	Tags        []string        `json:"tags"`
	Status      string          `json:"status"`
	PublishedAt *time.Time      `json:"published_at,omitempty"`
}

// BrowseFilter specifies filtering options when querying published content.
type BrowseFilter struct {
	Kind      *string  `json:"kind,omitempty"`
	CEFRLevel *string  `json:"cefr_level,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Limit     int      `json:"limit"`
	Offset    int      `json:"offset"`
}

// Reader provides batched and single-item access to published content versions.
// Lesson rendering resolves many content versions at once; GetManyVersions prevents N+1 queries.
type Reader interface {
	GetVersion(ctx context.Context, id uuid.UUID) (*Version, error)
	GetManyVersions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*Version, error)
	Browse(ctx context.Context, filter BrowseFilter) ([]*Version, int, error)
}

// AuthorSpec describes one piece of machine-authored content.
//
// Addressed by slug, not by id: a generator runs on a schedule and has to be
// able to say "this word's flashcard" without remembering what it created last
// time. The slug is the identity, and re-running with the same slug and the
// same body changes nothing.
type AuthorSpec struct {
	Slug      string
	Kind      string
	CEFRLevel string
	Body      json.RawMessage
	// AuthorID owns the item and is recorded as the reviewer. Machine-authored
	// content still leaves an approval row, because a published version that
	// nobody approved is a state the authoring API has never emitted, and the
	// first person to meet it would read it as a bug in the workflow.
	AuthorID uuid.UUID
}

// Author is the narrow authoring surface a content generator needs.
//
// Deliberately not the review state machine. `CreateItem` → `SubmitForReview`
// → `Review` → `Publish` exists for people, where each step is a decision
// somebody makes; driving it from a cron job would mean a machine approving its
// own work through four calls and a transaction it does not own. This is one
// call that says what the content is, and is idempotent on the slug.
type Author interface {
	// EnsurePublished creates or updates the item at this slug and returns the
	// id of its published version. A spec whose body already matches the
	// current version writes nothing and returns that version.
	EnsurePublished(ctx context.Context, spec AuthorSpec) (uuid.UUID, error)
}

// Published is emitted when a content version transitions to published.
type Published struct {
	ItemID     uuid.UUID `json:"item_id"`
	VersionID  uuid.UUID `json:"version_id"`
	Kind       string    `json:"kind"`
	CEFRLevel  string    `json:"cefr_level"`
	OccurredAt time.Time `json:"occurred_at"`
}

// Archived is emitted when a content item/version is archived.
type Archived struct {
	ItemID     uuid.UUID `json:"item_id"`
	VersionID  uuid.UUID `json:"version_id"`
	OccurredAt time.Time `json:"occurred_at"`
}
