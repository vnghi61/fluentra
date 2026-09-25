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

type tempVersionKey struct{}

// ContextWithTempVersion attaches an in-memory content version to ctx for grading uncommitted candidates.
func ContextWithTempVersion(ctx context.Context, v *Version) context.Context {
	return context.WithValue(ctx, tempVersionKey{}, v)
}

// TempVersionFromContext extracts an in-memory content version from ctx if present and matching id.
func TempVersionFromContext(ctx context.Context, id uuid.UUID) (*Version, bool) {
	if v, ok := ctx.Value(tempVersionKey{}).(*Version); ok && v != nil && v.ID == id {
		return v, true
	}
	return nil, false
}

// TagRef identifies a taxonomy node by namespace and code.
type TagRef struct {
	Namespace string
	Code      string
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
	// Tags are spine taxonomy nodes associated with this content.
	Tags []TagRef
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

	// EnsureDraft creates or updates the item at this slug and returns the
	// id of its draft version, without marking it published or approved.
	EnsureDraft(ctx context.Context, spec AuthorSpec) (uuid.UUID, error)

	// ApproveVerified publishes a draft that an independent verifier confirmed
	// (WO 22 Stage A). It walks the draft in_review → approved → published in
	// one transaction, emits content.published through the outbox like any
	// other publish, and records a content_reviews row naming the verifier. A
	// verification that is not confirmed is refused: this door publishes, it
	// never judges.
	ApproveVerified(ctx context.Context, versionID uuid.UUID, verification Verification) error
}

// VerificationRecorder records an independent verifier's outcome on a version
// that is not yet published, so a doubt stays in its batch with its reason.
//
// Separate from Author because a doubt changes no state: it writes the marking
// and leaves the version a draft for a person.
type VerificationRecorder interface {
	RecordVerification(ctx context.Context, versionID uuid.UUID, verification Verification) error
}

// VerifiedBatchPublisher publishes a generation batch the independent verifier
// confirmed whole (WO 22 D22-13).
//
// A Foundation node is one batch: its topic, exercises, quiz and review. It
// publishes only when every version in it was confirmed; one doubt leaves the
// whole batch as drafts for a person, who approves it as a batch. Nothing is
// published half-way, so a learner never meets a topic whose quiz is missing or
// exercises whose topic was doubted.
type VerifiedBatchPublisher interface {
	// ApproveVerifiedBatch publishes every version of the batch in one
	// transaction and returns how many it published. A batch holding a
	// doubt, or refused by a publication gate, is left untouched and reported
	// with a CONTENT_BATCH_ESCALATED error naming why.
	ApproveVerifiedBatch(ctx context.Context, batch string) (int, error)
}

// ReviewBacklog says how far behind the people reviewing generated content are
// (WO 22 Stage O): a job that generates faster than anyone reads the doubts only
// grows a backlog.
type ReviewBacklog interface {
	// PendingBatchDays is how many distinct days' batches under the prefix
	// still hold drafts nobody has reviewed.
	PendingBatchDays(ctx context.Context, batchPrefix string) (int, error)
}

// Verification is the outcome of an independent check on a machine-authored
// version (WO 22 Stage A).
//
// Confirmed means a model other than the one that wrote the version solved it,
// found the key defensible and the explanation sound. Model and CheckedAt are
// stored so a published item can always be traced to the verifier that let it
// through. A verification that is not confirmed carries the reason a person
// needs to look.
type Verification struct {
	Confirmed bool
	Model     string
	Reason    string
	CheckedAt time.Time
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

// TTSCache provides caching and retrieval of synthesised speech audio.
type TTSCache interface {
	Get(ctx context.Context, textHash, voice string) (objectKey string, found bool, err error)
	Put(ctx context.Context, textHash, voice, engine, engineVersion, objectKey string) error
}

// TaxonomyNode represents a controlled classification entry.
type TaxonomyNode struct {
	ID           uuid.UUID  `json:"id"`
	Namespace    string     `json:"namespace"`
	Code         string     `json:"code"`
	Label        string     `json:"label"`
	CEFRLevel    *string    `json:"cefr_level,omitempty"`
	DeprecatedAt *time.Time `json:"deprecated_at,omitempty"`
}

// TagIndex answers which content items carry a spine node, so that a module
// filtering its own rows by spine tag never joins content.content_tags (rule L2).
type TagIndex interface {
	ItemIDsTaggedWith(ctx context.Context, code string) ([]uuid.UUID, error)
}

// TaxonomyResolver resolves taxonomy codes to identifiers and metadata.
type TaxonomyResolver interface {
	ResolveTaxonomyID(ctx context.Context, namespace, code string) (*uuid.UUID, error)
	GetTaxonomyByCode(ctx context.Context, code string) (*TaxonomyNode, error)
	GetTaxonomyByID(ctx context.Context, id uuid.UUID) (*TaxonomyNode, error)
	ListTaxonomiesInNamespace(ctx context.Context, namespace string) ([]TaxonomyNode, error)
	ListPrerequisites(ctx context.Context, nodeID uuid.UUID) ([]TaxonomyNode, error)
	GetTaxonomyPath(ctx context.Context, targetCode *string, namespace *string) ([]TaxonomyNode, error)
}
