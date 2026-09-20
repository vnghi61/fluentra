package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/modules/content/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type foundationMockRepo struct {
	*fakeRepo
	taxonomies     map[string]domain.Taxonomy
	taxonomiesByID map[uuid.UUID]domain.Taxonomy
	prereqEdges    []domain.PrerequisiteEdge
	taggedCounts   map[uuid.UUID]map[string]int
	topicTags      map[uuid.UUID][]domain.Taxonomy
	topicBodies    map[uuid.UUID][]byte
}

func newFoundationMockRepo() *foundationMockRepo {
	return &foundationMockRepo{
		fakeRepo:       newFakeRepo(),
		taxonomies:     make(map[string]domain.Taxonomy),
		taxonomiesByID: make(map[uuid.UUID]domain.Taxonomy),
		taggedCounts:   make(map[uuid.UUID]map[string]int),
		topicTags:      make(map[uuid.UUID][]domain.Taxonomy),
		topicBodies:    make(map[uuid.UUID][]byte),
	}
}

func (m *foundationMockRepo) WithTx(_ pgx.Tx) service.Repository {
	return m
}

func (m *foundationMockRepo) CreateTaxonomy(
	_ context.Context,
	id uuid.UUID,
	namespace, code, label string,
	parentID *uuid.UUID,
	description string,
	cefrLevel *string,
	position int,
	deprecatedAt *time.Time,
) (domain.Taxonomy, error) {
	t := domain.Taxonomy{
		ID:           id,
		Namespace:    namespace,
		Code:         code,
		Label:        label,
		ParentID:     parentID,
		Description:  description,
		CEFRLevel:    cefrLevel,
		Position:     position,
		DeprecatedAt: deprecatedAt,
	}
	m.taxonomies[code] = t
	m.taxonomiesByID[id] = t
	return t, nil
}

func (m *foundationMockRepo) GetTaxonomyByID(_ context.Context, id uuid.UUID) (domain.Taxonomy, error) {
	if t, ok := m.taxonomiesByID[id]; ok {
		return t, nil
	}
	return domain.Taxonomy{}, domain.ErrTaxonomyNodeNotFound
}

func (m *foundationMockRepo) GetTaxonomyByCode(_ context.Context, code string) (domain.Taxonomy, error) {
	if t, ok := m.taxonomies[code]; ok {
		return t, nil
	}
	return domain.Taxonomy{}, domain.ErrTaxonomyNodeNotFound
}

func (m *foundationMockRepo) ListAllTaxonomiesInNamespace(_ context.Context, ns string) ([]domain.Taxonomy, error) {
	var list []domain.Taxonomy
	for _, t := range m.taxonomies {
		if t.Namespace == ns && t.DeprecatedAt == nil {
			list = append(list, t)
		}
	}
	return list, nil
}

func (m *foundationMockRepo) ListAllPrerequisiteEdgesInNamespace(
	_ context.Context, _ string,
) ([]domain.PrerequisiteEdge, error) {
	return m.prereqEdges, nil
}

func (m *foundationMockRepo) ReplacePrerequisites(_ context.Context, nodeID uuid.UUID, reqIDs []uuid.UUID) error {
	newEdges := make([]domain.PrerequisiteEdge, 0)
	for _, e := range m.prereqEdges {
		if e.NodeID != nodeID {
			newEdges = append(newEdges, e)
		}
	}
	for _, reqID := range reqIDs {
		newEdges = append(newEdges, domain.PrerequisiteEdge{NodeID: nodeID, RequiresNodeID: reqID})
	}
	m.prereqEdges = newEdges
	return nil
}

func (m *foundationMockRepo) ListPrerequisitesForNode(_ context.Context, nodeID uuid.UUID) ([]domain.Taxonomy, error) {
	var list []domain.Taxonomy
	for _, e := range m.prereqEdges {
		if e.NodeID == nodeID {
			if t, ok := m.taxonomiesByID[e.RequiresNodeID]; ok {
				list = append(list, t)
			}
		}
	}
	return list, nil
}

func (m *foundationMockRepo) ListDependantsForNode(_ context.Context, nodeID uuid.UUID) ([]domain.Taxonomy, error) {
	var list []domain.Taxonomy
	for _, e := range m.prereqEdges {
		if e.RequiresNodeID == nodeID {
			if t, ok := m.taxonomiesByID[e.NodeID]; ok {
				list = append(list, t)
			}
		}
	}
	return list, nil
}

