package storage

import (
	"context"
	"fmt"
	"net/http"
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
		// Content-Type is signed into the URL rather than left off it.
		//
		// PresignedPutObject signs only `host`, and a browser that then sends
		// Content-Type is sending a header the signature does not cover. AWS S3
		// ignores such a header; R2 refuses the request with 403, which is how
		// every avatar upload against R2 failed. Dropping the header instead
		// would move the failure rather than fix it: the object would land as
		// application/octet-stream, and VerifyUpload rejects an object whose
		// stored type disagrees with its bytes.
		//
		// Signing it is the better half of the trade anyway. The POST policy on
		// MinIO makes the store enforce the declared type; signing the header
		// here makes R2 enforce it too, so the client can no longer swap the
		// type after the intent was issued. It cannot lie about the *bytes* —
		// that is still VerifyUpload's job, on both paths.
		extra := http.Header{}
		extra.Set("Content-Type", contentType)
		uploadURL, err := s.client.PresignHeader(
			ctx, http.MethodPut, bucket, key, expiry, nil, extra)
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
