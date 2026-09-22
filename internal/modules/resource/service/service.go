package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/domain"
	"github.com/fluentra/fluentra/internal/modules/resource/job"
	"github.com/fluentra/fluentra/internal/platform/ai"
	platformjob "github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/media"
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
	ListResourcesByUserID(ctx context.Context, userID uuid.UUID) ([]contract.Resource, error)
	DeleteAllResourcesByUser(ctx context.Context, userID uuid.UUID) error
	InsertRenditionPending(ctx context.Context, resourceID uuid.UUID, kind string) (*contract.Rendition, error)
	ClaimPendingRenditions(ctx context.Context, limit int32) ([]contract.Rendition, error)
	UpdateRenditionReady(
		ctx context.Context, id uuid.UUID, objectKey, mimeType string,
		width, height, durationMS *int, byteSize *int64, toolVersion string,
	) (*contract.Rendition, error)
	UpdateRenditionSkipped(ctx context.Context, id uuid.UUID, reason string) (*contract.Rendition, error)
	UpdateRenditionFailed(ctx context.Context, id uuid.UUID, reason string) (*contract.Rendition, error)
	ListReadyRenditionsByResourceID(ctx context.Context, resourceID uuid.UUID) ([]contract.Rendition, error)
	ListRenditionsByResourceID(ctx context.Context, resourceID uuid.UUID) ([]contract.Rendition, error)
	ListRenditionKeysByUserID(ctx context.Context, userID uuid.UUID) ([]string, error)
	ListValidatedFileResourcesForRenditions(ctx context.Context, limit int32) ([]contract.Resource, error)
	UpsertExtraction(
		ctx context.Context, resourceID uuid.UUID, source, text string, charCount int32,
		truncated bool, language, toolVersion string,
	) (*contract.Extraction, error)
	UpsertExtractionTx(
		ctx context.Context, tx pgx.Tx, resourceID uuid.UUID, source, text string, charCount int32,
		truncated bool, language, toolVersion string,
	) (*contract.Extraction, error)
	GetExtractionByResourceID(ctx context.Context, resourceID uuid.UUID) (*contract.Extraction, error)
	UpsertClassification(
		ctx context.Context, resourceID uuid.UUID, cefrEstimate, skill *string,
		nodeCodes []string, promptVersion, model string, aiRequestID *uuid.UUID,
	) (*contract.Classification, error)
	GetClassificationByResourceID(ctx context.Context, resourceID uuid.UUID) (*contract.Classification, error)

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

// MediaRenderRequester asks for media renditions to be rendered now.
type MediaRenderRequester interface {
	RequestRender(ctx context.Context) error
}

// Service orchestrates resource intake, storage, and validation.
type Service struct {
	repo        Repository
	storage     storage.Store
	pool        *pgxpool.Pool
	enqueuer    platformjob.Enqueuer
	fetcher     URLFetcher
	nudger      WorkerNudger
	mediaRender MediaRenderRequester
	transcriber media.Transcriber
	aiClient    ai.Client
	taxonomies  contentcontract.TaxonomyResolver
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

// SetMediaRender configures an optional dispatcher for rendering media renditions.
func (s *Service) SetMediaRender(r MediaRenderRequester) {
	s.mediaRender = r
}

// SetTranscriber configures the speech-to-text transcriber.
func (s *Service) SetTranscriber(t media.Transcriber) {
	s.transcriber = t
}

// SetAIClient configures the AI client used for classification.
func (s *Service) SetAIClient(c ai.Client) {
	s.aiClient = c
}

// SetTaxonomies configures the taxonomy resolver for spine grounding.
func (s *Service) SetTaxonomies(tax contentcontract.TaxonomyResolver) {
	s.taxonomies = tax
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
		s.attachRenditions(ctx, res)
	}

	if ext, err := s.repo.GetExtractionByResourceID(ctx, id); err == nil && ext != nil {
		res.Extraction = ext
	}

	if cls, err := s.repo.GetClassificationByResourceID(ctx, id); err == nil && cls != nil {
		cls.Nodes = s.labelNodes(ctx, cls.NodeCodes)
		res.Classification = cls
	}

	return res, nil
}