func (m *foundationMockRepo) CountTaggedContentByKindForTaxonomy(
	_ context.Context, taxID uuid.UUID,
) (map[string]int, error) {
	if counts, ok := m.taggedCounts[taxID]; ok {
		return counts, nil
	}
	return map[string]int{}, nil
}

func (m *foundationMockRepo) GetPublishedTopicBodyByTaxonomyID(
	_ context.Context, taxID uuid.UUID,
) ([]byte, bool, error) {
	if b, ok := m.topicBodies[taxID]; ok {
		return b, true, nil
	}
	return nil, false, nil
}

func (m *foundationMockRepo) ListTagsForContentItem(_ context.Context, itemID uuid.UUID) ([]domain.Taxonomy, error) {
	return m.topicTags[itemID], nil
}

// foundationFixture is a service with the three grammar nodes the cycle and
// path tests both need. Built per test rather than shared, so one test's edges
// cannot decide another's result.
func foundationFixture(t *testing.T) (*service.Service, context.Context, uuid.UUID) {
	t.Helper()

	repo := newFoundationMockRepo()
	svc := service.New(service.Deps{
		Pool:   &fakeBeginner{},
		Repo:   repo,
		Events: &fakeEvents{},
		Clock:  clock.Real{},
		NewID:  uuid.New,
	})
	ctx := context.Background()
	adminID := uuid.New()

	nodes := []struct {
		code     string
		label    string
		position int
	}{
		{codeSentenceStructure, "Sentence Structure", 10},
		{codePresentSimple, labelPresentSimple, 20},
		{codePastSimple, "Past Simple", 30},
	}
	for _, n := range nodes {
		if _, err := svc.CreateFoundationTopic(ctx, adminID, service.CreateFoundationTopicRequest{
			Namespace: nsGrammar,
			Code:      n.code,
			Label:     n.label,
			Position:  n.position,
		}); err != nil {
			t.Fatalf("create %s: %v", n.code, err)
		}
	}
	return svc, ctx, adminID
}

// TestFoundationService_CreateValidatesNamespaceAndCode. A code is permanent
// (BR-FOUNDATION-01), so the only chance to refuse a malformed one is here.
func TestFoundationService_CreateValidatesNamespaceAndCode(t *testing.T) {
	t.Parallel()

	repo := newFoundationMockRepo()
	svc := service.New(service.Deps{
		Pool:   &fakeBeginner{},
		Repo:   repo,
		Events: &fakeEvents{},
		Clock:  clock.Real{},
		NewID:  uuid.New,
	})
	ctx := context.Background()
	adminID := uuid.New()

	cases := []struct {
		name      string
		namespace string
		code      string
	}{
		{"unknown namespace", "invalid_ns", codePresentSimple},
		{"kebab-case code in a spine namespace", nsGrammar, "present-simple"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.CreateFoundationTopic(ctx, adminID, service.CreateFoundationTopicRequest{
				Namespace: tc.namespace,
				Code:      tc.code,
				Label:     labelPresentSimple,
			}); err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
		})
	}

	created, err := svc.CreateFoundationTopic(ctx, adminID, service.CreateFoundationTopicRequest{
		Namespace: nsGrammar,
		Code:      codeSentenceStructure,
		Label:     "Sentence Structure",
		Position:  10,
	})
	if err != nil {
		t.Fatalf("create SENTENCE_STRUCTURE: %v", err)
	}
	if created.Code != codeSentenceStructure {
		t.Errorf("got code %s, want %s", created.Code, codeSentenceStructure)
	}
}

