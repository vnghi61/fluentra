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

// AvatarSize defines a target dimension and key suffix for avatar variants.
type AvatarSize struct {
	Width  int
	Height int
	Suffix string
}

// AvatarSizes defines the three required avatar dimensions (P3.1).
var AvatarSizes = []AvatarSize{
	{Width: 64, Height: 64, Suffix: "sm"},
	{Width: 128, Height: 128, Suffix: "md"},
	{Width: 256, Height: 256, Suffix: "lg"},
}

// AvatarVariant holds the encoded buffer for a single size variant.
type AvatarVariant struct {
	Suffix string
	Buffer *bytes.Buffer
}

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
// re-encodes it to three pure-Go JPEG sizes, publishes to the avatars bucket, updates
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

	variants, err := s.fetchAndProcessAvatar(ctx, objectKey)
	if err != nil {
		return Account{}, err
	}

	newAssetID, avatarAssets, err := s.storeProcessedAvatar(ctx, actorID, variants)
	if err != nil {
		return Account{}, err
	}

	existingProfile, err := s.repo.GetProfile(ctx, actorID)
	if err != nil {
		s.cleanupAssets(ctx, avatarAssets)
		return Account{}, err
	}

	updatedProfile, err := s.commitAvatarUpdate(ctx, actorID, newAssetID, avatarAssets)
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

func (s *Service) fetchAndProcessAvatar(ctx context.Context, objectKey string) ([]AvatarVariant, error) {
	stream, err := s.storage.Get(ctx, storage.BucketAvatars, objectKey)
	if err != nil {
		return nil, fmt.Errorf("read uploaded avatar: %w", err)
	}
	defer func() { _ = stream.Close() }()

	img, err := imaging.Decode(stream, imaging.AutoOrientation(true))
	if err != nil {
		return nil, domain.ErrAvatarProcessingFailed.WithCause(err)
	}

	variants := make([]AvatarVariant, 0, len(AvatarSizes))
	for _, sz := range AvatarSizes {
		resized := imaging.Fill(img, sz.Width, sz.Height, imaging.Center, imaging.Lanczos)
		var buf bytes.Buffer
		if err := imaging.Encode(&buf, resized, imaging.JPEG, imaging.JPEGQuality(85)); err != nil {
			return nil, domain.ErrAvatarProcessingFailed.WithCause(err)
		}
		variants = append(variants, AvatarVariant{
			Suffix: sz.Suffix,
			Buffer: &buf,
		})
	}
	return variants, nil
}

// storeProcessedAvatar writes each variant and reports where it put them.
//
// It used to return bare keys, which the caller used only to undo the writes if
// the transaction failed. The keys were then dropped -- and with them the only
// record of where the bytes were, since the key embeds the year and month of
// the upload and nothing else remembers those. Returning assets rather than
// keys is what lets commitAvatarUpdate write that record down.
func (s *Service) storeProcessedAvatar(
	ctx context.Context, actorID uuid.UUID, variants []AvatarVariant,
) (uuid.UUID, []domain.AvatarAsset, error) {
	newAssetID, err := s.newID(ctx)
	if err != nil {
		return uuid.Nil, nil, err
	}

	stored := make([]domain.AvatarAsset, 0, len(variants))
	now := s.clock.Now()
	for _, v := range variants {
		keyAssetID := fmt.Sprintf("%s_%s", newAssetID.String(), v.Suffix)
		avatarKey, err := storage.BuildKey("users", actorID.String(), now, keyAssetID, "jpg")
		if err != nil {
			s.cleanupAssets(ctx, stored)
			return uuid.Nil, nil, fmt.Errorf("build avatar key: %w", err)
		}

		size := int64(v.Buffer.Len())
		if err := s.storage.Put(
			ctx, storage.BucketAvatars, avatarKey, bytes.NewReader(v.Buffer.Bytes()), size, avatarImageMime,
		); err != nil {
			s.cleanupAssets(ctx, stored)
			return uuid.Nil, nil, fmt.Errorf("store processed avatar: %w", err)
		}
		stored = append(stored, domain.AvatarAsset{
			AssetID:   newAssetID,
			Variant:   domain.AvatarVariant(v.Suffix),
			UserID:    actorID,
			ObjectKey: avatarKey,
			MimeType:  avatarImageMime,
			ByteSize:  size,
		})
	}
	return newAssetID, stored, nil
}

