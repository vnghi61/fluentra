package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// defaultStrands defines the order of spine taxonomy strands evaluated for next topic recommendation.
var defaultStrands = []string{
	"grammar",
	"vocabulary",
	"pattern",
	"pronunciation",
	"skill",
}

// GetLearnerFoundationPath returns the prerequisite path to targetCode with user's mastery,
// marking the first unmastered node as next.
func (s *Service) GetLearnerFoundationPath(
	ctx context.Context,
	userID uuid.UUID,
	targetCode string,
) (*domain.LearnerFoundationPath, error) {
	code := strings.TrimSpace(targetCode)
	if code == "" {
		return nil, apperr.New(apperr.Validation, "TARGET_REQUIRED", "target topic code is required")
	}

	if s.taxonomies == nil {
		return nil, apperr.New(apperr.Internal, "TAXONOMY_UNAVAILABLE", "taxonomy resolver not configured")
	}

	nodes, err := s.taxonomies.GetTaxonomyPath(ctx, &code, nil)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, apperr.New(apperr.NotFound, "TOPIC_NOT_FOUND", "target topic not found")
	}

	masteryMap, err := s.loadUserMasteryMap(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := s.annotatePathNodesWithMastery(nodes, masteryMap)
	namespace := nodes[len(nodes)-1].Namespace

	return &domain.LearnerFoundationPath{
		Target:    code,
		Namespace: namespace,
		Items:     items,
	}, nil
}

// GetLearnerFoundationNext returns the next recommended topic across strands without a target,
// preferring user goals and nodes matching their current placement level.
func (s *Service) GetLearnerFoundationNext(
	ctx context.Context,
	userID uuid.UUID,
) (*domain.FoundationPathNode, error) {
	if s.taxonomies == nil {
		return nil, apperr.New(apperr.Internal, "TAXONOMY_UNAVAILABLE", "taxonomy resolver not configured")
	}

	userLevel, _, err := s.pathLevel(ctx, userID)
	if err != nil {
		return nil, err
	}

	strands := s.determineStrandOrder(ctx, userID)
	masteryMap, err := s.loadUserMasteryMap(ctx, userID)
	if err != nil {
		return nil, err
	}

	candidate, err := s.findNextEligibleNode(ctx, strands, userLevel, masteryMap)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, apperr.New(apperr.NotFound, "NO_NEXT_TOPIC", "no eligible next topic found across strands")
	}

	return candidate, nil
}

// loadUserMasteryMap loads all node mastery records for a user into a map keyed by node ID.
func (s *Service) loadUserMasteryMap(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]*domain.NodeMastery, error) {
	masteries, err := s.repo.ListNodeMasteryByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	mMap := make(map[uuid.UUID]*domain.NodeMastery, len(masteries))
	for i := range masteries {
		mMap[masteries[i].NodeID] = &masteries[i]
	}
	return mMap, nil
}

// annotatePathNodesWithMastery maps taxonomy nodes into FoundationPathNodes with mastery and next flags.
func (s *Service) annotatePathNodesWithMastery(
	nodes []contentcontract.TaxonomyNode,
	masteryMap map[uuid.UUID]*domain.NodeMastery,
) []domain.FoundationPathNode {
	items := make([]domain.FoundationPathNode, len(nodes))
	nextMarked := false

	for i, n := range nodes {
		item := domain.FoundationPathNode{
			ID:        n.ID,
			Namespace: n.Namespace,
			Code:      n.Code,
			Label:     n.Label,
			CEFRLevel: n.CEFRLevel,
		}
		m := masteryMap[n.ID]
		if m != nil {
			item.Attempts = m.Attempts
			item.Score = m.Score
			item.Mastered = m.IsNodeMastered()
		}
		if !item.Mastered && !nextMarked {
			item.Next = true
			nextMarked = true
		}
		items[i] = item
	}

	return items
}

// determineStrandOrder returns the list of strands to evaluate, prioritizing user goals.
func (s *Service) determineStrandOrder(ctx context.Context, userID uuid.UUID) []string {
	preferred := ""
	if s.user != nil {
		if profile, found, err := s.user.GetLearningProfile(ctx, userID); err == nil && found {
			for _, mot := range profile.Motivations {
				lower := strings.ToLower(mot)
				for _, strand := range defaultStrands {
					if strings.Contains(lower, strand) {
						preferred = strand
						break
					}
				}
				if preferred != "" {
					break
				}
			}
		}
	}

	if preferred == "" {
		return defaultStrands
	}

	result := make([]string, 0, len(defaultStrands))
	result = append(result, preferred)
	for _, strand := range defaultStrands {
		if strand != preferred {
			result = append(result, strand)
		}
	}
	return result
}

// findNextEligibleNode iterates across strands to locate the next unmastered node whose prerequisites are met.
func (s *Service) findNextEligibleNode(
	ctx context.Context,
	strands []string,
	userLevel string,
	masteryMap map[uuid.UUID]*domain.NodeMastery,
) (*domain.FoundationPathNode, error) {
	var fallbackCandidate *domain.FoundationPathNode

	for _, strand := range strands {
		st := strand
		nodes, err := s.taxonomies.GetTaxonomyPath(ctx, nil, &st)
		if err != nil {
			continue
		}

		for _, n := range nodes {
			m := masteryMap[n.ID]
			if m != nil && m.IsNodeMastered() {
				continue
			}

			prereqsMet, err := s.areNodePrerequisitesMastered(ctx, n.ID, masteryMap)
			if err != nil || !prereqsMet {
				continue
			}

			candidate := &domain.FoundationPathNode{
				ID:        n.ID,
				Namespace: n.Namespace,
				Code:      n.Code,
				Label:     n.Label,
				CEFRLevel: n.CEFRLevel,
				Next:      true,
			}
			if m != nil {
				candidate.Attempts = m.Attempts
				candidate.Score = m.Score
				candidate.Mastered = m.IsNodeMastered()
			}

			// If the candidate matches user's current placement/declared level, prefer it immediately.
			if n.CEFRLevel != nil && *n.CEFRLevel == userLevel {
				return candidate, nil
			}

			if fallbackCandidate == nil {
				fallbackCandidate = candidate
			}
		}
	}

	return fallbackCandidate, nil
}

// areNodePrerequisitesMastered checks whether all prerequisite nodes have been mastered by the user.
func (s *Service) areNodePrerequisitesMastered(
	ctx context.Context,
	nodeID uuid.UUID,
	masteryMap map[uuid.UUID]*domain.NodeMastery,
) (bool, error) {
	prereqs, err := s.taxonomies.ListPrerequisites(ctx, nodeID)
	if err != nil {
		return false, err
	}
	for _, p := range prereqs {
		pm := masteryMap[p.ID]
		if pm == nil || !pm.IsNodeMastered() {
			return false, nil
		}
	}
	return true, nil
}
