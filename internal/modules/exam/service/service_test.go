package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/exam/sqlc"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	"github.com/fluentra/fluentra/internal/modules/exam/service"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type mockExamRepo struct {
	// bestScore is what GetUserBestVersionScore answers.
	bestScore  *float64
	mu         sync.Mutex
	exams      []sqlc.AssessExam
	mockTests  []*domain.MockTest
	blueprint  *domain.Blueprint
	parts      []*domain.ExamPart
	version    *domain.ExamVersion
	attempts   map[uuid.UUID]sqlc.AssessExamAttempt
	reports    map[uuid.UUID]sqlc.AssessScoreReport
	events     []sqlc.RecordIntegrityEventParams
	dailyCount int64
}

func newMockExamRepo() *mockExamRepo {
	return &mockExamRepo{
		attempts: make(map[uuid.UUID]sqlc.AssessExamAttempt),
		reports:  make(map[uuid.UUID]sqlc.AssessScoreReport),
	}
}

func (m *mockExamRepo) ListExams(_ context.Context) ([]sqlc.AssessExam, error) { return m.exams, nil }

func (m *mockExamRepo) GetExamByID(_ context.Context, id uuid.UUID) (*sqlc.AssessExam, error) {
	for _, e := range m.exams {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (m *mockExamRepo) GetExamBySlug(_ context.Context, slug string) (*sqlc.AssessExam, error) {
	for _, e := range m.exams {
		if e.Slug == slug {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (m *mockExamRepo) ListExamSections(_ context.Context, _ uuid.UUID) ([]sqlc.AssessExamSection, error) {
	return nil, nil
}

func (m *mockExamRepo) CreateExamAttempt(
	_ context.Context, arg sqlc.CreateExamAttemptParams,
) (*sqlc.AssessExamAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	att := sqlc.AssessExamAttempt{
		ID:                    arg.ID,
		ExamID:                arg.ExamID,
		UserID:                arg.UserID,
		Mode:                  arg.Mode,
		ChosenDurationMinutes: arg.ChosenDurationMinutes,
		StartedAt:             arg.StartedAt,
		DeadlineAt:            arg.DeadlineAt,
		Status:                arg.Status,
		CurrentSection:        arg.CurrentSection,
		SectionActivities:     arg.SectionActivities,
		DraftAnswers:          arg.DraftAnswers,
		CreatedAt:             arg.StartedAt,
		UpdatedAt:             arg.StartedAt,
	}
	m.attempts[arg.ID] = att
	return &att, nil
}

func (m *mockExamRepo) GetExamAttemptByID(_ context.Context, id uuid.UUID) (*sqlc.AssessExamAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	att, ok := m.attempts[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return &att, nil
}

func (m *mockExamRepo) GetExamAttemptForUser(_ context.Context, id, userID uuid.UUID) (*sqlc.AssessExamAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	att, ok := m.attempts[id]
	if !ok || att.UserID != userID {
		return nil, pgx.ErrNoRows
	}
	return &att, nil
}

func (m *mockExamRepo) ListUserExamAttempts(
	_ context.Context, userID uuid.UUID, _, _ int32,
) ([]sqlc.AssessExamAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []sqlc.AssessExamAttempt
	for _, att := range m.attempts {
		if att.UserID == userID {
			res = append(res, att)
		}
	}
	return res, nil
}

func (m *mockExamRepo) CountUserExamAttempts(ctx context.Context, userID uuid.UUID) (int64, error) {
	rows, _ := m.ListUserExamAttempts(ctx, userID, 0, 0)
	return int64(len(rows)), nil
}

func (m *mockExamRepo) CountUserActiveAttempts(_ context.Context, userID uuid.UUID) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for _, att := range m.attempts {
		if att.UserID == userID && att.Status == domain.StatusInProgress {
			count++
		}
	}
	return count, nil
}

func (m *mockExamRepo) CountUserAttemptsToday(_ context.Context, _ uuid.UUID, _, _ time.Time) (int64, error) {
	return m.dailyCount, nil
}

func (m *mockExamRepo) UpdateDraftAnswers(
	_ context.Context, id uuid.UUID, draftAnswers []byte, updatedAt time.Time,
) (*sqlc.AssessExamAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	att, ok := m.attempts[id]
	if !ok || att.Status != domain.StatusInProgress {
		return nil, pgx.ErrNoRows
	}
	att.DraftAnswers = draftAnswers
	att.UpdatedAt = updatedAt
	m.attempts[id] = att
	return &att, nil
}

func (m *mockExamRepo) UpdateCurrentSection(
	_ context.Context, id uuid.UUID, section int32, updatedAt time.Time,
) (*sqlc.AssessExamAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	att, ok := m.attempts[id]
	if !ok || att.Status != domain.StatusInProgress {
		return nil, pgx.ErrNoRows
	}
	att.CurrentSection = section
	att.UpdatedAt = updatedAt
	m.attempts[id] = att
	return &att, nil
}

func (m *mockExamRepo) mark(id uuid.UUID, status, by string, at time.Time) (*sqlc.AssessExamAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	att, ok := m.attempts[id]
	if !ok || att.Status != domain.StatusInProgress {
		return nil, pgx.ErrNoRows
	}
	att.Status = status
	att.SubmittedAt = &at
	att.SubmittedBy = &by
	m.attempts[id] = att
	return &att, nil
}

func (m *mockExamRepo) MarkAttemptCompleted(
	_ context.Context, id uuid.UUID, at time.Time, by string,
) (*sqlc.AssessExamAttempt, error) {
	return m.mark(id, domain.StatusCompleted, by, at)
}

func (m *mockExamRepo) MarkAttemptExpired(
	_ context.Context, id uuid.UUID, at time.Time,
) (*sqlc.AssessExamAttempt, error) {
	return m.mark(id, domain.StatusExpired, domain.SubmittedByExpiry, at)
}

func (m *mockExamRepo) ListExpiredInProgressAttempts(
	_ context.Context, cutoff time.Time,
) ([]sqlc.AssessExamAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []sqlc.AssessExamAttempt
	for _, att := range m.attempts {
		if att.Status == domain.StatusInProgress && !att.DeadlineAt.After(cutoff) {
			res = append(res, att)
		}
	}
	return res, nil
}

func (m *mockExamRepo) CreateScoreReport(
	_ context.Context, arg sqlc.CreateScoreReportParams,
) (*sqlc.AssessScoreReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.reports[arg.AttemptID]; exists {
		return nil, errors.New("duplicate key value violates unique constraint")
	}
	rep := sqlc.AssessScoreReport{
		ID:               arg.ID,
		AttemptID:        arg.AttemptID,
		UserID:           arg.UserID,
		ExamID:           arg.ExamID,
		OverallScore:     arg.OverallScore,
		OverallBand:      arg.OverallBand,
		Status:           arg.Status,
		PerSection:       arg.PerSection,
		Feedback:         arg.Feedback,
		IntegritySignals: arg.IntegritySignals,
		CreatedAt:        arg.CreatedAt,
		UpdatedAt:        arg.CreatedAt,
	}
	m.reports[arg.AttemptID] = rep
	return &rep, nil
}

func (m *mockExamRepo) GetScoreReportByAttemptID(
	_ context.Context, attemptID uuid.UUID,
) (*sqlc.AssessScoreReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rep, ok := m.reports[attemptID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return &rep, nil
}

func (m *mockExamRepo) ListPendingScoreReports(_ context.Context) ([]sqlc.AssessScoreReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []sqlc.AssessScoreReport
	for _, rep := range m.reports {
		if rep.Status == domain.ReportStatusPending {
			res = append(res, rep)
		}
	}
	return res, nil
}

func (m *mockExamRepo) UpdateScoreReport(
	_ context.Context, arg sqlc.UpdateScoreReportParams,
) (*sqlc.AssessScoreReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rep, ok := m.reports[arg.AttemptID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	rep.OverallScore = arg.OverallScore
	rep.OverallBand = arg.OverallBand
	rep.Status = arg.Status
	rep.PerSection = arg.PerSection
	rep.Feedback = arg.Feedback
	rep.IntegritySignals = arg.IntegritySignals
	rep.UpdatedAt = arg.UpdatedAt
	m.reports[arg.AttemptID] = rep
	return &rep, nil
}

func (m *mockExamRepo) RecordIntegrityEvent(_ context.Context, arg sqlc.RecordIntegrityEventParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, arg)
	return nil
}

func (m *mockExamRepo) ListIntegrityEvents(
	_ context.Context, attemptID uuid.UUID,
) ([]sqlc.AssessIntegrityEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []sqlc.AssessIntegrityEvent
	for _, ev := range m.events {
		if ev.AttemptID == attemptID {
			res = append(
				res, sqlc.AssessIntegrityEvent{ID: ev.ID, AttemptID: ev.AttemptID, Kind: ev.Kind, OccurredAt: ev.OccurredAt},
			)
		}
	}
	return res, nil
}

func (m *mockExamRepo) ListCurrentExamVersions(_ context.Context) ([]*domain.ExamVersion, error) {
	if m.version == nil {
		return nil, nil
	}
	return []*domain.ExamVersion{m.version}, nil
}

func (m *mockExamRepo) GetExamVersionByID(_ context.Context, _ uuid.UUID) (*domain.ExamVersion, error) {
	return m.version, nil
}

func (m *mockExamRepo) GetExamVersionByCode(_ context.Context, _ string) (*domain.ExamVersion, error) {
	return m.version, nil
}

func (m *mockExamRepo) ListExamPartsByVersionID(_ context.Context, _ uuid.UUID) ([]*domain.ExamPart, error) {
	return m.parts, nil
}

func (m *mockExamRepo) GetExamPartByID(_ context.Context, _ uuid.UUID) (*domain.ExamPart, error) {
	return nil, nil
}

func (m *mockExamRepo) ListBlueprintsByVersionID(_ context.Context, _ uuid.UUID) ([]*domain.Blueprint, error) {
	if m.blueprint == nil {
		return nil, nil
	}
	return []*domain.Blueprint{m.blueprint}, nil
}

func (m *mockExamRepo) GetBlueprintByID(_ context.Context, _ uuid.UUID) (*domain.Blueprint, error) {
	return m.blueprint, nil
}

func (m *mockExamRepo) GetBlueprintByName(_ context.Context, _ uuid.UUID, _ string) (*domain.Blueprint, error) {
	return nil, nil
}

func (m *mockExamRepo) CreateMockTest(_ context.Context, mt *domain.MockTest) (*domain.MockTest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockTests = append(m.mockTests, mt)
	return mt, nil
}

func (m *mockExamRepo) GetMockTestByID(_ context.Context, _ uuid.UUID) (*domain.MockTest, error) {
	return nil, nil
}

func (m *mockExamRepo) ListMockTestsByOwner(_ context.Context, _ *uuid.UUID) ([]*domain.MockTest, error) {
	return m.mockTests, nil
}

func (m *mockExamRepo) GetUserBestVersionScore(_ context.Context, _, _ uuid.UUID) (*float64, error) {
	return m.bestScore, nil
}

func (m *mockExamRepo) ListFixedMockTests(_ context.Context, _ uuid.UUID) ([]*domain.MockTest, error) {
	return m.mockTests, nil
}

func (m *mockExamRepo) GetLatestUserMockTestAttempt(
	_ context.Context, _, _ uuid.UUID,
) (*sqlc.GetLatestUserMockTestAttemptRow, error) {
	return nil, nil
}

func (m *mockExamRepo) GetExamByVersionID(_ context.Context, _ uuid.UUID) (*sqlc.AssessExam, error) {
	return nil, nil
}

func (m *mockExamRepo) CreateMockTestAttempt(
	_ context.Context, _ sqlc.CreateMockTestAttemptParams,
) (*sqlc.AssessExamAttempt, error) {
	return nil, nil
}

func (m *mockExamRepo) CountUserMockTestAttempts(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}

type fakeDrawer struct {
	sections []service.SectionActivities
}

func (f *fakeDrawer) DrawSitting(_ context.Context, _ uuid.UUID, _ string) ([]service.SectionActivities, error) {
	return f.sections, nil
}

type fakeJobEnqueuer struct{}

func (f *fakeJobEnqueuer) EnqueueTx(
	_ context.Context, _ pgx.Tx, _ river.JobArgs, _ *river.InsertOpts,
) (*rivertype.JobInsertResult, error) {
	return &rivertype.JobInsertResult{}, nil
}

type fakeExposures struct {
	mu   sync.Mutex
	seen []uuid.UUID
}

func (f *fakeExposures) RecordItemExposures(_ context.Context, _ pgx.Tx, _ uuid.UUID, ids []uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seen = append(f.seen, ids...)
	return nil
}

func (f *fakeExposures) ListItemExposures(
	_ context.Context, _ uuid.UUID, _ []uuid.UUID,
) (map[uuid.UUID]time.Time, error) {
	return nil, nil
}

// fakeLearning grades by kind: writing is asynchronous, anything named in fail
// errors, everything else scores full marks.
type fakeLearning struct {
	mu        sync.Mutex
	fail      map[uuid.UUID]bool
	submitted map[uuid.UUID]uuid.UUID // idempotency key -> attempt
	calls     int
	kinds     map[uuid.UUID]string
	outcomes  map[uuid.UUID]*learningcontract.AttemptOutcome
}

func newFakeLearning() *fakeLearning {
	return &fakeLearning{
		fail:      make(map[uuid.UUID]bool),
		submitted: make(map[uuid.UUID]uuid.UUID),
		kinds:     make(map[uuid.UUID]string),
		outcomes:  make(map[uuid.UUID]*learningcontract.AttemptOutcome),
	}
}

func (f *fakeLearning) SubmitSittingAnswer(
	_ context.Context, req learningcontract.SittingAnswerRequest,
) (*learningcontract.SittingAnswerResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.fail[req.ActivityID] {
		return nil, errors.New("grader refused")
	}
	if existing, ok := f.submitted[req.IdempotencyKey]; ok {
		out := f.outcomes[existing]
		return &learningcontract.SittingAnswerResult{AttemptID: existing, Status: out.Status, MaxScore: out.MaxScore}, nil
	}
	attemptID := uuid.New()
	f.submitted[req.IdempotencyKey] = attemptID
	if f.kinds[req.ActivityID] == "writing_prompt" {
		f.outcomes[attemptID] = &learningcontract.AttemptOutcome{AttemptID: attemptID, Status: "grading"}
		return &learningcontract.SittingAnswerResult{AttemptID: attemptID, Status: "grading", Async: true}, nil
	}
	score := 10
	f.outcomes[attemptID] = &learningcontract.AttemptOutcome{
		AttemptID: attemptID, Status: "graded", Score: &score, MaxScore: 10,
	}
	return &learningcontract.SittingAnswerResult{AttemptID: attemptID, Status: "graded", Score: 10, MaxScore: 10}, nil
}

func (f *fakeLearning) GetAttemptOutcome(_ context.Context, attemptID uuid.UUID) (
	*learningcontract.AttemptOutcome, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out, ok := f.outcomes[attemptID]
	if !ok {
		return nil, errors.New("no attempt")
	}
	copied := *out
	return &copied, nil
}

func (f *fakeLearning) settle(attemptID uuid.UUID, status string, score int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outcomes[attemptID] = &learningcontract.AttemptOutcome{
		AttemptID: attemptID, Status: status, Score: &score, MaxScore: 100,
	}
}

// ---------------------------------------------------------------------------
// fixture
// ---------------------------------------------------------------------------

type fixture struct {
	repo      *mockExamRepo
	clock     *clock.Fake
	learning  *fakeLearning
	exposures *fakeExposures
	svc       *service.Service
	examID    uuid.UUID
	sections  []service.SectionActivities
}

var start = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		repo:      newMockExamRepo(),
		clock:     clock.NewFake(start),
		learning:  newFakeLearning(),
		exposures: &fakeExposures{},
		examID:    uuid.New(),
	}
	f.repo.exams = []sqlc.AssessExam{
		{ID: f.examID, Slug: "mock-toeic-b1", TitleEn: "B1 mock", Level: "B1", TotalMinutes: 75},
	}

	kinds := []struct {
		skill string
		kind  string
	}{
		{testSkillListening, "listening_comprehension"},
		{"reading", "reading_comprehension"},
		{"writing", "writing_prompt"},
		{"speaking", "speaking_task"},
	}
	for i, k := range kinds {
		act := service.SittingActivityDTO{ID: uuid.New(), Kind: k.kind, ContentVersionID: uuid.New(), Weight: 1}
		f.learning.kinds[act.ID] = k.kind
		f.sections = append(f.sections, service.SectionActivities{
			SectionPosition: i + 1, Skill: k.skill, Activities: []service.SittingActivityDTO{act},
		})
	}

	f.svc = service.New(service.Deps{
		Repo:       f.repo,
		Learning:   f.learning,
		Attempts:   f.learning,
		Exposures:  f.exposures,
		Drawer:     &fakeDrawer{sections: f.sections},
		Clock:      f.clock,
		DailyLimit: 5,
		Enqueuer:   &fakeJobEnqueuer{},
	})
	return f
}

func (f *fixture) activity(section int) service.SittingActivityDTO {
	return f.sections[section-1].Activities[0]
}

func (f *fixture) start(t *testing.T, userID uuid.UUID, mode string) *service.ExamAttemptDTO {
	t.Helper()
	att, err := f.svc.StartSitting(
		context.Background(), userID, f.examID, service.StartAttemptRequest{Mode: mode, ChosenDurationMinutes: 60},
	)
	require.NoError(t, err)
	return att
}

func answer(activityID uuid.UUID, body string) map[string]json.RawMessage {
	return map[string]json.RawMessage{activityID.String(): json.RawMessage(body)}
}

// ---------------------------------------------------------------------------
// starting
// ---------------------------------------------------------------------------

func TestStartSitting_TheSixthSittingIsRefusedAndMarksNothingSeen(t *testing.T) {
	f := newFixture(t)
	f.repo.dailyCount = 5

	_, err := f.svc.StartSitting(
		context.Background(), uuid.New(), f.examID, service.StartAttemptRequest{Mode: domain.ModeExam},
	)

	require.ErrorIs(t, err, domain.ErrExamDailyLimitReached)
	assert.Empty(t, f.exposures.seen)
}

func TestStartSitting_ExamModeIsFixedAndMarksEveryItemSeen(t *testing.T) {
	f := newFixture(t)

	att := f.start(t, uuid.New(), domain.ModeExam)

	assert.Equal(t, 75, att.ChosenDurationMinutes)
	assert.Equal(t, start.Add(75*time.Minute), att.DeadlineAt)
	assert.Len(t, f.exposures.seen, 4)
	require.NotNil(t, att.SectionRemainingSeconds)
	assert.Equal(t, 20*60, *att.SectionRemainingSeconds)
}

func TestStartSitting_PracticeDurationIsClamped(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct{ asked, got int }{{5, 10}, {200, 180}} {
		att, err := f.svc.StartSitting(context.Background(), uuid.New(), f.examID, service.StartAttemptRequest{
			Mode: domain.ModePractice, ChosenDurationMinutes: tc.asked,
		})
		require.NoError(t, err)
		assert.Equal(t, tc.got, att.ChosenDurationMinutes)
		assert.False(t, att.Unlimited)
	}
}

func TestStartSitting_PracticeUnlimitedIgnoresChosenDurationAndBacksStopsAtADay(t *testing.T) {
	f := newFixture(t)

	att, err := f.svc.StartSitting(context.Background(), uuid.New(), f.examID, service.StartAttemptRequest{
		Mode: domain.ModePractice, Unlimited: true, ChosenDurationMinutes: 60,
	})

	require.NoError(t, err)
	assert.Equal(t, domain.UnlimitedPracticeDurationMinutes, att.ChosenDurationMinutes)
	assert.Equal(t, start.Add(domain.UnlimitedPracticeDurationMinutes*time.Minute), att.DeadlineAt)
	assert.True(t, att.Unlimited)
}

func TestStartSitting_ExamModeIgnoresUnlimited(t *testing.T) {
	f := newFixture(t)

	att, err := f.svc.StartSitting(context.Background(), uuid.New(), f.examID, service.StartAttemptRequest{
		Mode: domain.ModeExam, Unlimited: true,
	})

	require.NoError(t, err)
	assert.Equal(t, 75, att.ChosenDurationMinutes)
	assert.False(t, att.Unlimited)
}

// ---------------------------------------------------------------------------
// the clock
// ---------------------------------------------------------------------------

func TestAutosave_OneSecondBeforeTheDeadlineCountsOneSecondAfterIsRefused(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att := f.start(t, userID, domain.ModePractice)
	deadline := att.DeadlineAt

	f.clock.Set(deadline.Add(-time.Second))
	_, err := f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		Answers: answer(f.activity(1).ID, `{"answers":{"q1":"A"}}`),
	})
	require.NoError(t, err)

	f.clock.Set(deadline.Add(domain.NetworkGracePeriod + time.Second))
	_, err = f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		Answers: answer(f.activity(1).ID, `{"answers":{"q1":"B"}}`),
	})
	require.ErrorIs(t, err, domain.ErrAttemptExpired)
}

