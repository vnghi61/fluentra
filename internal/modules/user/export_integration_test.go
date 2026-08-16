//go:build integration

package user_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/fluentra/fluentra/internal/modules/user"
	userjob "github.com/fluentra/fluentra/internal/modules/user/job"
	"github.com/fluentra/fluentra/internal/platform/mailer"
	"github.com/fluentra/fluentra/internal/platform/storage"
)

type recordingMailer struct {
	sent []mailer.Message
}

func (m *recordingMailer) Send(_ context.Context, msg mailer.Message) error {
	m.sent = append(m.sent, msg)
	return nil
}

type staticExportable struct {
	data map[string]interface{}
}

func (s *staticExportable) ExportUserData(_ context.Context, _ string) (map[string]interface{}, error) {
	return s.data, nil
}

func TestModule_ExportLifecycle(t *testing.T) {
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	const reset = `TRUNCATE core.users CASCADE; TRUNCATE ops.outbox_events; TRUNCATE core.user_exports CASCADE`
	if _, err := pool.Exec(context.Background(), reset); err != nil {
		t.Fatalf("reset tables: %v", err)
	}

	mailerMock := &recordingMailer{}
	store := newTestStorage(t)

	userModule := user.New(user.Deps{
		Pool:    pool,
		Storage: store,
		Mailer:  mailerMock,
		Providers: []user.NamedExportable{
			{Name: "auth", Provider: &staticExportable{data: map[string]interface{}{"sessions": []string{}}}},
			{Name: "rbac", Provider: &staticExportable{data: map[string]interface{}{"roles": []string{"user"}}}},
			{Name: "audit", Provider: &staticExportable{data: map[string]interface{}{"logs": []string{}}}},
		},
	})
	router := chi.NewRouter()
	router.Route("/api/v1", func(api chi.Router) { userModule.Routes(api) })

	actorID := register(t, userModule, "export-user@fluentra.test", "Export Learner")
	exportID := requestInitialExport(t, router, actorID)

	// Duplicate request while pending must return 409 Conflict
	dupRec := request(t, router, http.MethodPost, "/api/v1/me/export", actorID, "")
	if dupRec.Code != http.StatusConflict {
		t.Fatalf("duplicate POST /me/export status = %d, want 409 (body %s)", dupRec.Code, dupRec.Body)
	}

	processExportJob(t, userModule, exportID, actorID)
	verifyCompletedStatus(t, router, exportID, actorID)
	verifyExportEmail(t, mailerMock)
	verifyStoredZip(t, store, exportID)
	expireAndRunCleanup(t, userModule, exportID)
}

func requestInitialExport(t *testing.T, router http.Handler, actorID uuid.UUID) uuid.UUID {
	t.Helper()
	exportRec := request(t, router, http.MethodPost, "/api/v1/me/export", actorID, "")
	if exportRec.Code != http.StatusAccepted {
		t.Fatalf("POST /me/export status = %d, want 202 (body %s)", exportRec.Code, exportRec.Body)
	}

	var exportResp struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
	}
	if err := json.Unmarshal(exportRec.Body.Bytes(), &exportResp); err != nil {
		t.Fatalf("decode export response: %v", err)
	}
	if exportResp.Status != "pending" {
		t.Errorf("status = %q, want 'pending'", exportResp.Status)
	}
	return exportResp.ID
}

func processExportJob(t *testing.T, userModule *user.Module, exportID, actorID uuid.UUID) {
	t.Helper()
	worker := userModule.ExportWorker()
	job := &river.Job[userjob.ExportArgs]{
		Args: userjob.ExportArgs{
			ExportID: exportID,
			UserID:   actorID,
		},
	}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("worker.Work: %v", err)
	}
}

func verifyCompletedStatus(t *testing.T, router http.Handler, exportID, actorID uuid.UUID) {
	t.Helper()
	getRec := request(t, router, http.MethodGet, fmt.Sprintf("/api/v1/me/export/%s", exportID), actorID, "")
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /me/export/{id} status = %d, want 200 (body %s)", getRec.Code, getRec.Body)
	}
	var getResp struct {
		ID        uuid.UUID  `json:"id"`
		Status    string     `json:"status"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode get export response: %v", err)
	}
	if getResp.Status != "completed" {
		t.Errorf("get status = %q, want 'completed'", getResp.Status)
	}
	if getResp.ExpiresAt == nil {
		t.Error("expected non-nil expires_at")
	}
}

func verifyExportEmail(t *testing.T, mailerMock *recordingMailer) {
	t.Helper()
	if len(mailerMock.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(mailerMock.sent))
	}
	if mailerMock.sent[0].To != "export-user@fluentra.test" {
		t.Errorf("email recipient = %q, want export-user@fluentra.test", mailerMock.sent[0].To)
	}
}

func verifyStoredZip(t *testing.T, store storage.Store, exportID uuid.UUID) {
	t.Helper()
	var objectKey string
	const selectKeySQL = `SELECT object_key FROM core.user_exports WHERE id = $1`
	if err := pool.QueryRow(context.Background(), selectKeySQL, exportID).Scan(&objectKey); err != nil {
		t.Fatalf("query object_key: %v", err)
	}
	reader, err := store.Get(context.Background(), storage.BucketExports, objectKey)
	if err != nil {
		t.Fatalf("get zip from storage: %v", err)
	}
	defer func() { _ = reader.Close() }()
	zipBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("parse zip: %v", err)
	}
	if len(zr.File) < 4 {
		t.Errorf("expected at least 4 files in zip, got %d", len(zr.File))
	}
}

func expireAndRunCleanup(t *testing.T, userModule *user.Module, exportID uuid.UUID) {
	t.Helper()
	const expireSQL = `UPDATE core.user_exports SET expires_at = now() - interval '1 hour' WHERE id = $1`
	if _, err := pool.Exec(context.Background(), expireSQL, exportID); err != nil {
		t.Fatalf("expire export: %v", err)
	}

	cronJobs := userModule.CronJobs()
	if len(cronJobs) == 0 {
		t.Fatal("expected user module cron jobs, got 0")
	}
	if err := cronJobs[0].Task(context.Background()); err != nil {
		t.Fatalf("cron cleanup handler: %v", err)
	}

	var remainingCount int
	const selectCountSQL = `SELECT count(*) FROM core.user_exports WHERE id = $1`
	if err := pool.QueryRow(context.Background(), selectCountSQL, exportID).Scan(&remainingCount); err != nil {
		t.Fatalf("query remaining exports: %v", err)
	}
	if remainingCount != 0 {
		t.Errorf("expected export record to be cleaned up, found %d rows", remainingCount)
	}
}
