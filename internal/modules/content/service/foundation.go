package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

const (
	// KindFoundationTopic is the content kind holding a spine topic's body.
	KindFoundationTopic = "foundation_topic"
	// KindFoundationQuiz is a quiz item tagged to a spine node.
	KindFoundationQuiz = "foundation_quiz"
	// KindFoundationReview is a review question tagged to a spine node.
	KindFoundationReview = "foundation_review"

	defaultTopicPageSize = 20
	maxTopicPageSize     = 100
	maxTopicOffset       = 1 << 30
	maxPosition          = 1 << 30
)

// CreateFoundationTopicRequest carries inputs to create a new foundation taxonomy node.
type CreateFoundationTopicRequest struct {
	Namespace   string
	Code        string
	Label       string
	Description string
	CEFRLevel   *string
	ParentID    *uuid.UUID
	Position    int
}

// UpdateFoundationTopicRequest carries mutable fields for a foundation topic.
type UpdateFoundationTopicRequest struct {
	Label       *string
	Description *string
	CEFRLevel   *string
	SetCEFR     bool
	ParentID    *uuid.UUID
	SetParent   bool
	Position    *int
	Deprecated  *bool
}

// FoundationTopicFilter defines filtering options for browsing topics.
type FoundationTopicFilter struct {
	Namespace         *string
	CEFRLevel         *string
	ParentID          *uuid.UUID
	Query             *string
	IncludeDeprecated bool
	Limit             int
	Offset            int
}

// FoundationTopicDetail encapsulates a topic, its graph relations, content counts, and published body.
type FoundationTopicDetail struct {
	Topic         domain.Taxonomy
	Body          json.RawMessage
	Prerequisites []domain.Taxonomy
	Dependants    []domain.Taxonomy
	Related       []string
	ExerciseCount int
	QuizCount     int
	ReviewCount   int
}

// CreateFoundationTopic creates a new canonical spine taxonomy node (BR-FOUNDATION-01).
func (s *Service) CreateFoundationTopic(
	ctx context.Context,
	actorID uuid.UUID,
	req CreateFoundationTopicRequest,
) (domain.Taxonomy, error) {
	_ = actorID

	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Code = strings.TrimSpace(req.Code)
	req.Label = strings.TrimSpace(req.Label)

	if !domain.ValidateNamespace(req.Namespace) {
		return domain.Taxonomy{}, apperr.New(apperr.Validation, "INVALID_NAMESPACE", "Invalid taxonomy namespace.")
	}

	if !domain.ValidateTaxonomyCode(req.Namespace, req.Code) {
		return domain.Taxonomy{}, apperr.New(apperr.Validation, "INVALID_TAXONOMY_CODE", "Code must match namespace format.")
	}

	if req.Label == "" {
		return domain.Taxonomy{}, apperr.New(apperr.Validation, "INVALID_LABEL", "Label cannot be empty.")
	}

	if req.CEFRLevel != nil && *req.CEFRLevel != "" {
		if err := domain.ValidateCEFRLevel(*req.CEFRLevel); err != nil {
			return domain.Taxonomy{}, err
		}
	}

	if err := validatePosition(&req.Position); err != nil {
		return domain.Taxonomy{}, err
	}

	id := s.newID()
	return s.repo.CreateTaxonomy(
		ctx,
		id,
		req.Namespace,
		req.Code,
		req.Label,
		req.ParentID,
		req.Description,
		req.CEFRLevel,
		req.Position,
		nil,
	)
}

// UpdateFoundationTopic updates mutable properties of a topic node.
func (s *Service) UpdateFoundationTopic(
	ctx context.Context,
	actorID uuid.UUID,
	code string,
	req UpdateFoundationTopicRequest,
) (domain.Taxonomy, error) {
	_ = actorID

	existing, err := s.repo.GetTaxonomyByCode(ctx, code)
	if err != nil {
		return domain.Taxonomy{}, err
	}

	if req.CEFRLevel != nil && *req.CEFRLevel != "" {
		if err := domain.ValidateCEFRLevel(*req.CEFRLevel); err != nil {
			return domain.Taxonomy{}, err
		}
	}

	if err := validatePosition(req.Position); err != nil {
		return domain.Taxonomy{}, err
	}

	var depAt *time.Time
	setDep := false
	if req.Deprecated != nil {
		setDep = true
		if *req.Deprecated {
			now := s.clock.Now()
			depAt = &now
		}
	}

	return s.repo.UpdateTaxonomy(
		ctx,
		existing.ID,
		req.Label,
		req.Description,
		req.CEFRLevel,
		req.SetCEFR,
		req.ParentID,
		req.SetParent,
		req.Position,
		depAt,
		setDep,
	)
}

