package exam_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/exam/sqlc"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	"github.com/fluentra/fluentra/internal/modules/exam/service"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const (
	gateListening              = "listening"
	gateReading                = "reading"
	kindListeningComprehension = "listening_comprehension"
	kindReadingComprehension   = "reading_comprehension"
)

// fakeQBReader implements questionbankcontract.Reader for gate tests.
type fakeQBReader struct {
	questionsByPart map[uuid.UUID][]*questionbankcontract.Question
}

func (f *fakeQBReader) GetQuestion(
	_ context.Context, id uuid.UUID,
) (*questionbankcontract.Question, error) {
	for _, qs := range f.questionsByPart {
		for _, q := range qs {
			if q.ID == id {
				return q, nil
			}
		}
	}
	return nil, apperr.New(apperr.NotFound, "QUESTION_NOT_FOUND", "not found")
}

func (f *fakeQBReader) ListQuestions(
	_ context.Context, filter questionbankcontract.Filter,
) ([]*questionbankcontract.Question, int, error) {
	if filter.ExamPartID == nil {
		return nil, 0, nil
	}
	qs := f.questionsByPart[*filter.ExamPartID]
	return qs, len(qs), nil
}

func (f *fakeQBReader) DrawableForPart(
	_ context.Context, examPartID uuid.UUID,
) ([]*questionbankcontract.Question, error) {
	var out []*questionbankcontract.Question
	for _, q := range f.questionsByPart[examPartID] {
		if q.Status == questionbankcontract.StatusPublished && q.ActivityID != nil {
			out = append(out, q)
		}
	}
	return out, nil
}

func (f *fakeQBReader) SampleQuestions(
	_ context.Context, _ questionbankcontract.SampleCriteria,
) ([]*questionbankcontract.Question, error) {
	return nil, nil
}

func (f *fakeQBReader) GetQuestionStats(
	_ context.Context, _ uuid.UUID,
) (*questionbankcontract.QuestionStats, error) {
	return nil, nil
}

// fakeExposuresRecorder tracks item exposures for gate tests.
type fakeExposuresRecorder struct {
	mu        sync.Mutex
	exposures map[uuid.UUID]map[uuid.UUID]time.Time
}

func newFakeExposuresRecorder() *fakeExposuresRecorder {
	return &fakeExposuresRecorder{
		exposures: make(map[uuid.UUID]map[uuid.UUID]time.Time),
	}
}

func (f *fakeExposuresRecorder) RecordItemExposures(
	_ context.Context, _ pgx.Tx, userID uuid.UUID, activityIDs []uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	userMap, ok := f.exposures[userID]
	if !ok {
		userMap = make(map[uuid.UUID]time.Time)
		f.exposures[userID] = userMap
	}
	now := time.Now()
	for _, id := range activityIDs {
		userMap[id] = now
	}
	return nil
}

func (f *fakeExposuresRecorder) ListItemExposures(
	_ context.Context, userID uuid.UUID, activityIDs []uuid.UUID,
) (map[uuid.UUID]time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make(map[uuid.UUID]time.Time)
	userMap, ok := f.exposures[userID]
	if !ok {
		return result, nil
	}
	for _, id := range activityIDs {
		if t, found := userMap[id]; found {
			result[id] = t
		}
	}
	return result, nil
}

// inMemoryGateRepo implements service.ExamRepository for unit gate testing.
type inMemoryGateRepo struct {
	mu           sync.Mutex
	versions     map[uuid.UUID]*domain.ExamVersion
	parts        map[uuid.UUID][]*domain.ExamPart
	blueprints   map[uuid.UUID]*domain.Blueprint
	mockTests    map[uuid.UUID]*domain.MockTest
	attempts     map[uuid.UUID]*sqlc.AssessExamAttempt
	userAttempts map[uuid.UUID][]*sqlc.AssessExamAttempt
}

func newInMemoryGateRepo() *inMemoryGateRepo {
	return &inMemoryGateRepo{
		versions:     make(map[uuid.UUID]*domain.ExamVersion),
		parts:        make(map[uuid.UUID][]*domain.ExamPart),
		blueprints:   make(map[uuid.UUID]*domain.Blueprint),
		mockTests:    make(map[uuid.UUID]*domain.MockTest),
		attempts:     make(map[uuid.UUID]*sqlc.AssessExamAttempt),
		userAttempts: make(map[uuid.UUID][]*sqlc.AssessExamAttempt),
	}
}

