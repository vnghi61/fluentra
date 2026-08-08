package storage

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// safeSegment is what may appear in one path segment of an object key.
// Anything else — a slash, a dot-dot, a control character — could move the
// object outside the prefix its owner is confined to.
var safeSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ErrUnsafeKeySegment is returned when a key component could escape its prefix.
var ErrUnsafeKeySegment = fmt.Errorf("storage: key segment contains characters that could escape its prefix")

// BuildKey constructs a deterministic object key:
//
//	{owner_type}/{owner_id}/{yyyy}/{mm}/{asset_id}.{ext}
//
// Every segment is validated rather than cleaned. `filepath.Clean` is the wrong
// tool here twice over: it rewrites separators per operating system, so the
// same inputs would produce a different key on Windows, and it resolves `..`
// instead of rejecting it, which silently moves the object somewhere else.
func BuildKey(ownerType, ownerID string, at time.Time, assetID, ext string) (string, error) {
	ext = strings.TrimPrefix(ext, ".")
	if ext == "" {
		ext = "bin"
	}
	for name, segment := range map[string]string{
		"owner type": ownerType,
		"owner id":   ownerID,
		"asset id":   assetID,
		"extension":  ext,
	} {
		if !safeSegment.MatchString(segment) || segment == "." || segment == ".." {
			return "", fmt.Errorf("%w: %s %q", ErrUnsafeKeySegment, name, segment)
		}
	}

	utc := at.UTC()
	return fmt.Sprintf("%s/%s/%04d/%02d/%s.%s", ownerType, ownerID, utc.Year(), utc.Month(), assetID, ext), nil
}
