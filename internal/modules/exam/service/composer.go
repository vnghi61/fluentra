// Package service implements exam sitting management, composition, and coverage reporting.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/fluentra/fluentra/internal/generated/exam/sqlc"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	examjob "github.com/fluentra/fluentra/internal/modules/exam/job"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

const (
	skillListening    = "listening"
	skillReading      = "reading"
	skillWriting      = "writing"
	skillSpeaking     = "speaking"
	skillUseOfEnglish = "use_of_english"
)

// BlueprintSummaryDTO describes a blueprint for client listing.
type BlueprintSummaryDTO struct {
	ID               uuid.UUID       `json:"id"`
	Name             string          `json:"name"`
	CefrDistribution json.RawMessage `json:"cefr_distribution"`
	NodeDistribution json.RawMessage `json:"node_distribution"`
}

// ExamVersionDTO describes a verified exam version and its blueprints.
type ExamVersionDTO struct {
	ID           uuid.UUID             `json:"id"`
	ExamFamily   string                `json:"exam_family"`
	Code         string                `json:"code"`
	Title        string                `json:"title"`
	TotalMinutes int                   `json:"total_minutes"`
	Scoring      json.RawMessage       `json:"scoring"`
	SourceURL    string                `json:"source_url"`
	VerifiedAt   string                `json:"verified_at"`
	IsCurrent    bool                  `json:"is_current"`
	Notes        string                `json:"notes"`
	Blueprints   []BlueprintSummaryDTO `json:"blueprints"`
	// DistinctTestsPossible is the coverage report's number: how many distinct
	// tests the published bank can compose. Learners see it so the product
	// never implies more tests than the bank holds (WO 19 H.3).
	DistinctTestsPossible int `json:"distinct_tests_possible"`
	// Parts is the version's structure: what a "custom" composition chooses
	// between. Counts of available bank items are not here; those stay in the
	// admin coverage report.
	Parts []ExamVersionPartDTO `json:"parts"`
}

// ExamVersionPartDTO is one part of an exam version's structure.
type ExamVersionPartDTO struct {
	PartNumber    int    `json:"part_number"`
	Section       string `json:"section"`
	Kind          string `json:"kind"`
	QuestionCount int    `json:"question_count"`
	GroupSize     int    `json:"group_size"`
}

// ExamVersionListResponseDTO wraps the list of exam versions.
type ExamVersionListResponseDTO struct {
	Items []ExamVersionDTO `json:"items"`
}

// ComposeMockTestRequest holds the parameters to compose a mock test.
type ComposeMockTestRequest struct {
	BlueprintID uuid.UUID `json:"blueprint_id"`
	Mode        string    `json:"mode"`
	Parts       []int     `json:"parts,omitempty"`
}

// ExamPartCoverageDTO describes question bank coverage for a single exam part.
type ExamPartCoverageDTO struct {
	PartID                   uuid.UUID `json:"part_id"`
	PartNumber               int       `json:"part_number"`
	Section                  string    `json:"section"`
	Kind                     string    `json:"kind"`
	QuestionCount            int       `json:"question_count"`
	GroupSize                int       `json:"group_size"`
	PublishedGroupsAvailable int       `json:"published_groups_available"`
	GroupsNeededPerTest      int       `json:"groups_needed_per_test"`
	TestsPossible            int       `json:"tests_possible"`
}

// ExamCoverageReportDTO describes overall question bank coverage for an exam version.
type ExamCoverageReportDTO struct {
	VersionID             uuid.UUID             `json:"version_id"`
	ExamCode              string                `json:"exam_code"`
	Title                 string                `json:"title"`
	DistinctTestsPossible int                   `json:"distinct_tests_possible"`
	BottleneckPartID      *uuid.UUID            `json:"bottleneck_part_id"`
	Parts                 []ExamPartCoverageDTO `json:"parts"`
}

