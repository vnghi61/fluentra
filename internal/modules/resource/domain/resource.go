package domain

import (
	"time"
)

// Supported resource kinds.
const (
	KindFile = "file"
	KindURL  = "url"
)

// Supported resource lifecycle statuses.
const (
	StatusPending   = "pending"
	StatusUploaded  = "uploaded"
	StatusValidated = "validated"
	StatusRejected  = "rejected"
	StatusFailed    = "failed"
)

// MaxResourceBytes is the maximum allowed byte size for a single uploaded resource (50 MB per D17-5).
const MaxResourceBytes int64 = 50 * 1024 * 1024

// MaxUserResources is the maximum number of resources a single user may hold (BR-RESOURCE-08).
const MaxUserResources int64 = 50

// MaxUserTotalBytes is the maximum total storage allocated per user (250 MB per BR-RESOURCE-08).
const MaxUserTotalBytes int64 = 250 * 1024 * 1024

// Presign and TTL timeouts.
const (
	PresignPutExpiry time.Duration = 5 * time.Minute
	PresignGetExpiry time.Duration = 15 * time.Minute
	PendingIntentTTL time.Duration = 15 * time.Minute
	// StuckUploadTTL is how long a confirmed upload may sit unvalidated before
	// the sweeper gives up on it. Comfortably past River's three attempts with
	// backoff, so a slow run is never swept out from under itself.
	StuckUploadTTL time.Duration = time.Hour
)

// IsValidKind reports whether the resource kind is supported.
func IsValidKind(kind string) bool {
	return kind == KindFile || kind == KindURL
}

// IsValidStatus reports whether the status is a valid lifecycle state.
func IsValidStatus(status string) bool {
	switch status {
	case StatusPending, StatusUploaded, StatusValidated, StatusRejected, StatusFailed:
		return true
	default:
		return false
	}
}

// CanTransition validates whether moving from 'from' to 'to' is allowed by the lifecycle state machine.
func CanTransition(from, to string) bool {
	switch from {
	case StatusPending:
		return to == StatusUploaded || to == StatusFailed
	case StatusUploaded:
		return to == StatusValidated || to == StatusRejected || to == StatusFailed
	default:
		return false
	}
}
