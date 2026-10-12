package service_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	admindomain "github.com/fluentra/fluentra/internal/modules/admin/domain"
	adminsvc "github.com/fluentra/fluentra/internal/modules/admin/service"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// fakeImageStore keeps objects in memory and sniffs them the way the real
// store does, from the bytes rather than the declared type.
type fakeImageStore struct {
	objects map[string][]byte
	deleted []string
	signed  int
	gets    int
}

func newFakeImageStore() *fakeImageStore {
	return &fakeImageStore{objects: map[string][]byte{}}
}

func (f *fakeImageStore) PresignPut(
	_ context.Context, bucket, key, contentType string, maxBytes int64, expiry time.Duration,
) (storage.UploadIntent, error) {
	return storage.UploadIntent{
		URL: "https://storage.test/" + bucket, Method: http.MethodPost, ObjectKey: key,
		ExpiresAt: bannerNow.Add(expiry), MaxBytes: maxBytes, ContentType: contentType,
	}, nil
}

func (f *fakeImageStore) VerifyUpload(
	_ context.Context, _, key, _ string, maxBytes int64,
) (storage.ObjectStat, error) {
	data, ok := f.objects[key]
	if !ok {
		return storage.ObjectStat{}, errors.New("no such object")
	}
	if int64(len(data)) > maxBytes {
		return storage.ObjectStat{}, errors.New("too large")
	}
	return storage.ObjectStat{Key: key, Size: int64(len(data)), SniffedContentType: http.DetectContentType(data)}, nil
}

func (f *fakeImageStore) Get(_ context.Context, _, key string) (io.ReadCloser, error) {
	f.gets++
	data, ok := f.objects[key]
	if !ok {
		return nil, errors.New("no such object")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeImageStore) Put(_ context.Context, _, key string, reader io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	f.objects[key] = data
	return nil
}

func (f *fakeImageStore) Delete(_ context.Context, _, key string) error {
	f.deleted = append(f.deleted, key)
	delete(f.objects, key)
	return nil
}

func (f *fakeImageStore) PresignGet(_ context.Context, _, key string, _ time.Duration) (string, error) {
	f.signed++
	return "https://storage.test/signed/" + key, nil
}

func newImageService(repo *fakeBannerRepo, store *fakeImageStore) *adminsvc.Service {
	return adminsvc.New(adminsvc.Deps{
		Banners: repo, Images: store, Clock: clock.NewFake(bannerNow), Audit: &fakeRecorder{},
	})
}

func pngBytes(t *testing.T, width, height int, fill color.Color) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// pngHeaderOnly is a PNG that claims a canvas of width x height and carries
// no pixels: the shape of a decompression bomb, without the memory.
func pngHeaderOnly(width, height uint32) []byte {
	var buf bytes.Buffer
	buf.Write([]byte("\x89PNG\r\n\x1a\n"))
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], width)
	binary.BigEndian.PutUint32(ihdr[4:], height)
	ihdr[8], ihdr[9] = 8, 6 // 8-bit RGBA
	chunk := append([]byte("IHDR"), ihdr...)
	_ = binary.Write(&buf, binary.BigEndian, uint32(13)) // IHDR length
	buf.Write(chunk)
	_ = binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return buf.Bytes()
}

func rawKey(actor uuid.UUID) string {
	return "banners/raw/" + actor.String() + "/" + uuid.NewString()
}

