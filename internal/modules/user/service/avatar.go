package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	_ "golang.org/x/image/webp" // register webp decoder for image.Decode

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

const avatarImageMime = "image/jpeg"

// StorageStore defines the storage capabilities needed for avatar management.
type StorageStore interface {
	PresignPut(
		ctx context.Context, bucket, key, contentType string, maxBytes int64, expiry time.Duration,
	) (storage.UploadIntent, error)
	VerifyUpload(
		ctx context.Context, bucket, key, expectedContentType string, maxBytes int64,
	) (storage.ObjectStat, error)
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	Put(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, bucket, key string) error
	PresignGet(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
}

// RequestAvatarUploadIntent generates a presigned upload policy for an avatar.
func (s *Service) RequestAvatarUploadIntent(
	ctx context.Context, actorID uuid.UUID, contentType string,
) (domain.UploadIntent, error) {
	if s.storage == nil {
		return domain.UploadIntent{}, apperr.New(
			apperr.Unavailable, "STORAGE_UNAVAILABLE", "Storage service is not configured.")
	}

	if _, err := s.requireUsableAccount(ctx, actorID); err != nil {
		return domain.UploadIntent{}, err
	}

	if contentType == "" {
		contentType = domain.DefaultAvatarContentType
	}
	if err := domain.ValidateAvatarContentType(contentType); err != nil {
		return domain.UploadIntent{}, err
	}

	rawID, err := s.newID(ctx)
	if err != nil {
		return domain.UploadIntent{}, err
	}

	rawKey, err := storage.BuildKey("users", actorID.String(), s.clock.Now(), rawID.String(), "raw")
	if err != nil {
		return domain.UploadIntent{}, fmt.Errorf("build raw upload key: %w", err)
	}

	intent, err := s.storage.PresignPut(
		ctx, storage.BucketAvatars, rawKey, contentType, domain.AvatarMaxBytes, domain.AvatarUploadExpiry,
	)
	if err != nil {
		return domain.UploadIntent{}, fmt.Errorf("presign avatar upload: %w", err)
	}
	return domain.UploadIntent{
		URL:         intent.URL,
		Method:      intent.Method,
		FormData:    intent.FormData,
		FileField:   intent.FileField,
		ObjectKey:   intent.ObjectKey,
		ExpiresAt:   intent.ExpiresAt,
		MaxBytes:    intent.MaxBytes,
		ContentType: intent.ContentType,
	}, nil
}

// ConfirmAvatar verifies the uploaded raw image, strips EXIF metadata,
// re-encodes it to thumbnail size, publishes to the avatars bucket, updates
// the profile record in a transaction, and deletes the old avatar.
func (s *Service) ConfirmAvatar(
	ctx context.Context, actorID uuid.UUID, objectKey string,
) (Account, error) {
	if s.storage == nil {
		return Account{}, apperr.New(
			apperr.Unavailable, "STORAGE_UNAVAILABLE", "Storage service is not configured.")
	}

	user, err := s.requireUsableAccount(ctx, actorID)
	if err != nil {
		return Account{}, err
	}

	if err := s.validateRawUpload(ctx, actorID, objectKey); err != nil {
		return Account{}, err
	}

	buf, err := s.fetchAndProcessAvatar(ctx, objectKey)
	if err != nil {
		return Account{}, err
	}

	newAssetID, avatarKey, err := s.storeProcessedAvatar(ctx, actorID, buf)
	if err != nil {
		return Account{}, err
	}

	existingProfile, err := s.repo.GetProfile(ctx, actorID)
	if err != nil {
		_ = s.storage.Delete(ctx, storage.BucketAvatars, avatarKey)
		return Account{}, err
	}

	updatedProfile, err := s.commitAvatarUpdate(ctx, actorID, newAssetID, avatarKey)
	if err != nil {
		return Account{}, err
	}

	// Delete temporary raw upload object.
	_ = s.storage.Delete(ctx, storage.BucketAvatars, objectKey)

	// Delete the old avatar only AFTER the new one is verified and committed.
	s.cleanupOldAvatar(ctx, actorID, existingProfile, newAssetID)

	return Account{
		User:    user,
		Profile: updatedProfile,
	}, nil
}

func (s *Service) validateRawUpload(ctx context.Context, actorID uuid.UUID, objectKey string) error {
	if strings.TrimSpace(objectKey) == "" {
		return apperr.New(apperr.Validation, "VALIDATION_FAILED", "Validation failed.").
			WithFields(apperr.FieldViolation{
				Field: "object_key", Code: "REQUIRED", Message: "Object key is required.",
			})
	}

	expectedPrefix := "users/" + actorID.String() + "/"
	if !strings.HasPrefix(objectKey, expectedPrefix) {
		return domain.ErrAvatarVerificationFailed
	}

	stat, err := s.storage.VerifyUpload(ctx, storage.BucketAvatars, objectKey, "", domain.AvatarMaxBytes)
	if err != nil {
		return domain.ErrAvatarVerificationFailed.WithCause(err)
	}

	if !domain.IsAllowedAvatarContentType(stat.SniffedContentType) {
		return domain.ErrAvatarVerificationFailed
	}
	return nil
}

func (s *Service) fetchAndProcessAvatar(ctx context.Context, objectKey string) (*bytes.Buffer, error) {
	stream, err := s.storage.Get(ctx, storage.BucketAvatars, objectKey)
	if err != nil {
		return nil, fmt.Errorf("read uploaded avatar: %w", err)
	}
	defer func() { _ = stream.Close() }()

	img, err := imaging.Decode(stream, imaging.AutoOrientation(true))
	if err != nil {
		return nil, domain.ErrAvatarProcessingFailed.WithCause(err)
	}

	resized := imaging.Fill(img, 256, 256, imaging.Center, imaging.Lanczos)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, resized, imaging.JPEG, imaging.JPEGQuality(85)); err != nil {
		return nil, domain.ErrAvatarProcessingFailed.WithCause(err)
	}
	return &buf, nil
}

