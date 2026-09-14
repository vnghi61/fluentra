package service_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/speaking/domain"
	"github.com/fluentra/fluentra/internal/modules/speaking/service"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type mockStorageStore struct {
	presignPutFn func(
		ctx context.Context, bucket, key, contentType string, maxBytes int64, expiry time.Duration,
	) (storage.UploadIntent, error)
	deleteFn    func(ctx context.Context, bucket, key string) error
	getFn       func(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	statErr     error
	deletedKeys []string
}

func (m *mockStorageStore) PresignPut(
	ctx context.Context, bucket, key, contentType string, maxBytes int64, expiry time.Duration,
) (storage.UploadIntent, error) {
	if m.presignPutFn != nil {
		return m.presignPutFn(ctx, bucket, key, contentType, maxBytes, expiry)
	}
	return storage.UploadIntent{
		URL:       "https://storage.local/" + bucket + "/" + key,
		ObjectKey: key,
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

func (m *mockStorageStore) PresignGet(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "", nil
}

func (m *mockStorageStore) Stat(_ context.Context, _, key string) (storage.ObjectStat, error) {
	if m.statErr != nil {
		return storage.ObjectStat{}, m.statErr
	}
	return storage.ObjectStat{Key: key, Size: 1}, nil
}

func (m *mockStorageStore) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	if m.getFn != nil {
		return m.getFn(ctx, bucket, key)
	}
	return io.NopCloser(strings.NewReader("audio-bytes")), nil
}

func (m *mockStorageStore) Put(_ context.Context, _, _ string, _ io.Reader, _ int64, _ string) error {
	return nil
}

func (m *mockStorageStore) Copy(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (m *mockStorageStore) Delete(ctx context.Context, bucket, key string) error {
	m.deletedKeys = append(m.deletedKeys, key)
	if m.deleteFn != nil {
		return m.deleteFn(ctx, bucket, key)
	}
	return nil
}

func (m *mockStorageStore) VerifyUpload(_ context.Context, _, _ string, _ string, _ int64) (storage.ObjectStat, error) {
	return storage.ObjectStat{}, nil
}

type fakeAttemptCounter struct {
	count int
	err   error
}

func (f *fakeAttemptCounter) CountAttemptsTowardLimitSince(
	_ context.Context, _ uuid.UUID, _ string, _ time.Time,
) (int, error) {
	return f.count, f.err
}

func TestUploadIntent_Success(t *testing.T) {
	userID := uuid.New()
	mockStore := &mockStorageStore{}
	counter := &fakeAttemptCounter{count: 5}

	svc := service.New(service.Deps{
		Storage:    mockStore,
		Counter:    counter,
		DailyLimit: 30,
		Clock:      clock.NewFake(time.Now()),
	})

	res, err := svc.UploadIntent(context.Background(), userID, "audio/webm;codecs=opus")
	require.NoError(t, err)
	assert.NotEmpty(t, res.UploadURL)
	assert.Contains(t, res.ObjectKey, "recordings/"+userID.String()+"/")
	assert.True(t, strings.HasSuffix(res.ObjectKey, ".webm"))
	assert.Equal(t, 5, res.DailyRecordingsUsed)
	assert.Equal(t, 30, res.DailyRecordingsLimit)
}

func TestUploadIntent_InvalidFormat(t *testing.T) {
	svc := service.New(service.Deps{})
	_, err := svc.UploadIntent(context.Background(), uuid.New(), "image/png")
	assert.ErrorIs(t, err, domain.ErrUnsupportedAudioFormat)
}

func TestUploadIntent_DailyLimitExceeded(t *testing.T) {
	counter := &fakeAttemptCounter{count: 30}
	svc := service.New(service.Deps{
		Counter:    counter,
		DailyLimit: 30,
		Clock:      clock.NewFake(time.Now()),
	})

	_, err := svc.UploadIntent(context.Background(), uuid.New(), "audio/webm")
	assert.ErrorIs(t, err, domain.ErrDailyLimitReached)
}
