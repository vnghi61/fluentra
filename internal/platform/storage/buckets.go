package storage

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