// ListCurrentExamVersions returns all current verified exam versions with their blueprints.
func (s *Service) ListCurrentExamVersions(ctx context.Context) ([]ExamVersionDTO, error) {
	if s.repo == nil {
		return nil, errors.New("repository not configured")
	}

	versions, err := s.repo.ListCurrentExamVersions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list current exam versions: %w", err)
	}

	out := make([]ExamVersionDTO, 0, len(versions))
	for _, v := range versions {
		bps, err := s.repo.ListBlueprintsByVersionID(ctx, v.ID)
		if err != nil {
			return nil, fmt.Errorf("list blueprints for version %s: %w", v.ID, err)
		}
		bpDTOs := make([]BlueprintSummaryDTO, len(bps))
		for i, bp := range bps {
			bpDTOs[i] = BlueprintSummaryDTO{
				ID:               bp.ID,
				Name:             bp.Name,
				CefrDistribution: bp.CefrDistribution,
				NodeDistribution: bp.NodeDistribution,
			}
		}

		parts, err := s.repo.ListExamPartsByVersionID(ctx, v.ID)
		if err != nil {
			return nil, fmt.Errorf("list parts for version %s: %w", v.ID, err)
		}
		sortParts(parts)
		partDTOs := make([]ExamVersionPartDTO, 0, len(parts))
		for _, p := range parts {
			partDTOs = append(partDTOs, ExamVersionPartDTO{
				PartNumber:    p.PartNumber,
				Section:       p.Section,
				Kind:          p.Kind,
				QuestionCount: p.QuestionCount,
				GroupSize:     p.GroupSize,
			})
		}

		out = append(out, ExamVersionDTO{
			ID:           v.ID,
			ExamFamily:   v.ExamFamily,
			Code:         v.Code,
			Title:        v.Title,
			TotalMinutes: v.TotalMinutes,
			Scoring:      v.Scoring,
			SourceURL:    v.SourceURL,
			VerifiedAt:   v.VerifiedAt.Format("2006-01-02"),
			IsCurrent:    v.IsCurrent,
			Notes:        v.Notes,
			Blueprints:   bpDTOs,
			Parts:        partDTOs,
			// The same number the admin coverage report shows, so the learner's
			// test list and the operator's report cannot disagree.
			DistinctTestsPossible: s.coverageForParts(ctx, v, parts).DistinctTestsPossible,
		})
	}
	return out, nil
}

// GetExamVersionCoverage calculates published item coverage and distinct tests possible.
func (s *Service) GetExamVersionCoverage(ctx context.Context, versionID uuid.UUID) (*ExamCoverageReportDTO, error) {
	if s.repo == nil {
		return nil, errors.New("repository not configured")
	}

	version, err := s.repo.GetExamVersionByID(ctx, versionID)
	if err != nil || version == nil {
		return nil, apperr.New(apperr.NotFound, "EXAM_VERSION_NOT_FOUND", "exam version not found")
	}
	return s.coverageForVersion(ctx, version), nil
}

// coverageForVersion is the report for a version already in hand.
func (s *Service) coverageForVersion(ctx context.Context, version *domain.ExamVersion) *ExamCoverageReportDTO {
	parts, err := s.repo.ListExamPartsByVersionID(ctx, version.ID)
	if err != nil {
		// A version whose parts cannot be read has no coverage; the list still
		// renders, and the report route surfaces the error itself.
		return &ExamCoverageReportDTO{
			VersionID: version.ID,
			ExamCode:  version.Code,
			Title:     version.Title,
			Parts:     []ExamPartCoverageDTO{},
		}
	}
	return s.coverageForParts(ctx, version, parts)
}

// coverageForParts computes the report from parts already loaded. The exam
// version list calls it per version, so a learner's list and the operator's
// report are one computation rather than two that can disagree.
func (s *Service) coverageForParts(
	ctx context.Context, version *domain.ExamVersion, parts []*domain.ExamPart,
) *ExamCoverageReportDTO {
	sortParts(parts)

	partCoverages := make([]ExamPartCoverageDTO, 0, len(parts))
	minTestsPossible := -1
	var bottleneckID *uuid.UUID

	for _, p := range parts {
		cov := s.computePartCoverage(ctx, p)
		if minTestsPossible == -1 || cov.TestsPossible < minTestsPossible {
			minTestsPossible = cov.TestsPossible
			partID := p.ID
			bottleneckID = &partID
		}
		partCoverages = append(partCoverages, cov)
	}

	distinctTests := 0
	if minTestsPossible > 0 {
		distinctTests = minTestsPossible
	}

	return &ExamCoverageReportDTO{
		VersionID:             version.ID,
		ExamCode:              version.Code,
		Title:                 version.Title,
		DistinctTestsPossible: distinctTests,
		BottleneckPartID:      bottleneckID,
		Parts:                 partCoverages,
	}
}

