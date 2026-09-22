package contract

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Resource represents a stored intake asset (file or URL) belonging to a learner.
type Resource struct {
	ID               uuid.UUID       `json:"id"`
	UserID           uuid.UUID       `json:"user_id"`
	Kind             string          `json:"kind"`
	Title            string          `json:"title"`
	ObjectKey        *string         `json:"object_key,omitempty"`
	OriginalFilename string          `json:"original_filename"`
	DeclaredMIME     string          `json:"declared_mime"`
	DetectedMIME     string          `json:"detected_mime"`
	ByteSize         *int64          `json:"byte_size,omitempty"`
	Checksum         *string         `json:"checksum,omitempty"`
	SourceURL        *string         `json:"source_url,omitempty"`
	Status           string          `json:"status"`
	FailureReason    string          `json:"failure_reason"`
	DownloadURL      *string         `json:"download_url,omitempty"`
	Renditions       []Rendition     `json:"renditions,omitempty"`
	Extraction       *Extraction     `json:"extraction,omitempty"`
	Classification   *Classification `json:"classification,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	ValidatedAt      *time.Time      `json:"validated_at,omitempty"`
}

// Extraction represents extracted text from a document or audio/video transcription.
type Extraction struct {
	ResourceID  uuid.UUID `json:"resource_id"`
	Source      string    `json:"source"`
	Text        string    `json:"text,omitempty"`
	CharCount   int       `json:"char_count"`
	Truncated   bool      `json:"truncated"`
	Language    string    `json:"language"`
	ToolVersion string    `json:"tool_version"`
	Excerpt     string    `json:"excerpt,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ClassificationNode represents an attached spine taxonomy node.
type ClassificationNode struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// Classification represents pedagogical tagging of extracted resource text.
type Classification struct {
	ResourceID    uuid.UUID            `json:"resource_id"`
	CEFREstimate  *string              `json:"cefr_estimate,omitempty"`
	Skill         *string              `json:"skill,omitempty"`
	NodeCodes     []string             `json:"node_codes"`
	Nodes         []ClassificationNode `json:"nodes,omitempty"`
	PromptVersion string               `json:"prompt_version"`
	Model         string               `json:"model"`
	AIRequestID   *uuid.UUID           `json:"ai_request_id,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
}

// Rendition represents a processed thumbnail, display image, preview, or web media.
type Rendition struct {
	ID            uuid.UUID  `json:"id"`
	ResourceID    uuid.UUID  `json:"resource_id"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	ObjectKey     *string    `json:"object_key,omitempty"`
	MIMEType      string     `json:"mime_type"`
	Width         *int       `json:"width,omitempty"`
	Height        *int       `json:"height,omitempty"`
	DurationMS    *int       `json:"duration_ms,omitempty"`
	ByteSize      *int64     `json:"byte_size,omitempty"`
	ToolVersion   string     `json:"tool_version"`
	Attempts      int        `json:"attempts"`
	URL           *string    `json:"url,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	FailureReason string     `json:"failure_reason,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// UploadIntentResult holds the presigned S3 PUT instruction for client uploads.
type UploadIntentResult struct {
	ID        uuid.UUID `json:"id"`
	UploadURL string    `json:"upload_url"`
	ObjectKey string    `json:"object_key"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SubmitResult indicates acceptance of an intake submission.
type SubmitResult struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}

// ResourceList represents a paginated collection of resources.
type ResourceList struct {
	Items    []Resource `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// ResourceReader provides read-only access to a learner's resources.
type ResourceReader interface {
	GetResource(ctx context.Context, id, userID uuid.UUID) (*Resource, error)
}

// Material is a narrow view of a validated file resource for publication:
// its detected MIME, status, the original object key and every rendition with
// its own status. No presigned URLs: a publisher copies the bytes, it does not
// hand them to a browser.
type Material struct {
	ID           uuid.UUID   `json:"id"`
	UserID       uuid.UUID   `json:"user_id"`
	DetectedMIME string      `json:"detected_mime"`
	Status       string      `json:"status"`
	ObjectKey    string      `json:"object_key,omitempty"`
	ByteSize     int64       `json:"byte_size,omitempty"`
	Renditions   []Rendition `json:"renditions"`
}

// PublishedObjects holds the fluentra-media keys a published course material
// landed at. The content body stores these keys, never URLs (Gate 1 refuses a
// URL in any activity body).
type PublishedObjects struct {
	Original  string  `json:"original"`
	Poster    *string `json:"poster,omitempty"`
	Video360p *string `json:"video_360p,omitempty"`
	Video720p *string `json:"video_720p,omitempty"`
	Preview   *string `json:"preview,omitempty"`
	AudioWeb  *string `json:"audio_web,omitempty"`
}

// MaterialPublisher is the narrow read-and-copy surface a course publisher
// uses. It reads one resource for its owner, and it copies a resource's objects
// into the course's own storage. Deliberately not the resource lifecycle: a
// publisher can neither write nor delete a learner's resource.
type MaterialPublisher interface {
	// MaterialForOwner returns the caller-owned resource as a material, or the
	// same not-found a foreign resource answers (BR-RESOURCE-01).
	MaterialForOwner(ctx context.Context, ownerID, resourceID uuid.UUID) (*Material, error)
	// CopyForPublication copies the original and every ready rendition into
	// fluentra-media under destPrefix. Idempotent: re-running writes the same
	// destination keys.
	CopyForPublication(ctx context.Context, resourceID uuid.UUID, destPrefix string) (*PublishedObjects, error)
}

// Lifecycle values a publisher reasons about, exported so callers do not carry
// their own copies of the strings.
const (
	// MaterialValidated is the resource status a publishable material must hold.
	MaterialValidated = "validated"
	// RenditionReady is the rendition status the runner needs before a material
	// may be published.
	RenditionReady = "ready"
)
