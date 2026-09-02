package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

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

// AvatarVariant is one of the three sizes storeProcessedAvatar writes.
//
// The serving URL carries an asset id and nothing else, so a variant has to
// come from somewhere. It comes from a query parameter with a default rather
// than from the path, because a caller that does not care -- which is every
// caller today -- should not have to name a size to get a picture.
type AvatarVariant string

// The variants, matching the Suffix values in AvatarSizes.
const (
	AvatarVariantSmall  AvatarVariant = "sm"
	AvatarVariantMedium AvatarVariant = "md"
	AvatarVariantLarge  AvatarVariant = "lg"

	// DefaultAvatarVariant is 128 px: large enough for a profile header, small
	// enough that a leaderboard of forty rows does not download 40 x 256 px.
	DefaultAvatarVariant = AvatarVariantMedium
)

// ParseAvatarVariant resolves a requested size, defaulting when none is given.
//
// An unknown value is refused rather than quietly served as the default. A
// client asking for "medium" or "256" has a bug, and answering it with a
// picture hides that bug for as long as nobody looks closely at the pixels.
func ParseAvatarVariant(value string) (AvatarVariant, error) {
	switch AvatarVariant(strings.TrimSpace(strings.ToLower(value))) {
	case "":
		return DefaultAvatarVariant, nil
	case AvatarVariantSmall:
		return AvatarVariantSmall, nil
	case AvatarVariantMedium:
		return AvatarVariantMedium, nil
	case AvatarVariantLarge:
		return AvatarVariantLarge, nil
	default:
		return "", invalid("size", "UNSUPPORTED_VALUE", "Avatar size must be sm, md, or lg.")
	}
}

// AvatarAsset is a stored avatar object: which file, of what type, how big.
type AvatarAsset struct {
	AssetID   uuid.UUID
	Variant   AvatarVariant
	UserID    uuid.UUID
	ObjectKey string
	MimeType  string
	ByteSize  int64
	CreatedAt time.Time
}

// ErrAvatarAssetNotFound is returned when no object backs the requested id.
//
// It is a 404 and it is honest: before core.avatar_assets existed, every
// avatar_url the API handed out pointed at a route that did not exist, and the
// browser got the same status for a reason nobody could act on.
var ErrAvatarAssetNotFound = apperr.New(
	apperr.NotFound, "AVATAR_NOT_FOUND", "The avatar was not found.",
)
