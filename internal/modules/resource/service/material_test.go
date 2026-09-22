package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/domain"
	"github.com/fluentra/fluentra/internal/modules/resource/service"
)

const testVideoMIME = "video/mp4"

func validatedVideo(repo *mockRepo, owner, resourceID uuid.UUID, objectKey string) {
	repo.resources[resourceID] = &contract.Resource{
		ID:           resourceID,
		UserID:       owner,
		Kind:         domain.KindFile,
		Title:        "Intro video",
		ObjectKey:    &objectKey,
		DetectedMIME: testVideoMIME,
		Status:       domain.StatusValidated,
	}
}

func TestMaterialForOwner_OnlyTheOwner(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.New(repo, newMockStorage(), nil, nil, nil, nil)

	owner, stranger, resourceID := uuid.New(), uuid.New(), uuid.New()
	validatedVideo(repo, owner, resourceID, "user/owner/clip.mp4")

	ready := "derived/clip_360.mp4"
	pending := "derived/clip_720.mp4"
	repo.renditions[uuid.New()] = &contract.Rendition{
		ID: uuid.New(), ResourceID: resourceID, Kind: domain.RenditionKindVideo360p,
		Status: domain.RenditionStatusReady, ObjectKey: &ready,
	}
	repo.renditions[uuid.New()] = &contract.Rendition{
		ID: uuid.New(), ResourceID: resourceID, Kind: domain.RenditionKindVideo720p,
		Status: domain.RenditionStatusPending, ObjectKey: &pending,
	}

	mat, err := svc.MaterialForOwner(ctx, owner, resourceID)
	if err != nil {
		t.Fatalf("MaterialForOwner: %v", err)
	}
	if mat.DetectedMIME != testVideoMIME || mat.Status != domain.StatusValidated {
		t.Errorf("unexpected material: %+v", mat)
	}
	if len(mat.Renditions) != 2 {
		t.Errorf("expected both renditions with their statuses, got %d", len(mat.Renditions))
	}

	// A resource the caller does not own is the same 404 as any foreign id.
	if _, err := svc.MaterialForOwner(ctx, stranger, resourceID); !errors.Is(err, domain.ErrResourceNotFound) {
		t.Fatalf("expected ErrResourceNotFound for a foreign resource, got %v", err)
	}
}

func TestCopyForPublication_CopiesReadyRenditions(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := service.New(repo, newMockStorage(), nil, nil, nil, nil)

	owner, resourceID := uuid.New(), uuid.New()
	validatedVideo(repo, owner, resourceID, "user/owner/clip.mp4")
	ready360 := "derived/clip_360.mp4"
	pending720 := "derived/clip_720.mp4"
	repo.renditions[uuid.New()] = &contract.Rendition{
		ID: uuid.New(), ResourceID: resourceID, Kind: domain.RenditionKindVideo360p,
		Status: domain.RenditionStatusReady, ObjectKey: &ready360,
	}
	repo.renditions[uuid.New()] = &contract.Rendition{
		ID: uuid.New(), ResourceID: resourceID, Kind: domain.RenditionKindVideo720p,
		Status: domain.RenditionStatusPending, ObjectKey: &pending720,
	}

	prefix := "course-materials/course/slug-u1-l1-a1/"
	objects, err := svc.CopyForPublication(ctx, resourceID, prefix)
	if err != nil {
		t.Fatalf("CopyForPublication: %v", err)
	}
	if !strings.HasPrefix(objects.Original, prefix) || !strings.HasSuffix(objects.Original, "original.mp4") {
		t.Errorf("unexpected original key %q", objects.Original)
	}
	if objects.Video360p == nil || !strings.Contains(*objects.Video360p, "video_360p") {
		t.Errorf("expected a copied 360p rendition, got %+v", objects.Video360p)
	}
	if objects.Video720p != nil {
		t.Errorf("a pending rendition must not be copied, got %q", *objects.Video720p)
	}
}