func TestExamMode_ASectionWhoseTimeRanOutIsClosedAndTheSittingMovesOn(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att := f.start(t, userID, domain.ModeExam)

	f.clock.Set(start.Add(21 * time.Minute))

	got, err := f.svc.GetExamAttempt(context.Background(), userID, att.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.CurrentSection)

	_, err = f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		Answers: answer(f.activity(1).ID, `{"answers":{"q1":"A"}}`),
	})
	require.ErrorIs(t, err, domain.ErrSectionAlreadyCompleted)

	// Finishing the section the clock already closed is not an error; it reports where the sitting is.
	res, err := f.svc.CompleteSection(context.Background(), userID, att.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, res.CurrentSection)
}

func TestExamMode_NoGoingBackWhateverSectionNumberTheClientSends(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att := f.start(t, userID, domain.ModeExam)

	_, err := f.svc.CompleteSection(context.Background(), userID, att.ID, 1)
	require.NoError(t, err)

	_, err = f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		Answers: answer(f.activity(1).ID, `{"answers":{"q1":"A"}}`),
	})
	require.ErrorIs(t, err, domain.ErrSectionAlreadyCompleted)

	_, err = f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		Answers: answer(f.activity(3).ID, `{"text_answer":"early"}`),
	})
	require.ErrorIs(t, err, domain.ErrInvalidSectionProgression)
}