func (s *Service) computePartCoverage(ctx context.Context, p *domain.ExamPart) ExamPartCoverageDTO {
	groupsNeeded := groupsPerTest(p)

	testsPossible := 0
	publishedAvailable := 0
	if s.questionbank != nil {
		candidates, err := s.questionbank.DrawableForPart(ctx, p.ID)
		if err == nil {
			usable := fitPart(p, candidates)
			publishedAvailable = len(usable)
			// A test needs part.QuestionCount questions; count what the usable
			// groups hold, so a variable-size part (TOEIC Part 7) is measured in
			// questions and a fixed-size one comes out as groups / groups needed.
			held := 0
			for _, q := range usable {
				held += groupSize(q)
			}
			if p.QuestionCount > 0 {
				testsPossible = held / p.QuestionCount
			}
		}
	}

	return ExamPartCoverageDTO{
		PartID:                   p.ID,
		PartNumber:               p.PartNumber,
		Section:                  p.Section,
		Kind:                     p.Kind,
		QuestionCount:            p.QuestionCount,
		GroupSize:                p.GroupSize,
		PublishedGroupsAvailable: publishedAvailable,
		GroupsNeededPerTest:      groupsNeeded,
		TestsPossible:            testsPossible,
	}
}

// groupsPerTest is how many groups a fixed-size part draws. A part seeded with
// group_size 1 whose bank items hold several questions each (TOEIC Part 7's
// passages) has no fixed number; it is reported as its question count.
func groupsPerTest(p *domain.ExamPart) int {
	if p.GroupSize <= 0 {
		return max(p.QuestionCount, 1)
	}
	return max(p.QuestionCount/p.GroupSize, 1)
}

// groupSize is how many questions one bank item holds.
func groupSize(q *questionbankcontract.Question) int {
	return max(q.QuestionCount, 1)
}

// fitPart keeps the drawable items whose shape fits the part. A part with a
// fixed group size (VSTEP Listening Part 2: conversations of 4) takes only items
// of exactly that size, or a test would cite questions it never plays. A part
// seeded with group size 1 takes items of any size and is filled by question
// count instead (G.3: question_count is questions; drawing is by activity).
func fitPart(p *domain.ExamPart, candidates []*questionbankcontract.Question) []*questionbankcontract.Question {
	out := make([]*questionbankcontract.Question, 0, len(candidates))
	for _, q := range candidates {
		if q.ActivityID == nil || *q.ActivityID == uuid.Nil {
			continue
		}
		if p.GroupSize > 1 && groupSize(q) != p.GroupSize {
			continue
		}
		out = append(out, q)
	}
	return out
}