func TestRequestBannerImageUpload_ScopesTheKeyToTheActor(t *testing.T) {
	store := newFakeImageStore()
	svc := newImageService(&fakeBannerRepo{}, store)
	actor := uuid.New()

	intent, err := svc.RequestBannerImageUpload(context.Background(), actor, "image/webp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(intent.ObjectKey, "banners/raw/"+actor.String()+"/") ||
		intent.MaxBytes != 5<<20 || intent.ContentType != "image/webp" {
		t.Fatalf("intent = %+v", intent)
	}

	for _, refused := range []string{"image/gif", "image/svg+xml", "text/html", ""} {
		if _, err := svc.RequestBannerImageUpload(context.Background(), actor, refused); !apperr.Is(err, apperr.Validation) {
			t.Fatalf("%q: err = %v, want a validation error", refused, err)
		}
	}
}

func TestProcessBannerImage_ScalesFlattensAndDeletesTheRaw(t *testing.T) {
	store := newFakeImageStore()
	svc := newImageService(&fakeBannerRepo{}, store)
	actor := uuid.New()
	key := rawKey(actor)
	// Transparent: a JPEG has no alpha, and the result must be white, not black.
	store.objects[key] = pngBytes(t, 3200, 1200, color.NRGBA{})

	got, err := svc.ProcessBannerImage(context.Background(), actor, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Width != 1600 || got.Height != 600 {
		t.Fatalf("size = %dx%d, want 1600x600 (fit, aspect kept)", got.Width, got.Height)
	}
	if !admindomain.IsBannerImageKey(got.Key) || got.URL == "" {
		t.Fatalf("result = %+v", got)
	}
	if _, still := store.objects[key]; still {
		t.Fatalf("raw upload was not deleted")
	}

	decoded, err := jpeg.Decode(bytes.NewReader(store.objects[got.Key]))
	if err != nil {
		t.Fatalf("stored object is not a JPEG: %v", err)
	}
	r, g, b, _ := decoded.At(10, 10).RGBA()
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Fatalf("transparent pixel became (%d,%d,%d), want white", r>>8, g>>8, b>>8)
	}
}

func TestProcessBannerImage_NeverEnlarges(t *testing.T) {
	store := newFakeImageStore()
	svc := newImageService(&fakeBannerRepo{}, store)
	actor := uuid.New()
	key := rawKey(actor)
	store.objects[key] = pngBytes(t, 400, 200, color.White)

	got, err := svc.ProcessBannerImage(context.Background(), actor, key)
	if err != nil || got.Width != 400 || got.Height != 200 {
		t.Fatalf("got %+v, %v; want 400x200 unchanged", got, err)
	}
}

func TestProcessBannerImage_Refusals(t *testing.T) {
	actor := uuid.New()
	cases := map[string]struct {
		key  string
		data []byte
	}{
		"another admin's upload": {key: rawKey(uuid.New()), data: nil},
		"not under raw":          {key: "banners/images/" + uuid.NewString() + ".jpg"},
		"path tricks":            {key: "banners/raw/" + actor.String() + "/../x"},
		"html":                   {key: rawKey(actor), data: []byte("<!doctype html><script>x</script>")},
		"decompression bomb":     {key: rawKey(actor), data: pngHeaderOnly(10000, 5000)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store := newFakeImageStore()
			if tc.data == nil {
				tc.data = pngBytes(t, 10, 10, color.White)
			}
			store.objects[tc.key] = tc.data
			svc := newImageService(&fakeBannerRepo{}, store)

			_, err := svc.ProcessBannerImage(context.Background(), actor, tc.key)
			if !apperr.Is(err, apperr.Validation) {
				t.Fatalf("err = %v, want a validation error", err)
			}
			for stored := range store.objects {
				if stored != tc.key && strings.HasPrefix(stored, "banners/images/") {
					t.Fatalf("a refused upload was published as %s", stored)
				}
			}
		})
	}
}

func processedImage(t *testing.T, store *fakeImageStore, width, height int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, height)), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	key := "banners/images/" + uuid.NewString() + ".jpg"
	store.objects[key] = buf.Bytes()
	return key
}