func TestAutosave_RefusesItemsTheSittingDoesNotHoldAndOversizedAnswers(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att := f.start(t, userID, domain.ModePractice)

	_, err := f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		Answers: answer(uuid.New(), `"x"`),
	})
	require.ErrorIs(t, err, domain.ErrUnknownItem)

	huge := `"` + string(make([]byte, domain.MaxAnswerBytes)) + `"`
	_, err = f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		Answers: answer(f.activity(3).ID, huge),
	})
	require.ErrorIs(t, err, domain.ErrAnswerTooLarge)
}

func TestAutosave_RecordsOnlyKnownSignalsAtServerTime(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att := f.start(t, userID, domain.ModePractice)

	_, err := f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		IntegrityEvents: []service.IntegrityEventDTO{{Kind: domain.IntegrityPaste}, {Kind: "anything"}},
	})
	require.NoError(t, err)
	require.Len(t, f.repo.events, 1)
	assert.Equal(t, start, f.repo.events[0].OccurredAt)
}

func TestOtherLearnersCannotReachASitting(t *testing.T) {
	f := newFixture(t)
	att := f.start(t, uuid.New(), domain.ModeExam)

	_, err := f.svc.SubmitExam(context.Background(), uuid.New(), att.ID, domain.SubmittedByLearner)
	require.ErrorIs(t, err, domain.ErrAttemptNotFound)
}