// filterRequestedParts extracts requested parts or returns all parts if none requested.
func filterRequestedParts(allParts []*domain.ExamPart, requested []int) ([]*domain.ExamPart, error) {
	if len(requested) == 0 {
		return allParts, nil
	}
	wantedParts := make(map[int]bool, len(requested))
	for _, pn := range requested {
		wantedParts[pn] = true
	}
	var parts []*domain.ExamPart
	for _, p := range allParts {
		if wantedParts[p.PartNumber] {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return nil, apperr.New(apperr.BadRequest, "INVALID_PARTS_SELECTION", "none of the requested parts exist")
	}
	return parts, nil
}

// resolveMockTestParts loads and filters parts for a blueprint.
func (s *Service) resolveMockTestParts(
	ctx context.Context, blueprintID uuid.UUID, requestedParts []int,
) (*domain.Blueprint, []*domain.ExamPart, error) {
	blueprint, err := s.repo.GetBlueprintByID(ctx, blueprintID)
	if err != nil || blueprint == nil {
		return nil, nil, apperr.New(apperr.NotFound, "BLUEPRINT_NOT_FOUND", "blueprint not found")
	}

	allParts, err := s.repo.ListExamPartsByVersionID(ctx, blueprint.VersionID)
	if err != nil {
		return nil, nil, fmt.Errorf("list exam parts: %w", err)
	}
	if len(allParts) == 0 {
		return nil, nil, apperr.New(apperr.Conflict, "NO_EXAM_PARTS", "exam version has no parts defined")
	}

	parts, err := filterRequestedParts(allParts, requestedParts)
	if err != nil {
		return nil, nil, err
	}

	sortParts(parts)
	return blueprint, parts, nil
}

// drawPartActivities selects the activities for one exam part: exactly
// part.QuestionCount questions, in whole groups, preferring what this learner
// has not seen and following the blueprint's CEFR mix where the bank allows.
// A part the bank cannot fill is refused with the part named (H.5 trap 1).
func (s *Service) drawPartActivities(
	ctx context.Context, part *domain.ExamPart, cefrMix map[string]float64, seed int64, userID *uuid.UUID,
) ([]uuid.UUID, error) {
	var candidates []*questionbankcontract.Question
	if s.questionbank != nil {
		drawable, err := s.questionbank.DrawableForPart(ctx, part.ID)
		if err != nil {
			return nil, fmt.Errorf("fetch drawable questions for part %s: %w", part.ID, err)
		}
		candidates = fitPart(part, drawable)
	}

	ordered := s.orderByExposure(ctx, candidates, seed, int64(part.PartNumber), userID)
	picked, filled := pickByCEFR(ordered, part.QuestionCount, cefrMix)
	if filled != part.QuestionCount {
		held := 0
		for _, q := range candidates {
			held += groupSize(q)
		}
		needed, available := part.QuestionCount, held
		if part.GroupSize > 1 {
			needed, available = groupsPerTest(part), len(candidates)
		}
		return nil, apperr.New(
			apperr.Conflict,
			"INSUFFICIENT_ITEMS",
			fmt.Sprintf("insufficient published questions to compose part %s-%d (%s): needed %d, available %d",
				part.Section, part.PartNumber, part.ID, needed, available),
		)
	}

	ids := make([]uuid.UUID, len(picked))
	for i, q := range picked {
		ids[i] = *q.ActivityID
	}
	return ids, nil
}

// orderByExposure returns the candidates unseen-first, each half shuffled by the
// seed. The input arrives ordered by id, so the same seed gives the same order.
func (s *Service) orderByExposure(
	ctx context.Context, candidates []*questionbankcontract.Question, seed, partNum int64, userID *uuid.UUID,
) []*questionbankcontract.Question {
	var unseen, seen []*questionbankcontract.Question
	if userID != nil && s.exposures != nil {
		actIDs := make([]uuid.UUID, len(candidates))
		for i, q := range candidates {
			actIDs[i] = *q.ActivityID
		}
		exposures, err := s.exposures.ListItemExposures(ctx, *userID, actIDs)
		if err != nil {
			slog.WarnContext(ctx, "could not query item exposures", "user_id", userID, "error", err)
		}
		for _, q := range candidates {
			if _, served := exposures[*q.ActivityID]; served {
				seen = append(seen, q)
			} else {
				unseen = append(unseen, q)
			}
		}
	} else {
		unseen = append(unseen, candidates...)
	}

	//nolint:gosec // deterministic shuffle for pseudo-random mock test composition
	rng := rand.New(rand.NewSource(seed + partNum*1000003))
	rng.Shuffle(len(unseen), func(i, j int) { unseen[i], unseen[j] = unseen[j], unseen[i] })
	rng.Shuffle(len(seen), func(i, j int) { seen[i], seen[j] = seen[j], seen[i] })
	return append(unseen, seen...)
}

// pickByCEFR fills exactly target questions from ordered. Items whose level
// still has room in the blueprint's mix go first, then the rest, each in the
// order given (unseen first). The mix is a preference, not a refusal: a bank
// short of C1 still yields a test. It returns what it picked and how many
// questions that holds; less than target means the bank cannot fill the part.
func pickByCEFR(
	ordered []*questionbankcontract.Question, target int, cefrMix map[string]float64,
) ([]*questionbankcontract.Question, int) {
	quota := cefrQuotas(target, cefrMix)
	preferred := make([]*questionbankcontract.Question, 0, len(ordered))
	var rest []*questionbankcontract.Question
	for _, q := range ordered {
		if size := groupSize(q); quota[q.CEFRLevel] >= size {
			quota[q.CEFRLevel] -= size
			preferred = append(preferred, q)
			continue
		}
		rest = append(rest, q)
	}
	return fillExactly(append(preferred, rest...), target)
}

// fillExactly walks items in order and takes each one that still leaves the
// remainder reachable with the items after it. Plain greedy stops at 53 of 54
// when every passage left holds two or more questions; this finds 54 whenever
// the items can make it, and still prefers earlier items.
func fillExactly(items []*questionbankcontract.Question, target int) ([]*questionbankcontract.Question, int) {
	if target <= 0 {
		return nil, 0
	}
	// reach[i][r]: can items[i:] sum to exactly r.
	reach := make([][]bool, len(items)+1)
	reach[len(items)] = make([]bool, target+1)
	reach[len(items)][0] = true
	for i := len(items) - 1; i >= 0; i-- {
		size := groupSize(items[i])
		reach[i] = make([]bool, target+1)
		for r := 0; r <= target; r++ {
			reach[i][r] = reach[i+1][r] || (r >= size && reach[i+1][r-size])
		}
	}

	var picked []*questionbankcontract.Question
	remaining := target
	for i, q := range items {
		if remaining == 0 {
			break
		}
		size := groupSize(q)
		if size <= remaining && reach[i+1][remaining-size] {
			picked = append(picked, q)
			remaining -= size
		}
	}
	if remaining != 0 {
		// Unreachable: report the most a greedy pass could hold, for the message.
		picked, remaining = nil, target
		for _, q := range items {
			if size := groupSize(q); size <= remaining {
				picked = append(picked, q)
				remaining -= size
			}
		}
	}
	return picked, target - remaining
}

// cefrQuotas splits target questions across levels by the blueprint's weights,
// largest remainder first, so the quotas always sum to target.
func cefrQuotas(target int, mix map[string]float64) map[string]int {
	quota := make(map[string]int, len(mix))
	total := 0.0
	for _, w := range mix {
		if w > 0 {
			total += w
		}
	}
	if total == 0 || target <= 0 {
		return quota
	}
	type rem struct {
		level string
		frac  float64
	}
	var rems []rem
	assigned := 0
	for level, w := range mix {
		if w <= 0 {
			continue
		}
		exact := float64(target) * w / total
		whole := int(exact)
		quota[level] = whole
		assigned += whole
		rems = append(rems, rem{level, exact - float64(whole)})
	}
	sort.Slice(rems, func(i, j int) bool {
		if rems[i].frac != rems[j].frac {
			return rems[i].frac > rems[j].frac
		}
		return rems[i].level < rems[j].level
	})
	for i := 0; assigned < target && len(rems) > 0; i = (i + 1) % len(rems) {
		quota[rems[i].level]++
		assigned++
	}
	return quota
}

func parseCEFRMix(raw json.RawMessage) map[string]float64 {
	mix := map[string]float64{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &mix)
	}
	return mix
}

