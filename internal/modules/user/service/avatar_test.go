package service_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/modules/user/service"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const mimeImageJPEG = "image/jpeg"

type fakeStorageStore struct {
	mu sync.Mutex

	objects map[string][]byte
	stats   map[string]storage.ObjectStat
	deleted []string

	failPresignPut error
	failVerify     error
	failGet        error
	failPut        error
	failDelete     error
}

func newFakeStorageStore() *fakeStorageStore {
	return &fakeStorageStore{
		objects: make(map[string][]byte),
		stats:   make(map[string]storage.ObjectStat),
		deleted: make([]string, 0),
	}
}

func (s *fakeStorageStore) PresignPut(
	_ context.Context, bucket, key, contentType string, maxBytes int64, expiry time.Duration,
) (storage.UploadIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failPresignPut != nil {
		return storage.UploadIntent{}, s.failPresignPut
	}
	return storage.UploadIntent{
		URL:         "https://storage.local/" + bucket + "/" + key,
		Method:      "POST",
		ObjectKey:   key,
		ExpiresAt:   time.Now().Add(expiry),
		MaxBytes:    maxBytes,
		ContentType: contentType,
	}, nil
}

func (s *fakeStorageStore) VerifyUpload(
	_ context.Context, _, key, _ string, maxBytes int64,
) (storage.ObjectStat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failVerify != nil {
		return storage.ObjectStat{}, s.failVerify
	}
	stat, ok := s.stats[key]
	if !ok {
		data, exists := s.objects[key]
		if !exists {
			return storage.ObjectStat{}, storage.ErrObjectNotFound
		}
		stat = storage.ObjectStat{
			Key:                key,
			Size:               int64(len(data)),
			SniffedContentType: sniffMimeType(data),
		}
	}
	if maxBytes > 0 && stat.Size > maxBytes {
		return stat, storage.ErrSizeMismatch
	}
	return stat, nil
}