// ---------------------------------------------------------------------------
// expiry
// ---------------------------------------------------------------------------

func TestExpiry_TheJobTheSweepAndTheReadEachSubmitAnAbandonedSitting(t *testing.T) {
	paths := map[string]func(f *fixture, userID, attemptID uuid.UUID){
		"scheduled job": func(f *fixture, _, attemptID uuid.UUID) {
			require.NoError(t, f.svc.ExpireAttempt(context.Background(), attemptID))
		},
		"sweep": func(f *fixture, _, _ uuid.UUID) {
			require.NoError(t, f.svc.SweepExpired(context.Background()))
		},
		"read": func(f *fixture, userID, attemptID uuid.UUID) {
			_, err := f.svc.GetExamAttempt(context.Background(), userID, attemptID)
			require.NoError(t, err)
		},
	}
	for name, run := range paths {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			userID := uuid.New()
			att := f.start(t, userID, domain.ModePractice)
			_, err := f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
				Answers: answer(f.activity(2).ID, `{"answers":{"q1":"A"}}`),
			})
			require.NoError(t, err)

			f.clock.Set(att.DeadlineAt.Add(time.Minute))
			run(f, userID, att.ID)

			stored, _ := f.repo.GetExamAttemptByID(context.Background(), att.ID)
			assert.Equal(t, domain.StatusExpired, stored.Status)
			_, hasReport := f.repo.reports[att.ID]
			assert.True(t, hasReport)
		})
	}
}

