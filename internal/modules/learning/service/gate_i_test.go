package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// gateTaxonomyResolver implements contentcontract.TaxonomyResolver for Gate I tests.
type gateTaxonomyResolver struct {
	nodes   map[string]*contentcontract.TaxonomyNode
	byID    map[uuid.UUID]*contentcontract.TaxonomyNode
	prereqs map[uuid.UUID][]contentcontract.TaxonomyNode
}

func newGateTaxonomyResolver() *gateTaxonomyResolver {
	return &gateTaxonomyResolver{
		nodes:   make(map[string]*contentcontract.TaxonomyNode),
		byID:    make(map[uuid.UUID]*contentcontract.TaxonomyNode),
		prereqs: make(map[uuid.UUID][]contentcontract.TaxonomyNode),
	}
}

func (r *gateTaxonomyResolver) addNode(code string, prereqCodes ...string) *contentcontract.TaxonomyNode {
	id := uuid.New()
	node := &contentcontract.TaxonomyNode{
		ID:        id,
		Namespace: testSkillGrammar,
		Code:      code,
		Label:     code,
	}
	r.nodes[code] = node
	r.byID[id] = node

	for _, pCode := range prereqCodes {
		pNode, ok := r.nodes[pCode]
		if ok {
			r.prereqs[id] = append(r.prereqs[id], *pNode)
		}
	}
	return node
}

func (r *gateTaxonomyResolver) ResolveTaxonomyID(_ context.Context, namespace, code string) (*uuid.UUID, error) {
	if n, ok := r.nodes[code]; ok && n.Namespace == namespace {
		return &n.ID, nil
	}
	return nil, nil
}

func (r *gateTaxonomyResolver) GetTaxonomyByCode(
	_ context.Context, code string,
) (*contentcontract.TaxonomyNode, error) {
	if n, ok := r.nodes[code]; ok {
		return n, nil
	}
	return nil, nil
}

func (r *gateTaxonomyResolver) GetTaxonomyByID(_ context.Context, id uuid.UUID) (*contentcontract.TaxonomyNode, error) {
	if n, ok := r.byID[id]; ok {
		return n, nil
	}
	return nil, nil
}

func (r *gateTaxonomyResolver) ListTaxonomiesInNamespace(
	_ context.Context, namespace string,
) ([]contentcontract.TaxonomyNode, error) {
	var out []contentcontract.TaxonomyNode
	for _, n := range r.nodes {
		if n.Namespace == namespace {
			out = append(out, *n)
		}
	}
	return out, nil
}

func (r *gateTaxonomyResolver) ListPrerequisites(
	_ context.Context, nodeID uuid.UUID,
) ([]contentcontract.TaxonomyNode, error) {
	return r.prereqs[nodeID], nil
}

func (r *gateTaxonomyResolver) GetTaxonomyPath(
	_ context.Context, _ *string, _ *string,
) ([]contentcontract.TaxonomyNode, error) {
	var out []contentcontract.TaxonomyNode
	for _, n := range r.nodes {
		out = append(out, *n)
	}
	return out, nil
}

// TestGateI_TrapI1_MinimumAttempts verifies Trap I.1:
// A node with two attempts is not "weak", it is unknown. Minimum 3 attempts is required.
func TestGateI_TrapI1_MinimumAttempts(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	nodeID := uuid.New()

	// 2 attempts with 0 score
	m2 := domain.NodeMastery{
		UserID:   userID,
		NodeID:   nodeID,
		Attempts: 2,
		Correct:  0,
		Score:    0.0,
	}
	assert.False(t, m2.Attempts >= domain.MinAttemptsForWeakNode, "2 attempts must not qualify as weak node")

	// 3 attempts with 0 score
	m3 := domain.NodeMastery{
		UserID:   userID,
		NodeID:   nodeID,
		Attempts: 3,
		Correct:  0,
		Score:    0.0,
	}
	assert.True(t, m3.Attempts >= domain.MinAttemptsForWeakNode, "3 attempts qualifies as weak node")
	assert.True(t, m3.Score < domain.MasteryScoreThreshold, "score < 0.8 is weak")
}

