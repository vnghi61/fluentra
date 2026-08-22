package repository

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/generated/content/sqlc"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
)

const (
	testKind      = "vocab_word"
	testObjectKey = "audio/a.mp3"
	testTagCode   = "greetings"
)

func TestToDomainItem(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	owner := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	verID := uuid.New()
	row := sqlc.ContentContentItem{
		ID:               id,
		Kind:             testKind,
		Slug:             "hello-world",
		CurrentVersionID: &verID,
		Status:           "draft",
		OwnerID:          owner,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	got := toDomainItem(row)
	if got.ID != id || got.Slug != "hello-world" || got.Status != domain.StatusDraft || got.OwnerID != owner {
		t.Errorf("toDomainItem mismatch: %+v", got)
	}
	if got.CurrentVersionID == nil || *got.CurrentVersionID != verID {
		t.Error("CurrentVersionID mismatch")
	}
}

func TestToDomainVersion(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	itemID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	pubAt := now
	body := json.RawMessage(`{"word":"hello"}`)
	row := sqlc.ContentContentVersion{
		ID:          id,
		ItemID:      itemID,
		Version:     2,
		Kind:        testKind,
		Body:        []byte(body),
		CefrLevel:   "B1",
		Status:      "published",
		MediaRefs:   []string{testObjectKey},
		PublishedAt: &pubAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	got := toDomainVersion(row)
	if got.ID != id || got.ItemID != itemID || got.Version != 2 ||
		got.CEFRLevel != "B1" || got.Status != domain.StatusPublished {
		t.Errorf("toDomainVersion mismatch: %+v", got)
	}
	if string(got.Body) != string(body) {
		t.Error("Body mismatch")
	}
}

func TestToContractVersion(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	itemID := uuid.New()
	now := time.Now().UTC()
	v := domain.Version{
		ID:          id,
		ItemID:      itemID,
		Version:     1,
		Kind:        testKind,
		Body:        json.RawMessage(`{}`),
		CEFRLevel:   "A1",
		Status:      domain.StatusPublished,
		PublishedAt: &now,
	}
	tags := []string{testTagCode, "topic-a"}
	c := ToContractVersion(v, tags)
	if c.ID != id || c.CEFRLevel != "A1" || len(c.Tags) != 2 {
		t.Errorf("ToContractVersion mismatch: %+v", c)
	}
	// nil PublishedAt
	v2 := domain.Version{ID: id, Status: domain.StatusDraft}
	c2 := ToContractVersion(v2, nil)
	if c2.PublishedAt != nil {
		t.Error("expected nil PublishedAt")
	}
}

func TestToDomainMediaAsset(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	dur := int32(1200)
	cs := "abc"
	sz := int64(1024)
	mt := "audio/mpeg"
	row := sqlc.ContentMediaAsset{
		ID:         id,
		ObjectKey:  testObjectKey,
		Kind:       "audio",
		DurationMs: &dur,
		Checksum:   &cs,
		Status:     "ready",
		ByteSize:   &sz,
		MimeType:   &mt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	got := toDomainMediaAsset(row)
	if got.ObjectKey != testObjectKey || got.Status != domain.MediaStatusReady {
		t.Errorf("toDomainMediaAsset mismatch: %+v", got)
	}
}

func TestToDomainTaxonomyAndReview(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	taxRow := sqlc.ContentTaxonomy{
		ID:        id,
		Namespace: "topic",
		Code:      testTagCode,
		Label:     "Greetings",
		CreatedAt: now,
		UpdatedAt: now,
	}
	tax := toDomainTaxonomy(taxRow)
	if tax.Code != testTagCode {
		t.Errorf("toDomainTaxonomy: %+v", tax)
	}
	verID := uuid.New()
	revID := uuid.New()
	revRow := sqlc.ContentContentReview{
		ID:         revID,
		VersionID:  verID,
		ReviewerID: uuid.New(),
		Decision:   "approved",
		CreatedAt:  now,
	}
	rev := toDomainReview(revRow)
	if rev.Decision != domain.ReviewDecisionApproved {
		t.Errorf("toDomainReview: %+v", rev)
	}
}
