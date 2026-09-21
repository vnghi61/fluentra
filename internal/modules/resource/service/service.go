package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/domain"
	"github.com/fluentra/fluentra/internal/modules/resource/job"
	platformjob "github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository defines datastore operations needed by the resource service.
type Repository interface {
	CreateFileResourceIntent(
		ctx context.Context, id, userID uuid.UUID, title string, objectKey *string, originalFilename, declaredMime string,
	) (*contract.Resource, error)
	CreateURLResource(ctx context.Context, id, userID uuid.UUID, title, sourceURL string) (*contract.Resource, error)
	GetResourceByID(ctx context.Context, id uuid.UUID) (*contract.Resource, error)
	GetResourceByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error)
	ListResourcesByUser(
		ctx context.Context, userID uuid.UUID, status, kind *string, limit, offset int32,
	) ([]contract.Resource, int, error)
	GetUserResourceUsage(ctx context.Context, userID uuid.UUID) (int64, int64, error)
	ConfirmFileResource(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error)
	UpdateValidationSuccess(
		ctx context.Context, id uuid.UUID, title, detectedMime string, byteSize int64, checksum string,
	) (*contract.Resource, error)
	UpdateValidationRejected(
		ctx context.Context, id uuid.UUID, failureReason string, detectedMime *string,
	) (*contract.Resource, error)
	UpdateValidationFailed(
		ctx context.Context, id uuid.UUID, failureReason, fromStatus string,
	) (*contract.Resource, error)
	DeleteResource(ctx context.Context, id, userID uuid.UUID) (*string, string, error)
	ListExpiredPendingResources(ctx context.Context, before time.Time, limit int32) ([]contract.Resource, error)
	ListStuckUploadedResources(ctx context.Context, before time.Time, limit int32) ([]contract.Resource, error)

	// The two writes that share a transaction with a job enqueue.
	ConfirmFileResourceTx(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) (*contract.Resource, error)
	CreateURLResourceTx(
		ctx context.Context, tx pgx.Tx, id, userID uuid.UUID, title, sourceURL string,
	) (*contract.Resource, error)
}

// WorkerNudger signals a background worker to wake up after a job is enqueued.
type WorkerNudger interface {
	Nudge(ctx context.Context)
}

// Service orchestrates resource intake, storage, and validation.
type Service struct {
	repo     Repository
	storage  storage.Store
	pool     *pgxpool.Pool
	enqueuer platformjob.Enqueuer
	fetcher  URLFetcher
	nudger   WorkerNudger
}

// New constructs a new resource Service.
func New(
	repo Repository,
	store storage.Store,
	pool *pgxpool.Pool,
	enqueuer platformjob.Enqueuer,
	fetcher URLFetcher,
	nudger WorkerNudger,
) *Service {
	if fetcher == nil {
		fetcher = NewSafeHTTPFetcher()
	}
	return &Service{
		repo:     repo,
		storage:  store,
		pool:     pool,
		enqueuer: enqueuer,
		fetcher:  fetcher,
		nudger:   nudger,
	}
}

// CreateUploadIntent issues a presigned S3 PUT instruction and stores a pending file resource.
func (s *Service) CreateUploadIntent(
	ctx context.Context, userID uuid.UUID, filename, declaredMIME string,
) (*contract.UploadIntentResult, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, fmt.Errorf("%w: filename is required", domain.ErrResourceTypeNotSupported)
	}

	declaredMIME = domain.NormalizeMIME(declaredMIME)
	if declaredMIME == "" || !domain.IsAllowedMIME(declaredMIME) {
		return nil, fmt.Errorf("%w: unsupported declared MIME %q", domain.ErrResourceTypeNotSupported, declaredMIME)
	}

	// Check quotas (BR-RESOURCE-08)
	count, totalBytes, err := s.repo.GetUserResourceUsage(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check quota: %w", err)
	}
	if count >= domain.MaxUserResources {
		return nil, fmt.Errorf(
			"%w: maximum resource count (%d) reached", domain.ErrResourceQuotaExceeded, domain.MaxUserResources,
		)
	}
	if totalBytes+domain.MaxResourceBytes > domain.MaxUserTotalBytes {
		return nil, fmt.Errorf(
			"%w: total storage quota (%d bytes) would be exceeded",
			domain.ErrResourceQuotaExceeded, domain.MaxUserTotalBytes,
		)
	}

	resourceID := uuid.New()
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	if ext == "" {
		ext = "bin"
	}

	objectKey, err := storage.BuildKey("user", userID.String(), time.Now(), resourceID.String(), ext)
	if err != nil {
		return nil, fmt.Errorf("build storage key: %w", err)
	}

	intent, err := s.storage.PresignPut(
		ctx,
		storage.BucketUploads,
		objectKey,
		declaredMIME,
		domain.MaxResourceBytes,
		domain.PresignPutExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("presign put upload intent: %w", err)
	}

	if _, err := s.repo.CreateFileResourceIntent(
		ctx, resourceID, userID, filename, &objectKey, filename, declaredMIME,
	); err != nil {
		return nil, fmt.Errorf("save upload intent: %w", err)
	}

	return &contract.UploadIntentResult{
		ID:        resourceID,
		UploadURL: intent.URL,
		ObjectKey: objectKey,
		ExpiresAt: intent.ExpiresAt,
	}, nil
}