// TestGateI_PrerequisiteGating_DailySet verifies the Gate I requirement:
// A learner who answers PRESENT_PERFECT items wrongly gets a weak-node slot on the next daily
// set drawing PRESENT_PERFECT practice, and one who has not met PAST_SIMPLE does not get
// PRESENT_PERFECT at all.
func TestGateI_PrerequisiteGating_DailySet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Setup taxonomy: PAST_SIMPLE is a prerequisite for PRESENT_PERFECT
	taxonomies := newGateTaxonomyResolver()
	pastSimple := taxonomies.addNode("PAST_SIMPLE")
	presentPerfect := taxonomies.addNode("PRESENT_PERFECT", "PAST_SIMPLE")

	clk := clock.NewFake(time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC))
	f := poolFixture{
		repo:    newFakePoolRepo(),
		lessons: newFakePoolLessons(),
		content: newFakeContentReader(),
		authors: &fakeContentAuthor{},
	}
	f.svc = service.New(service.Deps{
		Repo:          f.repo,
		Lesson:        f.lessons,
		LessonAuthor:  f.lessons,
		Content:       f.content,
		ContentAuthor: f.authors,
		Clock:         clk,
		Taxonomies:    taxonomies,
	})
	require.NoError(t, f.svc.EnsurePracticePoolStructure(ctx))

	// Seed all slots to target
	f.seed(t, "B1", atTarget())

	// Add a specific PRESENT_PERFECT activity to the grammar tense choice slot
	presPerfVerID := uuid.New()
	presPerfBody := json.RawMessage(`{"prompt": "Present Perfect practice test"}`)
	f.content.versions[presPerfVerID] = &contentcontract.Version{
		ID:        presPerfVerID,
		Kind:      poolKindTense,
		CEFRLevel: "B1",
		Tags:      []string{"grammar.PRESENT_PERFECT"},
		Body:      presPerfBody,
	}
	presPerfActID := f.lessons.seed(t, "B1", poolTitleTense, poolKindTense, presPerfVerID, presPerfBody)

	// CASE 1: Learner Alice has answered PRESENT_PERFECT wrongly (3 attempts, score 0.2),
	// but has NOT met PAST_SIMPLE (0 attempts).
	aliceID := uuid.New()
	_, err := f.repo.UpsertNodeMastery(ctx, domain.NodeMastery{
		UserID:   aliceID,
		NodeID:   presentPerfect.ID,
		Attempts: 3,
		Correct:  0,
		Score:    0.2,
	})
	require.NoError(t, err)

	// Alice draws daily practice set
	aliceSet, err := f.svc.GetDailySet(ctx, aliceID, "B1")
	require.NoError(t, err)
	require.NotNil(t, aliceSet)

	// Verify Alice does NOT get PRESENT_PERFECT at all
	for _, act := range aliceSet.Activities {
		assert.NotEqual(t, presPerfActID, act.ID,
			"Learner Alice who has not met PAST_SIMPLE must NOT get PRESENT_PERFECT at all")
	}

	// CASE 2: Learner Bob has met PAST_SIMPLE (5 attempts, score 0.95),
	// and has answered PRESENT_PERFECT wrongly (4 attempts, score 0.2).
	bobID := uuid.New()
	_, err = f.repo.UpsertNodeMastery(ctx, domain.NodeMastery{
		UserID:   bobID,
		NodeID:   pastSimple.ID,
		Attempts: 5,
		Correct:  5,
		Score:    0.95,
	})
	require.NoError(t, err)

	_, err = f.repo.UpsertNodeMastery(ctx, domain.NodeMastery{
		UserID:   bobID,
		NodeID:   presentPerfect.ID,
		Attempts: 4,
		Correct:  0,
		Score:    0.2,
	})
	require.NoError(t, err)

	// Bob draws daily practice set
	bobSet, err := f.svc.GetDailySet(ctx, bobID, "B1")
	require.NoError(t, err)
	require.NotNil(t, bobSet)

	// Verify Bob DOES get a weak-node slot drawing PRESENT_PERFECT practice!
	var foundPresPerf bool
	for _, act := range bobSet.Activities {
		if act.ID == presPerfActID {
			foundPresPerf = true
			break
		}
	}
	assert.True(t, foundPresPerf,
		"Learner Bob who met PAST_SIMPLE and is weak on PRESENT_PERFECT must get PRESENT_PERFECT in daily set")
}
