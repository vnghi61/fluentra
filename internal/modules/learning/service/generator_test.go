package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const (
	testNodeCodePresentPerfect = "PRESENT_PERFECT"
	testLabelPresentPerfect    = "Present Perfect"
	testKindTenseChoice        = "grammar_tense_choice"
	testPurposeFoundation      = "foundation"
)

type generatorTestAuthor struct {
	mu        sync.Mutex
	drafts    []contentcontract.AuthorSpec
	published []contentcontract.AuthorSpec
}

func (a *generatorTestAuthor) EnsurePublished(_ context.Context, spec contentcontract.AuthorSpec) (uuid.UUID, error) {
	if spec.AuthorID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("spec has nil author id")
	}
	if !kebabSlug.MatchString(spec.Slug) {
		return uuid.Nil, fmt.Errorf("slug %q is not kebab-case", spec.Slug)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.published = append(a.published, spec)
	return uuid.New(), nil
}

func (a *generatorTestAuthor) EnsureDraft(_ context.Context, spec contentcontract.AuthorSpec) (uuid.UUID, error) {
	if spec.AuthorID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("spec has nil author id")
	}
	if !kebabSlug.MatchString(spec.Slug) {
		return uuid.Nil, fmt.Errorf("slug %q is not kebab-case", spec.Slug)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.drafts = append(a.drafts, spec)
	return uuid.New(), nil
}

type generatorTestTaxonomies struct {
	nodes map[string]*contentcontract.TaxonomyNode
}

func (t *generatorTestTaxonomies) ResolveTaxonomyID(_ context.Context, namespace, code string) (*uuid.UUID, error) {
	for _, n := range t.nodes {
		if n.Namespace == namespace && n.Code == code {
			return &n.ID, nil
		}
	}
	return nil, nil
}

func (t *generatorTestTaxonomies) GetTaxonomyByCode(
	_ context.Context, code string,
) (*contentcontract.TaxonomyNode, error) {
	node, ok := t.nodes[code]
	if !ok {
		return nil, fmt.Errorf("not found: %s", code)
	}
	return node, nil
}

func (t *generatorTestTaxonomies) ListTaxonomiesInNamespace(
	_ context.Context, namespace string,
) ([]contentcontract.TaxonomyNode, error) {
	var res []contentcontract.TaxonomyNode
	for _, n := range t.nodes {
		if n.Namespace == namespace {
			res = append(res, *n)
		}
	}
	return res, nil
}

func (t *generatorTestTaxonomies) ListPrerequisites(
	_ context.Context, _ uuid.UUID,
) ([]contentcontract.TaxonomyNode, error) {
	return nil, nil
}

func (t *generatorTestTaxonomies) GetTaxonomyByID(
	_ context.Context, id uuid.UUID,
) (*contentcontract.TaxonomyNode, error) {
	for _, n := range t.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, nil
}

func (t *generatorTestTaxonomies) GetTaxonomyPath(
	_ context.Context, _ *string, _ *string,
) ([]contentcontract.TaxonomyNode, error) {
	var res []contentcontract.TaxonomyNode
	for _, n := range t.nodes {
		res = append(res, *n)
	}
	return res, nil
}