// ConfirmUpload confirms that a client has uploaded their file and queues it for validation.
func (s *Service) ConfirmUpload(ctx context.Context, userID, resourceID uuid.UUID) (*contract.SubmitResult, error) {
	res, err := s.repo.GetResourceByIDAndUser(ctx, resourceID, userID)
	if err != nil {
		return nil, err // 404 if not found or unowned
	}

	if res.Status != domain.StatusPending {
		return nil, fmt.Errorf("%w: resource is already in %s status", domain.ErrInvalidStatusTransition, res.Status)
	}

	if res.ObjectKey == nil {
		return nil, domain.ErrResourceNotUploaded
	}

	// Verify object exists in storage (BR-RESOURCE-04, 409 if missing)
	if _, err := s.storage.Stat(ctx, storage.BucketUploads, *res.ObjectKey); err != nil {
		return nil, domain.ErrResourceNotUploaded
	}

	// Move status to uploaded and enqueue validation inside transaction
	if err := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.ConfirmFileResourceTx(txCtx, tx, resourceID, userID); err != nil {
			return err
		}
		if s.enqueuer != nil {
			args := job.ValidateResourceArgs{ResourceID: resourceID}
			if _, err := s.enqueuer.EnqueueTx(txCtx, tx, args, nil); err != nil {
				return fmt.Errorf("enqueue validation job: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("confirm upload transaction: %w", err)
	}

	if s.nudger != nil {
		s.nudger.Nudge(ctx)
	}

	return &contract.SubmitResult{
		ID:     resourceID,
		Status: domain.StatusUploaded,
	}, nil
}

// SubmitURL imports an external URL as a resource reference and queues it for validation.
func (s *Service) SubmitURL(
	ctx context.Context, userID uuid.UUID, rawURL, title string,
) (*contract.SubmitResult, error) {
	parsed, err := domain.ValidateURLScheme(rawURL)
	if err != nil {
		return nil, err
	}

	if err := domain.ResolveAndValidateHost(ctx, parsed.Hostname()); err != nil {
		return nil, err
	}

	// Check quotas (BR-RESOURCE-08)
	count, _, err := s.repo.GetUserResourceUsage(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check quota: %w", err)
	}
	if count >= domain.MaxUserResources {
		return nil, fmt.Errorf(
			"%w: maximum resource count (%d) reached", domain.ErrResourceQuotaExceeded, domain.MaxUserResources,
		)
	}

	resourceID := uuid.New()
	title = strings.TrimSpace(title)
	if title == "" {
		title = parsed.Hostname()
	}

	cleanURL := parsed.String()

	if err := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.CreateURLResourceTx(txCtx, tx, resourceID, userID, title, cleanURL); err != nil {
			return err
		}
		if s.enqueuer != nil {
			args := job.ValidateResourceArgs{ResourceID: resourceID}
			if _, err := s.enqueuer.EnqueueTx(txCtx, tx, args, nil); err != nil {
				return fmt.Errorf("enqueue validation job: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("submit url transaction: %w", err)
	}

	if s.nudger != nil {
		s.nudger.Nudge(ctx)
	}

	return &contract.SubmitResult{
		ID:     resourceID,
		Status: domain.StatusUploaded,
	}, nil
}

// ListResources returns the learner's paginated resources, newest first.
func (s *Service) ListResources(
	ctx context.Context, userID uuid.UUID, status, kind *string, page, pageSize int,
) (*contract.ResourceList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	//nolint:gosec // page >= 1 and pageSize <= 100 so multiplication cannot overflow int32
	offset := int32((page - 1) * pageSize)
	//nolint:gosec // pageSize <= 100 fits in int32
	limit := int32(pageSize)

	items, total, err := s.repo.ListResourcesByUser(ctx, userID, status, kind, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}

	return &contract.ResourceList{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetResource fetches resource details. Unowned resources answer 404 (BR-RESOURCE-01).
// For validated files, a presigned download URL is included.
func (s *Service) GetResource(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error) {
	res, err := s.repo.GetResourceByIDAndUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if res.Kind == domain.KindFile && res.Status == domain.StatusValidated && res.ObjectKey != nil {
		downloadURL, err := s.storage.PresignGet(ctx, storage.BucketUploads, *res.ObjectKey, domain.PresignGetExpiry)
		if err == nil {
			res.DownloadURL = &downloadURL
		}
	}

	return res, nil
}

// DeleteResource deletes a resource row and its underlying stored object (BR-RESOURCE-06).
func (s *Service) DeleteResource(ctx context.Context, id, userID uuid.UUID) error {
	res, err := s.repo.GetResourceByIDAndUser(ctx, id, userID)
	if err != nil {
		return err // 404 if not found or unowned
	}

	if res.Kind == domain.KindFile && res.ObjectKey != nil && *res.ObjectKey != "" {
		if err := s.storage.Delete(ctx, storage.BucketUploads, *res.ObjectKey); err != nil {
			return fmt.Errorf("delete object from storage: %w", err)
		}
	}

	if _, _, err := s.repo.DeleteResource(ctx, id, userID); err != nil {
		return fmt.Errorf("delete resource record: %w", err)
	}

	return nil
}

// ValidateResource executes the validation pipeline for a file or URL resource.
//
// Returning an error asks River to retry, so an error here means "we could not
// finish", never "the resource is bad". A bad resource is written as 'rejected'
// and the job succeeds.
func (s *Service) ValidateResource(ctx context.Context, resourceID uuid.UUID) error {
	res, err := s.repo.GetResourceByID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil // deleted before the job ran
		}
		return err
	}

	if res.Status != domain.StatusUploaded {
		return nil // Already processed or not ready
	}

	switch res.Kind {
	case domain.KindFile:
		err = s.validateFileResource(ctx, res)
	case domain.KindURL:
		err = s.validateURLResource(ctx, res)
	}
	// The row moved on while we worked: deleted by its owner, or finished by
	// another run of this job. Either way there is nothing left to do.
	if errors.Is(err, domain.ErrResourceStateChanged) {
		return nil
	}
	return err
}

// reject records a verdict about the resource itself. The reason is shown to the
// learner, so it is always one of domain's fixed sentences; the detail that led
// to it goes to the log, where it cannot tell a learner what our internal DNS
// resolves to.
func (s *Service) reject(ctx context.Context, res *contract.Resource, reason string, detail error) error {
	slog.InfoContext(ctx, "resource rejected",
		"resource_id", res.ID, "kind", res.Kind, "reason", reason, "detail", detail)
	_, err := s.repo.UpdateValidationRejected(ctx, res.ID, reason, nil)
	return err
}

func (s *Service) validateFileResource(ctx context.Context, res *contract.Resource) error {
	if res.ObjectKey == nil || *res.ObjectKey == "" {
		return s.reject(ctx, res, domain.ReasonIncomplete, nil)
	}

	stat, err := s.storage.Stat(ctx, storage.BucketUploads, *res.ObjectKey)
	if err != nil {
		return fmt.Errorf("stat storage object: %w", err) // Transient error, River will retry
	}

	if stat.Size == 0 {
		return s.reject(ctx, res, domain.ReasonEmptyFile, nil)
	}

	if stat.Size > domain.MaxResourceBytes {
		return s.reject(ctx, res, domain.ReasonTooLarge(), fmt.Errorf("size %d bytes", stat.Size))
	}

	rc, err := s.storage.Get(ctx, storage.BucketUploads, *res.ObjectKey)
	if err != nil {
		return fmt.Errorf("get storage object stream: %w", err) // Transient error, River will retry
	}
	defer func() { _ = rc.Close() }()

	head := make([]byte, 1024)
	n, readErr := io.ReadFull(rc, head)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return fmt.Errorf("read header bytes: %w", readErr)
	}
	head = head[:n]

	detectedMIME, err := domain.SniffAndValidateMIME(res.DeclaredMIME, head)
	if err != nil {
		return s.reject(ctx, res, domain.ReasonTypeNotSupported, err)
	}

	hasher := sha256.New()
	hasher.Write(head)
	if _, err := io.Copy(hasher, rc); err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	if _, err := s.repo.UpdateValidationSuccess(ctx, res.ID, res.Title, detectedMIME, stat.Size, checksum); err != nil {
		return fmt.Errorf("save validation success: %w", err)
	}

	return nil
}

func (s *Service) validateURLResource(ctx context.Context, res *contract.Resource) error {
	if res.SourceURL == nil || *res.SourceURL == "" {
		return s.reject(ctx, res, domain.ReasonIncomplete, nil)
	}

	meta, err := s.fetcher.FetchMetadata(ctx, *res.SourceURL)
	var statusErr *URLStatusError
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrResourceURLNotAllowed):
		return s.reject(ctx, res, domain.ReasonURLNotAllowed, err)
	case errors.As(err, &statusErr):
		return s.reject(ctx, res, domain.ReasonURLStatus(statusErr.StatusCode), err)
	default:
		// The network or the remote server let us down. That is not a verdict on
		// the resource, so it is 'failed' - which the learner can retry - rather
		// than 'rejected', which tells them their link was bad.
		slog.WarnContext(ctx, "resource url unreachable", "resource_id", res.ID, "error", err)
		_, ferr := s.repo.UpdateValidationFailed(ctx, res.ID, domain.ReasonURLUnreachable, domain.StatusUploaded)
		return ferr
	}

	title := res.Title
	if title == "" || title == *res.SourceURL {
		title = meta.Title
	}

	if _, err := s.repo.UpdateValidationSuccess(ctx, res.ID, title, "text/html", 0, ""); err != nil {
		return fmt.Errorf("save url validation success: %w", err)
	}

	return nil
}

// sweepBatch bounds one sweep; the cron runs again in ten minutes.
const sweepBatch = 50

// SweepExpiredPending closes out resources that will never finish on their own:
// upload intents nobody completed (BR-RESOURCE-07), and confirmed uploads whose
// validation never came back.
func (s *Service) SweepExpiredPending(ctx context.Context) (int, error) {
	expired, err := s.sweepPendingIntents(ctx)
	if err != nil {
		return expired, err
	}
	stuck, err := s.sweepStuckUploads(ctx)
	return expired + stuck, err
}

// sweepPendingIntents marks abandoned intents failed and removes their objects.
//
// The row is updated first and the object deleted only if that update matched.
// The other order deleted the object of a row the learner had confirmed between
// the sweeper's read and its write, leaving an 'uploaded' resource pointing at
// nothing.
func (s *Service) sweepPendingIntents(ctx context.Context) (int, error) {
	threshold := time.Now().Add(-domain.PendingIntentTTL)
	expired, err := s.repo.ListExpiredPendingResources(ctx, threshold, sweepBatch)
	if err != nil {
		return 0, fmt.Errorf("list expired pending resources: %w", err)
	}

	count := 0
	for _, res := range expired {
		if _, err := s.repo.UpdateValidationFailed(
			ctx, res.ID, domain.ReasonIntentExpired, domain.StatusPending,
		); err != nil {
			if errors.Is(err, domain.ErrResourceStateChanged) {
				continue // confirmed in the meantime; not ours to sweep
			}
			return count, err
		}
		count++
		if res.ObjectKey != nil && *res.ObjectKey != "" {
			if err := s.storage.Delete(ctx, storage.BucketUploads, *res.ObjectKey); err != nil {
				slog.WarnContext(ctx, "could not delete an expired intent's object",
					"resource_id", res.ID, "object_key", *res.ObjectKey, "error", err)
			}
		}
	}
	return count, nil
}

// sweepStuckUploads marks failed the confirmed uploads whose validation job has
// given up - its retries spent on a storage outage, or the job lost. The object
// is kept: it is the learner's confirmed upload, and deleting the resource is
// theirs to do.
func (s *Service) sweepStuckUploads(ctx context.Context) (int, error) {
	threshold := time.Now().Add(-domain.StuckUploadTTL)
	stuck, err := s.repo.ListStuckUploadedResources(ctx, threshold, sweepBatch)
	if err != nil {
		return 0, fmt.Errorf("list stuck uploaded resources: %w", err)
	}

	count := 0
	for _, res := range stuck {
		if _, err := s.repo.UpdateValidationFailed(
			ctx, res.ID, domain.ReasonValidationStuck, domain.StatusUploaded,
		); err != nil {
			if errors.Is(err, domain.ErrResourceStateChanged) {
				continue
			}
			return count, err
		}
		count++
	}
	return count, nil
}