// ComposeMockTest composes a new mock test from the question bank.
func (s *Service) ComposeMockTest(
	ctx context.Context, userID *uuid.UUID, req ComposeMockTestRequest,
) (*domain.MockTest, error) {
	if s.repo == nil {
		return nil, errors.New("repository not configured")
	}

	if req.BlueprintID == uuid.Nil {
		return nil, apperr.New(apperr.Validation, "INVALID_BLUEPRINT_ID", "blueprint_id is required")
	}

	mode := req.Mode
	if mode == "" {
		mode = domain.MockModeFull
	}

	switch mode {
	case domain.MockModeFixed, domain.MockModeRandom, domain.MockModeWeakTopic, domain.MockModeFull, domain.MockModeCustom:
	default:
		return nil, domain.ErrInvalidMockMode
	}

	if mode == domain.MockModeFixed && len(req.Parts) > 0 {
		return nil, apperr.New(apperr.Validation, "FIXED_TEST_HAS_NO_PART_SELECTION",
			"A fixed test is the whole blueprint; choose custom to pick parts.")
	}

	blueprint, parts, err := s.resolveMockTestParts(ctx, req.BlueprintID, req.Parts)
	if err != nil {
		return nil, err
	}

	// A fixed test is one stored composition everyone shares (H.2). It is
	// composed once, without anyone's exposures — otherwise the "same" test would
	// differ by who asked first — and every later request returns that row.
	var drawFor *uuid.UUID
	var seed int64
	if mode == domain.MockModeFixed {
		existing, findErr := s.findFixedMockTest(ctx, blueprint.ID)
		if findErr != nil {
			return nil, findErr
		}
		if existing != nil {
			return existing, nil
		}
		seed = hashUUID(blueprint.ID)
	} else {
		drawFor = userID
		seed = time.Now().UnixNano()
	}

	cefrMix := parseCEFRMix(blueprint.CefrDistribution)
	compositions := make([]domain.MockTestPartComposition, 0, len(parts))
	for _, part := range parts {
		picked, drawErr := s.drawPartActivities(ctx, part, cefrMix, seed, drawFor)
		if drawErr != nil {
			return nil, drawErr
		}
		compositions = append(compositions, domain.MockTestPartComposition{
			PartID:      part.ID,
			ActivityIDs: picked,
		})
	}

	var ownerID *uuid.UUID
	if mode != domain.MockModeFixed {
		ownerID = userID
	}

	mt := &domain.MockTest{
		ID:          uuid.New(),
		BlueprintID: blueprint.ID,
		Mode:        mode,
		Seed:        seed,
		Composition: compositions,
		OwnerID:     ownerID,
		CreatedAt:   s.clock.Now().UTC(),
	}

	saved, err := s.repo.CreateMockTest(ctx, mt)
	if err != nil {
		return nil, fmt.Errorf("create mock test: %w", err)
	}
	return saved, nil
}