// ReplacePrerequisites replaces a node's prerequisite edges, refusing a cycle
// with TAXONOMY_CYCLE (BR-FOUNDATION-02) and a cross-namespace edge with
// CROSS_NAMESPACE_PREREQUISITE (BR-FOUNDATION-03).
func (s *Service) ReplacePrerequisites(
	ctx context.Context,
	actorID uuid.UUID,
	code string,
	requiresCodes []string,
) error {
	_ = actorID

	target, err := s.repo.GetTaxonomyByCode(ctx, code)
	if err != nil {
		return err
	}

	// Resolve prerequisite codes
	reqIDs := make([]uuid.UUID, 0, len(requiresCodes))
	for _, reqCode := range requiresCodes {
		reqNode, err := s.repo.GetTaxonomyByCode(ctx, reqCode)
		if err != nil {
			return err
		}
		// BR-FOUNDATION-03: Must be in the same namespace
		if reqNode.Namespace != target.Namespace {
			return apperr.New(apperr.Validation, "CROSS_NAMESPACE_PREREQUISITE",
				fmt.Sprintf("Prerequisite %s belongs to namespace %s, expected %s", reqCode, reqNode.Namespace, target.Namespace))
		}
		if reqNode.ID == target.ID {
			return apperr.New(apperr.Validation, "TAXONOMY_CYCLE", "A node cannot require itself.")
		}
		reqIDs = append(reqIDs, reqNode.ID)
	}

	// Read the graph, check it, and write it in one transaction.
	//
	// The check used to run against a snapshot taken before the transaction
	// opened. Two requests arriving together each saw an acyclic graph, each
	// decided its own edge was safe, and between them closed a cycle that
	// neither could see - and a cyclic graph is not something a topological
	// sort reports, it is something it silently truncates.
	return dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		existingEdges, err := txRepo.ListAllPrerequisiteEdgesInNamespace(ctx, target.Namespace)
		if err != nil {
			return err
		}

		if hasCycle, cycleIDs := domain.CheckProposedEdges(existingEdges, target.ID, reqIDs); hasCycle {
			return s.cycleError(ctx, txRepo, target.Namespace, cycleIDs)
		}

		return txRepo.ReplacePrerequisites(ctx, target.ID, reqIDs)
	})
}

// cycleError names the offending chain in codes rather than UUIDs, because the
// person reading the 422 is the one who has to break the cycle.
func (s *Service) cycleError(
	ctx context.Context, repo Repository, namespace string, cycleIDs []uuid.UUID,
) error {
	nodeCodeMap := make(map[uuid.UUID]string)
	if allNodes, err := repo.ListAllTaxonomiesInNamespace(ctx, namespace); err == nil {
		for _, n := range allNodes {
			nodeCodeMap[n.ID] = n.Code
		}
	}
	chain := make([]string, len(cycleIDs))
	for i, id := range cycleIDs {
		if c, ok := nodeCodeMap[id]; ok {
			chain[i] = c
		} else {
			chain[i] = id.String()
		}
	}
	return apperr.New(apperr.Validation, "TAXONOMY_CYCLE",
		fmt.Sprintf("The proposed prerequisites would create a cyclic dependency: %s", strings.Join(chain, " -> ")))
}

// ListFoundationTopics retrieves a paginated slice of foundation topics.
func (s *Service) ListFoundationTopics(
	ctx context.Context,
	filter FoundationTopicFilter,
) ([]domain.Taxonomy, int64, error) {
	// Clamped before the narrowing conversion, not after: filter.Limit is an int
	// off the wire, and int32(math.MaxInt32+1) is negative.
	limit := int32(defaultTopicPageSize)
	if filter.Limit > 0 && filter.Limit <= maxTopicPageSize {
		limit = int32(filter.Limit)
	}
	offset := int32(0)
	if filter.Offset > 0 && filter.Offset <= maxTopicOffset {
		offset = int32(filter.Offset)
	}

	return s.repo.ListTaxonomiesFiltered(
		ctx,
		filter.Namespace,
		filter.CEFRLevel,
		filter.ParentID,
		filter.Query,
		filter.IncludeDeprecated,
		limit,
		offset,
	)
}

