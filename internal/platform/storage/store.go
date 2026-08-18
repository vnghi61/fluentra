package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	// ErrObjectNotFound reports that the key does not exist in the bucket.
	ErrObjectNotFound = errors.New("storage: object not found")
	// ErrSizeMismatch reports that the stored object exceeds the size the upload
	// intent was signed for.
	ErrSizeMismatch = errors.New("storage: size exceeds maximum permitted")
	// ErrContentTypeMismatch reports that the object's sniffed bytes disagree with
	// the content type the uploader declared.
	ErrContentTypeMismatch = errors.New("storage: content type mismatch")

	tracer                            = otel.Tracer("fluentra.platform.storage")
	meter                             = otel.Meter("fluentra.platform.storage")
	storageOperationDurationHistogram metric.Float64Histogram
	storageBytesCounter               metric.Int64Counter
)

func init() {
	var err error
	storageOperationDurationHistogram, err = meter.Float64Histogram(
		"storage_operation_duration_seconds",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of storage operations"),
	)
	if err != nil {
		slog.Error("failed to create storage_operation_duration_seconds metric", "error", err)
	}
	storageBytesCounter, err = meter.Int64Counter(
		"storage_bytes_total",
		metric.WithUnit("By"),
		metric.WithDescription("Total bytes transferred in storage operations"),
	)
	if err != nil {
		slog.Error("failed to create storage_bytes_total metric", "error", err)
	}
}

const (
	// DefaultPresignPutExpiry is the pinned lifetime of an upload intent. Writes
	// get a short window because a leaked upload URL is a write primitive.
	DefaultPresignPutExpiry = 5 * time.Minute
	// DefaultPresignGetExpiry is the lifetime of a download URL.
	DefaultPresignGetExpiry = 15 * time.Minute

	// sniffLength is what net/http.DetectContentType inspects.
	sniffLength = 512
)

// UploadIntent is everything a client needs to perform one constrained upload.
type UploadIntent struct {
	URL    string `json:"upload_url"`
	Method string `json:"method"`
	// FormData holds the signed policy fields. They must be sent as multipart
	// form fields, before the file part.
	FormData    map[string]string `json:"form_data"`
	FileField   string            `json:"file_field"`
	ObjectKey   string            `json:"object_key"`
	ExpiresAt   time.Time         `json:"expires_at"`
	MaxBytes    int64             `json:"max_bytes"`
	ContentType string            `json:"content_type"`
}

// ObjectStat holds metadata for an object in storage.
type ObjectStat struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type"`
	ETag         string    `json:"etag"`
	LastModified time.Time `json:"last_modified"`
	// SniffedContentType is what the bytes actually are, as opposed to what the
	// uploader declared.
	SniffedContentType string `json:"sniffed_content_type"`
}

// Store defines object storage capabilities over MinIO / S3.
type Store interface {
	PresignPut(
		ctx context.Context, bucket, key, contentType string, maxBytes int64, expiry time.Duration,
	) (UploadIntent, error)
	PresignGet(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	Stat(ctx context.Context, bucket, key string) (ObjectStat, error)
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	Put(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error
	Copy(ctx context.Context, srcBucket, srcKey, destBucket, destKey string) error
	Delete(ctx context.Context, bucket, key string) error
	VerifyUpload(ctx context.Context, bucket, key, expectedContentType string, maxBytes int64) (ObjectStat, error)
}

// MinIOStore implements Store using minio-go/v7.
type MinIOStore struct {
	client *minio.Client
}

// NewMinIOStore creates a new storage facade instance.
func NewMinIOStore(client *minio.Client) *MinIOStore { return &MinIOStore{client: client} }

// Get retrieves an object stream from storage. The caller must close the stream.
func (s *MinIOStore) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	start := time.Now()
	defer func() {
		dur := time.Since(start).Seconds()
		if storageOperationDurationHistogram != nil {
			storageOperationDurationHistogram.Record(ctx, dur, metric.WithAttributes(
				attribute.String("op", "get"), attribute.String("bucket", bucket)))
		}
	}()

	ctx, span := tracer.Start(ctx, "storage.Get")
	defer span.End()

	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	return obj, nil
}

// Put writes an object directly to storage.
func (s *MinIOStore) Put(
	ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string,
) error {
	start := time.Now()
	defer func() {
		dur := time.Since(start).Seconds()
		if storageOperationDurationHistogram != nil {
			storageOperationDurationHistogram.Record(ctx, dur, metric.WithAttributes(
				attribute.String("op", "put"), attribute.String("bucket", bucket)))
		}
	}()

	ctx, span := tracer.Start(ctx, "storage.Put")
	defer span.End()

	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}
	if _, err := s.client.PutObject(ctx, bucket, key, reader, size, opts); err != nil {
		return fmt.Errorf("put object: %w", err)
	}

	if storageBytesCounter != nil && size > 0 {
		storageBytesCounter.Add(ctx, size, metric.WithAttributes(
			attribute.String("direction", "upload"), attribute.String("bucket", bucket)))
	}
	return nil
}

