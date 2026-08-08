package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// BuildKey constructs a deterministic object key adhering to:
// {owner_type}/{owner_id}/{yyyy}/{mm}/{asset_id}.{ext}
func BuildKey(ownerType, ownerID string, t time.Time, assetID, ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	if ext == "" {
		ext = "bin"
	}
	utc := t.UTC()
	return fmt.Sprintf("%s/%s/%04d/%02d/%s.%s",
		filepath.Clean(ownerType),
		filepath.Clean(ownerID),
		utc.Year(),
		utc.Month(),
		filepath.Clean(assetID),
		ext,
	)
}
