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
	approved  []uuid.UUID
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

func (a *generatorTestAuthor) ApproveVerified(
	_ context.Context, versionID uuid.UUID, _ contentcontract.Verification,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.approved = append(a.approved, versionID)
	return nil
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

// autoPublishAI delegates generation and level judging to a real mock provider
// and answers the independent verifier itself, so a test can force a
// confirmation or a doubt.
type autoPublishAI struct {
	inner   ai.Client
	solve   string
	verdict string
}

func (a *autoPublishAI) Complete(ctx context.Context, req ai.Request) (ai.Response, error) {
	switch req.Task {
	case ai.TaskItemSolve:
		return ai.Response{
			Text:  fmt.Sprintf(`{"selected_option_id": %q}`, a.solve),
			Model: testVerifierModel,
		}, nil
	case ai.TaskItemVerify:
		return ai.Response{
			Text:  fmt.Sprintf(`{"verdict": %q, "reason": ""}`, a.verdict),
			Model: testVerifierModel,
		}, nil
	default:
		return a.inner.Complete(ctx, req)
	}
}

// fakeVerificationRecorder records the doubts a run leaves on its drafts.
type fakeVerificationRecorder struct {
	received []contentcontract.Verification
}

func (f *fakeVerificationRecorder) RecordVerification(
	_ context.Context, _ uuid.UUID, v contentcontract.Verification,
) error {
	f.received = append(f.received, v)
	return nil
}

// TestGenerate_AutoPublishPublishesWhatTheVerifierConfirms is the WO 22 Stage A
// gate: with an independent model confirming, a generated item publishes with
// no person.
func TestGenerate_AutoPublishPublishesWhatTheVerifierConfirms(t *testing.T) {
	author := &generatorTestAuthor{}
	recorder := &fakeVerificationRecorder{}
	svc := service.New(service.Deps{
		Lesson:            newFakePoolLessons(),
		LessonAuthor:      newFakePoolLessons(),
		Content:           newFakeContentReader(),
		ContentAuthor:     author,
		ContentRecorder:   recorder,
		AutoPublish:       true,
		Taxonomies:        generatorTestTaxonomySet(),
		Graders:           passingGraders(),
		AI:                &autoPublishAI{inner: ai.NewMockProvider(nil), solve: "A", verdict: "confirmed"},
		Clock:             clock.NewFake(time.Now()),
		GeneratorAuthorID: uuid.New(),
		Synthesiser:       &fakeAudioSynthesiser{},
	})

	items, err := svc.Generate(context.Background(), learningcontract.GenerateRequest{
		Kind:      testKindTenseChoice,
		CEFRLevel: "B1",
		NodeCodes: []string{testNodeCodePresentPerfect},
		Count:     1,
		Purpose:   "bank",
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Len(t, author.approved, 1, "a confirmed item publishes through ApproveVerified")
	assert.Empty(t, recorder.received, "a confirmed item leaves no doubt")
}

// TestGenerate_AFoundationItemWaitsForItsNode is D22-13: a confirmed Foundation
// item is not published on its own. The verdict is recorded, and the node's
// batch is published whole once its topic and every item are confirmed.
func TestGenerate_AFoundationItemWaitsForItsNode(t *testing.T) {
	author := &generatorTestAuthor{}
	recorder := &fakeVerificationRecorder{}
	svc := service.New(service.Deps{
		Lesson:            newFakePoolLessons(),
		LessonAuthor:      newFakePoolLessons(),
		Content:           newFakeContentReader(),
		ContentAuthor:     author,
		ContentRecorder:   recorder,
		AutoPublish:       true,
		Taxonomies:        generatorTestTaxonomySet(),
		Graders:           passingGraders(),
		AI:                &autoPublishAI{inner: ai.NewMockProvider(nil), solve: "A", verdict: "confirmed"},
		Clock:             clock.NewFake(time.Now()),
		GeneratorAuthorID: uuid.New(),
		Synthesiser:       &fakeAudioSynthesiser{},
	})

	_, err := svc.Generate(context.Background(), learningcontract.GenerateRequest{
		Kind:      testKindTenseChoice,
		CEFRLevel: "B1",
		NodeCodes: []string{testNodeCodePresentPerfect},
		Count:     1,
		Purpose:   testPurposeFoundation,
	})
	require.NoError(t, err)
	assert.Empty(t, author.approved, "a Foundation item publishes with its node, not alone")
	require.Len(t, recorder.received, 1, "the verdict is recorded for the node's batch")
	assert.True(t, recorder.received[0].Confirmed)
}

// TestGenerate_AutoPublishLeavesADoubtForAPerson. A verifier that does not
// confirm records the doubt on the draft instead of publishing it (D22-2).
func TestGenerate_AutoPublishLeavesADoubtForAPerson(t *testing.T) {
	author := &generatorTestAuthor{}
	recorder := &fakeVerificationRecorder{}
	svc := service.New(service.Deps{
		Lesson:            newFakePoolLessons(),
		LessonAuthor:      newFakePoolLessons(),
		Content:           newFakeContentReader(),
		ContentAuthor:     author,
		ContentRecorder:   recorder,
		AutoPublish:       true,
		Taxonomies:        generatorTestTaxonomySet(),
		Graders:           passingGraders(),
		AI:                &autoPublishAI{inner: ai.NewMockProvider(nil), solve: "A", verdict: "doubt"},
		Clock:             clock.NewFake(time.Now()),
		GeneratorAuthorID: uuid.New(),
		Synthesiser:       &fakeAudioSynthesiser{},
	})

	_, err := svc.Generate(context.Background(), learningcontract.GenerateRequest{
		Kind:      testKindTenseChoice,
		CEFRLevel: "B1",
		NodeCodes: []string{testNodeCodePresentPerfect},
		Count:     1,
		Purpose:   testPurposeFoundation,
	})
	require.NoError(t, err)
	assert.Empty(t, author.approved, "a doubted item does not publish")
	require.Len(t, recorder.received, 1, "the doubt is recorded on the draft")
	assert.False(t, recorder.received[0].Confirmed)
}

// TestGenerate_AppliesTheExamPartConstraints is the WO 22 Stage I enforcement
// at the generator: an exam part's option count reaches the structure check, so
// a four-option item cannot be generated for a three-option part.
func TestGenerate_AppliesTheExamPartConstraints(t *testing.T) {
	svc := service.New(service.Deps{
		Lesson:            newFakePoolLessons(),
		LessonAuthor:      newFakePoolLessons(),
		Content:           newFakeContentReader(),
		ContentAuthor:     &generatorTestAuthor{},
		Taxonomies:        generatorTestTaxonomySet(),
		Graders:           passingGraders(),
		AI:                ai.NewMockProvider(nil),
		Clock:             clock.NewFake(time.Now()),
		GeneratorAuthorID: uuid.New(),
		Synthesiser:       &fakeAudioSynthesiser{},
	})

	_, err := svc.Generate(context.Background(), learningcontract.GenerateRequest{
		Kind:      testKindTenseChoice,
		CEFRLevel: "B1",
		NodeCodes: []string{testNodeCodePresentPerfect},
		Count:     1,
		Purpose:   testPurposeFoundation,
		// The mock writes four options; TOEIC Part 2 has three.
		ExamConstraints: &learningcontract.ExamPartConstraints{OptionCount: 3},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exam structure")
}

// generatorTestTaxonomySet is the one-node taxonomy the auto-publish tests use.
func generatorTestTaxonomySet() *generatorTestTaxonomies {
	return &generatorTestTaxonomies{
		nodes: map[string]*contentcontract.TaxonomyNode{
			testNodeCodePresentPerfect: {
				ID:        uuid.New(),
				Namespace: testSkillGrammar,
				Code:      testNodeCodePresentPerfect,
				Label:     testLabelPresentPerfect,
			},
		},
	}
}

// The count sizes an allocation and a loop of model calls, and it arrives from a
// request body: past the spec's maximum it is refused before anything runs.
func TestGenerator_RefusesACountPastTheMaximum(t *testing.T) {
	mockAI := ai.NewMockProvider(nil)
	svc := service.New(service.Deps{
		Content:           newFakeContentReader(),
		ContentAuthor:     &generatorTestAuthor{},
		AI:                mockAI,
		Clock:             clock.NewFake(time.Now()),
		GeneratorAuthorID: uuid.New(),
	})

	_, err := svc.Generate(context.Background(), learningcontract.GenerateRequest{
		Kind:      testKindTenseChoice,
		CEFRLevel: "B1",
		NodeCodes: []string{testNodeCodePresentPerfect},
		Count:     1 << 30,
		Purpose:   testPurposeFoundation,
	})

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "GENERATOR_COUNT_TOO_LARGE", appErr.Code)
}

// TestGenerate_AOneQuestionRecordingForVSTEPPart1: VSTEP Listening Part 1 is
// eight announcements of one question each. The candidate check demanded four
// questions a recording and refused every Part 1 item; the part's own count
// now sets the floor.
func TestGenerate_AOneQuestionRecordingForVSTEPPart1(t *testing.T) {
	svc := service.New(service.Deps{
		Lesson:            newFakePoolLessons(),
		LessonAuthor:      newFakePoolLessons(),
		Content:           newFakeContentReader(),
		ContentAuthor:     &generatorTestAuthor{},
		Taxonomies:        generatorTestTaxonomySet(),
		Graders:           passingGraders(),
		AI:                ai.NewMockProvider(nil),
		Clock:             clock.NewFake(time.Now()),
		GeneratorAuthorID: uuid.New(),
	})

	items, err := svc.Generate(context.Background(), learningcontract.GenerateRequest{
		Kind:      "listening_comprehension",
		CEFRLevel: "B1",
		NodeCodes: []string{testNodeCodePresentPerfect},
		Count:     1,
		Purpose:   "bank",
		ExamConstraints: &learningcontract.ExamPartConstraints{
			OptionCount: 4, QuestionsPerGroup: 1, AudioRequired: true,
		},
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	var body struct {
		Questions []json.RawMessage `json:"questions"`
	}
	require.NoError(t, json.Unmarshal(items[0].Body, &body))
	assert.Len(t, body.Questions, 1, "one question per announcement, as the part states")
}
