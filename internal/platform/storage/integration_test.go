//go:build integration

package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// contentTypePNG is the type these uploads claim; the point of the suite is
// that claiming it is not the same as being it.
const contentTypePNG = "image/png"

const testBucket = "fluentra-test-uploads"

// pngBytes is a one-pixel PNG: a real magic-byte header for sniffing.
var pngBytes = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 'I', 'D', 'A', 'T',
	0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05,
	0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00,
	0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
}

// windowsExecutable starts with the MZ header DetectContentType recognises.
var windowsExecutable = append([]byte("MZ\x90\x00\x03\x00\x00\x00"), bytes.Repeat([]byte{0x00}, 64)...)

func newTestStore(t *testing.T) (*storage.MinIOStore, *minio.Client) {
	t.Helper()
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_S3_ENDPOINT is not set")
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("TEST_S3_ACCESS_KEY"), os.Getenv("TEST_S3_SECRET_KEY"), ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, testBucket)
	if err != nil {
		t.Fatalf("bucket exists: %v", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, testBucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatalf("make bucket: %v", err)
		}
	}
	return storage.NewMinIOStore(client), client
}

// postUpload performs the multipart POST a browser would, honouring the signed
// policy fields exactly.
func postUpload(t *testing.T, intent storage.UploadIntent, contentType string, body []byte) int {
	t.Helper()
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for field, value := range intent.FormData {
		if err := writer.WriteField(field, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	// Overriding Content-Type here is the attack: the policy must still win.
	if contentType != "" {
		_ = writer.WriteField("Content-Type", contentType)
	}
	part, err := writer.CreateFormFile(intent.FileField, "upload")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, intent.URL, &buffer)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode
}

// TestPresignPut_MinIORejectsMismatchedContentType is the P0.9 acceptance. The
// rejection must come from MinIO, not from our code: a presigned PUT could not
// deliver it at all.
func TestPresignPut_MinIORejectsMismatchedContentType(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	intent, err := store.PresignPut(ctx, testBucket, "probe/mismatched-type.png", "image/png", 1<<20, time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}

	status := postUpload(t, intent, "application/octet-stream", pngBytes)
	if status < 400 {
		t.Fatalf("MinIO accepted an upload declaring a content type the policy did not sign: status %d", status)
	}
}

func TestPresignPut_MinIORejectsOversizedBody(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	intent, err := store.PresignPut(ctx, testBucket, "probe/oversized.png", "image/png", 64, time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}

	status := postUpload(t, intent, "", bytes.Repeat([]byte{0x89}, 4096))
	if status < 400 {
		t.Fatalf("MinIO accepted a body larger than the signed length range: status %d", status)
	}
}

func TestPresignPut_AcceptsAConformingUpload(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	key := "probe/conforming.png"

	intent, err := store.PresignPut(ctx, testBucket, key, "image/png", 1<<20, time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if status := postUpload(t, intent, "", pngBytes); status >= 400 {
		t.Fatalf("a conforming upload was rejected: status %d", status)
	}

	stat, err := store.VerifyUpload(ctx, testBucket, key, contentTypePNG, 1<<20)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if stat.SniffedContentType != contentTypePNG {
		t.Errorf("sniffed = %q, want image/png", stat.SniffedContentType)
	}
	t.Cleanup(func() { _ = store.Delete(context.Background(), testBucket, key) })
}

// TestVerifyUpload_CatchesARenamedExecutable is the trap from the card: the
// policy constrains what the client claims, so only sniffing catches this.
func TestVerifyUpload_CatchesARenamedExecutable(t *testing.T) {
	store, client := newTestStore(t)
	ctx := context.Background()
	key := "probe/not-really.png"

	// Upload out of band with a lying Content-Type header, exactly as a client
	// that had a valid policy for image/png could do with its file bytes.
	_, err := client.PutObject(ctx, testBucket, key,
		bytes.NewReader(windowsExecutable), int64(len(windowsExecutable)),
		minio.PutObjectOptions{ContentType: contentTypePNG})
	if err != nil {
		t.Fatalf("put object: %v", err)
	}
	t.Cleanup(func() { _ = store.Delete(context.Background(), testBucket, key) })

	_, err = store.VerifyUpload(ctx, testBucket, key, contentTypePNG, 1<<20)
	if !errors.Is(err, storage.ErrContentTypeMismatch) {
		t.Fatalf("VerifyUpload error = %v, want ErrContentTypeMismatch", err)
	}
}

func TestVerifyUpload_CatchesMissingObjectAndOversize(t *testing.T) {
	store, client := newTestStore(t)
	ctx := context.Background()

	_, err := store.VerifyUpload(ctx, testBucket, "probe/does-not-exist.png", contentTypePNG, 1<<20)
	if !errors.Is(err, storage.ErrObjectNotFound) {
		t.Errorf("missing object error = %v, want ErrObjectNotFound", err)
	}

	key := "probe/too-big.png"
	_, err = client.PutObject(ctx, testBucket, key, bytes.NewReader(pngBytes), int64(len(pngBytes)),
		minio.PutObjectOptions{ContentType: contentTypePNG})
	if err != nil {
		t.Fatalf("put object: %v", err)
	}
	t.Cleanup(func() { _ = store.Delete(context.Background(), testBucket, key) })

	_, err = store.VerifyUpload(ctx, testBucket, key, contentTypePNG, 8)
	if !errors.Is(err, storage.ErrSizeMismatch) {
		t.Errorf("oversize error = %v, want ErrSizeMismatch", err)
	}
}

func TestPresignPut_RefusesUnconstrainedIntent(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if _, err := store.PresignPut(ctx, testBucket, "probe/x.png", "", 1<<20, time.Minute); err == nil {
		t.Error("expected a missing content type to be refused")
	}
	if _, err := store.PresignPut(ctx, testBucket, "probe/x.png", "image/png", 0, time.Minute); err == nil {
		t.Error("expected a missing size limit to be refused")
	}
}