func (s *fakeStorageStore) Get(_ context.Context, _, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failGet != nil {
		return nil, s.failGet
	}
	data, ok := s.objects[key]
	if !ok {
		return nil, storage.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *fakeStorageStore) Put(
	_ context.Context, _, key string, reader io.Reader, _ int64, contentType string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failPut != nil {
		return s.failPut
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.objects[key] = data
	s.stats[key] = storage.ObjectStat{
		Key:                key,
		Size:               int64(len(data)),
		ContentType:        contentType,
		SniffedContentType: sniffMimeType(data),
	}
	return nil
}

func (s *fakeStorageStore) Delete(_ context.Context, _, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failDelete != nil {
		return s.failDelete
	}
	delete(s.objects, key)
	delete(s.stats, key)
	s.deleted = append(s.deleted, key)
	return nil
}

func (s *fakeStorageStore) PresignGet(_ context.Context, bucket, key string, _ time.Duration) (string, error) {
	return "https://storage.local/" + bucket + "/" + key, nil
}

func (s *fakeStorageStore) wasDeleted(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, del := range s.deleted {
		if del == key {
			return true
		}
	}
	return false
}

func sniffMimeType(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("MZ")):
		return "application/x-dosexec"
	case bytes.HasPrefix(data, []byte("\xFF\xD8\xFF")):
		return mimeImageJPEG
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		return "image/png"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func createTestJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := range 100 {
		for y := range 100 {
			img.Set(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 2), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

// createTestJPEGWithEXIFGPS crafts a valid JPEG containing an APP1 EXIF segment with GPS tags.
func createTestJPEGWithEXIFGPS() []byte {
	base := createTestJPEG()
	if len(base) < 2 || base[0] != 0xFF || base[1] != 0xD8 {
		return base
	}

	// Minimal APP1 EXIF marker payload with GPS tag marker.
	// Header: 0xFF, 0xE1, length (2 bytes), "Exif\0\0", TIFF header, GPS IFD
	exifData := []byte{
		0xFF, 0xE1, // APP1 marker
		0x00, 0x30, // length = 48 bytes
		'E', 'x', 'i', 'f', 0x00, 0x00, // Exif header
		'I', 'I', 0x2A, 0x00, // TIFF header (little-endian)
		0x08, 0x00, 0x00, 0x00, // offset to IFD0
		0x01, 0x00, // 1 entry in IFD0
		0x25, 0x88, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x1A, 0x00, 0x00, 0x00, // GPSInfo tag (0x8825) pointing to GPS IFD at offset 26
		0x00, 0x00, 0x00, 0x00, // next IFD offset
		// GPS IFD at offset 26
		0x01, 0x00, // 1 GPS tag
		0x02, 0x00, 0x05, 0x00, 0x03, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, // GPSLatitude tag (0x0002)
		0x00, 0x00, 0x00, 0x00,
	}

	result := make([]byte, 0, len(base)+len(exifData))
	result = append(result, base[:2]...)
	result = append(result, exifData...)
	result = append(result, base[2:]...)
	return result
}

// hasEXIFGPSTag inspects whether a byte slice contains EXIF GPS markers.
func hasEXIFGPSTag(data []byte) bool {
	return bytes.Contains(data, []byte("Exif")) || bytes.Contains(data, []byte{0xFF, 0xE1})
}

func setupAvatarHarness(t *testing.T) (*harness, *fakeStorageStore) {
	t.Helper()
	h := newHarness(t)
	store := newFakeStorageStore()

	users := service.New(service.Deps{
		Pool:    h.pool,
		Repo:    h.repo,
		Events:  h.events,
		Clock:   clock.NewFake(testNow),
		NewID:   func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
		Storage: store,
	})
	h.service = users
	return h, store
}

func TestRequestAvatarUploadIntent_Success(t *testing.T) {
	t.Parallel()
	h, _ := setupAvatarHarness(t)

	intent, err := h.service.RequestAvatarUploadIntent(context.Background(), h.actor, mimeImageJPEG)
	if err != nil {
		t.Fatalf("RequestAvatarUploadIntent failed: %v", err)
	}

	if intent.Method != "POST" {
		t.Errorf("Method = %q, want POST", intent.Method)
	}
	if intent.ContentType != mimeImageJPEG {
		t.Errorf("ContentType = %q, want image/jpeg", intent.ContentType)
	}
	if intent.MaxBytes != domain.AvatarMaxBytes {
		t.Errorf("MaxBytes = %d, want %d", intent.MaxBytes, domain.AvatarMaxBytes)
	}
	if !strings.HasPrefix(intent.ObjectKey, "users/"+h.actor.String()+"/") {
		t.Errorf("ObjectKey = %q does not start with user prefix", intent.ObjectKey)
	}
}

func TestRequestAvatarUploadIntent_RejectsInvalidContentType(t *testing.T) {
	t.Parallel()
	h, _ := setupAvatarHarness(t)

	_, err := h.service.RequestAvatarUploadIntent(context.Background(), h.actor, "application/pdf")
	if err == nil {
		t.Fatal("RequestAvatarUploadIntent with pdf succeeded, want error")
	}
}

func TestConfirmAvatar_RejectsRenamedExecutableViaMagicBytes(t *testing.T) {
	t.Parallel()
	h, store := setupAvatarHarness(t)

	// An executable renamed to .jpg (contains MZ header).
	rawKey := "users/" + h.actor.String() + "/2026/08/avatar-raw.jpg"
	fakeExe := []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xFF\xFF\x00\x00malicious executable payload")
	store.objects[rawKey] = fakeExe
	store.stats[rawKey] = storage.ObjectStat{
		Key:                rawKey,
		Size:               int64(len(fakeExe)),
		SniffedContentType: "application/x-dosexec",
	}

	_, err := h.service.ConfirmAvatar(context.Background(), h.actor, rawKey)
	if err == nil {
		t.Fatal("ConfirmAvatar with renamed executable succeeded, want rejection")
	}

	// Verify profile avatar was not modified.
	profile, err := h.repo.GetProfile(context.Background(), h.actor)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile.AvatarAssetID != nil {
		t.Errorf("AvatarAssetID was updated on rejected upload: %v", profile.AvatarAssetID)
	}
}

func TestConfirmAvatar_StripsEXIFGPSMetadata(t *testing.T) {
	t.Parallel()
	h, store := setupAvatarHarness(t)

	rawKey := "users/" + h.actor.String() + "/2026/08/photo-with-gps.jpg"
	inputWithGPS := createTestJPEGWithEXIFGPS()

	// Prove the test input actually carries EXIF data.
	if !hasEXIFGPSTag(inputWithGPS) {
		t.Fatal("Test fixture createTestJPEGWithEXIFGPS does not contain EXIF tags")
	}

	store.objects[rawKey] = inputWithGPS
	store.stats[rawKey] = storage.ObjectStat{
		Key:                rawKey,
		Size:               int64(len(inputWithGPS)),
		SniffedContentType: mimeImageJPEG,
	}

	account, err := h.service.ConfirmAvatar(context.Background(), h.actor, rawKey)
	if err != nil {
		t.Fatalf("ConfirmAvatar failed: %v", err)
	}

	if account.Profile.AvatarAssetID == nil {
		t.Fatal("AvatarAssetID is nil after confirmation")
	}

	// Locate the stored processed avatar variants in storage.
	variantKeys := map[string]image.Point{
		"_sm.jpg": {X: 64, Y: 64},
		"_md.jpg": {X: 128, Y: 128},
		"_lg.jpg": {X: 256, Y: 256},
	}

	foundCount := 0
	for suffix, expectedSize := range variantKeys {
		expectedSubkey := account.Profile.AvatarAssetID.String() + suffix
		var outputKey string
		var outputBytes []byte
		for key, data := range store.objects {
			if strings.Contains(key, expectedSubkey) {
				outputKey = key
				outputBytes = data
				break
			}
		}

		if outputKey == "" || len(outputBytes) == 0 {
			t.Fatalf("Processed avatar variant %q was not found in storage", expectedSubkey)
		}
		foundCount++

		// Acceptance criterion: EXIF GPS data is ABSENT from the output object!
		if hasEXIFGPSTag(outputBytes) {
			t.Errorf("Output image %q still contains EXIF/GPS metadata tags!", outputKey)
		}

		// Confirm that the output object is a valid decodable JPEG image and has correct dimensions.
		img, format, decodeErr := image.Decode(bytes.NewReader(outputBytes))
		if decodeErr != nil {
			t.Fatalf("Output image decode error for %q: %v", outputKey, decodeErr)
		}
		if format != "jpeg" {
			t.Errorf("Output format for %q = %q, want jpeg", outputKey, format)
		}
		bounds := img.Bounds()
		if bounds.Dx() != expectedSize.X || bounds.Dy() != expectedSize.Y {
			t.Errorf(
				"Variant %q dimensions = %dx%d, want %dx%d",
				outputKey, bounds.Dx(), bounds.Dy(), expectedSize.X, expectedSize.Y,
			)
		}
	}

	if foundCount != len(variantKeys) {
		t.Errorf("Found %d variants, want %d", foundCount, len(variantKeys))
	}
}

func TestConfirmAvatar_DeletesOldAvatarOnlyAfterNewIsVerifiedAndCommitted(t *testing.T) {
	t.Parallel()
	h, store := setupAvatarHarness(t)

	// Set an existing avatar on the user's profile, recorded the way
	// ConfirmAvatar records one: a profile pointer and a row per variant.
	//
	// The row is what makes the delete possible. Cleanup used to rebuild the old
	// key from profiles.updated_at, which is a guess -- rename yourself between
	// two uploads and the delete aimed at the wrong month, so the object stayed
	// in the bucket for ever. Seeding the row here is not test scaffolding; it
	// is the state the real write path leaves behind.
	oldAssetID := uuid.New()
	oldKey := "users/" + h.actor.String() + "/2026/08/" + oldAssetID.String() + "_lg.jpg"
	store.objects[oldKey] = createTestJPEG()
	_, _ = h.repo.UpdateProfileAvatar(context.Background(), h.actor, &oldAssetID)
	_ = h.repo.InsertAvatarAsset(context.Background(), domain.AvatarAsset{
		AssetID:   oldAssetID,
		Variant:   domain.AvatarVariantLarge,
		UserID:    h.actor,
		ObjectKey: oldKey,
		MimeType:  mimeImageJPEG,
		ByteSize:  int64(len(store.objects[oldKey])),
	})

	// Stage a new valid avatar upload.
	newRawKey := "users/" + h.actor.String() + "/2026/08/new-avatar-raw.jpg"
	newJPEG := createTestJPEG()
	store.objects[newRawKey] = newJPEG
	store.stats[newRawKey] = storage.ObjectStat{
		Key:                newRawKey,
		Size:               int64(len(newJPEG)),
		SniffedContentType: mimeImageJPEG,
	}

	account, err := h.service.ConfirmAvatar(context.Background(), h.actor, newRawKey)
	if err != nil {
		t.Fatalf("ConfirmAvatar failed: %v", err)
	}

	if account.Profile.AvatarAssetID == nil || *account.Profile.AvatarAssetID == oldAssetID {
		t.Errorf("AvatarAssetID was not updated: %v", account.Profile.AvatarAssetID)
	}

	// The old object must be deleted from storage.
	if !store.wasDeleted(oldKey) {
		t.Errorf("Old avatar %q was not deleted after successful update", oldKey)
	}

	// The raw upload object must be cleaned up.
	if !store.wasDeleted(newRawKey) {
		t.Errorf("Raw upload %q was not deleted after processing", newRawKey)
	}
}

func TestConfirmAvatar_LeavesOldAvatarUntouchedWhenNewFails(t *testing.T) {
	t.Parallel()
	h, store := setupAvatarHarness(t)

	// Existing avatar.
	oldAssetID := uuid.New()
	oldKey := "users/" + h.actor.String() + "/2026/08/" + oldAssetID.String() + "_lg.jpg"
	store.objects[oldKey] = createTestJPEG()
	_, _ = h.repo.UpdateProfileAvatar(context.Background(), h.actor, &oldAssetID)

	// A broken raw upload (corrupted data).
	corruptedKey := "users/" + h.actor.String() + "/2026/08/corrupted.jpg"
	store.objects[corruptedKey] = []byte("not a valid image content")
	store.stats[corruptedKey] = storage.ObjectStat{
		Key:                corruptedKey,
		Size:               24,
		SniffedContentType: mimeImageJPEG, // pretend sniffed jpeg, but corrupted decode
	}

	_, err := h.service.ConfirmAvatar(context.Background(), h.actor, corruptedKey)
	if err == nil {
		t.Fatal("ConfirmAvatar with corrupted image succeeded, want failure")
	}

	// Old avatar MUST NOT be deleted!
	if store.wasDeleted(oldKey) {
		t.Errorf("Old avatar %q was deleted despite failure!", oldKey)
	}

	// Profile still points to old avatar.
	profile, err := h.repo.GetProfile(context.Background(), h.actor)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile.AvatarAssetID == nil || *profile.AvatarAssetID != oldAssetID {
		t.Errorf("Profile AvatarAssetID changed to %v, want %v", profile.AvatarAssetID, oldAssetID)
	}
}