// TestGenerator_WorkOrder19StageCGate verifies the WO-19 Stage C Gate:
// 1. Generation call for grammar_tense_choice, PRESENT_PERFECT, B1, count 3, purpose foundation.
// 2. Produces 3 draft versions authored via EnsureDraft.
// 3. Tagged grammar.PRESENT_PERFECT.
// 4. Each has _provenance with prompt_version, model, and ai_request_id.
// 5. RedactForLearner strips _provenance so none is visible to a learner.
func TestGenerator_WorkOrder19StageCGate(t *testing.T) {
	ctx := context.Background()
	author := &generatorTestAuthor{}
	taxonomies := &generatorTestTaxonomies{
		nodes: map[string]*contentcontract.TaxonomyNode{
			testNodeCodePresentPerfect: {
				ID:        uuid.New(),
				Namespace: testSkillGrammar,
				Code:      testNodeCodePresentPerfect,
				Label:     testLabelPresentPerfect,
			},
		},
	}

	lessons := newFakePoolLessons()
	mockAI := ai.NewMockProvider(nil)
	adminID := uuid.New()

	svc := service.New(service.Deps{
		Lesson:            lessons,
		LessonAuthor:      lessons,
		Content:           newFakeContentReader(),
		ContentAuthor:     author,
		Taxonomies:        taxonomies,
		Graders:           passingGraders(),
		AI:                mockAI,
		Clock:             clock.NewFake(time.Now()),
		GeneratorAuthorID: adminID,
		Synthesiser:       &fakeAudioSynthesiser{},
	})

	req := learningcontract.GenerateRequest{
		Kind:      testKindTenseChoice,
		CEFRLevel: "B1",
		NodeCodes: []string{testNodeCodePresentPerfect},
		Count:     3,
		Purpose:   testPurposeFoundation,
	}

	items, err := svc.Generate(ctx, req)
	require.NoError(t, err)
	require.Len(t, items, 3, "must produce exactly 3 items")

	// Verify authoring was through EnsureDraft
	assert.Empty(t, author.published, "foundation items must never be published directly")
	require.Len(t, author.drafts, 3, "foundation items must be authored as drafts")

	for i, item := range items {
		assert.NotEqual(t, uuid.Nil, item.ContentVersionID)
		assert.NotEmpty(t, item.PromptVersion)
		assert.NotEmpty(t, item.Model)
		assert.NotEqual(t, uuid.Nil, item.AIRequestID)

		// Verify tagging on the authored draft spec
		draftSpec := author.drafts[i]
		require.Len(t, draftSpec.Tags, 1)
		assert.Equal(t, "grammar", draftSpec.Tags[0].Namespace)
		assert.Equal(t, testNodeCodePresentPerfect, draftSpec.Tags[0].Code)

		// Verify _provenance exists in body
		var bodyMap map[string]any
		err := json.Unmarshal(item.Body, &bodyMap)
		require.NoError(t, err)

		provRaw, exists := bodyMap["_provenance"]
		require.True(t, exists, "body must contain _provenance")
		prov, ok := provRaw.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, item.PromptVersion, prov["prompt_version"])
		assert.Equal(t, item.Model, prov["model"])
		assert.Equal(t, item.AIRequestID.String(), prov["ai_request_id"])

		// Verify RedactForLearner strips _provenance
		redacted := contentcontract.RedactForLearner(item.Body)
		var redactedMap map[string]any
		err = json.Unmarshal(redacted, &redactedMap)
		require.NoError(t, err)
		_, provInRedacted := redactedMap["_provenance"]
		assert.False(t, provInRedacted, "_provenance must be stripped by RedactForLearner")
	}

	// Verify unknown spine node returns validation error
	_, err = svc.Generate(ctx, learningcontract.GenerateRequest{
		Kind:      testKindTenseChoice,
		CEFRLevel: "B1",
		NodeCodes: []string{"UNKNOWN_NODE_CODE"},
		Count:     1,
		Purpose:   "foundation",
	})
	require.Error(t, err)
	var aErr *apperr.Error
	require.True(t, errors.As(err, &aErr))
	assert.Equal(t, "GENERATOR_UNKNOWN_SPINE_CODE", aErr.Code)

	// Verify practice purpose uses EnsurePublished
	practiceItems, err := svc.Generate(ctx, learningcontract.GenerateRequest{
		Kind:      testKindTenseChoice,
		CEFRLevel: "B1",
		NodeCodes: []string{testNodeCodePresentPerfect},
		Count:     1,
		Purpose:   "practice",
	})
	require.NoError(t, err)
	require.Len(t, practiceItems, 1)
	assert.Len(t, author.published, 1, "practice items must be published directly")
}

// TestGenerator_FoundationContentKinds verifies that foundation_topic, foundation_quiz,
// and foundation_review can all be generated with purpose foundation, landing as drafts tagged to the node.
func TestGenerator_FoundationContentKinds(t *testing.T) {
	ctx := context.Background()
	author := &generatorTestAuthor{}
	taxonomies := &generatorTestTaxonomies{
		nodes: map[string]*contentcontract.TaxonomyNode{
			testNodeCodePresentPerfect: {
				ID:        uuid.New(),
				Namespace: testSkillGrammar,
				Code:      testNodeCodePresentPerfect,
				Label:     testLabelPresentPerfect,
			},
		},
	}

	lessons := newFakePoolLessons()
	mockAI := ai.NewMockProvider(nil)
	adminID := uuid.New()

	svc := service.New(service.Deps{
		Lesson:            lessons,
		LessonAuthor:      lessons,
		Content:           newFakeContentReader(),
		ContentAuthor:     author,
		Taxonomies:        taxonomies,
		Graders:           passingGraders(),
		AI:                mockAI,
		Clock:             clock.NewFake(time.Now()),
		GeneratorAuthorID: adminID,
		Synthesiser:       &fakeAudioSynthesiser{},
	})

	kinds := []string{
		learningcontract.KindFoundationTopic,
		learningcontract.KindFoundationQuiz,
		learningcontract.KindFoundationReview,
	}

	for _, kind := range kinds {
		t.Run("generate_"+kind, func(t *testing.T) {
			items, err := svc.Generate(ctx, learningcontract.GenerateRequest{
				Kind:       kind,
				CEFRLevel:  "B1",
				NodeCodes:  []string{testNodeCodePresentPerfect},
				Count:      1,
				Purpose:    testPurposeFoundation,
				SlugPrefix: "foundation-" + kind + "-present-perfect",
			})
			require.NoError(t, err)
			require.Len(t, items, 1)
			item := items[0]
			assert.NotEqual(t, uuid.Nil, item.ContentVersionID)

			if kind == learningcontract.KindFoundationTopic {
				assert.Equal(t, "foundation_topic_generate.v1", item.PromptVersion)
			} else {
				assert.Equal(t, "item_generate.v1", item.PromptVersion)
			}

			// Verify provenance
			var bodyMap map[string]any
			err = json.Unmarshal(item.Body, &bodyMap)
			require.NoError(t, err)
			provRaw, exists := bodyMap["_provenance"]
			require.True(t, exists, "item must contain _provenance")
			prov := provRaw.(map[string]any)
			assert.Equal(t, testPurposeFoundation, prov["purpose"])
		})
	}

	// All 3 items must have landed as drafts
	assert.Len(t, author.drafts, 3)
}