// findFixedMockTest returns the earliest fixed test composed for a blueprint.
func (s *Service) findFixedMockTest(ctx context.Context, blueprintID uuid.UUID) (*domain.MockTest, error) {
	public, err := s.repo.ListMockTestsByOwner(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list public mock tests: %w", err)
	}
	var found *domain.MockTest
	for _, mt := range public {
		if mt.OwnerID != nil || mt.BlueprintID != blueprintID || mt.Mode != domain.MockModeFixed {
			continue
		}
		if found == nil || mt.CreatedAt.Before(found.CreatedAt) {
			found = mt
		}
	}
	return found, nil
}

// compositionToSectionActivities converts a mock test composition into ordered SectionActivities.
func (s *Service) compositionToSectionActivities(
	ctx context.Context, comp []domain.MockTestPartComposition, partMap map[uuid.UUID]*domain.ExamPart,
) ([]SectionActivities, []uuid.UUID) {
	type sectionGroup struct {
		position int
		skill    string
		acts     []SittingActivityDTO
	}
	sectionGroups := make(map[int]*sectionGroup)

	var allActivityIDs []uuid.UUID
	for _, c := range comp {
		part, ok := partMap[c.PartID]
		skill := skillReading
		kind := "question"
		if ok {
			skill = part.Section
			kind = part.Kind
		}
		pos := skillToSectionPosition(skill)

		group, exists := sectionGroups[pos]
		if !exists {
			group = &sectionGroup{
				position: pos,
				skill:    skill,
				acts:     []SittingActivityDTO{},
			}
			sectionGroups[pos] = group
		}

		for _, actID := range c.ActivityIDs {
			allActivityIDs = append(allActivityIDs, actID)
			actDTO := SittingActivityDTO{
				ID:     actID,
				Kind:   kind,
				Weight: 1,
			}
			if s.lesson != nil {
				if h, err := s.lesson.ResolveActivity(ctx, actID); err == nil && h != nil {
					actDTO.Kind = h.Kind
					actDTO.ContentVersionID = h.ContentVersionID
					// Redacted, exactly as the exam pool's draw redacts. The
					// sitting is served from this stored copy, so a key that
					// reaches here reaches the learner (WO 19 H.6 trap 1); the
					// grader reads the content version, not this config.
					actDTO.Config = contentcontract.RedactForLearner(h.Config)
				}
			}
			group.acts = append(group.acts, actDTO)
		}
	}

	positions := make([]int, 0, len(sectionGroups))
	for pos := range sectionGroups {
		positions = append(positions, pos)
	}
	sort.Ints(positions)

	drawn := make([]SectionActivities, 0, len(positions))
	for _, pos := range positions {
		g := sectionGroups[pos]
		drawn = append(drawn, SectionActivities{
			SectionPosition: g.position,
			Skill:           g.skill,
			Activities:      g.acts,
		})
	}
	return drawn, allActivityIDs
}