// Stat retrieves object metadata as reported by the server.
func (s *MinIOStore) Stat(ctx context.Context, bucket, key string) (ObjectStat, error) {
	ctx, span := tracer.Start(ctx, "storage.Stat")
	defer span.End()

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
	ctx, span := tracer.Start(ctx, "storage.Copy")
	defer span.End()

	src := minio.CopySrcOptions{Bucket: srcBucket, Object: srcKey}
	dest := minio.CopyDestOptions{Bucket: destBucket, Object: destKey}
	if _, err := s.client.CopyObject(ctx, dest, src); err != nil {
		return fmt.Errorf("copy object: %w", err)
	}
	return nil
}

// Delete removes an object from storage.
func (s *MinIOStore) Delete(ctx context.Context, bucket, key string) error {
	ctx, span := tracer.Start(ctx, "storage.Delete")
	defer span.End()

	if err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove object: %w", err)
	}
	return nil
}

// VerifyUpload checks what actually landed.
//
// The presigned policy constrains what the client said it would send. Only
// reading the object back tells you what it sent: `Content-Type` is a header
// the uploader chose, so a renamed executable arrives labelled `image/png`.
// This sniffs the leading bytes and compares them with what was declared.
func (s *MinIOStore) VerifyUpload(
	ctx context.Context, bucket, key, expectedContentType string, maxBytes int64,
) (ObjectStat, error) {
	ctx, span := tracer.Start(ctx, "storage.VerifyUpload")
	defer span.End()

	stat, err := s.Stat(ctx, bucket, key)
	if err != nil {
		return ObjectStat{}, err
	}
	if maxBytes > 0 && stat.Size > maxBytes {
		return stat, ErrSizeMismatch
	}

	sniffed, err := s.sniff(ctx, bucket, key)
	if err != nil {
		return stat, err
	}
	stat.SniffedContentType = sniffed

	if expectedContentType != "" && !sameMediaType(sniffed, expectedContentType) {
		return stat, fmt.Errorf("%w: expected %q, bytes are %q", ErrContentTypeMismatch, expectedContentType, sniffed)
	}
	// A disagreement between the stored header and the bytes is a lie whether
	// or not the caller stated an expectation.
	if stat.ContentType != "" && !sameMediaType(sniffed, stat.ContentType) {
		return stat, fmt.Errorf("%w: header says %q, bytes are %q", ErrContentTypeMismatch, stat.ContentType, sniffed)
	}
	return stat, nil
}

// sniff reads the leading bytes of an object and classifies them.
func (s *MinIOStore) sniff(ctx context.Context, bucket, key string) (string, error) {
	object, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("open object for sniffing: %w", err)
	}
	defer func() { _ = object.Close() }()

	header := make([]byte, sniffLength)
	read, err := io.ReadFull(object, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", fmt.Errorf("read object for sniffing: %w", err)
	}
	return http.DetectContentType(header[:read]), nil
}

// sameMediaType compares media types ignoring parameters such as `; charset=`.
func sameMediaType(a, b string) bool { return strings.EqualFold(mediaType(a), mediaType(b)) }

func mediaType(value string) string {
	base, _, _ := strings.Cut(value, ";")
	return strings.TrimSpace(base)
}
