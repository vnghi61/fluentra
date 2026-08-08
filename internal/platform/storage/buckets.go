package storage

// The buckets this system owns. Each has its own lifecycle and access policy, so
// objects are never mixed: avatars are user-replaceable, media is content, and
// exports are generated artefacts that expire.
const (
	BucketAvatars = "fluentra-avatars"
	BucketMedia   = "fluentra-media"
	BucketExports = "fluentra-exports"
)

// DefaultBuckets returns the list of buckets managed by the system.
func DefaultBuckets() []string {
	return []string{
		BucketAvatars,
		BucketMedia,
		BucketExports,
	}
}