// TestFoundationService_RefusesCycles is BR-FOUNDATION-02, and the assertion
// that matters is the second one: a refused cycle must leave the graph alone.
func TestFoundationService_RefusesCycles(t *testing.T) {
	t.Parallel()

	svc, ctx, adminID := foundationFixture(t)

	if err := svc.ReplacePrerequisites(ctx, adminID, codePresentSimple, []string{codeSentenceStructure}); err != nil {
		t.Fatalf("set prerequisite: %v", err)
	}
	if err := svc.ReplacePrerequisites(ctx, adminID, codePastSimple, []string{codePresentSimple}); err != nil {
		t.Fatalf("set prerequisite: %v", err)
	}

	// SENTENCE_STRUCTURE requiring PAST_SIMPLE closes the loop.
	err := svc.ReplacePrerequisites(ctx, adminID, codeSentenceStructure, []string{codePastSimple})
	if err == nil {
		t.Fatal("a cycle was accepted")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "TAXONOMY_CYCLE" {
		t.Fatalf("expected TAXONOMY_CYCLE, got: %v", err)
	}

	topic, err := svc.GetFoundationTopicByCode(ctx, codeSentenceStructure)
	if err != nil {
		t.Fatalf("get topic: %v", err)
	}
	if len(topic.Prerequisites) != 0 {
		t.Errorf("a refused cycle must write nothing; SENTENCE_STRUCTURE has %d prerequisites",
			len(topic.Prerequisites))
	}
}

// TestFoundationService_PathGeneration is BR-FOUNDATION-06.
func TestFoundationService_PathGeneration(t *testing.T) {
	t.Parallel()

	svc, ctx, adminID := foundationFixture(t)

	if err := svc.ReplacePrerequisites(ctx, adminID, codePresentSimple, []string{codeSentenceStructure}); err != nil {
		t.Fatalf("set prerequisite: %v", err)
	}
	if err := svc.ReplacePrerequisites(ctx, adminID, codePastSimple, []string{codePresentSimple}); err != nil {
		t.Fatalf("set prerequisite: %v", err)
	}

	target := codePastSimple
	path, err := svc.GetFoundationPath(ctx, &target, nil)
	if err != nil {
		t.Fatalf("GetFoundationPath: %v", err)
	}

	want := []string{codeSentenceStructure, codePresentSimple, codePastSimple}
	if len(path) != len(want) {
		t.Fatalf("got path of %d, want %d", len(path), len(want))
	}
	for i, code := range want {
		if path[i].Code != code {
			t.Errorf("path[%d] = %s, want %s", i, path[i].Code, code)
		}
	}
}

func TestFoundationService_PublishCompletenessGate(t *testing.T) {
	t.Parallel()

	repo := newFoundationMockRepo()
	svc := service.New(service.Deps{
		Pool:   &fakeBeginner{},
		Repo:   repo,
		Events: &fakeEvents{},
		Clock:  clock.Real{},
		NewID:  uuid.New,
	})

	ctx := context.Background()
	adminID := uuid.New()
	taxID := uuid.New()

	topicTax := domain.Taxonomy{
		ID:        taxID,
		Namespace: nsGrammar,
		Code:      codePresentPerfect,
		Label:     "Present Perfect",
	}
	repo.taxonomies[topicTax.Code] = topicTax
	repo.taxonomiesByID[taxID] = topicTax

	itemID := uuid.New()
	verID := uuid.New()
	item := domain.Item{
		ID:               itemID,
		Kind:             kindFoundationTopic,
		Slug:             "foundation-present-perfect",
		CurrentVersionID: &verID,
		Status:           domain.StatusInReview,
		OwnerID:          adminID,
	}
	version := domain.Version{
		ID:        verID,
		ItemID:    itemID,
		Version:   1,
		Kind:      kindFoundationTopic,
		Status:    domain.StatusApproved,
		CEFRLevel: "B1",
	}

	repo.items[itemID] = item
	repo.versions[verID] = version
	repo.topicTags[itemID] = []domain.Taxonomy{topicTax}

	// Case 1: No exercises/quizzes/reviews tagged to node -> fail with FOUNDATION_INCOMPLETE
	repo.taggedCounts[taxID] = map[string]int{
		kindFoundationTopic: 1,
	}

	_, err := svc.Publish(ctx, adminID, itemID)
	if err == nil {
		t.Fatal("expected publish to fail when exercises/quizzes/reviews are missing")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "FOUNDATION_INCOMPLETE" {
		t.Fatalf("expected FOUNDATION_INCOMPLETE error code, got: %v", err)
	}

	// Case 2: Only exercise and quiz present, missing review -> still fails
	repo.taggedCounts[taxID] = map[string]int{
		kindFoundationTopic: 1,
		kindExercise:        2,
		kindFoundationQuiz:  1,
	}

	_, err = svc.Publish(ctx, adminID, itemID)
	if err == nil {
		t.Fatal("expected publish to fail when review questions are missing")
	}
	if !errors.As(err, &ae) || ae.Code != "FOUNDATION_INCOMPLETE" {
		t.Fatalf("expected FOUNDATION_INCOMPLETE error code, got: %v", err)
	}

	// Case 3: Exercise, quiz, and review question all present -> successfully publishes!
	repo.taggedCounts[taxID] = map[string]int{
		kindFoundationTopic: 1,
		kindExercise:        2,
		kindFoundationQuiz:  1,
		"foundation_review": 1,
	}

	publishedVer, err := svc.Publish(ctx, adminID, itemID)
	if err != nil {
		t.Fatalf("expected publish to succeed when all gates are met: %v", err)
	}
	if publishedVer.Status != domain.StatusPublished {
		t.Errorf("got version status %v, want published", publishedVer.Status)
	}
}
