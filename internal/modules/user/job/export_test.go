package job_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	userjob "github.com/fluentra/fluentra/internal/modules/user/job"
	"github.com/fluentra/fluentra/internal/platform/mailer"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const (
	testUserEmail  = "user@example.com"
	testUserModule = "user"
)

type fakeJobRepo struct {
	mu      sync.Mutex
	exports map[uuid.UUID]domain.ExportRequest
}

func (f *fakeJobRepo) GetExportByID(_ context.Context, id uuid.UUID) (domain.ExportRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	req, ok := f.exports[id]
	if !ok {
		return domain.ExportRequest{}, domain.ErrUserNotFound
	}
	return req, nil
}

func (f *fakeJobRepo) UpdateExportStatus(
	_ context.Context,
	id uuid.UUID,
	status domain.ExportStatus,
	startedAt, completedAt, expiresAt *time.Time,
	objectKey, errorMessage *string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	req, ok := f.exports[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	req.Status = status
	if startedAt != nil {
		req.StartedAt = startedAt
	}
	if completedAt != nil {
		req.CompletedAt = completedAt
	}
	if expiresAt != nil {
		req.ExpiresAt = expiresAt
	}
	if objectKey != nil {
		req.ObjectKey = objectKey
	}
	if errorMessage != nil {
		req.ErrorMessage = errorMessage
	}
	f.exports[id] = req
	return nil
}

func (f *fakeJobRepo) GetExpiredExports(_ context.Context, limit int32) ([]domain.ExportRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.ExportRequest
	now := time.Now().UTC()
	for _, req := range f.exports {
		if req.Status == domain.ExportStatusCompleted && req.ExpiresAt != nil && req.ExpiresAt.Before(now) {
			result = append(result, req)
			if int64(len(result)) >= int64(limit) {
				break
			}
		}
	}
	return result, nil
}

func (f *fakeJobRepo) DeleteExport(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.exports, id)
	return nil
}

type fakeStorage struct {
	mu          sync.Mutex
	files       map[string][]byte
	deleted     []string
	presignURLs map[string]string
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{
		files:       map[string][]byte{},
		deleted:     []string{},
		presignURLs: map[string]string{},
	}
}

func (s *fakeStorage) Put(_ context.Context, bucket, key string, reader io.Reader, _ int64, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.files[bucket+"/"+key] = data
	return nil
}

func (s *fakeStorage) PresignGet(_ context.Context, bucket, key string, _ time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	url := "https://storage.local/" + bucket + "/" + key + "?signed=true"
	s.presignURLs[key] = url
	return url, nil
}

func (s *fakeStorage) Delete(_ context.Context, bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, bucket+"/"+key)
	delete(s.files, bucket+"/"+key)
	return nil
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []mailer.Message
}