func (s *Service) storeProcessedAvatar(
	ctx context.Context, actorID uuid.UUID, buf *bytes.Buffer,
) (uuid.UUID, string, error) {
	newAssetID, err := s.newID(ctx)
	if err != nil {
		return uuid.Nil, "", err
	}

	avatarKey, err := storage.BuildKey("users", actorID.String(), s.clock.Now(), newAssetID.String(), "jpg")
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("build avatar key: %w", err)
	}

	if err := s.storage.Put(
		ctx, storage.BucketAvatars, avatarKey, bytes.NewReader(buf.Bytes()), int64(buf.Len()), avatarImageMime,
	); err != nil {
		return uuid.Nil, "", fmt.Errorf("store processed avatar: %w", err)
	}
	return newAssetID, avatarKey, nil
}

func (s *Service) commitAvatarUpdate(
	ctx context.Context, actorID, newAssetID uuid.UUID, avatarKey string,
) (domain.Profile, error) {
	var updatedProfile domain.Profile
	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		up, updateErr := repo.UpdateProfileAvatar(ctx, actorID, &newAssetID)
		if updateErr != nil {
			return updateErr
		}
		updatedProfile = up

		_, eventErr := s.events.Write(ctx, tx, contract.Aggregate, contract.EventProfileUpdated,
			contract.ProfileUpdated{
				UserID:        actorID,
				ChangedFields: []string{"avatar_asset_id"},
				ActorID:       actorID,
				OccurredAt:    s.clock.Now(),
			})
		return eventErr
	})
	if err != nil {
		_ = s.storage.Delete(ctx, storage.BucketAvatars, avatarKey)
		return domain.Profile{}, err
	}
	return updatedProfile, nil
}

func (s *Service) cleanupOldAvatar(
	ctx context.Context, actorID uuid.UUID, existingProfile domain.Profile, newAssetID uuid.UUID,
) {
	oldAssetID := existingProfile.AvatarAssetID
	if oldAssetID == nil || *oldAssetID == newAssetID {
		return
	}
	oldKey, buildErr := storage.BuildKey("users", actorID.String(), existingProfile.UpdatedAt, oldAssetID.String(), "jpg")
	if buildErr == nil {
		_ = s.storage.Delete(ctx, storage.BucketAvatars, oldKey)
	}
}