// attachRenditions adds the ready renditions, each with a presigned URL.
func (s *Service) attachRenditions(ctx context.Context, res *contract.Resource) {
	renditions, err := s.repo.ListReadyRenditionsByResourceID(ctx, res.ID)
	if err != nil || len(renditions) == 0 {
		return
	}
	expiry := time.Now().Add(domain.PresignGetExpiry)
	for i := range renditions {
		if renditions[i].ObjectKey == nil || *renditions[i].ObjectKey == "" {
			continue
		}
		getURL, err := s.storage.PresignGet(ctx, storage.BucketDerived, *renditions[i].ObjectKey, domain.PresignGetExpiry)
		if err == nil {
			renditions[i].URL = &getURL
			renditions[i].ExpiresAt = &expiry
		}
	}
	res.Renditions = renditions
}

// labelNodes pairs each stored spine code with its label; a code the spine no
// longer knows keeps itself as its label.
func (s *Service) labelNodes(ctx context.Context, codes []string) []contract.ClassificationNode {
	if s.taxonomies == nil || len(codes) == 0 {
		return nil
	}
	nodes := make([]contract.ClassificationNode, 0, len(codes))
	for _, code := range codes {
		label := code
		if node, err := s.taxonomies.GetTaxonomyByCode(ctx, code); err == nil && node != nil {
			label = node.Label
		}
		nodes = append(nodes, contract.ClassificationNode{Code: code, Label: label})
	}
	return nodes
}

// DeleteResource deletes a resource row and its underlying stored object (BR-RESOURCE-06).
func (s *Service) DeleteResource(ctx context.Context, id, userID uuid.UUID) error {
	res, err := s.repo.GetResourceByIDAndUser(ctx, id, userID)
	if err != nil {
		return err // 404 if not found or unowned
	}

	if res.Kind == domain.KindFile && res.ObjectKey != nil && *res.ObjectKey != "" {
		if err := s.deleteStorageObject(ctx, storage.BucketUploads, *res.ObjectKey); err != nil {
			return fmt.Errorf("delete object from storage: %w", err)
		}
	}

	// Also delete any derived renditions
	renditions, err := s.repo.ListRenditionsByResourceID(ctx, id)
	if err == nil {
		for _, r := range renditions {
			if r.ObjectKey != nil && *r.ObjectKey != "" {
				_ = s.deleteStorageObject(ctx, storage.BucketDerived, *r.ObjectKey)
			}
		}
	}

	if _, _, err := s.repo.DeleteResource(ctx, id, userID); err != nil {
		return fmt.Errorf("delete resource row: %w", err)
	}

	return nil
}

// MaterialForOwner returns a validated file resource with its renditions, for
// its owner only (BR-RESOURCE-01). A resource the caller does not own is the
// same 404 as any other foreign id.
func (s *Service) MaterialForOwner(
	ctx context.Context, ownerID, resourceID uuid.UUID,
) (*contract.Material, error) {
	res, err := s.repo.GetResourceByIDAndUser(ctx, resourceID, ownerID)
	if err != nil {
		return nil, err
	}
	mat := &contract.Material{
		ID:           res.ID,
		UserID:       res.UserID,
		DetectedMIME: res.DetectedMIME,
		Status:       res.Status,
		Renditions:   nil,
	}
	if res.ObjectKey != nil {
		mat.ObjectKey = *res.ObjectKey
	}
	if res.ByteSize != nil {
		mat.ByteSize = *res.ByteSize
	}
	renditions, err := s.repo.ListRenditionsByResourceID(ctx, resourceID)
	if err == nil {
		mat.Renditions = renditions
	}
	return mat, nil
}