func (m *fakeMailer) Send(_ context.Context, msg mailer.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

type fakeContactReader struct {
	contact contract.Contact
	err     error
}

func (c *fakeContactReader) Recipient(_ context.Context, _ uuid.UUID) (contract.Contact, error) {
	return c.contact, c.err
}

type fakeExportable struct {
	data map[string]interface{}
	err  error
}

func (e *fakeExportable) ExportUserData(_ context.Context, _ string) (map[string]interface{}, error) {
	return e.data, e.err
}

func TestExportWorker_Work_Success(t *testing.T) {
	t.Parallel()

	exportID := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def111111111")
	userID := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def222222222")
	fixedNow := time.Date(2026, time.August, 16, 10, 0, 0, 0, time.UTC)

	repo := &fakeJobRepo{
		exports: map[uuid.UUID]domain.ExportRequest{
			exportID: {
				ID:        exportID,
				UserID:    userID,
				Status:    domain.ExportStatusPending,
				CreatedAt: fixedNow,
			},
		},
	}

	storage := newFakeStorage()
	mailerMock := &fakeMailer{}
	contactReader := &fakeContactReader{
		contact: contract.Contact{
			Email:       testUserEmail,
			DisplayName: "Learner",
			Locale:      "en",
		},
	}

	providers := buildTestProviders()
	worker := userjob.NewExportWorker(userjob.ExportWorkerOptions{
		Repo:        repo,
		Storage:     storage,
		Mailer:      mailerMock,
		UserContact: contactReader,
		Providers:   providers,
		Clock:       clock.NewFake(fixedNow),
		Bucket:      "fluentra-exports",
		LinkTTL:     24 * time.Hour,
		Retention:   7 * 24 * time.Hour,
	})

	job := &river.Job[userjob.ExportArgs]{
		Args: userjob.ExportArgs{
			ExportID: exportID,
			UserID:   userID,
		},
	}

	ctx := context.Background()
	if err := worker.Work(ctx, job); err != nil {
		t.Fatalf("worker.Work failed: %v", err)
	}

	req := verifyExportRecord(ctx, t, repo, exportID, fixedNow)
	verifyZipArchive(t, storage, *req.ObjectKey, exportID)
	verifyEmailSent(t, mailerMock)
}

func buildTestProviders() []userjob.NamedExportable {
	return []userjob.NamedExportable{
		{
			Name: testUserModule,
			Provider: &fakeExportable{
				data: map[string]interface{}{"email": testUserEmail},
			},
		},
		{
			Name: "auth",
			Provider: &fakeExportable{
				data: map[string]interface{}{"sessions": []string{"sess_1"}},
			},
		},
		{
			Name: "rbac",
			Provider: &fakeExportable{
				data: map[string]interface{}{"roles": []string{"user"}},
			},
		},
		{
			Name: "audit",
			Provider: &fakeExportable{
				data: map[string]interface{}{"audit_logs": []string{}},
			},
		},
	}
}

func verifyExportRecord(
	ctx context.Context, t *testing.T, repo *fakeJobRepo, exportID uuid.UUID, fixedNow time.Time,
) domain.ExportRequest {
	t.Helper()
	req, err := repo.GetExportByID(ctx, exportID)
	if err != nil {
		t.Fatalf("get export record: %v", err)
	}
	if req.Status != domain.ExportStatusCompleted {
		t.Errorf("expected completed status, got: %v", req.Status)
	}
	if req.ObjectKey == nil || *req.ObjectKey == "" {
		t.Fatal("expected non-empty object key")
	}
	if req.ExpiresAt == nil || !req.ExpiresAt.Equal(fixedNow.Add(7*24*time.Hour)) {
		t.Errorf("unexpected expires_at: %v", req.ExpiresAt)
	}
	return req
}

func verifyZipArchive(t *testing.T, storage *fakeStorage, objectKey string, exportID uuid.UUID) {
	t.Helper()
	storedBytes, ok := storage.files["fluentra-exports/"+objectKey]
	if !ok {
		t.Fatalf("file not found in storage at %s", objectKey)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(storedBytes), int64(len(storedBytes)))
	if err != nil {
		t.Fatalf("invalid zip archive: %v", err)
	}

	foundEntries := make(map[string]bool)
	for _, f := range zipReader.File {
		foundEntries[f.Name] = true
		if f.Name == "metadata.json" {
			rc, openErr := f.Open()
			if openErr != nil {
				t.Fatalf("open metadata.json: %v", openErr)
			}
			metaBytes, _ := io.ReadAll(rc)
			_ = rc.Close()

			var meta map[string]interface{}
			if err := json.Unmarshal(metaBytes, &meta); err != nil {
				t.Fatalf("unmarshal metadata: %v", err)
			}
			if meta["export_id"] != exportID.String() {
				t.Errorf("expected export_id %s, got: %v", exportID, meta["export_id"])
			}
		}
	}

	expectedEntries := []string{"user.json", "auth.json", "rbac.json", "audit.json", "metadata.json"}
	for _, expected := range expectedEntries {
		if !foundEntries[expected] {
			t.Errorf("missing expected entry in zip: %s", expected)
		}
	}
}

func verifyEmailSent(t *testing.T, mailerMock *fakeMailer) {
	t.Helper()
	if len(mailerMock.sent) != 1 {
		t.Fatalf("expected 1 email sent, got: %d", len(mailerMock.sent))
	}
	emailMsg := mailerMock.sent[0]
	if emailMsg.To != testUserEmail {
		t.Errorf("expected email to %s, got: %s", testUserEmail, emailMsg.To)
	}
	if emailMsg.Template != "data_export" {
		t.Errorf("expected data_export template, got: %s", emailMsg.Template)
	}
}

func TestExportWorker_Idempotent(t *testing.T) {
	t.Parallel()

	exportID := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def333333333")
	userID := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def444444444")
	key := "users/export-333.zip"

	repo := &fakeJobRepo{
		exports: map[uuid.UUID]domain.ExportRequest{
			exportID: {
				ID:        exportID,
				UserID:    userID,
				Status:    domain.ExportStatusCompleted,
				ObjectKey: &key,
			},
		},
	}

	worker := userjob.NewExportWorker(userjob.ExportWorkerOptions{
		Repo: repo,
	})

	job := &river.Job[userjob.ExportArgs]{
		Args: userjob.ExportArgs{
			ExportID: exportID,
			UserID:   userID,
		},
	}

	ctx := context.Background()
	if err := worker.Work(ctx, job); err != nil {
		t.Fatalf("expected no error on idempotent retry, got: %v", err)
	}
}

func TestExportWorker_ProviderError(t *testing.T) {
	t.Parallel()

	exportID := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def555555555")
	userID := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def666666666")

	repo := &fakeJobRepo{
		exports: map[uuid.UUID]domain.ExportRequest{
			exportID: {
				ID:     exportID,
				UserID: userID,
				Status: domain.ExportStatusPending,
			},
		},
	}

	failingProviders := []userjob.NamedExportable{
		{
			Name: testUserModule,
			Provider: &fakeExportable{
				err: errors.New("database failure"),
			},
		},
	}

	worker := userjob.NewExportWorker(userjob.ExportWorkerOptions{
		Repo:      repo,
		Providers: failingProviders,
	})

	job := &river.Job[userjob.ExportArgs]{
		Args: userjob.ExportArgs{
			ExportID: exportID,
			UserID:   userID,
		},
	}

	ctx := context.Background()
	err := worker.Work(ctx, job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	req, _ := repo.GetExportByID(ctx, exportID)
	if req.Status != domain.ExportStatusFailed {
		t.Errorf("expected failed status, got: %v", req.Status)
	}
	if req.ErrorMessage == nil || *req.ErrorMessage == "" {
		t.Error("expected error message to be recorded")
	}
}

func TestExportCleaner_Run(t *testing.T) {
	t.Parallel()

	exportID := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def777777777")
	expiredTime := time.Now().UTC().Add(-24 * time.Hour)
	key := "users/export-777.zip"

	repo := &fakeJobRepo{
		exports: map[uuid.UUID]domain.ExportRequest{
			exportID: {
				ID:        exportID,
				Status:    domain.ExportStatusCompleted,
				ExpiresAt: &expiredTime,
				ObjectKey: &key,
			},
		},
	}

	storage := newFakeStorage()
	storage.files["fluentra-exports/"+key] = []byte("zip content")

	cleaner := userjob.NewExportCleaner(repo, storage, "fluentra-exports")

	ctx := context.Background()
	if err := cleaner.Run(ctx); err != nil {
		t.Fatalf("cleaner.Run failed: %v", err)
	}

	// 1. Storage file deleted
	if _, ok := storage.files["fluentra-exports/"+key]; ok {
		t.Error("expected storage file to be deleted")
	}

	// 2. DB record deleted
	if _, ok := repo.exports[exportID]; ok {
		t.Error("expected db export record to be deleted")
	}
}
