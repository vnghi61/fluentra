package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

// countingTaxonomies resolves codes and prerequisites from maps, counting reads.
type countingTaxonomies struct {
	contentcontract.TaxonomyResolver
	byCode      map[string]*contentcontract.TaxonomyNode
	prereqs     map[uuid.UUID][]contentcontract.TaxonomyNode
	codeReads   int
	prereqReads int
}

func (c *countingTaxonomies) GetTaxonomyByCode(_ context.Context, code string) (*contentcontract.TaxonomyNode, error) {
	c.codeReads++
	return c.byCode[code], nil
}

func (c *countingTaxonomies) ListPrerequisites(
	_ context.Context, nodeID uuid.UUID,
) ([]contentcontract.TaxonomyNode, error) {
	c.prereqReads++
	return c.prereqs[nodeID], nil
}

// countingMastery answers ListNodeMasteryByUser and fails the test on a
// per-node read.
type countingMastery struct {
	Repository
	t        *testing.T
	rows     []domain.NodeMastery
	listings int
}

func (c *countingMastery) ListNodeMasteryByUser(context.Context, uuid.UUID) ([]domain.NodeMastery, error) {
	c.listings++
	return c.rows, nil
}

func (c *countingMastery) GetNodeMastery(context.Context, uuid.UUID, uuid.UUID) (*domain.NodeMastery, error) {
	c.t.Fatal("the gate must not read mastery one node at a time")
	return nil, nil
}

// TestPrereqGate_ReadsEachNodeOnceAcrossManyActivities pins the daily set's N+1:
// forty candidate activities over two tags cost two code reads, two
// prerequisite reads and one mastery read, not a read per activity.
func TestPrereqGate_ReadsEachNodeOnceAcrossManyActivities(t *testing.T) {
	user := uuid.New()
	open := contentcontract.TaxonomyNode{ID: uuid.New(), Code: "open"}
	locked := contentcontract.TaxonomyNode{ID: uuid.New(), Code: "locked"}
	mastered := contentcontract.TaxonomyNode{ID: uuid.New(), Code: "mastered"}
	unmastered := contentcontract.TaxonomyNode{ID: uuid.New(), Code: "unmastered"}
	tax := &countingTaxonomies{
		byCode: map[string]*contentcontract.TaxonomyNode{"open": &open, "locked": &locked},
		prereqs: map[uuid.UUID][]contentcontract.TaxonomyNode{
			open.ID:   {mastered},
			locked.ID: {unmastered},
		},
	}
	repo := &countingMastery{t: t, rows: []domain.NodeMastery{{
		UserID: user, NodeID: mastered.ID,
		Attempts: domain.MinAttemptsForWeakNode, Score: domain.MasteryScoreThreshold,
	}}}
	gate := (&Service{taxonomies: tax, repo: repo}).newPrereqGate(user)

	met, refused := 0, 0
	for i := range 40 {
		tags := []string{"grammar.open"}
		if i%2 == 1 {
			tags = []string{"grammar.locked"}
		}
		if gate.activityMet(context.Background(), tags) {
			met++
		} else {
			refused++
		}
	}

	assert.Equal(t, 20, met, "activities whose prerequisite is mastered pass")
	assert.Equal(t, 20, refused, "activities whose prerequisite is not mastered are held back")
	assert.Equal(t, 2, tax.codeReads, "one read per distinct tag code")
	assert.Equal(t, 2, tax.prereqReads, "one read per distinct node")
	assert.Equal(t, 1, repo.listings, "the learner's mastery is read once")
}