// CopyForPublication copies a resource's original and every ready rendition
// into fluentra-media under destPrefix, so a published course owns its bytes
// and is decoupled from the creator's account and quota (D20-2).
//
// The copy is server-side and idempotent: the same destPrefix writes the same
// destination keys, so re-publishing a revision replaces them rather than
// accumulating. Called only after MaterialForOwner has proven ownership.
func (s *Service) CopyForPublication(
	ctx context.Context, resourceID uuid.UUID, destPrefix string,
) (*contract.PublishedObjects, error) {
	res, err := s.repo.GetResourceByID(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if res.Kind != domain.KindFile || res.ObjectKey == nil || *res.ObjectKey == "" {
		return nil, fmt.Errorf("%w: resource has no stored original to publish",
			domain.ErrResourceTypeNotSupported)
	}
	if destPrefix == "" {
		return nil, fmt.Errorf("publish resource: destination prefix is required")
	}
	if !strings.HasSuffix(destPrefix, "/") {
		destPrefix += "/"
	}

	out := &contract.PublishedObjects{
		Original: destPrefix + "original" + objectExt(*res.ObjectKey),
	}
	if err := s.storage.Copy(ctx,
		storage.BucketUploads, *res.ObjectKey, storage.BucketMedia, out.Original); err != nil {
		return nil, fmt.Errorf("copy original to course storage: %w", err)
	}

	renditions, err := s.repo.ListReadyRenditionsByResourceID(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("list ready renditions for publish: %w", err)
	}
	if err := s.copyRenditions(ctx, destPrefix, renditions, out); err != nil {
		return nil, err
	}
	return out, nil
}

// copyRenditions copies every ready rendition into the course's storage and
// records the destination key on the right field of out.
func (s *Service) copyRenditions(
	ctx context.Context, destPrefix string, renditions []contract.Rendition, out *contract.PublishedObjects,
) error {
	for _, r := range renditions {
		if r.ObjectKey == nil || *r.ObjectKey == "" {
			continue
		}
		destKey := destPrefix + r.Kind + objectExt(*r.ObjectKey)
		if err := s.storage.Copy(ctx,
			storage.BucketDerived, *r.ObjectKey, storage.BucketMedia, destKey); err != nil {
			return fmt.Errorf("copy %s rendition to course storage: %w", r.Kind, err)
		}
		assignPublishedObject(out, r.Kind, destKey)
	}
	return nil
}

// assignPublishedObject maps a rendition kind to the field it lands in.
func assignPublishedObject(out *contract.PublishedObjects, kind, key string) {
	switch kind {
	case domain.RenditionKindPoster:
		out.Poster = &key
	case domain.RenditionKindVideo360p:
		out.Video360p = &key
	case domain.RenditionKindVideo720p:
		out.Video720p = &key
	case domain.RenditionKindPreview:
		out.Preview = &key
	case domain.RenditionKindAudioWeb:
		out.AudioWeb = &key
	}
}

// objectExt keeps a copied object's extension so a browser and the media
// pipeline still recognise its format.
func objectExt(key string) string {
	if ext := path.Ext(key); ext != "" {
		return ext
	}
	return ".bin"
}

// DeleteUserResources purges all resources, original upload files, and derived renditions
// for a user upon account erasure (BR-RESOURCE-15).
func (s *Service) DeleteUserResources(ctx context.Context, userID uuid.UUID) error {
	renditionKeys, err := s.repo.ListRenditionKeysByUserID(ctx, userID)
	if err == nil {
		for _, rk := range renditionKeys {
			_ = s.deleteStorageObject(ctx, storage.BucketDerived, rk)
		}
	}

	resources, err := s.repo.ListResourcesByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("list resources for user erasure: %w", err)
	}

	for _, res := range resources {
		if res.ObjectKey != nil && *res.ObjectKey != "" {
			if err := s.deleteStorageObject(ctx, storage.BucketUploads, *res.ObjectKey); err != nil {
				return fmt.Errorf("delete upload object %s: %w", *res.ObjectKey, err)
			}
		}
	}

	if err := s.repo.DeleteAllResourcesByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete user resource rows: %w", err)
	}

	return nil
}

// PlanRenditions finds validated file resources and inserts pending renditions for missing kinds.
func (s *Service) PlanRenditions(ctx context.Context, limit int32) (int, error) {
	resources, err := s.repo.ListValidatedFileResourcesForRenditions(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("list validated files for renditions: %w", err)
	}

	count := 0
	for _, res := range resources {
		kinds := domain.PlannedRenditionsForMIME(res.DetectedMIME)
		for _, k := range kinds {
			created, err := s.repo.InsertRenditionPending(ctx, res.ID, k)
			if err != nil {
				return count, fmt.Errorf("insert rendition pending: %w", err)
			}
			if created != nil {
				count++
			}
		}
	}
	return count, nil
}