// resolveExamForVersion finds the assess.exams row a version's sittings are
// recorded under (1700000870 seeds one per blueprinted version). A version with
// none is refused: recording the sitting under some other exam would put a
// VSTEP attempt in a TOEIC history.
func (s *Service) resolveExamForVersion(ctx context.Context, versionID uuid.UUID) (*sqlc.AssessExam, error) {
	exam, err := s.repo.GetExamByVersionID(ctx, versionID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find exam for version %s: %w", versionID, err)
	}
	if exam == nil || err != nil {
		return nil, apperr.New(apperr.Conflict, "EXAM_VERSION_HAS_NO_EXAM",
			"This exam version has no exam to record sittings under.")
	}
	return exam, nil
}

// validateMockTestAttempt loads and checks permissions and structures for a mock test.
func (s *Service) validateMockTestAttempt(
	ctx context.Context, userID, mockTestID uuid.UUID,
) (*domain.MockTest, *domain.ExamVersion, map[uuid.UUID]*domain.ExamPart, error) {
	mockTest, err := s.repo.GetMockTestByID(ctx, mockTestID)
	if err != nil || mockTest == nil {
		return nil, nil, nil, domain.ErrMockTestNotFound
	}

	if mockTest.OwnerID != nil && *mockTest.OwnerID != userID {
		return nil, nil, nil, domain.ErrUnauthorizedAttempt
	}

	blueprint, err := s.repo.GetBlueprintByID(ctx, mockTest.BlueprintID)
	if err != nil || blueprint == nil {
		return nil, nil, nil, apperr.New(apperr.NotFound, "BLUEPRINT_NOT_FOUND", "blueprint not found")
	}

	version, err := s.repo.GetExamVersionByID(ctx, blueprint.VersionID)
	if err != nil || version == nil {
		return nil, nil, nil, apperr.New(apperr.NotFound, "EXAM_VERSION_NOT_FOUND", "exam version not found")
	}

	parts, err := s.repo.ListExamPartsByVersionID(ctx, version.ID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list exam parts: %w", err)
	}

	partMap := make(map[uuid.UUID]*domain.ExamPart, len(parts))
	for _, p := range parts {
		partMap[p.ID] = p
	}
	return mockTest, version, partMap, nil
}

// StartMockTestAttempt starts or retakes a sitting for a composed mock test.
func (s *Service) StartMockTestAttempt(
	ctx context.Context, userID, mockTestID uuid.UUID,
) (*ExamAttemptDTO, error) {
	if s.repo == nil {
		return nil, errors.New("repository not configured")
	}

	mockTest, version, partMap, err := s.validateMockTestAttempt(ctx, userID, mockTestID)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now().UTC()
	if err := s.checkCanStart(ctx, userID, now); err != nil {
		return nil, err
	}

	drawn, allActivityIDs := s.compositionToSectionActivities(ctx, mockTest.Composition, partMap)
	if len(drawn) == 0 {
		return nil, domain.ErrInsufficientItems
	}

	drawnBytes, err := json.Marshal(drawn)
	if err != nil {
		return nil, fmt.Errorf("serialize section activities: %w", err)
	}

	attemptsCount, err := s.repo.CountUserMockTestAttempts(ctx, userID, mockTestID)
	if err != nil {
		return nil, fmt.Errorf("count user mock test attempts: %w", err)
	}

	var activityIDsToExpose []uuid.UUID
	if attemptsCount == 0 {
		activityIDsToExpose = allActivityIDs
	}

	exam, err := s.resolveExamForVersion(ctx, version.ID)
	if err != nil {
		return nil, err
	}
	examID := exam.ID

	duration := version.TotalMinutes
	if duration <= 0 {
		duration = domain.DefaultExamDurationMinutes
	}
	deadlineAt := now.Add(time.Duration(duration) * time.Minute)

	params := sqlc.CreateMockTestAttemptParams{
		ID:                    uuid.New(),
		UserID:                userID,
		ExamID:                examID,
		MockTestID:            &mockTestID,
		Mode:                  domain.ModeExam,
		ChosenDurationMinutes: clampInt32(duration),
		StartedAt:             now,
		DeadlineAt:            deadlineAt,
		CurrentSection:        clampInt32(drawn[0].SectionPosition),
		Status:                domain.StatusInProgress,
		SectionActivities:     drawnBytes,
		DraftAnswers:          []byte("{}"),
	}

	attempt, err := s.createMockSitting(ctx, params, activityIDsToExpose)
	if err != nil {
		return nil, err
	}

	dto := s.attemptDTO(attempt, exam, now, true)
	return &dto, nil
}