// GetFoundationTopicByCode returns detailed topic data including graph edges, counts, and body.
func (s *Service) GetFoundationTopicByCode(ctx context.Context, code string) (FoundationTopicDetail, error) {
	topic, err := s.repo.GetTaxonomyByCode(ctx, code)
	if err != nil {
		return FoundationTopicDetail{}, err
	}

	prereqs, err := s.repo.ListPrerequisitesForNode(ctx, topic.ID)
	if err != nil {
		return FoundationTopicDetail{}, err
	}

	dependants, err := s.repo.ListDependantsForNode(ctx, topic.ID)
	if err != nil {
		return FoundationTopicDetail{}, err
	}

	counts, err := s.repo.CountTaggedContentByKindForTaxonomy(ctx, topic.ID)
	if err != nil {
		return FoundationTopicDetail{}, err
	}

	exerciseCount, quizCount, reviewCount := splitFoundationCounts(counts)

	// A missing body is ordinary - the node exists before anybody writes its
	// topic. A failure to read one is not, and returning the topic anyway would
	// render it as though nothing had been written yet.
	raw, found, err := s.repo.GetPublishedTopicBodyByTaxonomyID(ctx, topic.ID)
	if err != nil {
		return FoundationTopicDetail{}, err
	}
	var bodyBytes []byte
	var related []string
	if found {
		bodyBytes = raw
		var parsed domain.FoundationTopicBody
		if jsonErr := json.Unmarshal(raw, &parsed); jsonErr == nil {
			related = parsed.Related
		}
	}

	if related == nil {
		related = []string{}
	}

	return FoundationTopicDetail{
		Topic:         topic,
		Body:          bodyBytes,
		Prerequisites: prereqs,
		Dependants:    dependants,
		Related:       related,
		ExerciseCount: exerciseCount,
		QuizCount:     quizCount,
		ReviewCount:   reviewCount,
	}, nil
}

// GetFoundationPath returns a topologically sorted path of topics (BR-FOUNDATION-06, BR-FOUNDATION-07).
func (s *Service) GetFoundationPath(
	ctx context.Context,
	targetCode *string,
	namespace *string,
) ([]domain.Taxonomy, error) {
	if targetCode != nil && strings.TrimSpace(*targetCode) != "" {
		target, err := s.repo.GetTaxonomyByCode(ctx, strings.TrimSpace(*targetCode))
		if err != nil {
			return nil, err
		}

		allNodes, err := s.repo.ListAllTaxonomiesInNamespace(ctx, target.Namespace)
		if err != nil {
			return nil, err
		}
		nodeMap := make(map[uuid.UUID]domain.Taxonomy, len(allNodes))
		for _, n := range allNodes {
			nodeMap[n.ID] = n
		}

		edges, err := s.repo.ListAllPrerequisiteEdgesInNamespace(ctx, target.Namespace)
		if err != nil {
			return nil, err
		}

		return domain.PathTo(target, nodeMap, edges)
	}

	// Entire namespace topological sort
	ns := domain.NamespaceGrammar
	if namespace != nil && strings.TrimSpace(*namespace) != "" {
		ns = strings.TrimSpace(*namespace)
	}
	if !domain.ValidateNamespace(ns) {
		return nil, apperr.New(apperr.Validation, "INVALID_NAMESPACE", "Invalid taxonomy namespace.")
	}

	nodes, err := s.repo.ListAllTaxonomiesInNamespace(ctx, ns)
	if err != nil {
		return nil, err
	}

	edges, err := s.repo.ListAllPrerequisiteEdgesInNamespace(ctx, ns)
	if err != nil {
		return nil, err
	}

	return domain.TopologicalOrder(nodes, edges)
}

// splitFoundationCounts divides published content tagged to a node into the
// three things BR-FOUNDATION-05 requires a topic to have.
//
// Anything that is not the topic itself, a quiz item or a review question is an
// exercise: the exercise kinds are the runner's, they grow with it, and a list
// enumerated here would silently stop counting the next one added.
func splitFoundationCounts(counts map[string]int) (exercises, quiz, review int) {
	quiz = counts[KindFoundationQuiz]
	review = counts[KindFoundationReview]
	for kind, count := range counts {
		switch kind {
		case KindFoundationTopic, KindFoundationQuiz, KindFoundationReview:
		default:
			exercises += count
		}
	}
	return exercises, quiz, review
}

// hasCompleteFoundationSet reports whether a node carries at least one of each.
func hasCompleteFoundationSet(counts map[string]int) bool {
	exercises, quiz, review := splitFoundationCounts(counts)
	return exercises >= 1 && quiz >= 1 && review >= 1
}

// validatePosition keeps a position inside the column that stores it.
//
// position orders a strand for display, so it is small by nature; the bound is
// here so an absurd one is refused at the door rather than silently narrowed by
// the driver into a negative number that sorts to the front.
func validatePosition(position *int) error {
	if position == nil {
		return nil
	}
	if *position < 0 || *position > maxPosition {
		return apperr.New(apperr.Validation, "INVALID_POSITION",
			fmt.Sprintf("Position must be between 0 and %d.", maxPosition))
	}
	return nil
}