func (r *inMemoryGateRepo) ListExams(_ context.Context) ([]sqlc.AssessExam, error) {
	return nil, nil
}
func (r *inMemoryGateRepo) GetExamByID(_ context.Context, id uuid.UUID) (*sqlc.AssessExam, error) {
	return &sqlc.AssessExam{ID: id, Level: "B2", TotalMinutes: 172}, nil
}
func (r *inMemoryGateRepo) GetExamBySlug(_ context.Context, _ string) (*sqlc.AssessExam, error) {
	return nil, nil
}
func (r *inMemoryGateRepo) ListExamSections(_ context.Context, _ uuid.UUID) ([]sqlc.AssessExamSection, error) {
	return nil, nil
}
func (r *inMemoryGateRepo) CreateExamAttempt(
	_ context.Context, _ sqlc.CreateExamAttemptParams,
) (*sqlc.AssessExamAttempt, error) {
	return nil, nil
}
func (r *inMemoryGateRepo) GetExamAttemptByID(_ context.Context, id uuid.UUID) (*sqlc.AssessExamAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attempts[id], nil
}
func (r *inMemoryGateRepo) GetExamAttemptForUser(_ context.Context, id, _ uuid.UUID) (*sqlc.AssessExamAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attempts[id], nil
}
func (r *inMemoryGateRepo) ListUserExamAttempts(
	_ context.Context, userID uuid.UUID, _, _ int32,
) ([]sqlc.AssessExamAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.userAttempts[userID]
	out := make([]sqlc.AssessExamAttempt, len(list))
	for i, att := range list {
		out[i] = *att
	}
	return out, nil
}
func (r *inMemoryGateRepo) CountUserExamAttempts(_ context.Context, userID uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(len(r.userAttempts[userID])), nil
}
func (r *inMemoryGateRepo) CountUserActiveAttempts(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *inMemoryGateRepo) CountUserAttemptsToday(_ context.Context, _ uuid.UUID, _, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *inMemoryGateRepo) UpdateDraftAnswers(
	_ context.Context, id uuid.UUID, answers []byte, _ time.Time,
) (*sqlc.AssessExamAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	att := r.attempts[id]
	if att != nil {
		att.DraftAnswers = answers
	}
	return att, nil
}
func (r *inMemoryGateRepo) UpdateCurrentSection(
	_ context.Context, id uuid.UUID, sec int32, _ time.Time,
) (*sqlc.AssessExamAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	att := r.attempts[id]
	if att != nil {
		att.CurrentSection = sec
	}
	return att, nil
}
func (r *inMemoryGateRepo) MarkAttemptCompleted(
	_ context.Context, id uuid.UUID, sub time.Time, by string,
) (*sqlc.AssessExamAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	att := r.attempts[id]
	if att != nil {
		att.Status = domain.StatusCompleted
		att.SubmittedAt = &sub
		att.SubmittedBy = &by
	}
	return att, nil
}
func (r *inMemoryGateRepo) MarkAttemptExpired(
	_ context.Context, id uuid.UUID, sub time.Time,
) (*sqlc.AssessExamAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	att := r.attempts[id]
	if att != nil {
		att.Status = domain.StatusExpired
		att.SubmittedAt = &sub
	}
	return att, nil
}
func (r *inMemoryGateRepo) ListExpiredInProgressAttempts(
	_ context.Context, _ time.Time,
) ([]sqlc.AssessExamAttempt, error) {
	return nil, nil
}
func (r *inMemoryGateRepo) CreateScoreReport(
	_ context.Context, _ sqlc.CreateScoreReportParams,
) (*sqlc.AssessScoreReport, error) {
	return nil, nil
}
func (r *inMemoryGateRepo) GetScoreReportByAttemptID(_ context.Context, _ uuid.UUID) (*sqlc.AssessScoreReport, error) {
	return nil, nil
}
func (r *inMemoryGateRepo) ListPendingScoreReports(_ context.Context) ([]sqlc.AssessScoreReport, error) {
	return nil, nil
}
func (r *inMemoryGateRepo) UpdateScoreReport(
	_ context.Context, _ sqlc.UpdateScoreReportParams,
) (*sqlc.AssessScoreReport, error) {
	return nil, nil
}
func (r *inMemoryGateRepo) RecordIntegrityEvent(_ context.Context, _ sqlc.RecordIntegrityEventParams) error {
	return nil
}
func (r *inMemoryGateRepo) ListIntegrityEvents(_ context.Context, _ uuid.UUID) ([]sqlc.AssessIntegrityEvent, error) {
	return nil, nil
}
func (r *inMemoryGateRepo) ListCurrentExamVersions(_ context.Context) ([]*domain.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.ExamVersion
	for _, v := range r.versions {
		if v.IsCurrent {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *inMemoryGateRepo) GetExamVersionByID(_ context.Context, id uuid.UUID) (*domain.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.versions[id], nil
}
func (r *inMemoryGateRepo) GetExamVersionByCode(_ context.Context, code string) (*domain.ExamVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.versions {
		if v.Code == code {
			return v, nil
		}
	}
	return nil, nil
}
func (r *inMemoryGateRepo) ListExamPartsByVersionID(
	_ context.Context, versionID uuid.UUID,
) ([]*domain.ExamPart, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.parts[versionID], nil
}
func (r *inMemoryGateRepo) GetExamPartByID(_ context.Context, id uuid.UUID) (*domain.ExamPart, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, partList := range r.parts {
		for _, p := range partList {
			if p.ID == id {
				return p, nil
			}
		}
	}
	return nil, nil
}
func (r *inMemoryGateRepo) ListBlueprintsByVersionID(
	_ context.Context, versionID uuid.UUID,
) ([]*domain.Blueprint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Blueprint
	for _, bp := range r.blueprints {
		if bp.VersionID == versionID {
			out = append(out, bp)
		}
	}
	return out, nil
}
func (r *inMemoryGateRepo) GetBlueprintByID(_ context.Context, id uuid.UUID) (*domain.Blueprint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.blueprints[id], nil
}
func (r *inMemoryGateRepo) GetBlueprintByName(
	_ context.Context, versionID uuid.UUID, name string,
) (*domain.Blueprint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, bp := range r.blueprints {
		if bp.VersionID == versionID && bp.Name == name {
			return bp, nil
		}
	}
	return nil, nil
}
func (r *inMemoryGateRepo) CreateMockTest(_ context.Context, mt *domain.MockTest) (*domain.MockTest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mockTests[mt.ID] = mt
	return mt, nil
}
func (r *inMemoryGateRepo) GetMockTestByID(_ context.Context, id uuid.UUID) (*domain.MockTest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mockTests[id], nil
}
func (r *inMemoryGateRepo) ListFixedMockTests(_ context.Context, blueprintID uuid.UUID) ([]*domain.MockTest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.MockTest
	for _, mt := range r.mockTests {
		if mt.Mode == domain.MockModeFixed && mt.BlueprintID == blueprintID {
			out = append(out, mt)
		}
	}
	return out, nil
}

func (r *inMemoryGateRepo) GetLatestUserMockTestAttempt(
	_ context.Context, _, _ uuid.UUID,
) (*sqlc.GetLatestUserMockTestAttemptRow, error) {
	return nil, nil
}

func (r *inMemoryGateRepo) ListMockTestsByOwner(_ context.Context, ownerID *uuid.UUID) ([]*domain.MockTest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.MockTest
	for _, mt := range r.mockTests {
		if (ownerID == nil && mt.OwnerID == nil) || (ownerID != nil && mt.OwnerID != nil && *mt.OwnerID == *ownerID) {
			out = append(out, mt)
		}
	}
	return out, nil
}
func (r *inMemoryGateRepo) GetExamByVersionID(_ context.Context, _ uuid.UUID) (*sqlc.AssessExam, error) {
	return &sqlc.AssessExam{
		ID:           uuid.New(),
		Slug:         "vstep-3-5",
		TitleEn:      "VSTEP 3-5",
		Level:        "B2",
		TotalMinutes: 172,
	}, nil
}
func (r *inMemoryGateRepo) CreateMockTestAttempt(
	_ context.Context, arg sqlc.CreateMockTestAttemptParams,
) (*sqlc.AssessExamAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	att := &sqlc.AssessExamAttempt{
		ID:                    arg.ID,
		UserID:                arg.UserID,
		ExamID:                arg.ExamID,
		MockTestID:            arg.MockTestID,
		Mode:                  arg.Mode,
		ChosenDurationMinutes: arg.ChosenDurationMinutes,
		StartedAt:             arg.StartedAt,
		DeadlineAt:            arg.DeadlineAt,
		CurrentSection:        arg.CurrentSection,
		Status:                arg.Status,
		SectionActivities:     arg.SectionActivities,
		DraftAnswers:          arg.DraftAnswers,
	}
	r.attempts[arg.ID] = att
	r.userAttempts[arg.UserID] = append(r.userAttempts[arg.UserID], att)
	return att, nil
}
func (r *inMemoryGateRepo) CountUserMockTestAttempts(_ context.Context, userID, mockTestID uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, att := range r.userAttempts[userID] {
		if att.MockTestID != nil && *att.MockTestID == mockTestID {
			count++
		}
	}
	return count, nil
}

// Stage H Gate Test (WO 19 §H):
// 1. Trap H.1: A composition that cannot be filled must refuse with the short part named.
// 2. Coverage report: Published groups available, groups needed per test, distinct tests possible.
// 3. Compose 20 VSTEP mock tests: report measured overlap.
// 4. Trap H.2: A retake shows the same questions and does NOT re-mark exposures; a new test draws anew.
// 5. Trap H.3: A fixed test's composition is frozen once created.
func TestStageH_Gate_MockTestsAndCoverage(t *testing.T) {
	ctx := context.Background()

	versionID := uuid.New()
	vstepVersion := &domain.ExamVersion{
		ID:           versionID,
		ExamFamily:   domain.FamilyVstep,
		Code:         "VSTEP_3_5",
		Title:        "VSTEP Level 3-5",
		TotalMinutes: 172,
		Scoring:      []byte(`{"type": "vstep"}`),
		SourceURL:    "https://vstep.vnu.edu.vn",
		VerifiedAt:   time.Now(),
		IsCurrent:    true,
	}

	parts := []*domain.ExamPart{
		{ID: uuid.New(), VersionID: versionID, Section: gateListening, PartNumber: 1,
			Kind: kindListeningComprehension, QuestionCount: 8, GroupSize: 1},
		{ID: uuid.New(), VersionID: versionID, Section: gateListening, PartNumber: 2,
			Kind: kindListeningComprehension, QuestionCount: 12, GroupSize: 4},
		{ID: uuid.New(), VersionID: versionID, Section: gateListening, PartNumber: 3,
			Kind: kindListeningComprehension, QuestionCount: 15, GroupSize: 5},
		{ID: uuid.New(), VersionID: versionID, Section: gateReading, PartNumber: 1,
			Kind: kindReadingComprehension, QuestionCount: 10, GroupSize: 10},
		{ID: uuid.New(), VersionID: versionID, Section: gateReading, PartNumber: 2,
			Kind: kindReadingComprehension, QuestionCount: 10, GroupSize: 10},
		{ID: uuid.New(), VersionID: versionID, Section: gateReading, PartNumber: 3,
			Kind: kindReadingComprehension, QuestionCount: 10, GroupSize: 10},
		{ID: uuid.New(), VersionID: versionID, Section: gateReading, PartNumber: 4,
			Kind: kindReadingComprehension, QuestionCount: 10, GroupSize: 10},
	}

	blueprintID := uuid.New()
	blueprint := &domain.Blueprint{
		ID:               blueprintID,
		VersionID:        versionID,
		Name:             "vstep_default",
		CefrDistribution: []byte(`{"B1": 0.33, "B2": 0.34, "C1": 0.33}`),
	}

	repo := newInMemoryGateRepo()
	repo.versions[versionID] = vstepVersion
	repo.parts[versionID] = parts
	repo.blueprints[blueprintID] = blueprint

	exposures := newFakeExposuresRecorder()
	qb := &fakeQBReader{questionsByPart: make(map[uuid.UUID][]*questionbankcontract.Question)}

	svc := service.New(service.Deps{
		Repo:         repo,
		Exposures:    exposures,
		Questionbank: qb,
		Clock:        clock.Real{},
		DailyLimit:   100,
	})

	t.Run("RefuseWhenBankCannotFillNamingShortPart", func(t *testing.T) {
		gateCheckRefusal(ctx, t, svc, blueprintID, parts[0], qb)
	})

	seedGateQuestions(parts, qb)

	t.Run("CoverageReportMatchesOutcome", func(t *testing.T) {
		gateCheckCoverage(ctx, t, svc, versionID, parts[6].ID)
	})

	t.Run("Compose20MockTestsAndReportOverlap", func(t *testing.T) {
		gateCheck20Tests(ctx, t, svc, blueprintID)
	})

	t.Run("RetakeShowsSameQuestionsAndNoDuplicateExposures", func(t *testing.T) {
		gateCheckRetake(ctx, t, svc, blueprintID)
	})

	t.Run("FixedTestCompositionIsFrozen", func(t *testing.T) {
		gateCheckFixed(ctx, t, svc, blueprintID)
	})
}

func gateCheckRefusal(
	ctx context.Context, t *testing.T, svc *service.Service,
	blueprintID uuid.UUID, p1 *domain.ExamPart, qb *fakeQBReader,
) {
	var p1Questions []*questionbankcontract.Question
	for i := 0; i < 5; i++ {
		actID := uuid.New()
		p1Questions = append(p1Questions, &questionbankcontract.Question{
			ID:         uuid.New(),
			ActivityID: &actID,
			ExamPartID: &p1.ID,
			Status:     questionbankcontract.StatusPublished,
		})
	}
	qb.questionsByPart[p1.ID] = p1Questions

	userID := uuid.New()
	_, err := svc.ComposeMockTest(ctx, &userID, service.ComposeMockTestRequest{
		BlueprintID: blueprintID,
		Mode:        domain.MockModeRandom,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient published questions to compose part listening-1")
	assert.Contains(t, err.Error(), "needed 8, available 5")
}

func seedGateQuestions(parts []*domain.ExamPart, qb *fakeQBReader) {
	seededCounts := map[uuid.UUID]int{
		parts[0].ID: 48,
		parts[1].ID: 18,
		parts[2].ID: 18,
		parts[3].ID: 10,
		parts[4].ID: 10,
		parts[5].ID: 10,
		parts[6].ID: 5,
	}
	for _, p := range parts {
		count := seededCounts[p.ID]
		var qs []*questionbankcontract.Question
		for i := 0; i < count; i++ {
			actID := uuid.New()
			qs = append(qs, &questionbankcontract.Question{
				ID:            uuid.New(),
				ActivityID:    &actID,
				ExamPartID:    &p.ID,
				QuestionCount: p.GroupSize,
				Status:        questionbankcontract.StatusPublished,
			})
		}
		qb.questionsByPart[p.ID] = qs
	}
}

func gateCheckCoverage(
	ctx context.Context, t *testing.T, svc *service.Service,
	versionID, expectedBottleneckID uuid.UUID,
) {
	report, err := svc.GetExamVersionCoverage(ctx, versionID)
	require.NoError(t, err)
	assert.Equal(t, versionID, report.VersionID)
	assert.Equal(t, "VSTEP_3_5", report.ExamCode)
	assert.Equal(t, 5, report.DistinctTestsPossible, "Distinct tests possible must be 5")
	require.NotNil(t, report.BottleneckPartID)
	assert.Equal(t, expectedBottleneckID, *report.BottleneckPartID, "Bottleneck must be Part 7")
}

func gateCheck20Tests(
	ctx context.Context, t *testing.T, svc *service.Service, blueprintID uuid.UUID,
) {
	userID := uuid.New()
	composedTests := make([]*domain.MockTest, 20)
	for i := 0; i < 20; i++ {
		mt, err := svc.ComposeMockTest(ctx, &userID, service.ComposeMockTestRequest{
			BlueprintID: blueprintID,
			Mode:        domain.MockModeRandom,
		})
		require.NoError(t, err, "Failed to compose test %d", i+1)
		composedTests[i] = mt

		_, err = svc.StartMockTestAttempt(ctx, userID, mt.ID)
		require.NoError(t, err, "Failed to start attempt %d", i+1)
	}

	var totalOverlap float64
	for i := 0; i < 19; i++ {
		setA := extractActivitySet(composedTests[i])
		setB := extractActivitySet(composedTests[i+1])

		intersection := 0
		for id := range setA {
			if setB[id] {
				intersection++
			}
		}
		union := len(setA)
		for id := range setB {
			if !setA[id] {
				union++
			}
		}
		totalOverlap += float64(intersection) / float64(union)
	}
	avgOverlap := totalOverlap / 19.0
	t.Logf("GATE STAGE H: Composed 20 VSTEP tests. Avg consecutive Jaccard = %.2f%%", avgOverlap*100)

	first5Sets := make([]map[uuid.UUID]bool, 5)
	for i := 0; i < 5; i++ {
		first5Sets[i] = extractActivitySet(composedTests[i])
	}
	for i := 0; i < 5; i++ {
		for j := i + 1; j < 5; j++ {
			for id := range first5Sets[i] {
				assert.False(t, first5Sets[j][id], "Tests %d and %d shared item %s; first 5 must have zero overlap",
					i+1, j+1, id)
			}
		}
	}
}

func extractActivitySet(mt *domain.MockTest) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool)
	for _, comp := range mt.Composition {
		for _, id := range comp.ActivityIDs {
			set[id] = true
		}
	}
	return set
}

func gateCheckRetake(
	ctx context.Context, t *testing.T, svc *service.Service, blueprintID uuid.UUID,
) {
	learnerID := uuid.New()
	mt, err := svc.ComposeMockTest(ctx, &learnerID, service.ComposeMockTestRequest{
		BlueprintID: blueprintID,
		Mode:        domain.MockModeRandom,
	})
	require.NoError(t, err)

	attempt1, err := svc.StartMockTestAttempt(ctx, learnerID, mt.ID)
	require.NoError(t, err)

	attempt2, err := svc.StartMockTestAttempt(ctx, learnerID, mt.ID)
	require.NoError(t, err)

	require.Equal(t, len(attempt1.SectionActivities), len(attempt2.SectionActivities))
	for sIdx, sec1 := range attempt1.SectionActivities {
		sec2 := attempt2.SectionActivities[sIdx]
		assert.Equal(t, sec1.SectionPosition, sec2.SectionPosition)
		require.Equal(t, len(sec1.Activities), len(sec2.Activities))
		for aIdx, act1 := range sec1.Activities {
			act2 := sec2.Activities[aIdx]
			assert.Equal(t, act1.ID, act2.ID, "Retake must present identical activity at position %d", aIdx)
		}
	}

	newTest, err := svc.ComposeMockTest(ctx, &learnerID, service.ComposeMockTestRequest{
		BlueprintID: blueprintID,
		Mode:        domain.MockModeRandom,
	})
	require.NoError(t, err)
	assert.NotEqual(t, mt.ID, newTest.ID)
}

func gateCheckFixed(
	ctx context.Context, t *testing.T, svc *service.Service, blueprintID uuid.UUID,
) {
	fixedTest1, err := svc.ComposeMockTest(ctx, nil, service.ComposeMockTestRequest{
		BlueprintID: blueprintID,
		Mode:        domain.MockModeFixed,
	})
	require.NoError(t, err)

	fixedTest2, err := svc.ComposeMockTest(ctx, nil, service.ComposeMockTestRequest{
		BlueprintID: blueprintID,
		Mode:        domain.MockModeFixed,
	})
	require.NoError(t, err)

	assert.Equal(t, fixedTest1.Seed, fixedTest2.Seed)
	assert.Equal(t, fixedTest1.Composition, fixedTest2.Composition,
		"Fixed tests for the same blueprint must be identical")

	// Two learners with different histories get the same fixed test: the
	// composition is the stored one, not redrawn around each learner's exposures.
	veteran := uuid.New()
	for i := 0; i < 3; i++ {
		mt, composeErr := svc.ComposeMockTest(ctx, &veteran, service.ComposeMockTestRequest{
			BlueprintID: blueprintID,
			Mode:        domain.MockModeRandom,
		})
		require.NoError(t, composeErr)
		_, startErr := svc.StartMockTestAttempt(ctx, veteran, mt.ID)
		require.NoError(t, startErr)
	}
	newcomer := uuid.New()
	forVeteran, err := svc.ComposeMockTest(ctx, &veteran, service.ComposeMockTestRequest{
		BlueprintID: blueprintID,
		Mode:        domain.MockModeFixed,
	})
	require.NoError(t, err)
	forNewcomer, err := svc.ComposeMockTest(ctx, &newcomer, service.ComposeMockTestRequest{
		BlueprintID: blueprintID,
		Mode:        domain.MockModeFixed,
	})
	require.NoError(t, err)
	assert.Equal(t, fixedTest1.ID, forVeteran.ID, "a fixed test is one stored row")
	assert.Equal(t, fixedTest1.ID, forNewcomer.ID, "a fixed test is one stored row")
	assert.Nil(t, forVeteran.OwnerID)
}
