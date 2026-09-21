package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	learninghttp "github.com/fluentra/fluentra/internal/modules/learning/transport/http"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const (
	testNodeCodeSentenceStructure = "SENTENCE_STRUCTURE"
	testNodeCodePresentSimple     = "PRESENT_SIMPLE"
	testNodeCodePresentContinuous = "PRESENT_CONTINUOUS"
)

type fakeTaxonomyResolverForGateJ struct {
	nodes        []contentcontract.TaxonomyNode
	prereqMap    map[uuid.UUID][]contentcontract.TaxonomyNode
	taxonomyPath []contentcontract.TaxonomyNode
}

func (f *fakeTaxonomyResolverForGateJ) ResolveTaxonomyID(
	_ context.Context, _, code string,
) (*uuid.UUID, error) {
	for _, n := range f.nodes {
		if n.Code == code {
			return &n.ID, nil
		}
	}
	return nil, nil
}

func (f *fakeTaxonomyResolverForGateJ) GetTaxonomyByCode(
	_ context.Context, code string,
) (*contentcontract.TaxonomyNode, error) {
	for _, n := range f.nodes {
		if n.Code == code {
			return &n, nil
		}
	}
	return nil, nil
}

func (f *fakeTaxonomyResolverForGateJ) GetTaxonomyByID(
	_ context.Context, id uuid.UUID,
) (*contentcontract.TaxonomyNode, error) {
	for _, n := range f.nodes {
		if n.ID == id {
			return &n, nil
		}
	}
	return nil, nil
}

func (f *fakeTaxonomyResolverForGateJ) ListTaxonomiesInNamespace(
	_ context.Context, _ string,
) ([]contentcontract.TaxonomyNode, error) {
	return f.nodes, nil
}

func (f *fakeTaxonomyResolverForGateJ) ListPrerequisites(
	_ context.Context, nodeID uuid.UUID,
) ([]contentcontract.TaxonomyNode, error) {
	return f.prereqMap[nodeID], nil
}

func (f *fakeTaxonomyResolverForGateJ) GetTaxonomyPath(
	_ context.Context, targetCode *string, _ *string,
) ([]contentcontract.TaxonomyNode, error) {
	if targetCode != nil {
		return f.taxonomyPath, nil
	}
	return f.nodes, nil
}

// gateJFixture constructs standard nodes for the WO 16 acceptance chain:
// SENTENCE_STRUCTURE -> PRESENT_SIMPLE -> PRESENT_CONTINUOUS -> PRESENT_PERFECT.
func gateJFixture() (
	nodeSentenceStructure uuid.UUID,
	nodePresentSimple uuid.UUID,
	nodePresentContinuous uuid.UUID,
	nodePresentPerfect uuid.UUID,
	fakeTax *fakeTaxonomyResolverForGateJ,
) {
	lvlA1 := "A1"
	lvlB1 := "B1"

	nodeSentenceStructure = uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def01234567a")
	nodePresentSimple = uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def01234567b")
	nodePresentContinuous = uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def01234567c")
	nodePresentPerfect = uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def01234567d")

	nodes := []contentcontract.TaxonomyNode{
		{
			ID:        nodeSentenceStructure,
			Namespace: testSkillGrammar,
			Code:      testNodeCodeSentenceStructure,
			Label:     "Sentence Structure",
			CEFRLevel: &lvlA1,
		},
		{
			ID:        nodePresentSimple,
			Namespace: testSkillGrammar,
			Code:      testNodeCodePresentSimple,
			Label:     "Present Simple",
			CEFRLevel: &lvlA1,
		},
		{
			ID:        nodePresentContinuous,
			Namespace: testSkillGrammar,
			Code:      testNodeCodePresentContinuous,
			Label:     "Present Continuous",
			CEFRLevel: &lvlA1,
		},
		{
			ID:        nodePresentPerfect,
			Namespace: testSkillGrammar,
			Code:      testNodeCodePresentPerfect,
			Label:     testLabelPresentPerfect,
			CEFRLevel: &lvlB1,
		},
	}

	prereqMap := map[uuid.UUID][]contentcontract.TaxonomyNode{
		nodeSentenceStructure: {},
		nodePresentSimple:     {nodes[0]},
		nodePresentContinuous: {nodes[1]},
		nodePresentPerfect:    {nodes[2]},
	}

	fakeTax = &fakeTaxonomyResolverForGateJ{
		nodes:        nodes,
		prereqMap:    prereqMap,
		taxonomyPath: nodes,
	}

	return nodeSentenceStructure, nodePresentSimple, nodePresentContinuous, nodePresentPerfect, fakeTax
}

