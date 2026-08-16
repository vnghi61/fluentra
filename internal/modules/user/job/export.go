// Package job implements asynchronous background workers for the user module.
package job

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/platform/mailer"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// ExportArgs are the River job parameters for a user data export.
type ExportArgs struct {
	ExportID uuid.UUID `json:"export_id"`
	UserID   uuid.UUID `json:"user_id"`
}

// Kind identifies this job type to River.
func (ExportArgs) Kind() string { return "user.data_export" }

// InsertOpts configures execution limits for the job.
func (ExportArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "default",
		MaxAttempts: 3,
	}
}

// ExportRepository is the data access interface the export worker needs.
type ExportRepository interface {
	GetExportByID(ctx context.Context, id uuid.UUID) (domain.ExportRequest, error)
	UpdateExportStatus(
		ctx context.Context,
		id uuid.UUID,
		status domain.ExportStatus,
		startedAt, completedAt, expiresAt *time.Time,
		objectKey, errorMessage *string,
	) error
}

// StorageStore defines object storage operations needed by the export worker.
type StorageStore interface {
	Put(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error
	PresignGet(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, bucket, key string) error
}

// Mailer defines the notification delivery interface.
type Mailer interface {
	Send(ctx context.Context, msg mailer.Message) error
}

// UserContactReader gets contact details for sending emails.
type UserContactReader interface {
	Recipient(ctx context.Context, userID uuid.UUID) (contract.Contact, error)
}

// NamedExportable binds a module name to its exportable contract.
type NamedExportable struct {
	Name     string
	Provider contract.Exportable
}

// ExportWorkerOptions holds collaborators for the ExportWorker.
type ExportWorkerOptions struct {
	Repo        ExportRepository
	Storage     StorageStore
	Mailer      Mailer
	UserContact UserContactReader
	Providers   []NamedExportable
	Clock       clock.Clock
	Bucket      string
	LinkTTL     time.Duration
	Retention   time.Duration
}

// ExportWorker executes the asynchronous GDPR data gathering and ZIP packaging.
type ExportWorker struct {
	river.WorkerDefaults[ExportArgs]
	repo        ExportRepository
	storage     StorageStore
	mailer      Mailer
	userContact UserContactReader
	providers   []NamedExportable
	clock       clock.Clock
	bucket      string
	linkTTL     time.Duration
	retention   time.Duration
}

// NewExportWorker constructs a new ExportWorker.
func NewExportWorker(opts ExportWorkerOptions) *ExportWorker {
	timekeeper := opts.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}
	bucket := opts.Bucket
	if bucket == "" {
		bucket = storage.BucketExports
	}
	linkTTL := opts.LinkTTL
	if linkTTL <= 0 {
		linkTTL = 24 * time.Hour
	}
	retention := opts.Retention
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}

	return &ExportWorker{
		repo:        opts.Repo,
		storage:     opts.Storage,
		mailer:      opts.Mailer,
		userContact: opts.UserContact,
		providers:   opts.Providers,
		clock:       timekeeper,
		bucket:      bucket,
		linkTTL:     linkTTL,
		retention:   retention,
	}
}

// Work processes the data export job (BR-JOB-02 / BR-USER-07).
func (w *ExportWorker) Work(ctx context.Context, job *river.Job[ExportArgs]) error {
	exportID := job.Args.ExportID
	userID := job.Args.UserID

	exportReq, err := w.repo.GetExportByID(ctx, exportID)
	if err != nil {
		return fmt.Errorf("get export record: %w", err)
	}

	// Idempotency: if already completed, nothing more to do.
	if exportReq.Status == domain.ExportStatusCompleted && exportReq.ObjectKey != nil {
		return nil
	}

	now := w.clock.Now()
	if err := w.repo.UpdateExportStatus(
		ctx, exportID, domain.ExportStatusProcessing, &now, nil, nil, nil, nil,
	); err != nil {
		return fmt.Errorf("set export status to processing: %w", err)
	}

	if err := w.processExport(ctx, exportID, userID, now); err != nil {
		errMsg := err.Error()
		_ = w.repo.UpdateExportStatus(ctx, exportID, domain.ExportStatusFailed, nil, nil, nil, nil, &errMsg)
		return err
	}

	return nil
}

