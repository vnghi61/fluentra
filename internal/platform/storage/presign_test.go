package storage_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/fluentra/fluentra/internal/platform/storage"
)

// r2Client is an offline client pointed at an R2-shaped endpoint. Presigning
// computes the signature locally, so none of these tests reach the network.
func r2Client(t *testing.T) *minio.Client {
	t.Helper()
	client, err := minio.New("accountid.r2.cloudflarestorage.com", &minio.Options{
		Creds:  credentials.NewStaticV4("accesskey", "secretkey", ""),
		Secure: true,
		Region: "auto",
	})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}
	return client
}

func signedHeaders(t *testing.T, raw string) []string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse presigned url: %v", err)
	}
	value := parsed.Query().Get("X-Amz-SignedHeaders")
	if value == "" {
		t.Fatalf("presigned url carries no X-Amz-SignedHeaders: %s", raw)
	}
	return strings.Split(value, ";")
}

// TestPresignPut_R2SignsContentType is the regression for the 403 every avatar
// upload against Cloudflare R2 returned.
//
// PresignedPutObject signs `host` and nothing else. The browser then PUT the
// file with a Content-Type header, R2 saw a header outside the signature and
// refused the request. Signing the header is what makes the upload the client
// actually sends the one the signature describes.
func TestPresignPut_R2SignsContentType(t *testing.T) {
	t.Parallel()
	store := storage.NewMinIOStoreNoPostPolicy(r2Client(t))

	intent, err := store.PresignPut(context.Background(),
		"fluentra-avatars", "users/abc/2026/08/def.raw", "image/png", 1<<20, 5*time.Minute)
	if err != nil {
		t.Fatalf("presign put: %v", err)
	}

	if intent.Method != "PUT" {
		t.Fatalf("method = %q, want PUT — R2 does not implement S3 POST policies", intent.Method)
	}

	var hasContentType bool
	for _, header := range signedHeaders(t, intent.URL) {
		if strings.EqualFold(header, "content-type") {
			hasContentType = true
		}
	}
	if !hasContentType {
		t.Errorf("X-Amz-SignedHeaders omits content-type; R2 will reject the upload with 403.\nurl: %s", intent.URL)
	}
	if intent.ContentType != "image/png" {
		t.Errorf("intent content type = %q, want image/png — the client must send exactly what was signed", intent.ContentType)
	}
}

// TestPresignPut_PostPolicyPathUnchanged guards the MinIO and AWS path: it must
// keep returning a POST policy, because that is what enforces the length range
// the presigned PUT cannot.
func TestPresignPut_PostPolicyPathUnchanged(t *testing.T) {
	t.Parallel()
	store := storage.NewMinIOStore(r2Client(t))

	intent, err := store.PresignPut(context.Background(),
		"fluentra-avatars", "users/abc/2026/08/def.raw", "image/png", 1<<20, 5*time.Minute)
	if err != nil {
		t.Fatalf("presign put: %v", err)
	}
	if intent.Method != "POST" {
		t.Fatalf("method = %q, want POST", intent.Method)
	}
	if intent.FileField != "file" {
		t.Errorf("file field = %q, want file", intent.FileField)
	}
	if intent.FormData["Content-Type"] != "image/png" {
		t.Errorf("policy did not carry the content type: %v", intent.FormData)
	}
}
