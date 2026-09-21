package contract

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Resource represents a stored intake asset (file or URL) belonging to a learner.
type Resource struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	Kind             string     `json:"kind"`
	Title            string     `json:"title"`
	ObjectKey        *string    `json:"object_key,omitempty"`
	OriginalFilename string     `json:"original_filename"`
	DeclaredMIME     string     `json:"declared_mime"`
	DetectedMIME     string     `json:"detected_mime"`
	ByteSize         *int64     `json:"byte_size,omitempty"`
	Checksum         *string    `json:"checksum,omitempty"`
	SourceURL        *string    `json:"source_url,omitempty"`
	Status           string     `json:"status"`
	FailureReason    string     `json:"failure_reason"`
	DownloadURL      *string    `json:"download_url,omitempty"`
	Renditions       []Rendition `json:"renditions,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ValidatedAt      *time.Time `json:"validated_at,omitempty"`
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