func TestExpiry_TwoPathsRacingProduceOneSubmission(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att := f.start(t, userID, domain.ModePractice)
	_, err := f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		Answers: answer(f.activity(2).ID, `{"answers":{"q1":"A"}}`),
	})
	require.NoError(t, err)
	f.clock.Set(att.DeadlineAt.Add(time.Minute))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = f.svc.ExpireAttempt(context.Background(), att.ID)
			_ = f.svc.SweepExpired(context.Background())
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, f.learning.calls, "one answered item, one submission")
}

func TestSubmit_ALearnerSubmissionPastTheDeadlineIsTheExpirys(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att := f.start(t, userID, domain.ModeExam)
	f.clock.Set(att.DeadlineAt.Add(time.Minute))

	res, err := f.svc.SubmitExam(context.Background(), userID, att.ID, domain.SubmittedByLearner)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusExpired, res.Status)
}

// ---------------------------------------------------------------------------
// the report
// ---------------------------------------------------------------------------

func submitPractice(
	t *testing.T, f *fixture, userID uuid.UUID, answers map[string]json.RawMessage,
) *service.ExamAttemptDTO {
	t.Helper()
	att := f.start(t, userID, domain.ModePractice)
	_, err := f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{Answers: answers})
	require.NoError(t, err)
	_, err = f.svc.SubmitExam(context.Background(), userID, att.ID, domain.SubmittedByLearner)
	require.NoError(t, err)
	return att
}

