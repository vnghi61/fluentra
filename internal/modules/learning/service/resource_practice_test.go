package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	resourcecontract "github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// testWeakNodeCode is the spine node the fixtures classify against.
const testWeakNodeCode = "PRESENT_PERFECT"

// ---------------------------------------------------------------- fixtures

type stubResourceReader struct {
	resource *resourcecontract.Resource
	err      error
}

func (s stubResourceReader) GetResource(
	context.Context, uuid.UUID, uuid.UUID,
) (*resourcecontract.Resource, error) {
	return s.resource, s.err
}

type stubPracticeStore struct {
	set     *domain.ResourcePracticeSet
	upserts int
	claimed []domain.ResourcePracticeSet
	ready   int
	failed  int
	deleted int
}

func (s *stubPracticeStore) UpsertResourcePracticeSet(
	_ context.Context, resourceID, userID uuid.UUID,
) (*domain.ResourcePracticeSet, error) {
	s.upserts++
	s.set = &domain.ResourcePracticeSet{
		ResourceID:  resourceID,
		UserID:      userID,
		Status:      domain.ResourcePracticeGenerating,
		GeneratedOn: time.Now(),
	}
	return s.set, nil
}

func (s *stubPracticeStore) GetResourcePracticeSet(
	context.Context, uuid.UUID, uuid.UUID,
) (*domain.ResourcePracticeSet, error) {
	return s.set, nil
}

func (s *stubPracticeStore) MarkResourcePracticeSetReady(
	_ context.Context, resourceID, userID, lessonID uuid.UUID, activityIDs []uuid.UUID,
) (*domain.ResourcePracticeSet, error) {
	s.ready++
	s.set = &domain.ResourcePracticeSet{
		ResourceID: resourceID, UserID: userID, LessonID: &lessonID,
		ActivityIDs: activityIDs, Status: domain.ResourcePracticeReady,
	}
	return s.set, nil
}

func (s *stubPracticeStore) MarkResourcePracticeSetFailed(
	_ context.Context, resourceID, userID uuid.UUID, reason string,
) (*domain.ResourcePracticeSet, error) {
	s.failed++
	s.set = &domain.ResourcePracticeSet{
		ResourceID: resourceID, UserID: userID,
		Status: domain.ResourcePracticeFailed, FailureReason: reason,
	}
	return s.set, nil
}

func (s *stubPracticeStore) ClaimGeneratingResourcePracticeSets(
	context.Context, int32,
) ([]domain.ResourcePracticeSet, error) {
	return s.claimed, nil
}

func (s *stubPracticeStore) DeleteResourcePracticeSetsForUser(context.Context, uuid.UUID) error {
	s.deleted++
	return nil
}

func resourcePracticeService(
	t *testing.T, resource *resourcecontract.Resource, store *stubPracticeStore,
) *Service {
	t.Helper()
	return New(Deps{
		Clock:            clock.NewFake(time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)),
		Resource:         stubResourceReader{resource: resource},
		ResourcePractice: store,
	})
}

func classifiedResource() *resourcecontract.Resource {
	level := "B1"
	skill := domain.SkillGrammar
	return &resourcecontract.Resource{
		ID:     uuid.New(),
		UserID: uuid.New(),
		Status: resourcecontract.MaterialValidated,
		Extraction: &resourcecontract.Extraction{
			Text:      "Present perfect: she has lived here for three years.",
			CharCount: 50,
		},
		Classification: &resourcecontract.Classification{
			CEFREstimate: &level,
			Skill:        &skill,
			NodeCodes:    []string{testWeakNodeCode},
		},
	}
}

// ------------------------------------------------------------------- tests

func TestRequestResourcePractice_QueuesWhenNothingExistsYet(t *testing.T) {
	store := &stubPracticeStore{}
	resource := classifiedResource()
	svc := resourcePracticeService(t, resource, store)

	set, err := svc.RequestResourcePractice(context.Background(), resource.UserID, resource.ID)
	if err != nil {
		t.Fatalf("RequestResourcePractice: %v", err)
	}
	if set.Status != domain.ResourcePracticeGenerating {
		t.Errorf("status = %q, want generating", set.Status)
	}
	if store.upserts != 1 {
		t.Errorf("upserted %d times, want 1", store.upserts)
	}
}

