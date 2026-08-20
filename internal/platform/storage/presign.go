package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
)

// PresignPut issues a browser-uploadable upload target that the store itself
// enforces.
//
// On S3-compatible stores that support it (MinIO, AWS S3) this returns a
// presigned POST policy: the policy carries the content type and a length range
// as signed conditions, so the store rejects a mismatch before a single byte
// lands. Cloudflare R2 does not implement S3 POST policies and only accepts a
// presigned PUT URL, so when the store is configured with post_policy disabled
// (see NewMinIOStoreNoPostPolicy) a presigned PUT is returned instead and
// enforcement of size and type is deferred to VerifyUpload.
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

	// Stores that do not support S3 POST policy (Cloudflare R2) receive a
	// presigned PUT URL instead; MinIO and AWS S3 use the POST policy below.
	if !s.usePostPolicy {
		uploadURL, err := s.client.PresignedPutObject(ctx, bucket, key, expiry)
		if err != nil {
			return UploadIntent{}, fmt.Errorf("presign put (no post policy): %w", err)
		}
		return UploadIntent{
			URL:         uploadURL.String(),
			Method:      "PUT",
			FormData:    nil,
			FileField:   "",
			ObjectKey:   key,
			ExpiresAt:   expiresAt,
			MaxBytes:    maxBytes,
			ContentType: contentType,
		}, nil
	}

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