func TestReport_PendingUntilTheAsynchronousGradeArrivesThenReady(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	answers := map[string]json.RawMessage{
		f.activity(1).ID.String(): json.RawMessage(`{"answers":{"q1":"A"}}`),
		f.activity(2).ID.String(): json.RawMessage(`{"answers":{"q1":"A"}}`),
		f.activity(3).ID.String(): json.RawMessage(`{"text_answer":"an essay"}`),
		f.activity(4).ID.String(): json.RawMessage(`{"audio_object_key":"recordings/x/y.webm"}`),
	}
	att := submitPractice(t, f, userID, answers)

	report, err := f.svc.GetScoreReport(context.Background(), userID, att.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReportStatusPending, report.Status)
	assert.Equal(t, service.OfficialDisclaimer, report.Disclaimer)

	writing := report.PerSection[2].Items[0]
	require.NotNil(t, writing.AttemptID)
	f.learning.settle(*writing.AttemptID, "graded", 60)

	report, err = f.svc.GetScoreReport(context.Background(), userID, att.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReportStatusReady, report.Status)
	require.NotNil(t, report.PerSection[2].Score)
	assert.Equal(t, 60, *report.PerSection[2].Score)
	assert.InDelta(t, 90, report.OverallScore, 0.01) // (100+100+60+100)/4
}