func TestRequestResourcePractice_ReusesTodaysReadySet(t *testing.T) {
	resource := classifiedResource()
	store := &stubPracticeStore{set: &domain.ResourcePracticeSet{
		ResourceID:  resource.ID,
		UserID:      resource.UserID,
		Status:      domain.ResourcePracticeReady,
		GeneratedOn: time.Date(2026, 9, 22, 6, 0, 0, 0, time.UTC),
	}}
	svc := resourcePracticeService(t, resource, store)

	set, err := svc.RequestResourcePractice(context.Background(), resource.UserID, resource.ID)
	if err != nil {
		t.Fatalf("RequestResourcePractice: %v", err)
	}
	if set.Status != domain.ResourcePracticeReady {
		t.Errorf("status = %q, want ready", set.Status)
	}
	if store.upserts != 0 {
		t.Errorf("regenerated %d times inside one day, want 0", store.upserts)
	}
}

func TestRequestResourcePractice_RegeneratesTheNextDay(t *testing.T) {
	resource := classifiedResource()
	store := &stubPracticeStore{set: &domain.ResourcePracticeSet{
		ResourceID:  resource.ID,
		UserID:      resource.UserID,
		Status:      domain.ResourcePracticeReady,
		GeneratedOn: time.Date(2026, 9, 21, 6, 0, 0, 0, time.UTC),
	}}
	svc := resourcePracticeService(t, resource, store)

	if _, err := svc.RequestResourcePractice(context.Background(), resource.UserID, resource.ID); err != nil {
		t.Fatalf("RequestResourcePractice: %v", err)
	}
	if store.upserts != 1 {
		t.Errorf("upserted %d times, want 1 on a new day", store.upserts)
	}
}

func TestRequestResourcePractice_RefusesAFileWithNoText(t *testing.T) {
	resource := classifiedResource()
	resource.Extraction = nil
	store := &stubPracticeStore{}
	svc := resourcePracticeService(t, resource, store)

	_, err := svc.RequestResourcePractice(context.Background(), resource.UserID, resource.ID)
	if !apperr.Is(err, apperr.Validation) || !strings.Contains(err.Error(), "no text") {
		t.Fatalf("error = %v, want a validation error about missing text", err)
	}
	if store.upserts != 0 {
		t.Error("a file with no text was queued for generation")
	}
}

func TestRequestResourcePractice_RefusesAnUnclassifiedFile(t *testing.T) {
	resource := classifiedResource()
	resource.Classification = nil
	svc := resourcePracticeService(t, resource, &stubPracticeStore{})

	_, err := svc.RequestResourcePractice(context.Background(), resource.UserID, resource.ID)
	if !apperr.Is(err, apperr.Validation) || !strings.Contains(err.Error(), "not been classified") {
		t.Fatalf("error = %v, want a validation error about classification", err)
	}
}

func TestRequestResourcePractice_AForeignResourceIsNotFound(t *testing.T) {
	store := &stubPracticeStore{}
	svc := New(Deps{
		Clock:            clock.NewFake(time.Now()),
		Resource:         stubResourceReader{err: errors.New("no rows")},
		ResourcePractice: store,
	})

	_, err := svc.RequestResourcePractice(context.Background(), uuid.New(), uuid.New())
	if !apperr.Is(err, apperr.NotFound) {
		t.Fatalf("error = %v, want not found", err)
	}
	if store.upserts != 0 {
		t.Error("a foreign resource was queued for generation")
	}
}

func TestResourcePracticeKind_UsesTheGradedChoiceKinds(t *testing.T) {
	if got := resourcePracticeKind(domain.SkillGrammar); got != kindGrammarTenseChoice {
		t.Errorf("grammar file → %q, want %q", got, kindGrammarTenseChoice)
	}
	if got := resourcePracticeKind(domain.SkillVocabulary); got != kindVocabMultipleChoice {
		t.Errorf("vocabulary file → %q, want %q", got, kindVocabMultipleChoice)
	}
}

func TestResourcePracticeSource_TruncatesToTheModelsWindow(t *testing.T) {
	resource := classifiedResource()
	resource.Extraction.Text = strings.Repeat("a", resourcePracticeSourceChars+500)

	if got := len([]rune(resourcePracticeSource(resource))); got != resourcePracticeSourceChars {
		t.Errorf("source length = %d, want %d", got, resourcePracticeSourceChars)
	}
}

func TestWeakNodeMatches_FindsTheNodeInTheVersionsTags(t *testing.T) {
	weak := &domain.WeakNodeLabel{Code: testWeakNodeCode, Label: "Present Perfect"}

	if !weakNodeMatches([]string{"SENTENCE_STRUCTURE", testWeakNodeCode}, weak) {
		t.Error("the weak node is in the tags but was not matched")
	}
	if weakNodeMatches([]string{"SENTENCE_STRUCTURE"}, weak) {
		t.Error("a version without the weak node was matched")
	}
}