func (s *Service) createMockSitting(
	ctx context.Context, params sqlc.CreateMockTestAttemptParams, activityIDs []uuid.UUID,
) (*sqlc.AssessExamAttempt, error) {
	if s.pool == nil {
		created, err := s.repo.CreateMockTestAttempt(ctx, params)
		if err != nil {
			return nil, err
		}
		if s.exposures != nil && len(activityIDs) > 0 {
			if err := s.exposures.RecordItemExposures(ctx, nil, params.UserID, activityIDs); err != nil {
				return nil, fmt.Errorf("record item exposures: %w", err)
			}
		}
		return created, nil
	}

	var attempt *sqlc.AssessExamAttempt
	txErr := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		created, err := s.repo.CreateMockTestAttempt(txCtx, params)
		if err != nil {
			return err
		}
		attempt = created

		if s.exposures != nil && len(activityIDs) > 0 {
			if err := s.exposures.RecordItemExposures(txCtx, tx, params.UserID, activityIDs); err != nil {
				return fmt.Errorf("record item exposures: %w", err)
			}
		}

		if s.enqueuer != nil {
			args := examjob.ExpireAttemptArgs{AttemptID: params.ID}
			opts := &river.InsertOpts{ScheduledAt: params.DeadlineAt}
			if _, err := s.enqueuer.EnqueueTx(txCtx, tx, args, opts); err != nil {
				slog.WarnContext(txCtx, "could not schedule mock sitting expiry job", "error", err)
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return attempt, nil
}

// BlueprintDrawer implements PoolDrawer for a specific blueprint and seed.
type BlueprintDrawer struct {
	svc       *Service
	blueprint *domain.Blueprint
	seed      int64
}

// NewBlueprintDrawer constructs a BlueprintDrawer.
func NewBlueprintDrawer(svc *Service, blueprint *domain.Blueprint, seed int64) *BlueprintDrawer {
	return &BlueprintDrawer{
		svc:       svc,
		blueprint: blueprint,
		seed:      seed,
	}
}

// DrawSitting implements PoolDrawer.
func (d *BlueprintDrawer) DrawSitting(ctx context.Context, userID uuid.UUID, _ string) ([]SectionActivities, error) {
	mt, err := d.svc.ComposeMockTest(ctx, &userID, ComposeMockTestRequest{
		BlueprintID: d.blueprint.ID,
		Mode:        domain.MockModeRandom,
	})
	if err != nil {
		return nil, err
	}

	parts, err := d.svc.repo.ListExamPartsByVersionID(ctx, d.blueprint.VersionID)
	if err != nil {
		return nil, err
	}
	partMap := make(map[uuid.UUID]*domain.ExamPart, len(parts))
	for _, p := range parts {
		partMap[p.ID] = p
	}

	drawn, _ := d.svc.compositionToSectionActivities(ctx, mt.Composition, partMap)
	return drawn, nil
}

func sortParts(parts []*domain.ExamPart) {
	sort.Slice(parts, func(i, j int) bool {
		pi := skillToSectionPosition(parts[i].Section)
		pj := skillToSectionPosition(parts[j].Section)
		if pi != pj {
			return pi < pj
		}
		return parts[i].PartNumber < parts[j].PartNumber
	})
}

func skillToSectionPosition(skill string) int {
	switch skill {
	case skillListening:
		return 1
	case skillReading:
		return 2
	case skillWriting:
		return 3
	case skillSpeaking:
		return 4
	case skillUseOfEnglish:
		return 5
	default:
		return 1
	}
}

func hashUUID(id uuid.UUID) int64 {
	h := sha256.Sum256(id[:])
	//nolint:gosec // bit pattern conversion for deterministic seed
	return int64(binary.BigEndian.Uint64(h[:8]) & 0x7fffffffffffffff)
}