// ClaimRenditions claims up to limit pending renditions for processing.
func (s *Service) ClaimRenditions(ctx context.Context, limit int32) ([]contract.Rendition, error) {
	return s.repo.ClaimPendingRenditions(ctx, limit)
}

// RecordRenditionReady marks a rendition as ready with metadata and object key.
func (s *Service) RecordRenditionReady(
	ctx context.Context, id uuid.UUID, objectKey, mimeType string,
	width, height, durationMS *int, byteSize *int64, toolVersion string,
) (*contract.Rendition, error) {
	updated, err := s.repo.UpdateRenditionReady(
		ctx, id, objectKey, mimeType, width, height, durationMS, byteSize, toolVersion,
	)
	if err != nil {
		return nil, err
	}

	// Ready or skipped, audio_web settles the audio a transcript is made from.
	if updated.Kind == domain.RenditionKindAudioWeb {
		if err := s.enqueue(ctx, job.TranscribeResourceArgs{ResourceID: updated.ResourceID}); err != nil {
			return nil, err
		}
	}

	return updated, nil
}

// RecordRenditionSkipped marks a rendition as skipped with a reason.
func (s *Service) RecordRenditionSkipped(
	ctx context.Context, id uuid.UUID, reason string,
) (*contract.Rendition, error) {
	updated, err := s.repo.UpdateRenditionSkipped(ctx, id, reason)
	if err != nil {
		return nil, err
	}

	// Ready or skipped, audio_web settles the audio a transcript is made from.
	if updated.Kind == domain.RenditionKindAudioWeb {
		if err := s.enqueue(ctx, job.TranscribeResourceArgs{ResourceID: updated.ResourceID}); err != nil {
			return nil, err
		}
	}

	return updated, nil
}

// RecordRenditionFailed marks a rendition failure.
func (s *Service) RecordRenditionFailed(
	ctx context.Context, id uuid.UUID, reason string,
) (*contract.Rendition, error) {
	return s.repo.UpdateRenditionFailed(ctx, id, reason)
}

// enqueue queues a job on its own and wakes the worker. Nothing to write goes
// with it; a write that must not happen without its job uses a shared
// transaction instead (see RecordExtraction).
func (s *Service) enqueue(ctx context.Context, args river.JobArgs) error {
	if s.enqueuer == nil || s.pool == nil {
		return nil
	}
	if err := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		_, err := s.enqueuer.EnqueueTx(txCtx, tx, args, nil)
		return err
	}); err != nil {
		return fmt.Errorf("enqueue %s: %w", args.Kind(), err)
	}
	if s.nudger != nil {
		s.nudger.Nudge(ctx)
	}
	return nil
}

// RecordExtraction persists extracted text for a resource and, when there is
// text, queues its classification in the same transaction: an extraction whose
// classification job was lost would never be classified, because the job is
// what reads it.
func (s *Service) RecordExtraction(
	ctx context.Context, resourceID uuid.UUID, source, text string, truncated bool, language, toolVersion string,
) error {
	charCount := int32(min(utf8.RuneCountInString(text), maxExtractionChars)) //nolint:gosec // G115: bounded by min
	if s.pool == nil {
		_, err := s.repo.UpsertExtraction(ctx, resourceID, source, text, charCount, truncated, language, toolVersion)
		if err != nil {
			return fmt.Errorf("record extraction: %w", err)
		}
		return nil
	}
	err := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.UpsertExtractionTx(
			txCtx, tx, resourceID, source, text, charCount, truncated, language, toolVersion,
		); err != nil {
			return err
		}
		if text == "" || s.enqueuer == nil {
			return nil
		}
		_, err := s.enqueuer.EnqueueTx(txCtx, tx, job.ClassifyResourceArgs{ResourceID: resourceID}, nil)
		return err
	})
	if err != nil {
		return fmt.Errorf("record extraction: %w", err)
	}
	if text != "" && s.nudger != nil {
		s.nudger.Nudge(ctx)
	}
	return nil
}

// Extraction limits (WO 19 B.3).
const (
	maxExtractionChars = 400_000
	classifierChars    = 8_000
	maxTranscribeBytes = 25 * 1024 * 1024 // the transcription API's upload cap
	classifyPromptVer  = "resource_classify.v1"
)