// cleanupAssets removes objects written by a confirmation that then failed.
func (s *Service) cleanupAssets(ctx context.Context, assets []domain.AvatarAsset) {
	for _, asset := range assets {
		_ = s.storage.Delete(ctx, storage.BucketAvatars, asset.ObjectKey)
	}
}

func (s *Service) commitAvatarUpdate(
	ctx context.Context, actorID, newAssetID uuid.UUID, avatarAssets []domain.AvatarAsset,
) (domain.Profile, error) {
	var updatedProfile domain.Profile
	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		up, updateErr := repo.UpdateProfileAvatar(ctx, actorID, &newAssetID)
		if updateErr != nil {
			return updateErr
		}
		updatedProfile = up

		// In the same transaction as the profile pointer. A profile naming an
		// asset id with no rows behind it is precisely the state that made
		// every avatar_url a 404, and it must not be reachable by a partial
		// write either.
		for _, asset := range avatarAssets {
			if insertErr := repo.InsertAvatarAsset(ctx, asset); insertErr != nil {
				return insertErr
			}
		}

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
		s.cleanupAssets(ctx, avatarAssets)
		return domain.Profile{}, err
	}
	return updatedProfile, nil
}

// cleanupOldAvatar deletes the objects the replaced avatar left behind.
//
// It used to rebuild the old keys from `existingProfile.UpdatedAt`, which is a
// guess and is wrong whenever anything else touched the profile in between --
// change a display name, then upload a new avatar, and the delete was aimed at
// a month the file was never in. The old objects then stayed in the bucket for
// ever, paid for and unreachable.
//
// Now the keys are read back from the rows that recorded them. A failure to
// delete is still swallowed: the new avatar is already committed, and refusing
// the upload because an old file could not be tidied would trade a storage leak
// for a lost change.
func (s *Service) cleanupOldAvatar(
	ctx context.Context, _ uuid.UUID, existingProfile domain.Profile, newAssetID uuid.UUID,
) {
	oldAssetID := existingProfile.AvatarAssetID
	if oldAssetID == nil || *oldAssetID == newAssetID {
		return
	}
	for _, sz := range AvatarSizes {
		asset, err := s.repo.GetAvatarAsset(ctx, *oldAssetID, domain.AvatarVariant(sz.Suffix))
		if err != nil {
			continue
		}
		_ = s.storage.Delete(ctx, storage.BucketAvatars, asset.ObjectKey)
	}
	_ = s.repo.DeleteAvatarAssetsByAssetID(ctx, *oldAssetID)
}

// AvatarBlob opens the stored avatar behind an asset id.
//
// Any signed-in learner may read any avatar. That is deliberate and it is what
// the leaderboard needs: a ranking that shows forty faces cannot ask forty
// permission questions, and an avatar is already shown to everyone the learner
// competes with. The bytes are proxied rather than redirected to a presigned
// URL so that no bucket URL, signed or otherwise, ever reaches the browser.
//
// The caller owns the reader and must close it.
func (s *Service) AvatarBlob(
	ctx context.Context, assetID uuid.UUID, variant domain.AvatarVariant,
) (io.ReadCloser, domain.AvatarAsset, error) {
	if s.storage == nil {
		return nil, domain.AvatarAsset{}, apperr.New(
			apperr.Unavailable, "STORAGE_UNAVAILABLE", "Storage service is not configured.")
	}

	asset, err := s.repo.GetAvatarAsset(ctx, assetID, variant)
	if err != nil {
		return nil, domain.AvatarAsset{}, err
	}

	body, err := s.storage.Get(ctx, storage.BucketAvatars, asset.ObjectKey)
	if err != nil {
		// The row says the object is there and the bucket disagrees. That is a
		// 404 to the caller -- there is no picture -- but it is not the same
		// event as an unknown id, and the cause travels with it.
		return nil, domain.AvatarAsset{}, domain.ErrAvatarAssetNotFound.WithCause(err)
	}
	return body, asset, nil
}
