package service_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/domain"
	"github.com/fluentra/fluentra/internal/modules/resource/service"
	"github.com/fluentra/fluentra/internal/platform/storage"
)

type mockRepo struct {
	resources  map[uuid.UUID]*contract.Resource
	usageCount int64
	usageBytes int64
	// staleExpired, when set, is what ListExpiredPendingResources returns: the
	// snapshot a sweeper read before the learner confirmed.
	staleExpired []contract.Resource
}

func newMockRepo() *mockRepo {
	return &mockRepo{resources: make(map[uuid.UUID]*contract.Resource)}
}

func (m *mockRepo) CreateFileResourceIntent(
	_ context.Context, id, userID uuid.UUID, title string, objectKey *string, originalFilename, declaredMime string,
) (*contract.Resource, error) {
	r := &contract.Resource{
		ID:               id,
		UserID:           userID,
		Kind:             domain.KindFile,
		Title:            title,
		ObjectKey:        objectKey,
		OriginalFilename: originalFilename,
		DeclaredMIME:     declaredMime,
		Status:           domain.StatusPending,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	m.resources[id] = r
	return r, nil
}

func (m *mockRepo) CreateURLResource(
	_ context.Context, id, userID uuid.UUID, title, sourceURL string,
) (*contract.Resource, error) {
	r := &contract.Resource{
		ID:        id,
		UserID:    userID,
		Kind:      domain.KindURL,
		Title:     title,
		SourceURL: &sourceURL,
		Status:    domain.StatusUploaded,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.resources[id] = r
	return r, nil
}

func (m *mockRepo) GetResourceByID(_ context.Context, id uuid.UUID) (*contract.Resource, error) {
	r, ok := m.resources[id]
	if !ok {
		return nil, domain.ErrResourceNotFound
	}
	return r, nil
}

func (m *mockRepo) GetResourceByIDAndUser(_ context.Context, id, userID uuid.UUID) (*contract.Resource, error) {
	r, ok := m.resources[id]
	if !ok || r.UserID != userID {
		return nil, domain.ErrResourceNotFound
	}
	return r, nil
}

func (m *mockRepo) ListResourcesByUser(
	_ context.Context, userID uuid.UUID, _, _ *string, _, _ int32,
) ([]contract.Resource, int, error) {
	var list []contract.Resource
	for _, r := range m.resources {
		if r.UserID == userID {
			list = append(list, *r)
		}
	}
	return list, len(list), nil
}

func (m *mockRepo) GetUserResourceUsage(_ context.Context, _ uuid.UUID) (int64, int64, error) {
	return m.usageCount, m.usageBytes, nil
}

func (m *mockRepo) ConfirmFileResource(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error) {
	r, err := m.GetResourceByIDAndUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	r.Status = domain.StatusUploaded
	return r, nil
}

func (m *mockRepo) UpdateValidationSuccess(
	_ context.Context, id uuid.UUID, title, detectedMime string, byteSize int64, checksum string,
) (*contract.Resource, error) {
	r, ok := m.resources[id]
	if !ok || r.Status != domain.StatusUploaded {
		return nil, domain.ErrResourceStateChanged
	}
	r.Status = domain.StatusValidated
	r.Title = title
	r.DetectedMIME = detectedMime
	r.ByteSize = &byteSize
	r.Checksum = &checksum
	now := time.Now()
	r.ValidatedAt = &now
	return r, nil
}

func (m *mockRepo) UpdateValidationRejected(
	_ context.Context, id uuid.UUID, failureReason string, detectedMime *string,
) (*contract.Resource, error) {
	r, ok := m.resources[id]
	if !ok || r.Status != domain.StatusUploaded {
		return nil, domain.ErrResourceStateChanged
	}
	r.Status = domain.StatusRejected
	r.FailureReason = failureReason
	if detectedMime != nil {
		r.DetectedMIME = *detectedMime
	}
	return r, nil
}

func (m *mockRepo) UpdateValidationFailed(
	_ context.Context, id uuid.UUID, failureReason, fromStatus string,
) (*contract.Resource, error) {
	r, ok := m.resources[id]
	if !ok || r.Status != fromStatus {
		return nil, domain.ErrResourceStateChanged
	}
	r.Status = domain.StatusFailed
	r.FailureReason = failureReason
	return r, nil
}

func (m *mockRepo) DeleteResource(ctx context.Context, id, userID uuid.UUID) (*string, string, error) {
	r, err := m.GetResourceByIDAndUser(ctx, id, userID)
	if err != nil {
		return nil, "", err
	}
	delete(m.resources, id)
	return r.ObjectKey, r.Kind, nil
}

func (m *mockRepo) ListExpiredPendingResources(
	_ context.Context, before time.Time, _ int32,
) ([]contract.Resource, error) {
	if m.staleExpired != nil {
		return m.staleExpired, nil
	}
	var list []contract.Resource
	for _, r := range m.resources {
		if r.Status == domain.StatusPending && r.CreatedAt.Before(before) {
			list = append(list, *r)
		}
	}
	return list, nil
}

func (m *mockRepo) ListStuckUploadedResources(
	_ context.Context, before time.Time, _ int32,
) ([]contract.Resource, error) {
	var list []contract.Resource
	for _, r := range m.resources {
		if r.Status == domain.StatusUploaded && r.UpdatedAt.Before(before) {
			list = append(list, *r)
		}
	}
	return list, nil
}

func (m *mockRepo) ConfirmFileResourceTx(
	ctx context.Context, _ pgx.Tx, id, userID uuid.UUID,
) (*contract.Resource, error) {
	return m.ConfirmFileResource(ctx, id, userID)
}

func (m *mockRepo) CreateURLResourceTx(
	ctx context.Context, _ pgx.Tx, id, userID uuid.UUID, title, sourceURL string,
) (*contract.Resource, error) {
	return m.CreateURLResource(ctx, id, userID, title, sourceURL)
}

type mockStorage struct {
	objects   map[string][]byte
	deleteErr error
}

func newMockStorage() *mockStorage {
	return &mockStorage{objects: make(map[string][]byte)}
}

func (s *mockStorage) PresignPut(
	_ context.Context, bucket, key, _ string, _ int64, expiry time.Duration,
) (storage.UploadIntent, error) {
	return storage.UploadIntent{
		URL:       "https://storage.local/" + bucket + "/" + key,
		ObjectKey: key,
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

func (s *mockStorage) PresignGet(
	_ context.Context, bucket, key string, _ time.Duration,
) (string, error) {
	return "https://storage.local/download/" + bucket + "/" + key, nil
}

func (s *mockStorage) Stat(_ context.Context, _, key string) (storage.ObjectStat, error) {
	data, ok := s.objects[key]
	if !ok {
		return storage.ObjectStat{}, errors.New("object not found")
	}
	return storage.ObjectStat{
		Key:  key,
		Size: int64(len(data)),
	}, nil
}

func (s *mockStorage) Get(_ context.Context, _, key string) (io.ReadCloser, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, errors.New("object not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *mockStorage) Put(
	_ context.Context, _, key string, reader io.Reader, _ int64, _ string,
) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.objects[key] = data
	return nil
}

func (s *mockStorage) Copy(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (s *mockStorage) Delete(_ context.Context, _, key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.objects, key)
	return nil
}

func (s *mockStorage) VerifyUpload(
	ctx context.Context, bucket, key, _ string, _ int64,
) (storage.ObjectStat, error) {
	return s.Stat(ctx, bucket, key)
}

func TestService_CreateUploadIntent_QuotaExceeded(t *testing.T) {
	repo := newMockRepo()
	repo.usageCount = domain.MaxUserResources
	store := newMockStorage()

	svc := service.New(repo, store, nil, nil, nil, nil)
	userID := uuid.New()

	_, err := svc.CreateUploadIntent(context.Background(), userID, "test.pdf", "application/pdf")
	if err == nil {
		t.Fatalf("expected quota error, got nil")
	}
	if !errors.Is(err, domain.ErrResourceQuotaExceeded) {
		t.Errorf("expected ErrResourceQuotaExceeded, got %v", err)
	}
}

func TestService_ConfirmUpload_UnownedReturns404(t *testing.T) {
	repo := newMockRepo()
	store := newMockStorage()
	svc := service.New(repo, store, nil, nil, nil, nil)

	user1 := uuid.New()
	user2 := uuid.New()

	intent, err := svc.CreateUploadIntent(context.Background(), user1, "file.pdf", "application/pdf")
	if err != nil {
		t.Fatalf("create intent failed: %v", err)
	}

	// User2 tries to confirm User1's resource -> must return 404 (BR-RESOURCE-01)
	_, err = svc.ConfirmUpload(context.Background(), user2, intent.ID)
	if err == nil {
		t.Fatalf("expected 404 error, got nil")
	}
	if !errors.Is(err, domain.ErrResourceNotFound) {
		t.Errorf("expected ErrResourceNotFound, got %v", err)
	}
}

func TestService_ConfirmUpload_MissingObjectReturns409(t *testing.T) {
	repo := newMockRepo()
	store := newMockStorage()
	svc := service.New(repo, store, nil, nil, nil, nil)

	user1 := uuid.New()

	intent, err := svc.CreateUploadIntent(context.Background(), user1, "file.pdf", "application/pdf")
	if err != nil {
		t.Fatalf("create intent failed: %v", err)
	}

	// Object was not put in storage
	_, err = svc.ConfirmUpload(context.Background(), user1, intent.ID)
	if err == nil {
		t.Fatalf("expected 409 error, got nil")
	}
	if !errors.Is(err, domain.ErrResourceNotUploaded) {
		t.Errorf("expected ErrResourceNotUploaded, got %v", err)
	}
}

func TestService_GetResource_UnownedReturns404(t *testing.T) {
	repo := newMockRepo()
	store := newMockStorage()
	svc := service.New(repo, store, nil, nil, nil, nil)

	user1 := uuid.New()
	user2 := uuid.New()

	intent, _ := svc.CreateUploadIntent(context.Background(), user1, "file.pdf", "application/pdf")

	_, err := svc.GetResource(context.Background(), intent.ID, user2)
	if err == nil {
		t.Fatalf("expected 404, got nil")
	}
	if !errors.Is(err, domain.ErrResourceNotFound) {
		t.Errorf("expected ErrResourceNotFound, got %v", err)
	}
}

func TestService_DeleteResource_DeletesStorageObject(t *testing.T) {
	repo := newMockRepo()
	store := newMockStorage()
	svc := service.New(repo, store, nil, nil, nil, nil)

	user1 := uuid.New()
	intent, _ := svc.CreateUploadIntent(context.Background(), user1, "file.pdf", "application/pdf")

	// Store dummy object
	store.objects[intent.ObjectKey] = []byte("%PDF-1.4\ncontent")

	// Delete resource
	err := svc.DeleteResource(context.Background(), intent.ID, user1)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify object deleted from storage (BR-RESOURCE-06)
	if _, ok := store.objects[intent.ObjectKey]; ok {
		t.Errorf("expected storage object %q to be deleted", intent.ObjectKey)
	}

	// Verify row deleted from repo
	_, err = repo.GetResourceByID(context.Background(), intent.ID)
	if !errors.Is(err, domain.ErrResourceNotFound) {
		t.Errorf("expected row to be deleted from repo")
	}
}

func TestService_DeleteResource_StorageFailureLeavesRow(t *testing.T) {
	repo := newMockRepo()
	store := newMockStorage()
	store.deleteErr = errors.New("s3 connection timeout")
	svc := service.New(repo, store, nil, nil, nil, nil)

	user1 := uuid.New()
	intent, _ := svc.CreateUploadIntent(context.Background(), user1, "file.pdf", "application/pdf")
	store.objects[intent.ObjectKey] = []byte("%PDF-1.4\ncontent")

	err := svc.DeleteResource(context.Background(), intent.ID, user1)
	if err == nil {
		t.Fatalf("expected error when storage delete fails, got nil")
	}

	// Verify row was left intact (BR-RESOURCE-06)
	row, getErr := repo.GetResourceByID(context.Background(), intent.ID)
	if getErr != nil || row == nil {
		t.Errorf("row should have been kept in database when storage fails")
	}
}

func TestService_ValidateResource_SniffDisagreementRejects(t *testing.T) {
	repo := newMockRepo()
	store := newMockStorage()
	svc := service.New(repo, store, nil, nil, nil, nil)

	user1 := uuid.New()
	intent, _ := svc.CreateUploadIntent(context.Background(), user1, "photo.png", "image/png")

	// But user uploaded a PDF file!
	store.objects[intent.ObjectKey] = []byte("%PDF-1.4\nnot a png")

	// Mark uploaded manually for validate test
	repo.resources[intent.ID].Status = domain.StatusUploaded

	err := svc.ValidateResource(context.Background(), intent.ID)
	if err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	res, _ := repo.GetResourceByID(context.Background(), intent.ID)
	if res.Status != domain.StatusRejected {
		t.Errorf("expected status %s, got %s", domain.StatusRejected, res.Status)
	}
	// The learner reads a fixed sentence. The sniffer's detail - what it found
	// and why - goes to the log, not to someone probing it.
	if res.FailureReason != domain.ReasonTypeNotSupported {
		t.Errorf("failure reason = %q, want the fixed learner-facing sentence", res.FailureReason)
	}
}

func TestService_ValidateResource_ValidPDFSucceeds(t *testing.T) {
	repo := newMockRepo()
	store := newMockStorage()
	svc := service.New(repo, store, nil, nil, nil, nil)

	user1 := uuid.New()
	intent, _ := svc.CreateUploadIntent(context.Background(), user1, "lecture.pdf", "application/pdf")
	store.objects[intent.ObjectKey] = []byte("%PDF-1.4\nvalid pdf content")
	repo.resources[intent.ID].Status = domain.StatusUploaded

	err := svc.ValidateResource(context.Background(), intent.ID)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	res, _ := repo.GetResourceByID(context.Background(), intent.ID)
	if res.Status != domain.StatusValidated {
		t.Errorf("expected validated, got %s (reason: %s)", res.Status, res.FailureReason)
	}
	if res.DetectedMIME != domain.MIMEPDF {
		t.Errorf("expected detected mime %s, got %s", domain.MIMEPDF, res.DetectedMIME)
	}
	if res.Checksum == nil || *res.Checksum == "" {
		t.Errorf("expected checksum to be set")
	}
}

func TestService_SweepExpiredPending(t *testing.T) {
	repo := newMockRepo()
	store := newMockStorage()
	svc := service.New(repo, store, nil, nil, nil, nil)

	user1 := uuid.New()
	intent, _ := svc.CreateUploadIntent(context.Background(), user1, "forgotten.pdf", "application/pdf")

	// Backdate the row past PendingIntentTTL (e.g. 30 minutes ago)
	repo.resources[intent.ID].CreatedAt = time.Now().Add(-30 * time.Minute)
	store.objects[intent.ObjectKey] = []byte("orphaned")

	swept, err := svc.SweepExpiredPending(context.Background())
	if err != nil {
		t.Fatalf("sweep failed: %v", err)
	}
	if swept != 1 {
		t.Errorf("expected 1 swept, got %d", swept)
	}

	// Check orphaned object deleted from storage
	if _, ok := store.objects[intent.ObjectKey]; ok {
		t.Errorf("expected orphaned storage object to be deleted")
	}

	// Check row marked failed
	res, _ := repo.GetResourceByID(context.Background(), intent.ID)
	if res.Status != domain.StatusFailed {
		t.Errorf("expected status %s, got %s", domain.StatusFailed, res.Status)
	}
}

// TestService_Sweep_SparesAnIntentConfirmedMeanwhile is the race the sweeper's
// order exists for. It read the row as pending; by the time it writes, the
// learner has confirmed. The row must stay uploaded and - the part that used to
// go wrong - its object must survive, because the old sweeper deleted first.
func TestService_Sweep_SparesAnIntentConfirmedMeanwhile(t *testing.T) {
	repo := newMockRepo()
	store := newMockStorage()
	svc := service.New(repo, store, nil, nil, nil, nil)

	intent, err := svc.CreateUploadIntent(context.Background(), uuid.New(), "late.pdf", "application/pdf")
	if err != nil {
		t.Fatalf("intent: %v", err)
	}
	store.objects[intent.ObjectKey] = []byte("%PDF-1.4 confirmed just in time")

	snapshot := *repo.resources[intent.ID] // what the sweeper saw: pending
	snapshot.CreatedAt = time.Now().Add(-time.Hour)
	repo.staleExpired = []contract.Resource{snapshot}
	repo.resources[intent.ID].Status = domain.StatusUploaded // confirmed since

	if _, err := svc.SweepExpiredPending(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := repo.resources[intent.ID].Status; got != domain.StatusUploaded {
		t.Errorf("status = %s, want uploaded: the sweeper overwrote a confirmed upload", got)
	}
	if _, ok := store.objects[intent.ObjectKey]; !ok {
		t.Error("the sweeper deleted the object of an upload the learner had confirmed")
	}
}

// TestService_Sweep_FailsStuckUploadsAndKeepsTheirObjects. A confirmed upload
// whose validation job gave up used to sit at 'uploaded' for ever.
func TestService_Sweep_FailsStuckUploadsAndKeepsTheirObjects(t *testing.T) {
	repo := newMockRepo()
	store := newMockStorage()
	svc := service.New(repo, store, nil, nil, nil, nil)

	intent, err := svc.CreateUploadIntent(context.Background(), uuid.New(), "stuck.pdf", "application/pdf")
	if err != nil {
		t.Fatalf("intent: %v", err)
	}
	store.objects[intent.ObjectKey] = []byte("%PDF-1.4")
	row := repo.resources[intent.ID]
	row.Status = domain.StatusUploaded
	row.UpdatedAt = time.Now().Add(-2 * domain.StuckUploadTTL)

	if _, err := svc.SweepExpiredPending(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if row.Status != domain.StatusFailed {
		t.Errorf("status = %s, want failed", row.Status)
	}
	if row.FailureReason != domain.ReasonValidationStuck {
		t.Errorf("reason = %q", row.FailureReason)
	}
	if _, ok := store.objects[intent.ObjectKey]; !ok {
		t.Error("a confirmed upload's object was deleted; deleting it is the learner's call")
	}
}

// TestService_ValidateResource_DeletedMeanwhileIsDone. The learner deleted the
// resource while its job was queued. Returning an error would have River retry
// a job that can never succeed.
func TestService_ValidateResource_DeletedMeanwhileIsDone(t *testing.T) {
	svc := service.New(newMockRepo(), newMockStorage(), nil, nil, nil, nil)
	if err := svc.ValidateResource(context.Background(), uuid.New()); err != nil {
		t.Fatalf("a job for a deleted resource should finish quietly, got %v", err)
	}
}

type stubFetcher struct {
	err error
}

func (f stubFetcher) FetchMetadata(_ context.Context, _ string) (*service.URLMetadata, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &service.URLMetadata{Title: "A page"}, nil
}

// TestService_ValidateURL_RejectedVersusFailed is the split §5 of the work
// order draws: 'rejected' is a verdict on the link, 'failed' is us. A network
// blip used to reject a learner's link permanently.
func TestService_ValidateURL_RejectedVersusFailed(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus string
		wantReason string
	}{
		{"remote server down", fmt.Errorf("%w: status 503", service.ErrURLUnreachable),
			domain.StatusFailed, domain.ReasonURLUnreachable},
		{"connection refused", fmt.Errorf("%w: dial tcp: refused", service.ErrURLUnreachable),
			domain.StatusFailed, domain.ReasonURLUnreachable},
		{"page not found", &service.URLStatusError{StatusCode: http.StatusNotFound},
			domain.StatusRejected, domain.ReasonURLStatus(http.StatusNotFound)},
		{"private address", fmt.Errorf("%w: host resolves to 10.1.2.3", domain.ErrResourceURLNotAllowed),
			domain.StatusRejected, domain.ReasonURLNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockRepo()
			svc := service.New(repo, newMockStorage(), nil, nil, stubFetcher{err: tc.err}, nil)

			id := uuid.New()
			src := "https://example.com/lesson"
			repo.resources[id] = &contract.Resource{
				ID: id, UserID: uuid.New(), Kind: domain.KindURL,
				SourceURL: &src, Status: domain.StatusUploaded,
			}

			if err := svc.ValidateResource(context.Background(), id); err != nil {
				t.Fatalf("validate: %v", err)
			}
			got := repo.resources[id]
			if got.Status != tc.wantStatus {
				t.Errorf("status = %s, want %s", got.Status, tc.wantStatus)
			}
			if got.FailureReason != tc.wantReason {
				t.Errorf("reason = %q, want %q", got.FailureReason, tc.wantReason)
			}
			if strings.Contains(got.FailureReason, "10.1.2.3") {
				t.Error("an internal address reached the learner")
			}
		})
	}
}