func TestCreateBanner_ResolvesTheImageFromStorage(t *testing.T) {
	store := newFakeImageStore()
	repo := &fakeBannerRepo{}
	svc := newImageService(repo, store)
	key := processedImage(t, store, 1200, 500)

	banner, err := svc.CreateBanner(context.Background(), uuid.New(), admindomain.BannerInput{
		Title: "t", ImageKey: &key, ImageAlt: "  A class  ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stored := repo.images[0]
	if stored == nil || stored.Key != key || stored.Width != 1200 || stored.Height != 500 || stored.Alt != "A class" {
		t.Fatalf("stored image = %+v", stored)
	}
	if banner.Image == nil || banner.Image.URL == "" {
		t.Fatalf("returned banner has no signed image: %+v", banner.Image)
	}
}

func TestCreateBanner_RefusesAnImageThatIsNotThere(t *testing.T) {
	store := newFakeImageStore()
	repo := &fakeBannerRepo{}
	svc := newImageService(repo, store)
	missing := "banners/images/" + uuid.NewString() + ".jpg"

	_, err := svc.CreateBanner(context.Background(), uuid.New(), admindomain.BannerInput{
		Title: "t", ImageKey: &missing,
	})
	if !apperr.Is(err, apperr.Validation) || len(repo.created) != 0 {
		t.Fatalf("err = %v, created %d; want a validation error and nothing stored", err, len(repo.created))
	}

	foreign := "users/someone/avatar.jpg"
	_, err = svc.CreateBanner(context.Background(), uuid.New(), admindomain.BannerInput{
		Title: "t", ImageKey: &foreign,
	})
	if !apperr.Is(err, apperr.Validation) {
		t.Fatalf("a key outside banners/images was accepted: %v", err)
	}
}

func TestUpdateBanner_ReplacingTheImageDeletesTheOldOne(t *testing.T) {
	store := newFakeImageStore()
	oldKey := processedImage(t, store, 800, 400)
	newKey := processedImage(t, store, 1600, 900)
	existing := admindomain.Banner{
		ID: uuid.New(), Title: "t",
		Image: &admindomain.BannerImage{Key: oldKey, Width: 800, Height: 400},
	}
	repo := &fakeBannerRepo{rows: []admindomain.Banner{existing}}
	svc := newImageService(repo, store)

	if _, err := svc.UpdateBanner(context.Background(), uuid.New(), existing.ID, admindomain.BannerInput{
		Title: "t", ImageKey: &newKey,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != oldKey {
		t.Fatalf("deleted = %v, want only the replaced image", store.deleted)
	}
}

func TestUpdateBanner_KeepingTheImageReusesItsSize(t *testing.T) {
	store := newFakeImageStore()
	key := "banners/images/" + uuid.NewString() + ".jpg" // not in the store: must not be read
	existing := admindomain.Banner{
		ID: uuid.New(), Title: "t",
		Image: &admindomain.BannerImage{Key: key, Alt: "old", Width: 800, Height: 400},
	}
	repo := &fakeBannerRepo{rows: []admindomain.Banner{existing}}
	svc := newImageService(repo, store)

	if _, err := svc.UpdateBanner(context.Background(), uuid.New(), existing.ID, admindomain.BannerInput{
		Title: "t", ImageKey: &key, ImageAlt: "new",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kept := repo.images[0]
	if kept.Width != 800 || kept.Alt != "new" || store.gets != 0 || len(store.deleted) != 0 {
		t.Fatalf("kept = %+v, gets %d, deleted %v", kept, store.gets, store.deleted)
	}
}

func TestUpdateBanner_ClearingAndDeletingRemoveTheImage(t *testing.T) {
	store := newFakeImageStore()
	first := admindomain.Banner{
		ID: uuid.New(), Title: "a",
		Image: &admindomain.BannerImage{Key: processedImage(t, store, 10, 10), Width: 10, Height: 10},
	}
	second := admindomain.Banner{
		ID: uuid.New(), Title: "b",
		Image: &admindomain.BannerImage{Key: processedImage(t, store, 10, 10), Width: 10, Height: 10},
	}
	repo := &fakeBannerRepo{rows: []admindomain.Banner{first, second}}
	svc := newImageService(repo, store)
	ctx := context.Background()

	if _, err := svc.UpdateBanner(ctx, uuid.New(), first.ID, admindomain.BannerInput{Title: "a"}); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if err := svc.DeleteBanner(ctx, uuid.New(), second.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(store.deleted) != 2 || store.deleted[0] != first.Image.Key || store.deleted[1] != second.Image.Key {
		t.Fatalf("deleted = %v", store.deleted)
	}
}

func TestListActiveBanners_SignsOncePerCacheFill(t *testing.T) {
	store := newFakeImageStore()
	banner := liveBanner("with image")
	banner.Image = &admindomain.BannerImage{Key: "banners/images/a.jpg", Width: 10, Height: 10}
	repo := &fakeBannerRepo{rows: []admindomain.Banner{banner}}
	svc := newImageService(repo, store)

	for range 3 {
		got, err := svc.ListActiveBanners(context.Background())
		if err != nil || len(got) != 1 || got[0].Image.URL == "" {
			t.Fatalf("got %+v, %v", got, err)
		}
	}
	if store.signed != 1 {
		t.Fatalf("signed %d times, want once per cache fill", store.signed)
	}
	if repo.rows[0].Image.URL != "" {
		t.Fatalf("signing mutated the repository's row")
	}
}

func TestBannerImages_WithoutStorage(t *testing.T) {
	svc := adminsvc.New(adminsvc.Deps{Banners: &fakeBannerRepo{}})
	_, err := svc.RequestBannerImageUpload(context.Background(), uuid.New(), "image/png")
	if !apperr.Is(err, apperr.Unavailable) {
		t.Fatalf("err = %v, want unavailable", err)
	}
	key := "banners/images/" + uuid.NewString() + ".jpg"
	_, err = svc.CreateBanner(context.Background(), uuid.New(), admindomain.BannerInput{Title: "t", ImageKey: &key})
	if !apperr.Is(err, apperr.Unavailable) {
		t.Fatalf("create with an image and no storage = %v, want unavailable", err)
	}
}
