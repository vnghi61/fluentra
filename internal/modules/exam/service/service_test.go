package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/exam/sqlc"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	"github.com/fluentra/fluentra/internal/modules/exam/service"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type mockExamRepo struct {
	exams           []sqlc.AssessExam
	sections        []sqlc.AssessExamSection
	attempts        map[uuid.UUID]sqlc.AssessExamAttempt
	reports         map[uuid.UUID]sqlc.AssessScoreReport
	dailyCount      int64
	saveAnswersErr  error
	updateStatusErr error
}

func newMockExamRepo() *mockExamRepo {
	return &mockExamRepo{
		attempts: make(map[uuid.UUID]sqlc.AssessExamAttempt),
		reports:  make(map[uuid.UUID]sqlc.AssessScoreReport),
	}
}

func (m *mockExamRepo) ListExams(_ context.Context) ([]sqlc.AssessExam, error) {
	return m.exams, nil
}

func (m *mockExamRepo) GetExamByID(_ context.Context, id uuid.UUID) (*sqlc.AssessExam, error) {
	for _, e := range m.exams {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, domain.ErrExamNotFound
}

func (m *mockExamRepo) GetExamBySlug(_ context.Context, slug string) (*sqlc.AssessExam, error) {
	for _, e := range m.exams {
		if e.Slug == slug {
			return &e, nil
		}
	}
	return nil, domain.ErrExamNotFound
}

func (m *mockExamRepo) ListExamSections(_ context.Context, examID uuid.UUID) ([]sqlc.AssessExamSection, error) {
	var res []sqlc.AssessExamSection
	for _, s := range m.sections {
		if s.ExamID == examID {
			res = append(res, s)
		}
	}
	return res, nil
}

func (m *mockExamRepo) CreateExamAttempt(_ context.Context, arg sqlc.CreateExamAttemptParams) (*sqlc.AssessExamAttempt, error) {
	att := sqlc.AssessExamAttempt{
		ID:                    arg.ID,
		ExamID:                arg.ExamID,
		UserID:                arg.UserID,
		Mode:                  arg.Mode,
		ChosenDurationMinutes: arg.ChosenDurationMinutes,
		StartedAt:             arg.StartedAt,
		DeadlineAt:            arg.DeadlineAt,
		Status:                "in_progress",
		CurrentSection:        1,
		SectionActivities:     arg.SectionActivities,
		DraftAnswers:          arg.DraftAnswers,
		CreatedAt:             arg.StartedAt,
		UpdatedAt:             arg.StartedAt,
	}
	m.attempts[arg.ID] = att
	return &att, nil
}

func (m *mockExamRepo) GetExamAttemptByID(_ context.Context, id uuid.UUID) (*sqlc.AssessExamAttempt, error) {
	att, ok := m.attempts[id]
	if !ok {
		return nil, domain.ErrAttemptNotFound
	}
	return &att, nil
}

func (m *mockExamRepo) GetExamAttemptForUser(_ context.Context, id, userID uuid.UUID) (*sqlc.AssessExamAttempt, error) {
	att, ok := m.attempts[id]
	if !ok {
		return nil, domain.ErrAttemptNotFound
	}
	if att.UserID != userID {
		return nil, domain.ErrUnauthorizedAttempt
	}
	return &att, nil
}

func (m *mockExamRepo) CountUserActiveAttempts(_ context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	for _, att := range m.attempts {
		if att.UserID == userID && att.Status == "in_progress" {
			count++
		}
	}
	return count, nil
}

func (m *mockExamRepo) CountUserAttemptsToday(_ context.Context, _ uuid.UUID, _, _ time.Time) (int64, error) {
	return m.dailyCount, nil
}

func (m *mockExamRepo) UpdateDraftAnswers(_ context.Context, id uuid.UUID, draftAnswers []byte, updatedAt time.Time) (*sqlc.AssessExamAttempt, error) {
	if m.saveAnswersErr != nil {
		return nil, m.saveAnswersErr
	}
	att, ok := m.attempts[id]
	if !ok {
		return nil, domain.ErrAttemptNotFound
	}
	att.DraftAnswers = draftAnswers
	att.UpdatedAt = updatedAt
	m.attempts[id] = att
	return &att, nil
}

func (m *mockExamRepo) UpdateCurrentSection(_ context.Context, id uuid.UUID, section int32, updatedAt time.Time) (*sqlc.AssessExamAttempt, error) {
	att, ok := m.attempts[id]
	if !ok {
		return nil, domain.ErrAttemptNotFound
	}
	att.CurrentSection = section
	att.UpdatedAt = updatedAt
	m.attempts[id] = att
	return &att, nil
}

func (m *mockExamRepo) MarkAttemptCompleted(_ context.Context, id uuid.UUID, submittedAt time.Time, submittedBy string) (*sqlc.AssessExamAttempt, error) {
	if m.updateStatusErr != nil {
		return nil, m.updateStatusErr
	}
	att, ok := m.attempts[id]
	if !ok {
		return nil, domain.ErrAttemptNotFound
	}
	att.Status = "completed"
	att.SubmittedAt = &submittedAt
	att.SubmittedBy = &submittedBy
	att.UpdatedAt = submittedAt
	m.attempts[id] = att
	return &att, nil
}

func (m *mockExamRepo) MarkAttemptExpired(_ context.Context, id uuid.UUID, submittedAt time.Time) (*sqlc.AssessExamAttempt, error) {
	if m.updateStatusErr != nil {
		return nil, m.updateStatusErr
	}
	att, ok := m.attempts[id]
	if !ok {
		return nil, domain.ErrAttemptNotFound
	}
	subBy := "system:expired"
	att.Status = "expired"
	att.SubmittedAt = &submittedAt
	att.SubmittedBy = &subBy
	att.UpdatedAt = submittedAt
	m.attempts[id] = att
	return &att, nil
}

func (m *mockExamRepo) ListExpiredInProgressAttempts(_ context.Context, now time.Time) ([]sqlc.AssessExamAttempt, error) {
	var expired []sqlc.AssessExamAttempt
	for _, att := range m.attempts {
		if att.Status == "in_progress" && att.DeadlineAt.Before(now) {
			expired = append(expired, att)
		}
	}
	return expired, nil
}

func (m *mockExamRepo) RecordIntegrityEvent(_ context.Context, _ sqlc.RecordIntegrityEventParams) error {
	return nil
}

func (m *mockExamRepo) ListIntegrityEvents(_ context.Context, _ uuid.UUID) ([]sqlc.AssessIntegrityEvent, error) {
	return nil, nil
}

func (m *mockExamRepo) CreateScoreReport(_ context.Context, arg sqlc.CreateScoreReportParams) (*sqlc.AssessScoreReport, error) {
	rep := sqlc.AssessScoreReport{
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

func (m *mockExamRepo) GetScoreReportByAttemptID(_ context.Context, attemptID uuid.UUID) (*sqlc.AssessScoreReport, error) {
	rep, ok := m.reports[attemptID]
	if !ok {
		return nil, domain.ErrReportNotReady
	}
	return &rep, nil
}

func (m *mockExamRepo) UpdateScoreReport(_ context.Context, arg sqlc.UpdateScoreReportParams) (*sqlc.AssessScoreReport, error) {
	rep, ok := m.reports[arg.AttemptID]
	if !ok {
		return nil, domain.ErrReportNotReady
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

func (m *mockExamRepo) ListUserExamAttempts(_ context.Context, userID uuid.UUID, _, _ int32) ([]sqlc.AssessExamAttempt, error) {
	var res []sqlc.AssessExamAttempt
	for _, att := range m.attempts {
		if att.UserID == userID {
			res = append(res, att)
		}
	}
	return res, nil
}

func (m *mockExamRepo) CountUserExamAttempts(_ context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	for _, att := range m.attempts {
		if att.UserID == userID {
			count++
		}
	}
	return count, nil
}

type fakeDrawer struct {
	sections []service.SectionActivities
}

func (f *fakeDrawer) DrawSitting(_ context.Context, _ uuid.UUID, _ string) ([]service.SectionActivities, error) {
	return f.sections, nil
}

type fakeJobEnqueuer struct{}

func (f *fakeJobEnqueuer) EnqueueTx(_ context.Context, _ pgx.Tx, _ river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	return &rivertype.JobInsertResult{}, nil
}

func setupTestService(repo *mockExamRepo, now time.Time) (*service.Service, uuid.UUID) {
	examID := uuid.New()
	repo.exams = []sqlc.AssessExam{
		{
			ID:           examID,
			Slug:         "a2-full",
			TitleEn:      "A2 Full Mock Exam",
			TitleVi:      "Bài thi thử A2",
			Level:        "A2",
			Format:       "full",
			TotalMinutes: 75,
		},
	}
	repo.sections = []sqlc.AssessExamSection{
		{ID: uuid.New(), ExamID: examID, Position: 1, Skill: "listening", ExamDurationMinutes: 20, ItemCount: 10},
		{ID: uuid.New(), ExamID: examID, Position: 2, Skill: "reading", ExamDurationMinutes: 25, ItemCount: 10},
		{ID: uuid.New(), ExamID: examID, Position: 3, Skill: "writing", ExamDurationMinutes: 20, ItemCount: 2},
		{ID: uuid.New(), ExamID: examID, Position: 4, Skill: "speaking", ExamDurationMinutes: 10, ItemCount: 2},
	}

	drawer := &fakeDrawer{
		sections: []service.SectionActivities{
			{SectionPosition: 1, Skill: "listening", Activities: []service.SittingActivityDTO{{ID: uuid.New(), Kind: "listening_choice", Weight: 10}}},
			{SectionPosition: 2, Skill: "reading", Activities: []service.SittingActivityDTO{{ID: uuid.New(), Kind: "reading_choice", Weight: 10}}},
			{SectionPosition: 3, Skill: "writing", Activities: []service.SittingActivityDTO{{ID: uuid.New(), Kind: "writing_prompt", Weight: 10}}},
			{SectionPosition: 4, Skill: "speaking", Activities: []service.SittingActivityDTO{{ID: uuid.New(), Kind: "speaking_task", Weight: 10}}},
		},
	}

	svc := service.New(service.Deps{
		Repo:       repo,
		Drawer:     drawer,
		Clock:      clock.NewFake(now),
		DailyLimit: 5,
		Enqueuer:   &fakeJobEnqueuer{},
	})
	return svc, examID
}

func TestStartSitting_DailyLimitEnforced(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	repo := newMockExamRepo()
	repo.dailyCount = 5 // User already had 5 sittings today
	svc, examID := setupTestService(repo, now)

	userID := uuid.New()
	_, err := svc.StartSitting(context.Background(), userID, examID, service.StartAttemptRequest{
		Mode: "exam",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrExamDailyLimitReached)
}

func TestStartSitting_ExamMode_FixedDuration(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	repo := newMockExamRepo()
	svc, examID := setupTestService(repo, now)

	userID := uuid.New()
	attempt, err := svc.StartSitting(context.Background(), userID, examID, service.StartAttemptRequest{
		Mode:                  "exam",
		ChosenDurationMinutes: 120, // Requesting 120, but exam mode MUST be fixed to 75
	})
	require.NoError(t, err)
	assert.Equal(t, 75, attempt.ChosenDurationMinutes)
	assert.Equal(t, "exam", attempt.Mode)
	assert.Equal(t, 4500, attempt.RemainingSeconds)
	assert.Equal(t, now.Add(75*time.Minute), attempt.DeadlineAt)
}

func TestStartSitting_PracticeMode_Clamping(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	repo := newMockExamRepo()
	svc, examID := setupTestService(repo, now)
	userID := uuid.New()

	// Under min (5 min -> 10 min)
	att1, err := svc.StartSitting(context.Background(), userID, examID, service.StartAttemptRequest{
		Mode:                  "practice",
		ChosenDurationMinutes: 5,
	})
	require.NoError(t, err)
	assert.Equal(t, 10, att1.ChosenDurationMinutes)

	// Over max (200 min -> 180 min)
	userID2 := uuid.New()
	att2, err := svc.StartSitting(context.Background(), userID2, examID, service.StartAttemptRequest{
		Mode:                  "practice",
		ChosenDurationMinutes: 200,
	})
	require.NoError(t, err)
	assert.Equal(t, 180, att2.ChosenDurationMinutes)
}

func TestAutosaveAnswers_ForwardOnlyAndCompletedCheck(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	repo := newMockExamRepo()
	svc, examID := setupTestService(repo, now)
	userID := uuid.New()

	att, err := svc.StartSitting(context.Background(), userID, examID, service.StartAttemptRequest{
		Mode: "exam",
	})
	require.NoError(t, err)

	// Section 2 while CurrentSection is 1 in Exam mode should fail
	_, err = svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		SectionNumber: 2,
		Answers:       map[string]any{"q1": "val"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidSectionProgression)

	// Section 1 should succeed
	res, err := svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		SectionNumber: 1,
		Answers:       map[string]any{"q1": "val"},
	})
	require.NoError(t, err)
	assert.True(t, res.Saved)

	// Complete section 1 -> advances to section 2
	compRes, err := svc.CompleteSection(context.Background(), userID, att.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, compRes.CurrentSection)

	// Trying to go back to save section 1 should fail in Exam mode
	_, err = svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		SectionNumber: 1,
		Answers:       map[string]any{"q1": "val"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrSectionAlreadyCompleted)
}

func TestGetExamAttempt_LazyExpiry(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	repo := newMockExamRepo()
	svc, examID := setupTestService(repo, now)
	userID := uuid.New()

	att, err := svc.StartSitting(context.Background(), userID, examID, service.StartAttemptRequest{
		Mode: "exam",
	})
	require.NoError(t, err)

	// Advance time past deadline + grace period
	expiredNow := now.Add(75*time.Minute + 10*time.Second)
	svcExpired := service.New(service.Deps{
		Repo:       repo,
		Clock:      clock.NewFake(expiredNow),
		DailyLimit: 5,
		Enqueuer:   &fakeJobEnqueuer{},
	})

	got, err := svcExpired.GetExamAttempt(context.Background(), userID, att.ID)
	require.NoError(t, err)
	// Status was transitioned to expired
	assert.Equal(t, "expired", got.Status)
	assert.Equal(t, 0, got.RemainingSeconds)
}

func TestScoreReport_OverallCalculationAndDisclaimer(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	repo := newMockExamRepo()
	svc, examID := setupTestService(repo, now)
	userID := uuid.New()

	attemptID := uuid.New()
	var scoreNumeric pgtype.Numeric
	_ = scoreNumeric.Scan("78.50")

	repo.reports[attemptID] = sqlc.AssessScoreReport{
		AttemptID:        attemptID,
		UserID:           userID,
		ExamID:           examID,
		OverallScore:     scoreNumeric,
		OverallBand:      "B2",
		Status:           "completed",
		PerSection:       []byte(`[{"skill":"listening","score":80,"percentage":80,"band":"B2"}]`),
		Feedback:         []byte(`{}`),
		IntegritySignals: []byte(`[]`),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	report, err := svc.GetScoreReport(context.Background(), userID, attemptID)
	require.NoError(t, err)
	assert.Equal(t, "B2", report.OverallBand)
	assert.Equal(t, 78.50, report.OverallScore)
	assert.Equal(t, "completed", report.Status)
	assert.Equal(t, service.OfficialDisclaimer, report.Disclaimer)
}
