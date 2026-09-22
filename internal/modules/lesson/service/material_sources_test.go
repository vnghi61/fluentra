package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
	"github.com/fluentra/fluentra/internal/platform/storage"
)

const kindLessonMaterial = "lesson_material"

// fakePresignStore stands in for storage so the read path can be exercised
// without an object store. Every other method is the embedded interface's nil.
type fakePresignStore struct {
	storage.Store
	lastExpiry time.Duration
	lastBucket string
}

func (f *fakePresignStore) PresignGet(
	_ context.Context, bucket, key string, expiry time.Duration,
) (string, error) {
	f.lastExpiry = expiry
	f.lastBucket = bucket
	return "https://cdn.example/" + bucket + "/" + key + "?sig=1", nil
}

// A material's stored object keys become signed URLs only at read time, after
// the paywall, and never leave as keys the browser would have to sign itself.
func TestGetLessonDetail_SignsMaterialSources(t *testing.T) {
	t.Parallel()

	lessonID, unitID, versionID := uuid.New(), uuid.New(), uuid.New()
	config := json.RawMessage(`{
		"material_kind": "video",
		"title": "Intro to verbs",
		"objects": {
			"original": "course-materials/course/slug-u1-l1-a1/original.mp4",
			"video_720p": "course-materials/course/slug-u1-l1-a1/video_720p.mp4",
			"video_360p": "course-materials/course/slug-u1-l1-a1/video_360p.mp4",
			"poster": "course-materials/course/slug-u1-l1-a1/poster.jpg"
		}
	}`)
	activities := []contract.Activity{{
		ID:               uuid.New(),
		LessonID:         lessonID,
		Position:         1,
		Kind:             kindLessonMaterial,
		ContentVersionID: versionID,
		Config:           config,
		Weight:           0,
	}}
	repo := &fakeLessonRepo{
		lesson: &contract.Lesson{
			ID: lessonID, UnitID: unitID, Position: 1,
			Title: "Watch", Status: statusPublished, Activities: activities,
		},
		activities: activities,
	}
	store := &fakePresignStore{}
	svc := service.New(service.Deps{
		Repo:    repo,
		Content: &countingContentReader{},
		Storage: store,
	})

	detail, err := svc.GetLessonDetail(context.Background(), lessonID, uuid.Nil)
	if err != nil {
		t.Fatalf("GetLessonDetail: %v", err)
	}
	if len(detail.Activities) != 1 {
		t.Fatalf("expected one activity, got %d", len(detail.Activities))
	}

	var decoded struct {
		Sources struct {
			PosterURL string `json:"poster_url"`
			Video     []struct {
				URL    string `json:"url"`
				Height int    `json:"height"`
			} `json:"video"`
			Document *struct {
				URL string `json:"url"`
			} `json:"document"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(detail.Activities[0].Config, &decoded); err != nil {
		t.Fatalf("decode signed config: %v", err)
	}
	// The raw upload is never handed out for a video; only its renditions are.
	if decoded.Sources.Document != nil {
		t.Errorf("a video must not carry a document source to its original, got %s",
			detail.Activities[0].Config)
	}
	if decoded.Sources.PosterURL == "" {
		t.Errorf("expected a signed poster_url, got config: %s", detail.Activities[0].Config)
	}
	if len(decoded.Sources.Video) != 2 {
		t.Fatalf("expected 720p and 360p sources, got %d", len(decoded.Sources.Video))
	}
	if decoded.Sources.Video[0].Height != 720 || decoded.Sources.Video[1].Height != 360 {
		t.Errorf("expected 720p first then 360p, got %+v", decoded.Sources.Video)
	}
	for _, source := range decoded.Sources.Video {
		if !strings.Contains(source.URL, storage.BucketMedia) {
			t.Errorf("expected a fluentra-media URL, got %q", source.URL)
		}
	}
	if store.lastExpiry != time.Hour {
		t.Errorf("expected a one-hour URL, got %s", store.lastExpiry)
	}
	if store.lastBucket != storage.BucketMedia {
		t.Errorf("expected material URLs to come from fluentra-media, got %q", store.lastBucket)
	}
}

// Without a storage client, nothing is signed and the config is returned
// unchanged: the read path must not depend on storage being configured.
func TestGetLessonDetail_NoStorageLeavesMaterialConfig(t *testing.T) {
	t.Parallel()

	lessonID, unitID, versionID := uuid.New(), uuid.New(), uuid.New()
	config := json.RawMessage(`{"material_kind":"document","title":"Doc","objects":{"original":"k.pdf"}}`)
	activities := []contract.Activity{{
		ID: uuid.New(), LessonID: lessonID, Position: 1, Kind: kindLessonMaterial,
		ContentVersionID: versionID, Config: config, Weight: 0,
	}}
	repo := &fakeLessonRepo{
		lesson: &contract.Lesson{
			ID: lessonID, UnitID: unitID, Position: 1,
			Title: "Read", Status: statusPublished, Activities: activities,
		},
		activities: activities,
	}
	svc := service.New(service.Deps{Repo: repo, Content: &countingContentReader{}})

	detail, err := svc.GetLessonDetail(context.Background(), lessonID, uuid.Nil)
	if err != nil {
		t.Fatalf("GetLessonDetail: %v", err)
	}
	if strings.Contains(string(detail.Activities[0].Config), "https://") {
		t.Errorf("no URL should be present without storage: %s", detail.Activities[0].Config)
	}
}

// A document offers its original to open in a new tab, and its preview image.
func TestGetLessonDetail_SignsDocumentSource(t *testing.T) {
	t.Parallel()

	lessonID, unitID := uuid.New(), uuid.New()
	config := json.RawMessage(`{"material_kind":"document","title":"Doc",` +
		`"objects":{"original":"cm/c/s/original.pdf","preview":"cm/c/s/preview.png"}}`)
	activities := []contract.Activity{{
		ID: uuid.New(), LessonID: lessonID, Position: 1, Kind: kindLessonMaterial,
		ContentVersionID: uuid.New(), Config: config, Weight: 0,
	}}
	repo := &fakeLessonRepo{
		lesson: &contract.Lesson{
			ID: lessonID, UnitID: unitID, Position: 1,
			Title: "Read", Status: statusPublished, Activities: activities,
		},
		activities: activities,
	}
	svc := service.New(service.Deps{
		Repo: repo, Content: &countingContentReader{}, Storage: &fakePresignStore{},
	})

	detail, err := svc.GetLessonDetail(context.Background(), lessonID, uuid.Nil)
	if err != nil {
		t.Fatalf("GetLessonDetail: %v", err)
	}
	var decoded struct {
		Sources struct {
			Video    []any `json:"video"`
			Document struct {
				URL        string `json:"url"`
				PreviewURL string `json:"preview_url"`
			} `json:"document"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(detail.Activities[0].Config, &decoded); err != nil {
		t.Fatalf("decode signed config: %v", err)
	}
	if !strings.Contains(decoded.Sources.Document.URL, "original.pdf") {
		t.Errorf("expected the original PDF to be signed, got %s", detail.Activities[0].Config)
	}
	if !strings.Contains(decoded.Sources.Document.PreviewURL, "preview.png") {
		t.Errorf("expected a signed preview, got %s", detail.Activities[0].Config)
	}
	if len(decoded.Sources.Video) != 0 {
		t.Errorf("a document has no video sources, got %v", decoded.Sources.Video)
	}
}
