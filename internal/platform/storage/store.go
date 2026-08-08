package storage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

var (
	ErrObjectNotFound     = errors.New("storage: object not found")
	ErrSizeMismatch       = errors.New("storage: size exceeds maximum permitted")
	ErrContentTypeMismatch = errors.New("storage: content type mismatch")
)

const (
	DefaultPresignPutExpiry = 5 * time.Minute
	DefaultPresignGetExpiry = 15 * time.Minute
)

// UploadIntent holds presigned upload details.
type UploadIntent struct {
	URL         string        `json:"upload_url"`
	ObjectKey   string        `json:"object_key"`
	ExpiresAt   time.Time     `json:"expires_at"`
	MaxBytes    int64         `json:"max_bytes"`
	ContentType string        `json:"content_type"`
}

// ObjectStat holds metadata for an object in storage.
type ObjectStat struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type"`
	ETag         string    `json:"etag"`
	LastModified time.Time `json:"last_modified"`
}

// Store defines object storage capabilities over MinIO / S3.
type Store interface {
	PresignPut(ctx context.Context, bucket, key, contentType string, maxBytes int64, expiry time.Duration) (UploadIntent, error)
	PresignGet(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	Stat(ctx context.Context, bucket, key string) (ObjectStat, error)
	Copy(ctx context.Context, srcBucket, srcKey, destBucket, destKey string) error
	Delete(ctx context.Context, bucket, key string) error
	VerifyUpload(ctx context.Context, bucket, key, expectedContentType string, maxBytes int64) (ObjectStat, error)
}

// MinIOStore implements Store facade using minio-go/v7.
type MinIOStore struct {
	client *minio.Client
}

// NewMinIOStore creates a new storage facade instance.
func NewMinIOStore(client *minio.Client) *MinIOStore {
	return &MinIOStore{client: client}
}

// PresignPut generates a 5-minute pinned presigned PUT URL.
func (s *MinIOStore) PresignPut(ctx context.Context, bucket, key, contentType string, maxBytes int64, expiry time.Duration) (UploadIntent, error) {
	if expiry <= 0 {
		expiry = DefaultPresignPutExpiry
	}
	reqParams := make(url.Values)
	if contentType != "" {
		reqParams.Set("response-content-type", contentType)
	}

	presignedURL, err := s.client.PresignedPutObject(ctx, bucket, key, expiry)
	if err != nil {
		return UploadIntent{}, fmt.Errorf("generate presigned put: %w", err)
	}

	expiresAt := time.Now().UTC().Add(expiry)
	return UploadIntent{
		URL:         presignedURL.String(),
		ObjectKey:   key,
		ExpiresAt:   expiresAt,
		MaxBytes:    maxBytes,
		ContentType: contentType,
	}, nil
}

// PresignGet generates a presigned GET URL for viewing/downloading an object.
func (s *MinIOStore) PresignGet(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		expiry = DefaultPresignGetExpiry
	}
	presignedURL, err := s.client.PresignedGetObject(ctx, bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("generate presigned get: %w", err)
	}
	return presignedURL.String(), nil
}

// Stat retrieves object metadata.
func (s *MinIOStore) Stat(ctx context.Context, bucket, key string) (ObjectStat, error) {
	info, err := s.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" || errResp.StatusCode == http.StatusNotFound {
			return ObjectStat{}, ErrObjectNotFound
		}
		return ObjectStat{}, fmt.Errorf("stat object: %w", err)
	}
	return ObjectStat{
		Key:          info.Key,
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		LastModified: info.LastModified,
	}, nil
}

// Copy copies an object from one bucket/key to another.
func (s *MinIOStore) Copy(ctx context.Context, srcBucket, srcKey, destBucket, destKey string) error {
	src := minio.CopySrcOptions{Bucket: srcBucket, Object: srcKey}
	dest := minio.CopyDestOptions{Bucket: destBucket, Object: destKey}
	_, err := s.client.CopyObject(ctx, dest, src)
	if err != nil {
		return fmt.Errorf("copy object: %w", err)
	}
	return nil
}

// Delete removes an object from storage.
func (s *MinIOStore) Delete(ctx context.Context, bucket, key string) error {
	err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("remove object: %w", err)
	}
	return nil
}

// VerifyUpload ensures uploaded object exists, size <= maxBytes, and content-type matches expected.
func (s *MinIOStore) VerifyUpload(ctx context.Context, bucket, key, expectedContentType string, maxBytes int64) (ObjectStat, error) {
	stat, err := s.Stat(ctx, bucket, key)
	if err != nil {
		return ObjectStat{}, err
	}
	if maxBytes > 0 && stat.Size > maxBytes {
		return stat, ErrSizeMismatch
	}
	if expectedContentType != "" {
		if !strings.EqualFold(stat.ContentType, expectedContentType) && !strings.HasPrefix(stat.ContentType, expectedContentType) {
			return stat, ErrContentTypeMismatch
		}
	}
	return stat, nil
}