func (w *ExportWorker) processExport(
	ctx context.Context, exportID, userID uuid.UUID, startedAt time.Time,
) error {
	zipBuffer, err := w.buildZipArchive(ctx, exportID, userID, startedAt)
	if err != nil {
		return err
	}

	objectKey, err := storage.BuildKey("user", userID.String(), startedAt, exportID.String(), "zip")
	if err != nil {
		return fmt.Errorf("build storage key: %w", err)
	}

	if err := w.uploadAndNotify(ctx, userID, objectKey, zipBuffer); err != nil {
		return err
	}

	completedAt := w.clock.Now()
	expiresAt := completedAt.Add(w.retention)

	if err := w.repo.UpdateExportStatus(
		ctx,
		exportID,
		domain.ExportStatusCompleted,
		nil,
		&completedAt,
		&expiresAt,
		&objectKey,
		nil,
	); err != nil {
		return fmt.Errorf("mark export completed: %w", err)
	}

	return nil
}

func (w *ExportWorker) buildZipArchive(
	ctx context.Context, exportID, userID uuid.UUID, startedAt time.Time,
) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	moduleNames := make([]string, 0, len(w.providers))
	for _, item := range w.providers {
		moduleNames = append(moduleNames, item.Name)
		if err := w.writeProviderEntry(ctx, zipWriter, item, userID); err != nil {
			return nil, err
		}
	}

	if err := w.writeMetadataEntry(zipWriter, exportID, userID, startedAt, moduleNames); err != nil {
		return nil, err
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close zip archive: %w", err)
	}

	return &buf, nil
}

func (w *ExportWorker) writeProviderEntry(
	ctx context.Context, zipWriter *zip.Writer, item NamedExportable, userID uuid.UUID,
) error {
	data, err := item.Provider.ExportUserData(ctx, userID.String())
	if err != nil {
		return fmt.Errorf("export data from module %q: %w", item.Name, err)
	}
	if data == nil {
		data = map[string]interface{}{}
	}

	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s json: %w", item.Name, err)
	}

	entry, err := zipWriter.Create(item.Name + ".json")
	if err != nil {
		return fmt.Errorf("create zip entry %s.json: %w", item.Name, err)
	}
	if _, err := entry.Write(encoded); err != nil {
		return fmt.Errorf("write zip entry %s.json: %w", item.Name, err)
	}
	return nil
}

func (w *ExportWorker) writeMetadataEntry(
	zipWriter *zip.Writer, exportID, userID uuid.UUID, startedAt time.Time, moduleNames []string,
) error {
	sort.Strings(moduleNames)
	metadata := map[string]any{
		"export_id":    exportID.String(),
		"user_id":      userID.String(),
		"requested_at": startedAt.UTC().Format(time.RFC3339),
		"exported_at":  w.clock.Now().UTC().Format(time.RFC3339),
		"modules":      moduleNames,
	}
	metaEncoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata json: %w", err)
	}
	metaEntry, err := zipWriter.Create("metadata.json")
	if err != nil {
		return fmt.Errorf("create zip entry metadata.json: %w", err)
	}
	if _, err := metaEntry.Write(metaEncoded); err != nil {
		return fmt.Errorf("write zip entry metadata.json: %w", err)
	}
	return nil
}

func (w *ExportWorker) uploadAndNotify(
	ctx context.Context, userID uuid.UUID, objectKey string, zipBuffer *bytes.Buffer,
) error {
	if w.storage == nil {
		return nil
	}

	if err := w.storage.Put(
		ctx, w.bucket, objectKey, bytes.NewReader(zipBuffer.Bytes()), int64(zipBuffer.Len()), "application/zip",
	); err != nil {
		return fmt.Errorf("upload export zip to storage: %w", err)
	}

	downloadURL, err := w.storage.PresignGet(ctx, w.bucket, objectKey, w.linkTTL)
	if err != nil {
		return fmt.Errorf("presign download url: %w", err)
	}

	w.sendNotificationEmail(ctx, userID, downloadURL)
	return nil
}

func (w *ExportWorker) sendNotificationEmail(ctx context.Context, userID uuid.UUID, downloadURL string) {
	if w.mailer == nil || w.userContact == nil {
		return
	}

	contact, err := w.userContact.Recipient(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "failed to get recipient contact for export email", "user_id", userID, "error", err)
		return
	}

	msg := mailer.Message{
		To:       contact.Email,
		Template: "data_export",
		Locale:   contact.Locale,
		Category: "transactional",
		Data: map[string]any{
			"DisplayName":  contact.DisplayName,
			"DownloadURL":  downloadURL,
			"ExpiresHours": int(w.linkTTL.Hours()),
		},
	}
	if err := w.mailer.Send(ctx, msg); err != nil {
		slog.ErrorContext(ctx, "failed to send export ready email", "user_id", userID, "error", err)
	}
}
