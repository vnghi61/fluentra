package domain

import (
	"time"

	"github.com/google/uuid"
)

// MediaStatus represents the lifecycle of a stored media asset.
type MediaStatus string

// MediaStatus constants.
const (
	// MediaStatusPending indicates media upload/processing is in progress.
	MediaStatusPending MediaStatus = "pending"
	// MediaStatusReady indicates media is ready and verified.
	MediaStatusReady MediaStatus = "ready"
	// MediaStatusFailed indicates media processing failed.
	MediaStatusFailed MediaStatus = "failed"
)

// MediaAsset represents a media file attached to learning materials.
type MediaAsset struct {
	ID         uuid.UUID
	ObjectKey  string
	Kind       string
	DurationMS *int32
	Checksum   *string
	Status     MediaStatus
	ByteSize   *int64
	MIMEType   *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