func TestReport_AFailedGradeIsNotScoredAndTheReportPartial(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	f.learning.fail[f.activity(3).ID] = true
	att := submitPractice(t, f, userID, answer(f.activity(3).ID, `{"text_answer":"an essay"}`))

	report, err := f.svc.GetScoreReport(context.Background(), userID, att.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReportStatusPartial, report.Status)
	assert.Equal(t, domain.SectionNotScored, report.PerSection[2].Status)
	assert.Nil(t, report.PerSection[2].Score, "not scored is not zero")
	// Unanswered sections are scored, at zero.
	require.NotNil(t, report.PerSection[0].Score)
	assert.Equal(t, 0, *report.PerSection[0].Score)
}

func TestReport_StillPendingAfterAnHourBecomesPartial(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att := submitPractice(t, f, userID, answer(f.activity(3).ID, `{"text_answer":"an essay"}`))

	f.clock.Set(start.Add(2 * time.Hour))
	require.NoError(t, f.svc.SweepExpired(context.Background()))

	report, err := f.svc.GetScoreReport(context.Background(), userID, att.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReportStatusPartial, report.Status)
	assert.Equal(t, domain.SectionNotScored, report.PerSection[2].Status)
}

func TestReport_CountsIntegritySignals(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att := f.start(t, userID, domain.ModePractice)
	_, err := f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		IntegrityEvents: []service.IntegrityEventDTO{{Kind: domain.IntegrityTabHidden}, {Kind: domain.IntegrityTabHidden}},
	})
	require.NoError(t, err)
	_, err = f.svc.SubmitExam(context.Background(), userID, att.ID, domain.SubmittedByLearner)
	require.NoError(t, err)

	report, err := f.svc.GetScoreReport(context.Background(), userID, att.ID)
	require.NoError(t, err)
	require.Len(t, report.IntegritySignals, 1)
	assert.Equal(t, 2, report.IntegritySignals[0].Count)
}

