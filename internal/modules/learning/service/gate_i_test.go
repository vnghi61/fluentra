package service_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
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

// TestGateI_LiveDatabaseVerification verifies §2 live database checks:
// 1. learn.node_mastery table existence and schema.
// 2. fluentra_app permissions (SELECT, INSERT, UPDATE, DELETE).
// 3. User erasure drops records for user_id.
func TestGateI_LiveDatabaseVerification(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		dsn = os.Getenv("DB_DSN")
	}
	if dsn == "" {
		t.Skip("Neither TEST_DATABASE_URL nor DB_DSN is set; skipping database verification")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("cannot connect to db: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("db ping failed: %v", err)
	}

	// 1. Check fluentra_app privileges on learn.node_mastery
	for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
		var hasPriv bool
		err := pool.QueryRow(ctx,
			"SELECT has_table_privilege('fluentra_app', 'learn.node_mastery', $1)", priv,
		).Scan(&hasPriv)
		require.NoError(t, err, "Check privilege %s on learn.node_mastery", priv)
		assert.True(t, hasPriv, "fluentra_app must have %s on learn.node_mastery", priv)
	}

	// 2. Check table columns
	var exists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'learn' AND table_name = 'node_mastery'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "learn.node_mastery must exist")

	// 3. Test CRUD and erasure via Repository
	repo := repository.New(pool)
	testUser := uuid.New()
	testNode := uuid.New()

	// Ensure core.users row exists for FK
	_, err = pool.Exec(ctx, `
		INSERT INTO core.users (id, email, status, created_at, updated_at)
		VALUES ($1, $2, 'active', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, testUser, "gate-i-"+testUser.String()[:8]+"@test.local")
	require.NoError(t, err)

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM learn.node_mastery WHERE user_id = $1", testUser)
		_, _ = pool.Exec(ctx, "DELETE FROM core.users WHERE id = $1", testUser)
	}()

	now := time.Now().UTC().Truncate(time.Microsecond)
	m := domain.NodeMastery{
		UserID:     testUser,
		NodeID:     testNode,
		Attempts:   3,
		Correct:    1,
		Score:      0.333,
		LastSeenAt: &now,
	}

	upserted, err := repo.UpsertNodeMastery(ctx, m)
	require.NoError(t, err)
	require.NotNil(t, upserted)
	assert.Equal(t, testUser, upserted.UserID)
	assert.Equal(t, testNode, upserted.NodeID)
	assert.Equal(t, 3, upserted.Attempts)
	assert.Equal(t, 1, upserted.Correct)
	assert.InDelta(t, 0.333, upserted.Score, 0.001)

	// Read back
	readBack, err := repo.GetNodeMastery(ctx, testUser, testNode)
	require.NoError(t, err)
	require.NotNil(t, readBack)
	assert.Equal(t, 3, readBack.Attempts)

	// List weak nodes
	weak, err := repo.ListWeakNodesByUser(ctx, testUser, domain.MinAttemptsForWeakNode)
	require.NoError(t, err)
	require.NotEmpty(t, weak)
	assert.Equal(t, testNode, weak[0].NodeID)

	// User erasure
	err = repo.DeleteNodeMasteryByUser(ctx, testUser)
	require.NoError(t, err)

	afterErasure, err := repo.GetNodeMastery(ctx, testUser, testNode)
	require.NoError(t, err)
	assert.Nil(t, afterErasure, "record must be deleted after user erasure")
}
