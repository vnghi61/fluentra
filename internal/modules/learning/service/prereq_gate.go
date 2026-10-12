package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

// prereqGate answers "has this learner met the prerequisites" for one request.
// It reads the learner's node mastery once, and each tag code and each node's
// prerequisites once. Asked per candidate activity, the direct reads cost a
// query per activity, per tag, per prerequisite: hundreds for one daily set.
type prereqGate struct {
	s       *Service
	userID  uuid.UUID
	mastery map[uuid.UUID]*domain.NodeMastery
	nodes   map[string]*contentcontract.TaxonomyNode
	met     map[uuid.UUID]bool
}

func (s *Service) newPrereqGate(userID uuid.UUID) *prereqGate {
	return &prereqGate{
		s:      s,
		userID: userID,
		nodes:  map[string]*contentcontract.TaxonomyNode{},
		met:    map[uuid.UUID]bool{},
	}
}

// nodeMet reports whether every prerequisite of nodeID is mastered. A node whose
// prerequisites cannot be read counts as met; a prerequisite whose mastery
// cannot be read counts as not met — as the per-query check behaved.
func (g *prereqGate) nodeMet(ctx context.Context, nodeID uuid.UUID) bool {
	if g.s.taxonomies == nil || g.s.repo == nil {
		return true
	}
	if met, ok := g.met[nodeID]; ok {
		return met
	}
	met := true
	prereqs, err := g.s.taxonomies.ListPrerequisites(ctx, nodeID)
	if err == nil && len(prereqs) > 0 {
		mastery := g.loadMastery(ctx)
		for _, p := range prereqs {
			if !mastery[p.ID].IsNodePrerequisiteMet() {
				met = false
				break
			}
		}
	}
	g.met[nodeID] = met
	return met
}

// activityMet reports whether every node an activity is tagged with has its
// prerequisites met. A tag that names no node is ignored.
func (g *prereqGate) activityMet(ctx context.Context, tags []string) bool {
	for _, tag := range tags {
		code := tag
		if idx := strings.LastIndex(tag, "."); idx >= 0 {
			code = tag[idx+1:]
		}
		node, seen := g.nodes[code]
		if !seen {
			node, _ = g.s.taxonomies.GetTaxonomyByCode(ctx, code)
			g.nodes[code] = node
		}
		if node != nil && !g.nodeMet(ctx, node.ID) {
			return false
		}
	}
	return true
}

func (g *prereqGate) loadMastery(ctx context.Context) map[uuid.UUID]*domain.NodeMastery {
	if g.mastery != nil {
		return g.mastery
	}
	g.mastery = map[uuid.UUID]*domain.NodeMastery{}
	rows, err := g.s.repo.ListNodeMasteryByUser(ctx, g.userID)
	if err != nil {
		return g.mastery
	}
	for i := range rows {
		g.mastery[rows[i].NodeID] = &rows[i]
	}
	return g.mastery
}