// truncateRunes keeps the first n characters, never splitting one.
func truncateRunes(s string, n int) (string, bool) {
	if utf8.RuneCountInString(s) <= n {
		return s, false
	}
	return string([]rune(s)[:n]), true
}

// TranscribeResource transcribes audio or video media and stores the transcript extraction.
func (s *Service) TranscribeResource(ctx context.Context, resourceID uuid.UUID) error {
	// Idempotency (Trap 3): skip if extraction already exists
	existing, err := s.repo.GetExtractionByResourceID(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("get extraction for %s: %w", resourceID, err)
	}
	if existing != nil || s.transcriber == nil {
		return nil
	}

	res, err := s.repo.GetResourceByID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil
		}
		return err
	}
	if res.Kind != domain.KindFile || res.Status != domain.StatusValidated {
		return nil
	}

	bucket, objectKey := s.audioSource(ctx, res)
	if objectKey == "" {
		return nil // No audio to transcribe
	}

	stat, err := s.storage.Stat(ctx, bucket, objectKey)
	if err != nil {
		return fmt.Errorf("stat audio object: %w", err)
	}
	readLimit := min(stat.Size, maxTranscribeBytes)
	truncated := stat.Size > maxTranscribeBytes

	rc, err := s.storage.Get(ctx, bucket, objectKey)
	if err != nil {
		return fmt.Errorf("get audio stream: %w", err)
	}
	defer func() { _ = rc.Close() }()

	// The transcription API reads the format from the file name, so it carries
	// the object's own extension: .m4a for audio_web, the upload's otherwise.
	result, err := s.transcriber.Transcribe(ctx, io.LimitReader(rc, readLimit), "audio"+path.Ext(objectKey))
	if err != nil {
		return fmt.Errorf("transcribe audio: %w", err)
	}

	text, cut := truncateRunes(result.Text, maxExtractionChars)
	toolVersion := "whisper-1"
	if t, ok := s.transcriber.(interface{ Model() string }); ok {
		toolVersion = t.Model()
	}
	return s.RecordExtraction(ctx, resourceID, "transcript", text, truncated || cut, result.Language, toolVersion)
}

// audioSource is what a transcript is made from: the audio_web rendition when
// there is one, else an audio upload itself (B.5 trap 4).
func (s *Service) audioSource(ctx context.Context, res *contract.Resource) (string, string) {
	if renditions, err := s.repo.ListReadyRenditionsByResourceID(ctx, res.ID); err == nil {
		for _, r := range renditions {
			if r.Kind == domain.RenditionKindAudioWeb && r.ObjectKey != nil && *r.ObjectKey != "" {
				return storage.BucketDerived, *r.ObjectKey
			}
		}
	}
	if domain.IsAudioMIME(res.DetectedMIME) && res.ObjectKey != nil && *res.ObjectKey != "" {
		return storage.BucketUploads, *res.ObjectKey
	}
	return "", ""
}

// ClassifyResource classifies extracted resource text to estimate CEFR and map spine nodes.
func (s *Service) ClassifyResource(ctx context.Context, resourceID uuid.UUID) error {
	// Idempotency: skip if classification already exists
	existing, err := s.repo.GetClassificationByResourceID(ctx, resourceID)
	if err == nil && existing != nil {
		return nil
	}

	ext, err := s.repo.GetExtractionByResourceID(ctx, resourceID)
	if err != nil {
		// A database error is the job's to retry; answering nil would mark the
		// resource done and it would never be classified. No row is (nil, nil).
		return fmt.Errorf("get extraction for %s: %w", resourceID, err)
	}
	if ext == nil || ext.CharCount == 0 || strings.TrimSpace(ext.Text) == "" || s.aiClient == nil {
		return nil
	}

	// 1. The classifier reads the first 8 000 characters (B.3).
	content, _ := truncateRunes(ext.Text, classifierChars)

	// 2. Grounding: the model may only answer with spine codes that exist.
	allowed, allowedList := s.allowedSpineCodes(ctx)

	var parsed struct {
		CEFREstimate string   `json:"cefr_estimate"`
		Skill        string   `json:"skill"`
		NodeCodes    []string `json:"node_codes"`
	}
	aiResp, err := ai.CompleteJSONWithResponse(ctx, s.aiClient, ai.Request{
		Task: ai.TaskResourceClassify,
		Vars: map[string]any{
			"Content":      content,
			"AllowedNodes": strings.Join(allowedList, ", "),
		},
	}, &parsed)
	if err != nil {
		return fmt.Errorf("generate classification: %w", err)
	}

	// Drop any code not in the spine (BR: classification stores only codes that exist).
	grounded := make([]string, 0, len(parsed.NodeCodes))
	for _, code := range parsed.NodeCodes {
		if trimmed := strings.TrimSpace(code); allowed[trimmed] {
			grounded = append(grounded, trimmed)
		}
	}

	var cefr *string
	switch level := strings.ToUpper(strings.TrimSpace(parsed.CEFREstimate)); level {
	case "A1", "A2", "B1", "B2", "C1", "C2":
		cefr = &level
	}
	var skill *string
	if sk := strings.ToLower(strings.TrimSpace(parsed.Skill)); sk != "" {
		skill = &sk
	}
	model := aiResp.Model
	if model == "" {
		model = "ai"
	}

	// ai_request_id stays null: platform/ai does not return the request row's id.
	if _, err := s.repo.UpsertClassification(
		ctx, resourceID, cefr, skill, grounded, classifyPromptVer, model, nil,
	); err != nil {
		return fmt.Errorf("save classification: %w", err)
	}
	return nil
}

