package domain

import (
	"strings"
	"time"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

const (
	// AvatarMaxBytes is the 5 MB ceiling for avatar uploads.
	AvatarMaxBytes int64 = 5 * 1024 * 1024

	// AvatarUploadExpiry is how long an avatar presigned upload policy remains valid.
	AvatarUploadExpiry = 5 * time.Minute

	// DefaultAvatarContentType is the fallback upload type when none is specified.
	DefaultAvatarContentType = "image/jpeg"
)

var allowedAvatarContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// IsAllowedAvatarContentType checks whether the given MIME type is permitted for avatars.
func IsAllowedAvatarContentType(contentType string) bool {
	base, _, _ := strings.Cut(contentType, ";")
	return allowedAvatarContentTypes[strings.TrimSpace(strings.ToLower(base))]
}

// ValidateAvatarContentType ensures the requested avatar MIME type is supported.
func ValidateAvatarContentType(contentType string) error {
	if contentType == "" {
		return invalid("content_type", "REQUIRED", "Content type is required.")
	}
	if !IsAllowedAvatarContentType(contentType) {
		return invalid("content_type", "UNSUPPORTED_MEDIA_TYPE",
			"Avatar must be image/jpeg, image/png, or image/webp.")
	}
	return nil
}

// ErrAvatarVerificationFailed indicates the uploaded raw avatar failed size or type verification.
var ErrAvatarVerificationFailed = apperr.New(
	apperr.Validation, "UPLOAD_VERIFICATION_FAILED", "The uploaded avatar image failed verification.",
)

// ErrAvatarProcessingFailed indicates the avatar image decoding or resizing failed.
var ErrAvatarProcessingFailed = apperr.New(
	apperr.Validation, "AVATAR_PROCESSING_FAILED", "Could not process or re-encode the uploaded avatar image.",
)

// UploadIntent holds the upload instructions returned to the client.
type UploadIntent struct {
	URL         string
	Method      string
	FormData    map[string]string
	FileField   string
	ObjectKey   string
	ExpiresAt   time.Time
	MaxBytes    int64
	ContentType string
}
