package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp" // register the WebP decoder for image.Decode

	admindomain "github.com/fluentra/fluentra/internal/modules/admin/domain"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Banner image limits. The processed size is repeated by the CHECK on
// core.banners and by the OpenAPI description.
const (
	bannerImageMaxBytes     int64 = 5 << 20
	bannerImageUploadExpiry       = 10 * time.Minute
	// bannerImageURLExpiry outlives the active-banner cache many times over, so
	// a URL signed when the cache filled is still good when it is served.
	bannerImageURLExpiry = time.Hour
	// bannerImageMaxPixels refuses decompression bombs before decoding: a 5 MB
	// PNG can claim a canvas that would need gigabytes of memory.
	bannerImageMaxPixels = 40_000_000
	bannerImageMaxWidth  = 1600
	bannerImageMaxHeight = 900
	bannerImageQuality   = 85

	bannerRawPrefix   = "banners/raw/"
	bannerImagePrefix = "banners/images/"
	bannerImageMime   = "image/jpeg"
)

// bannerImageTypes are the formats an administrator may upload; every one is
// re-encoded as JPEG.
var bannerImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// BannerImageStore is the slice of object storage banner images need.
type BannerImageStore interface {
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

var (
	errBannerImagesUnavailable = apperr.New(
		apperr.Unavailable, "BANNER_IMAGES_UNAVAILABLE", "Banner images are not available right now.")

	errBannerImageType = apperr.New(
		apperr.Validation, "BANNER_IMAGE_INVALID", "The banner image is invalid.",
	).WithFields(apperr.FieldViolation{
		Field: "content_type", Code: "unsupported", Message: "Banner images must be JPEG, PNG or WebP.",
	})

	errBannerImageRejected = apperr.New(
		apperr.Validation, "BANNER_IMAGE_INVALID", "The banner image is invalid.",
	).WithFields(apperr.FieldViolation{
		Field:   "object_key",
		Code:    "rejected",
		Message: "The upload is not a JPEG, PNG or WebP image of at most 5 MB and 40 megapixels.",
	})

	errBannerImageMissing = apperr.New(
		apperr.Validation, "BANNER_INVALID", "The banner has invalid fields.",
	).WithFields(apperr.FieldViolation{
		Field: "image_key", Code: "not_found", Message: "The image was not found; upload it again.",
	})
)

// RequestBannerImageUpload issues a presigned upload into the holding area.
// The key carries the administrator's id so that only they can process it.
func (s *Service) RequestBannerImageUpload(
	ctx context.Context, actorID uuid.UUID, contentType string,
) (admindomain.BannerImageUpload, error) {
	if s.images == nil || s.banners == nil {
		return admindomain.BannerImageUpload{}, errBannerImagesUnavailable
	}
	if !bannerImageTypes[contentType] {
		return admindomain.BannerImageUpload{}, errBannerImageType
	}
	id, err := uuid.NewV7()
	if err != nil {
		return admindomain.BannerImageUpload{}, fmt.Errorf("banner image id: %w", err)
	}
	key := bannerRawPrefix + actorID.String() + "/" + id.String()
	intent, err := s.images.PresignPut(
		ctx, storage.BucketMedia, key, contentType, bannerImageMaxBytes, bannerImageUploadExpiry)
	if err != nil {
		return admindomain.BannerImageUpload{}, errBannerImagesUnavailable.WithCause(err)
	}
	return admindomain.BannerImageUpload{
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

// ProcessBannerImage checks an upload's bytes, orients it, scales it to fit
// 1600x900 and re-encodes it as JPEG, which drops all metadata. The raw upload
// is deleted whatever the outcome. The result belongs to no banner until a
// create or update names its key.
func (s *Service) ProcessBannerImage(
	ctx context.Context, actorID uuid.UUID, objectKey string,
) (admindomain.BannerImage, error) {
	if s.images == nil || s.banners == nil {
		return admindomain.BannerImage{}, errBannerImagesUnavailable
	}
	if !isOwnRawBannerKey(actorID, objectKey) {
		return admindomain.BannerImage{}, errBannerImageRejected
	}
	defer func() { _ = s.images.Delete(context.WithoutCancel(ctx), storage.BucketMedia, objectKey) }()

	stat, err := s.images.VerifyUpload(ctx, storage.BucketMedia, objectKey, "", bannerImageMaxBytes)
	if err != nil || !bannerImageTypes[stat.SniffedContentType] {
		return admindomain.BannerImage{}, errBannerImageRejected.WithCause(err)
	}

	encoded, width, height, err := s.readAndEncodeBannerImage(ctx, objectKey)
	if err != nil {
		return admindomain.BannerImage{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return admindomain.BannerImage{}, fmt.Errorf("banner image id: %w", err)
	}
	key := bannerImagePrefix + id.String() + ".jpg"
	if err := s.images.Put(
		ctx, storage.BucketMedia, key, bytes.NewReader(encoded), int64(len(encoded)), bannerImageMime,
	); err != nil {
		return admindomain.BannerImage{}, errBannerImagesUnavailable.WithCause(err)
	}

	processed := admindomain.BannerImage{Key: key, Width: width, Height: height}
	processed.URL, _ = s.images.PresignGet(ctx, storage.BucketMedia, key, bannerImageURLExpiry)
	return processed, nil
}

// isOwnRawBannerKey accepts exactly banners/raw/<actor>/<uuid>.
func isOwnRawBannerKey(actorID uuid.UUID, key string) bool {
	rest, ok := strings.CutPrefix(key, bannerRawPrefix+actorID.String()+"/")
	if !ok {
		return false
	}
	_, err := uuid.Parse(rest)
	return err == nil && len(rest) == 36
}

func (s *Service) readAndEncodeBannerImage(ctx context.Context, key string) ([]byte, int, int, error) {
	stream, err := s.images.Get(ctx, storage.BucketMedia, key)
	if err != nil {
		return nil, 0, 0, errBannerImagesUnavailable.WithCause(err)
	}
	defer func() { _ = stream.Close() }()

	raw, err := io.ReadAll(io.LimitReader(stream, bannerImageMaxBytes+1))
	if err != nil {
		return nil, 0, 0, errBannerImagesUnavailable.WithCause(err)
	}
	if int64(len(raw)) > bannerImageMaxBytes {
		return nil, 0, 0, errBannerImageRejected
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width <= 0 || config.Height <= 0 ||
		config.Width*config.Height > bannerImageMaxPixels {
		return nil, 0, 0, errBannerImageRejected.WithCause(err)
	}

	img, err := imaging.Decode(bytes.NewReader(raw), imaging.AutoOrientation(true))
	if err != nil {
		return nil, 0, 0, errBannerImageRejected.WithCause(err)
	}

	// Fit only ever scales down. JPEG has no alpha, so a transparent PNG is
	// laid on white rather than left to turn black.
	fitted := imaging.Fit(img, bannerImageMaxWidth, bannerImageMaxHeight, imaging.Lanczos)
	bounds := fitted.Bounds()
	flat := imaging.Overlay(
		imaging.New(bounds.Dx(), bounds.Dy(), color.White), fitted, image.Pt(0, 0), 1.0)

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, flat, imaging.JPEG, imaging.JPEGQuality(bannerImageQuality)); err != nil {
		return nil, 0, 0, errBannerImageRejected.WithCause(err)
	}
	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

// resolveBannerImage turns the submitted image key into the image to store.
// A key the banner already shows keeps its recorded size; a new one is read
// back from storage, which proves it exists and was processed here.
func (s *Service) resolveBannerImage(
	ctx context.Context, in admindomain.BannerInput, current *admindomain.BannerImage,
) (*admindomain.BannerImage, error) {
	if in.ImageKey == nil {
		return nil, nil
	}
	if current != nil && current.Key == *in.ImageKey {
		kept := *current
		kept.Alt = in.ImageAlt
		kept.URL = ""
		return &kept, nil
	}
	if s.images == nil {
		return nil, errBannerImagesUnavailable
	}

	stream, err := s.images.Get(ctx, storage.BucketMedia, *in.ImageKey)
	if err != nil {
		return nil, errBannerImageMissing.WithCause(err)
	}
	defer func() { _ = stream.Close() }()

	config, _, err := image.DecodeConfig(stream)
	if err != nil {
		return nil, errBannerImageMissing.WithCause(err)
	}
	if config.Width < 1 || config.Width > bannerImageMaxWidth ||
		config.Height < 1 || config.Height > bannerImageMaxHeight {
		return nil, errBannerImageMissing
	}
	return &admindomain.BannerImage{
		Key:    *in.ImageKey,
		Alt:    in.ImageAlt,
		Width:  config.Width,
		Height: config.Height,
	}, nil
}

// deleteReplacedBannerImage removes an image its banner no longer shows. A
// failure is swallowed: the banner change is committed, and refusing it over a
// stray object would trade a storage leak for a lost edit.
func (s *Service) deleteReplacedBannerImage(
	ctx context.Context, previous *admindomain.BannerImage, kept *admindomain.BannerImage,
) {
	if previous == nil || s.images == nil {
		return
	}
	if kept != nil && kept.Key == previous.Key {
		return
	}
	_ = s.images.Delete(context.WithoutCancel(ctx), storage.BucketMedia, previous.Key)
}

// withImageURLs returns copies of banners with a signed URL on each image. The
// input is never modified: it may be the shared active-banner cache. An image
// that cannot be signed is dropped rather than served as a broken picture.
func (s *Service) withImageURLs(ctx context.Context, banners []admindomain.Banner) []admindomain.Banner {
	out := make([]admindomain.Banner, len(banners))
	copy(out, banners)
	for i := range out {
		if out[i].Image == nil {
			continue
		}
		img := *out[i].Image
		img.URL = ""
		if s.images != nil {
			if url, err := s.images.PresignGet(ctx, storage.BucketMedia, img.Key, bannerImageURLExpiry); err == nil {
				img.URL = url
			}
		}
		out[i].Image = &img
	}
	return out
}

// bannerWithImageURL signs one banner's image.
func (s *Service) bannerWithImageURL(ctx context.Context, banner admindomain.Banner) admindomain.Banner {
	return s.withImageURLs(ctx, []admindomain.Banner{banner})[0]
}
