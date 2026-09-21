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

	parts, err := s.repo.ListExamPartsByVersionID(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("list exam parts: %w", err)
	}

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
	}, nil
}

func (s *Service) computePartCoverage(ctx context.Context, p *domain.ExamPart) ExamPartCoverageDTO {
	groupsNeeded := p.QuestionCount / p.GroupSize
	if groupsNeeded <= 0 {
		groupsNeeded = 1
	}

	publishedAvailable := 0
	if s.questionbank != nil {
		pubStatus := questionbankcontract.StatusPublished
		_, total, err := s.questionbank.ListQuestions(ctx, questionbankcontract.Filter{
			ExamPartID: &p.ID,
			Status:     &pubStatus,
			Limit:      1,
		})
		if err == nil {
			publishedAvailable = total
		}
	}

	testsPossible := publishedAvailable / groupsNeeded

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

// drawPartActivities selects the required number of activity IDs for one exam part.
func (s *Service) drawPartActivities(
	ctx context.Context, part *domain.ExamPart, seed int64, userID *uuid.UUID,
) ([]uuid.UUID, error) {
	needed := part.QuestionCount / part.GroupSize
	if needed <= 0 {
		needed = 1
	}

	var candidateQuestions []*questionbankcontract.Question
	if s.questionbank != nil {
		pubStatus := questionbankcontract.StatusPublished
		questions, _, err := s.questionbank.ListQuestions(ctx, questionbankcontract.Filter{
			ExamPartID: &part.ID,
			Status:     &pubStatus,
			Limit:      1000,
		})
		if err != nil {
			return nil, fmt.Errorf("fetch published questions for part %s: %w", part.ID, err)
		}
		candidateQuestions = questions
	}

	if len(candidateQuestions) < needed {
		return nil, apperr.New(
			apperr.Conflict,
			"INSUFFICIENT_ITEMS",
			fmt.Sprintf("insufficient published questions to compose part %s-%d (%s): needed %d, available %d",
				part.Section, part.PartNumber, part.ID, needed, len(candidateQuestions)),
		)
	}

	candidateActIDs := make([]uuid.UUID, len(candidateQuestions))
	for i, q := range candidateQuestions {
		if q.ActivityID != nil && *q.ActivityID != uuid.Nil {
			candidateActIDs[i] = *q.ActivityID
		} else {
			candidateActIDs[i] = q.ID
		}
	}

	return s.sampleActivities(ctx, candidateActIDs, needed, seed, int64(part.PartNumber), userID)
}

func (s *Service) sampleActivities(
	ctx context.Context, candidateActIDs []uuid.UUID, needed int, seed, partNum int64, userID *uuid.UUID,
) ([]uuid.UUID, error) {
	var unseen, seen []uuid.UUID
	if userID != nil && s.exposures != nil {
		exposures, expErr := s.exposures.ListItemExposures(ctx, *userID, candidateActIDs)
		if expErr != nil {
			slog.WarnContext(ctx, "could not query item exposures", "user_id", userID, "error", expErr)
		}
		for _, actID := range candidateActIDs {
			if _, served := exposures[actID]; served {
				seen = append(seen, actID)
			} else {
				unseen = append(unseen, actID)
			}
		}
	} else {
		unseen = candidateActIDs
	}

	//nolint:gosec // deterministic shuffle for pseudo-random mock test composition
	rng := rand.New(rand.NewSource(seed + partNum*1000003))

	rng.Shuffle(len(unseen), func(i, j int) {
		unseen[i], unseen[j] = unseen[j], unseen[i]
	})
	rng.Shuffle(len(seen), func(i, j int) {
		seen[i], seen[j] = seen[j], seen[i]
	})

	if len(unseen) >= needed {
		return unseen[:needed], nil
	}
	picked := append([]uuid.UUID{}, unseen...)
	shortfall := needed - len(picked)
	picked = append(picked, seen[:shortfall]...)
	return picked, nil
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

	blueprint, parts, err := s.resolveMockTestParts(ctx, req.BlueprintID, req.Parts)
	if err != nil {
		return nil, err
	}

	var seed int64
	if mode == domain.MockModeFixed {
		seed = hashUUID(blueprint.ID)
	} else {
		seed = time.Now().UnixNano()
	}

	compositions := make([]domain.MockTestPartComposition, 0, len(parts))
	for _, part := range parts {
		picked, drawErr := s.drawPartActivities(ctx, part, seed, userID)
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
					actDTO.Config = h.Config
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

// resolveExamForVersion finds an assess.exam row for the version.
func (s *Service) resolveExamForVersion(ctx context.Context, versionID uuid.UUID) (*sqlc.AssessExam, uuid.UUID) {
	exam, err := s.repo.GetExamByVersionID(ctx, versionID)
	if err == nil && exam != nil {
		return exam, exam.ID
	}
	existingExams, _ := s.repo.ListExams(ctx)
	if len(existingExams) > 0 {
		return &existingExams[0], existingExams[0].ID
	}
	return nil, uuid.New()
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

	exam, examID := s.resolveExamForVersion(ctx, version.ID)

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
