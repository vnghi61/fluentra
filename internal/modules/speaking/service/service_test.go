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

	"github.com/fluentra/fluentra/internal/modules/speaking/contract"
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

// fakeConsent answers the consent gate without a database.
type fakeConsent struct {
	consented bool
	err       error
}

func (f *fakeConsent) GetConsent(
	_ context.Context, _ uuid.UUID,
) (*contract.SpeakingConsent, error) {
	if f.err != nil {
		return nil, f.err
	}
	at := time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC)
	if !f.consented {
		return &contract.SpeakingConsent{Consented: false}, nil
	}
	return &contract.SpeakingConsent{Consented: true, ConsentedAt: &at}, nil
}

func (f *fakeConsent) RecordConsent(
	_ context.Context, _ uuid.UUID,
) (*contract.SpeakingConsent, error) {
	at := time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC)
	return &contract.SpeakingConsent{Consented: true, ConsentedAt: &at}, nil
}

func TestUploadIntent_Success(t *testing.T) {
	userID := uuid.New()
	mockStore := &mockStorageStore{}
	counter := &fakeAttemptCounter{count: 5}

	svc := service.New(service.Deps{
		Storage:    mockStore,
		Counter:    counter,
		Consent:    &fakeConsent{consented: true},
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
		Consent:    &fakeConsent{consented: true},
		DailyLimit: 30,
		Clock:      clock.NewFake(time.Now()),
	})

	_, err := svc.UploadIntent(context.Background(), uuid.New(), "audio/webm")
	assert.ErrorIs(t, err, domain.ErrDailyLimitReached)
}

// TestUploadIntent_ConsentRequired covers BR-SPEAKING-03 at the only place that
// can enforce it. The consent screen lived in the browser, so the endpoint
// answered anyone who called it directly — which is every caller that is not the
// app.
func TestUploadIntent_ConsentRequired(t *testing.T) {
	svc := service.New(service.Deps{
		Storage:    &mockStorageStore{},
		Counter:    &fakeAttemptCounter{},
		Consent:    &fakeConsent{consented: false},
		DailyLimit: 30,
		Clock:      clock.NewFake(time.Now()),
	})

	_, err := svc.UploadIntent(context.Background(), uuid.New(), "audio/webm")
	assert.ErrorIs(t, err, domain.ErrConsentRequired)
}

// A service with no way to check consent has not obtained it. Failing open here
// would make the rule depend on wiring nobody checks.
func TestUploadIntent_NoConsentStoreFailsClosed(t *testing.T) {
	svc := service.New(service.Deps{
		Storage:    &mockStorageStore{},
		DailyLimit: 30,
		Clock:      clock.NewFake(time.Now()),
	})

	_, err := svc.UploadIntent(context.Background(), uuid.New(), "audio/webm")
	assert.ErrorIs(t, err, domain.ErrConsentRequired)
}

func TestRecordConsent_ReportsTheTimestamp(t *testing.T) {
	svc := service.New(service.Deps{Consent: &fakeConsent{}})

	consent, err := svc.RecordConsent(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.True(t, consent.Consented)
	require.NotNil(t, consent.ConsentedAt, "BR-SPEAKING-03 asks for the timestamp, not just the fact")
}