func TestReport_ElapsedSecondsIsSubmittedAtMinusStartedAt(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att := f.start(t, userID, domain.ModePractice)

	f.clock.Set(start.Add(12*time.Minute + 34*time.Second))
	_, err := f.svc.SubmitExam(context.Background(), userID, att.ID, domain.SubmittedByLearner)
	require.NoError(t, err)

	report, err := f.svc.GetScoreReport(context.Background(), userID, att.ID)
	require.NoError(t, err)
	assert.Equal(t, 12*60+34, report.ElapsedSeconds)
}

// ---------------------------------------------------------------------------
// listening plays
// ---------------------------------------------------------------------------

func TestListeningPlayPolicy(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	listening := f.activity(1)

	exam := f.start(t, userID, domain.ModeExam)
	plays, err := f.svc.ListeningPlayPolicy(context.Background(), userID, exam.ID, listening.ContentVersionID)
	require.NoError(t, err)
	assert.Equal(t, 1, plays)

	_, err = f.svc.ListeningPlayPolicy(context.Background(), uuid.New(), exam.ID, listening.ContentVersionID)
	require.ErrorIs(t, err, domain.ErrPlayNotAllowed, "another learner's sitting")

	_, err = f.svc.ListeningPlayPolicy(context.Background(), userID, exam.ID, uuid.New())
	require.ErrorIs(t, err, domain.ErrPlayNotAllowed, "a clip the sitting does not hold")

	f.clock.Set(start.Add(21 * time.Minute))
	_, err = f.svc.ListeningPlayPolicy(context.Background(), userID, exam.ID, listening.ContentVersionID)
	require.ErrorIs(t, err, domain.ErrPlayNotAllowed, "the listening section is closed")

	_, err = f.svc.SubmitExam(context.Background(), userID, exam.ID, domain.SubmittedByLearner)
	require.NoError(t, err)
	f.clock.Set(start)
	practice := f.start(t, userID, domain.ModePractice)
	plays, err = f.svc.ListeningPlayPolicy(context.Background(), userID, practice.ID, listening.ContentVersionID)
	require.NoError(t, err)
	assert.Equal(t, 3, plays)
}

// TestListeningPlayPolicy_CoversTOEICPartsOneAndTwo. A photograph with spoken
// statements and a question with spoken responses are heard, not read, and
// carry the same one-play rule as a comprehension clip (WO 22 I.3.5).
func TestListeningPlayPolicy_CoversTOEICPartsOneAndTwo(t *testing.T) {
	for _, kind := range []string{"photo_description", "question_response"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			act := f.sections[0].Activities[0]
			act.Kind = kind
			f.learning.kinds[act.ID] = kind
			f.sections[0].Activities[0] = act

			userID := uuid.New()
			exam := f.start(t, userID, domain.ModeExam)
			plays, err := f.svc.ListeningPlayPolicy(
				context.Background(), userID, exam.ID, act.ContentVersionID)
			require.NoError(t, err)
			assert.Equal(t, 1, plays, "TOEIC %s is one play in exam mode", kind)

			_, err = f.svc.ListeningPlayPolicy(
				context.Background(), uuid.New(), exam.ID, act.ContentVersionID)
			require.ErrorIs(t, err, domain.ErrPlayNotAllowed, "another learner's sitting")
		})
	}
}
