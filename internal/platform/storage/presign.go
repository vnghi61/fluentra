package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
)

// PresignPut issues a browser-uploadable form that S3 itself enforces.
//
// It deliberately does not return a presigned PUT URL. A presigned PUT
// constrains only the bucket, the key and the deadline: the client may send any
// content type and any number of bytes, and the server finds out afterwards. A
// POST policy carries the content type and a length range as signed
// conditions, so MinIO rejects a mismatch before a single byte lands.
func (s *MinIOStore) PresignPut(
	ctx context.Context, bucket, key, contentType string, maxBytes int64, expiry time.Duration,
) (UploadIntent, error) {
	if expiry <= 0 {
		expiry = DefaultPresignPutExpiry
	}
	if contentType == "" {
		return UploadIntent{}, fmt.Errorf("presign put: a content type is required to constrain the upload")
	}
	if maxBytes <= 0 {
		return UploadIntent{}, fmt.Errorf("presign put: a positive max size is required to constrain the upload")
	}

	expiresAt := time.Now().UTC().Add(expiry)

	policy := minio.NewPostPolicy()
	if err := policy.SetBucket(bucket); err != nil {
		return UploadIntent{}, fmt.Errorf("presign put: set bucket: %w", err)
	}
	if err := policy.SetKey(key); err != nil {
		return UploadIntent{}, fmt.Errorf("presign put: set key: %w", err)
	}
	if err := policy.SetExpires(expiresAt); err != nil {
		return UploadIntent{}, fmt.Errorf("presign put: set expiry: %w", err)
	}
	if err := policy.SetContentType(contentType); err != nil {
		return UploadIntent{}, fmt.Errorf("presign put: set content type: %w", err)
	}
	// A one-byte floor rejects the empty upload that an interrupted client
	// otherwise leaves behind as a valid-looking object.
	if err := policy.SetContentLengthRange(1, maxBytes); err != nil {
		return UploadIntent{}, fmt.Errorf("presign put: set size range: %w", err)
	}

	ctx, span := tracer.Start(ctx, "storage.PresignPut")
	defer span.End()

	uploadURL, formData, err := s.client.PresignedPostPolicy(ctx, policy)
	if err != nil {
		return UploadIntent{}, fmt.Errorf("presign put: %w", err)
	}

	return UploadIntent{
		URL:         uploadURL.String(),
		Method:      "POST",
		FormData:    formData,
		FileField:   "file",
		ObjectKey:   key,
		ExpiresAt:   expiresAt,
		MaxBytes:    maxBytes,
		ContentType: contentType,
	}, nil
}

// PresignGet generates a presigned GET URL for viewing or downloading.
func (s *MinIOStore) PresignGet(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		expiry = DefaultPresignGetExpiry
	}
	ctx, span := tracer.Start(ctx, "storage.PresignGet")
	defer span.End()

	presignedURL, err := s.client.PresignedGetObject(ctx, bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("generate presigned get: %w", err)
	}
	return presignedURL.String(), nil
}
