package main

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/fluentra/fluentra/internal/platform/storage"
)

// defaultBannerFixtureDir holds the dashboard slides tools/banners renders.
const defaultBannerFixtureDir = "db/fixtures/banners"

// seedSlide is one promotional slide in the dashboard carousel.
//
// The ids are fixed so a re-run finds the same rows and the same objects. The
// object key follows ck_banners_image_key: banners/images/<uuid>.jpg.
type seedSlide struct {
	ID       uuid.UUID
	File     string
	Title    string
	Alt      string
	Link     string
	Priority int
}

var seedSlides = []seedSlide{
	{
		ID:       uuid.MustParse("0f1a2b3c-0001-4a00-8000-00000000b001"),
		File:     "slide-1.jpg",
		Title:    "Học chắc nền tảng, mỗi ngày một bài",
		Alt:      "13 khóa học nền tảng: học chắc nền tảng, mỗi ngày một bài. Bắt đầu học.",
		Link:     "/learn",
		Priority: 30,
	},
	{
		ID:       uuid.MustParse("0f1a2b3c-0002-4a00-8000-00000000b002"),
		File:     "slide-2.jpg",
		Title:    "Từ vựng của bạn, nhớ lâu hơn",
		Alt:      "Ôn tập ngắt quãng: thêm từ vựng của bạn và nhớ lâu hơn.",
		Link:     "/practice/my-words",
		Priority: 20,
	},
	{
		ID:       uuid.MustParse("0f1a2b3c-0003-4a00-8000-00000000b003"),
		File:     "slide-3.jpg",
		Title:    "Luyện đề sát thật, xem điểm từng phần",
		Alt:      "Luyện đề IELTS, TOEIC và VSTEP, xem điểm từng phần. Luyện đề ngay.",
		Link:     "/exams",
		Priority: 10,
	},
}

func (s seedSlide) objectKey() string { return "banners/images/" + s.ID.String() + ".jpg" }

// slideImage is a fixture slide read from disk.
type slideImage struct {
	data          []byte
	width, height int
}

// readSlideImage reads a rendered slide and checks it is what core.banners
// accepts: a JPEG inside 1600×900 (ck_banners_image_size).
func readSlideImage(dir, file string) (slideImage, error) {
	data, err := os.ReadFile(filepath.Join(dir, file)) //nolint:gosec // a repository fixture path
	if err != nil {
		return slideImage{}, err
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return slideImage{}, fmt.Errorf("%s is not a JPEG: %w", file, err)
	}
	if cfg.Width < 1 || cfg.Width > 1600 || cfg.Height < 1 || cfg.Height > 900 {
		return slideImage{}, fmt.Errorf("%s is %dx%d; a banner image must fit 1600x900", file, cfg.Width, cfg.Height)
	}
	return slideImage{data: data, width: cfg.Width, height: cfg.Height}, nil
}

// objectPutter is the one storage call the slides need.
type objectPutter interface {
	Put(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error
}

// seedBannerSlides uploads the three dashboard slides and creates their rows.
//
// A first run creates each slide switched on. A later run only replaces the
// picture, so a re-render reaches every environment, and leaves the rest as
// the admin screen left it: a slide switched off, retitled or reordered there
// stays that way.
func seedBannerSlides(
	ctx context.Context, pool *pgxpool.Pool, store objectPutter, adminID uuid.UUID, dir string, out io.Writer,
) error {
	const upsert = `
		INSERT INTO core.banners (
			id, kind, title, body, cta_label, cta_url, tone, is_active, priority, dismissible,
			image_key, image_alt, image_width, image_height, created_by)
		VALUES ($1, 'slide', $2, '', NULL, $3, 'promo', true, $4, false, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE
		SET image_key = EXCLUDED.image_key,
		    image_width = EXCLUDED.image_width,
		    image_height = EXCLUDED.image_height,
		    updated_at = now()
		RETURNING (xmax = 0)`

	created := 0
	for _, slide := range seedSlides {
		img, err := readSlideImage(dir, slide.File)
		if err != nil {
			return fmt.Errorf("read slide %s: %w", slide.File, err)
		}
		if err := store.Put(ctx, storage.BucketMedia, slide.objectKey(),
			bytes.NewReader(img.data), int64(len(img.data)), "image/jpeg"); err != nil {
			return fmt.Errorf("upload slide %s: %w", slide.File, err)
		}
		var inserted bool
		if err := pool.QueryRow(ctx, upsert,
			slide.ID, slide.Title, slide.Link, slide.Priority, slide.objectKey(), slide.Alt,
			img.width, img.height, adminID,
		).Scan(&inserted); err != nil {
			return fmt.Errorf("save slide %s: %w", slide.File, err)
		}
		if inserted {
			created++
		}
	}
	_, _ = fmt.Fprintf(out, "  ✓ Banners: %d dashboard slide(s), %d new\n", len(seedSlides), created)
	return nil
}

// newSeedStore connects to the object store the API serves banner images from.
func newSeedStore(cfg seedConfig) (*storage.MinIOStore, error) {
	endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.Storage.Endpoint, "https://"), "http://")
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""),
		Secure: cfg.Storage.UseSSL,
		Region: cfg.Storage.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}
	if cfg.Storage.UsePostPolicy {
		return storage.NewMinIOStore(client), nil
	}
	return storage.NewMinIOStoreNoPostPolicy(client), nil
}