// TestGateJ_AcceptanceChain verifies:
// "For a learner with mastery on SENTENCE_STRUCTURE and PRESENT_SIMPLE only,
// the path to PRESENT_PERFECT marks PRESENT_CONTINUOUS as next."
func TestGateJ_AcceptanceChain(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	nodeSS, nodePS, nodePC, nodePP, fakeTax := gateJFixture()

	fakeRepo := newFakeRepo()
	for _, m := range []domain.NodeMastery{
		{
			UserID:   userID,
			NodeID:   nodeSS,
			Attempts: 6,
			Correct:  6,
			Score:    0.95,
		},
		{
			UserID:   userID,
			NodeID:   nodePS,
			Attempts: 5,
			Correct:  5,
			Score:    0.85,
		},
		{
			UserID:   userID,
			NodeID:   nodePC,
			Attempts: 2,
			Correct:  1,
			Score:    0.50, // not mastered: attempts < 5 and score < 0.8
		},
		// PRESENT_PERFECT has 0 attempts
	} {
		_, err := fakeRepo.UpsertNodeMastery(ctx, m)
		require.NoError(t, err)
	}

	svc := service.New(service.Deps{
		Repo:       fakeRepo,
		Taxonomies: fakeTax,
	})

	path, err := svc.GetLearnerFoundationPath(ctx, userID, testNodeCodePresentPerfect)
	require.NoError(t, err)
	require.Equal(t, testNodeCodePresentPerfect, path.Target)
	require.Len(t, path.Items, 4)

	// 1. SENTENCE_STRUCTURE: mastered, not next
	assert.True(t, path.Items[0].Mastered)
	assert.False(t, path.Items[0].Next)

	// 2. PRESENT_SIMPLE: mastered, not next
	assert.True(t, path.Items[1].Mastered)
	assert.False(t, path.Items[1].Next)

	// 3. PRESENT_CONTINUOUS: not mastered, MUST BE NEXT (Gate J primary assertion)
	assert.False(t, path.Items[2].Mastered)
	assert.True(t, path.Items[2].Next)
	assert.Equal(t, testNodeCodePresentContinuous, path.Items[2].Code)

	// 4. PRESENT_PERFECT: not mastered, not next
	assert.Equal(t, nodePP, path.Items[3].ID)
	assert.False(t, path.Items[3].Mastered)
	assert.False(t, path.Items[3].Next)
}

// TestGateJ_AllNodesMastered verifies:
// "A path to a node already mastered returns it as mastered, not as next."
func TestGateJ_AllNodesMastered(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	nodeSS, nodePS, nodePC, nodePP, fakeTax := gateJFixture()

	fakeRepo := newFakeRepo()
	for _, m := range []domain.NodeMastery{
		{UserID: userID, NodeID: nodeSS, Attempts: 5, Score: 0.85},
		{UserID: userID, NodeID: nodePS, Attempts: 6, Score: 0.90},
		{UserID: userID, NodeID: nodePC, Attempts: 7, Score: 0.88},
		{UserID: userID, NodeID: nodePP, Attempts: 5, Score: 0.82},
	} {
		_, err := fakeRepo.UpsertNodeMastery(ctx, m)
		require.NoError(t, err)
	}

	svc := service.New(service.Deps{
		Repo:       fakeRepo,
		Taxonomies: fakeTax,
	})

	path, err := svc.GetLearnerFoundationPath(ctx, userID, testNodeCodePresentPerfect)
	require.NoError(t, err)

	for _, item := range path.Items {
		assert.True(t, item.Mastered, "node %s should be mastered", item.Code)
		assert.False(t, item.Next, "already mastered node %s must NOT be next", item.Code)
	}
}