// allowedSpineCodes lists the codes of the namespaces the classifier may use.
func (s *Service) allowedSpineCodes(ctx context.Context) (map[string]bool, []string) {
	allowed := map[string]bool{}
	var list []string
	if s.taxonomies == nil {
		return allowed, list
	}
	for _, ns := range []string{"grammar", "topic", "skill"} {
		nodes, err := s.taxonomies.ListTaxonomiesInNamespace(ctx, ns)
		if err != nil {
			continue
		}
		for _, n := range nodes {
			if !allowed[n.Code] {
				allowed[n.Code] = true
				list = append(list, n.Code)
			}
		}
	}
	return allowed, list
}

func (s *Service) deleteStorageObject(ctx context.Context, bucket, key string) error {
	if err := s.storage.Delete(ctx, bucket, key); err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
		return err
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

	head, err := readHead(rc)
	if err != nil {
		return err
	}

	detectedMIME, err := domain.SniffAndValidateMIME(res.DeclaredMIME, head)
	if err != nil {
		return s.reject(ctx, res, domain.ReasonTypeNotSupported, err)
	}

	checksum, err := checksumOf(head, rc)
	if err != nil {
		return err
	}

	if _, err := s.repo.UpdateValidationSuccess(ctx, res.ID, res.Title, detectedMIME, stat.Size, checksum); err != nil {
		return fmt.Errorf("save validation success: %w", err)
	}

	return s.afterValidated(ctx, res.ID, detectedMIME)
}

// afterValidated plans a validated file's renditions and asks for them.
func (s *Service) afterValidated(ctx context.Context, resourceID uuid.UUID, detectedMIME string) error {
	for _, k := range domain.PlannedRenditionsForMIME(detectedMIME) {
		_, _ = s.repo.InsertRenditionPending(ctx, resourceID, k)
	}

	// Dispatch render workflow in Actions (WO-18 §8). cmd/media queues the
	// transcription once audio_web settles.
	if s.mediaRender != nil {
		if err := s.mediaRender.RequestRender(ctx); err != nil {
			slog.WarnContext(ctx, "could not request media render dispatch", "resource_id", resourceID, "error", err)
		}
		return nil
	}
	// No renderer attached (tests, a direct audio file): transcribe the upload.
	if domain.IsAudioMIME(detectedMIME) {
		return s.enqueue(ctx, job.TranscribeResourceArgs{ResourceID: resourceID})
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

// readHead reads the first 1 KiB, which is all type sniffing needs.
func readHead(rc io.Reader) ([]byte, error) {
	head := make([]byte, 1024)
	n, err := io.ReadFull(rc, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("read header bytes: %w", err)
	}
	return head[:n], nil
}

// checksumOf is the sha256 of head followed by the rest of the stream.
func checksumOf(head []byte, rest io.Reader) (string, error) {
	hasher := sha256.New()
	hasher.Write(head)
	if _, err := io.Copy(hasher, rest); err != nil {
		return "", fmt.Errorf("compute checksum: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