// TestGateJ_NextEndpoint verifies GET /api/v1/me/foundation/next returns the next unmastered topic
// whose prerequisites are met.
func TestGateJ_NextEndpoint(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	nodeSS, nodePS, _, _, fakeTax := gateJFixture()

	fakeRepo := newFakeRepo()
	for _, m := range []domain.NodeMastery{
		{UserID: userID, NodeID: nodeSS, Attempts: 5, Score: 0.85},
		{UserID: userID, NodeID: nodePS, Attempts: 5, Score: 0.85},
	} {
		_, err := fakeRepo.UpsertNodeMastery(ctx, m)
		require.NoError(t, err)
	}

	svc := service.New(service.Deps{
		Repo:       fakeRepo,
		Taxonomies: fakeTax,
	})

	next, err := svc.GetLearnerFoundationNext(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, next)

	// Because SENTENCE_STRUCTURE and PRESENT_SIMPLE are mastered, PRESENT_CONTINUOUS is next
	assert.Equal(t, testNodeCodePresentContinuous, next.Code)
	assert.True(t, next.Next)
}

type fakeGuardGateJ struct{}

func (f *fakeGuardGateJ) Require(_ context.Context, _ string) error {
	return nil
}

// TestGateJ_HTTPRouteIntegration verifies HTTP transport for /me/foundation/path and /me/foundation/next.
func TestGateJ_HTTPRouteIntegration(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	nodeSS, nodePS, _, _, fakeTax := gateJFixture()

	fakeRepo := newFakeRepo()
	for _, m := range []domain.NodeMastery{
		{UserID: userID, NodeID: nodeSS, Attempts: 6, Score: 0.90},
		{UserID: userID, NodeID: nodePS, Attempts: 5, Score: 0.85},
	} {
		_, err := fakeRepo.UpsertNodeMastery(ctx, m)
		require.NoError(t, err)
	}

	svc := service.New(service.Deps{
		Repo:       fakeRepo,
		Taxonomies: fakeTax,
	})
	handler, err := learninghttp.NewHandler(svc, &fakeGuardGateJ{})
	require.NoError(t, err)

	r := chi.NewRouter()
	handler.Routes(r)

	// 1. GET /me/foundation/path?target=PRESENT_PERFECT
	req := httptest.NewRequest(http.MethodGet, "/me/foundation/path?target=PRESENT_PERFECT", nil)
	actx := httpx.WithActor(req.Context(), httpx.Actor{UserID: userID, Role: "user"})
	req = req.WithContext(actx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var pathResp domain.LearnerFoundationPath
	err = json.NewDecoder(rec.Body).Decode(&pathResp)
	require.NoError(t, err)

	require.Len(t, pathResp.Items, 4)
	assert.True(t, pathResp.Items[2].Next)
	assert.Equal(t, testNodeCodePresentContinuous, pathResp.Items[2].Code)

	// 2. GET /me/foundation/path without target returns 422
	reqBad := httptest.NewRequest(http.MethodGet, "/me/foundation/path", nil)
	reqBad = reqBad.WithContext(actx)
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, reqBad)
	assert.Equal(t, http.StatusUnprocessableEntity, recBad.Code)

	// 3. GET /me/foundation/next
	reqNext := httptest.NewRequest(http.MethodGet, "/me/foundation/next", nil)
	reqNext = reqNext.WithContext(actx)
	recNext := httptest.NewRecorder()
	r.ServeHTTP(recNext, reqNext)

	require.Equal(t, http.StatusOK, recNext.Code, recNext.Body.String())

	var nextResp domain.FoundationPathNode
	err = json.NewDecoder(recNext.Body).Decode(&nextResp)
	require.NoError(t, err)
	assert.Equal(t, testNodeCodePresentContinuous, nextResp.Code)
}
