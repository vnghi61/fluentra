package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/fluentra/fluentra/internal/generated/exam/sqlc"
	examcontract "github.com/fluentra/fluentra/internal/modules/exam/contract"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	examjob "github.com/fluentra/fluentra/internal/modules/exam/job"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	platformjob "github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

const (
	DefaultDailySittingsLimit = 5
	HoChiMinhTimeZone         = "Asia/Ho_Chi_Minh"
	ExamPoolCourseSlug        = "pool-exam"
	OfficialDisclaimer        = "Not an official TOEIC score. Pronunciation not assessed."
)

// WorkerNudger signals a background worker to wake up after a job is enqueued.
type WorkerNudger interface {
	Nudge(ctx context.Context)
}

// PoolDrawer draws items for an exam sitting across the 4 sections.
type PoolDrawer interface {
	DrawSitting(ctx context.Context, userID uuid.UUID, level string) ([]SectionActivities, error)
}

// SectionActivities represents drawn activities for a section.
type SectionActivities struct {
	SectionPosition int                  `json:"section_position"`
	Skill           string               `json:"skill"`
	Activities      []SittingActivityDTO `json:"activities"`
}

// SittingActivityDTO represents an activity inside a sitting.
type SittingActivityDTO struct {
	ID               uuid.UUID       `json:"id"`
	Kind             string          `json:"kind"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	Config           json.RawMessage `json:"config,omitempty"`
	Weight           int             `json:"weight"`
}

// ExamRepository defines persistent operations required by the exam service.
type ExamRepository interface {
	ListExams(ctx context.Context) ([]sqlc.AssessExam, error)
	GetExamByID(ctx context.Context, id uuid.UUID) (*sqlc.AssessExam, error)
	GetExamBySlug(ctx context.Context, slug string) (*sqlc.AssessExam, error)
	ListExamSections(ctx context.Context, examID uuid.UUID) ([]sqlc.AssessExamSection, error)
	CreateExamAttempt(ctx context.Context, arg sqlc.CreateExamAttemptParams) (*sqlc.AssessExamAttempt, error)
	GetExamAttemptByID(ctx context.Context, id uuid.UUID) (*sqlc.AssessExamAttempt, error)
	GetExamAttemptForUser(ctx context.Context, id, userID uuid.UUID) (*sqlc.AssessExamAttempt, error)
	ListUserExamAttempts(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]sqlc.AssessExamAttempt, error)
	CountUserExamAttempts(ctx context.Context, userID uuid.UUID) (int64, error)
	CountUserActiveAttempts(ctx context.Context, userID uuid.UUID) (int64, error)
	CountUserAttemptsToday(ctx context.Context, userID uuid.UUID, start, end time.Time) (int64, error)
	UpdateDraftAnswers(ctx context.Context, id uuid.UUID, draftAnswers []byte, updatedAt time.Time) (*sqlc.AssessExamAttempt, error)
	UpdateCurrentSection(ctx context.Context, id uuid.UUID, section int32, updatedAt time.Time) (*sqlc.AssessExamAttempt, error)
	MarkAttemptCompleted(ctx context.Context, id uuid.UUID, submittedAt time.Time, submittedBy string) (*sqlc.AssessExamAttempt, error)
	MarkAttemptExpired(ctx context.Context, id uuid.UUID, submittedAt time.Time) (*sqlc.AssessExamAttempt, error)
	ListExpiredInProgressAttempts(ctx context.Context, now time.Time) ([]sqlc.AssessExamAttempt, error)
	CreateScoreReport(ctx context.Context, arg sqlc.CreateScoreReportParams) (*sqlc.AssessScoreReport, error)
	GetScoreReportByAttemptID(ctx context.Context, attemptID uuid.UUID) (*sqlc.AssessScoreReport, error)
	UpdateScoreReport(ctx context.Context, arg sqlc.UpdateScoreReportParams) (*sqlc.AssessScoreReport, error)
	RecordIntegrityEvent(ctx context.Context, arg sqlc.RecordIntegrityEventParams) error
	ListIntegrityEvents(ctx context.Context, attemptID uuid.UUID) ([]sqlc.AssessIntegrityEvent, error)
}

// Deps defines dependencies for the exam service.
type Deps struct {
	Pool        *pgxpool.Pool
	Repo        ExamRepository
	Learning    learningcontract.SittingAnswerSubmitter
	Exposures   learningcontract.ItemExposureRecorder
	Lesson      lessoncontract.Reader
	Drawer      PoolDrawer
	Clock       clock.Clock
	DailyLimit  int
	Enqueuer    platformjob.Enqueuer
	Nudger      WorkerNudger
}

// Service orchestrates exam sittings, timing, auto-submission, and scoring.
type Service struct {
	pool       *pgxpool.Pool
	repo       ExamRepository
	learning   learningcontract.SittingAnswerSubmitter
	exposures  learningcontract.ItemExposureRecorder
	lesson     lessoncontract.Reader
	drawer     PoolDrawer
	clock      clock.Clock
	dailyLimit int
	enqueuer   platformjob.Enqueuer
	nudger     WorkerNudger
}

// New constructs an exam service.
func New(deps Deps) *Service {
	dailyLimit := deps.DailyLimit
	if dailyLimit <= 0 {
		dailyLimit = DefaultDailySittingsLimit
	}
	clk := deps.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	return &Service{
		pool:       deps.Pool,
		repo:       deps.Repo,
		learning:   deps.Learning,
		exposures:  deps.Exposures,
		lesson:     deps.Lesson,
		drawer:     deps.Drawer,
		clock:      clk,
		dailyLimit: dailyLimit,
		enqueuer:   deps.Enqueuer,
		nudger:     deps.Nudger,
	}
}

// ExamDTO describes an available exam template.
type ExamDTO struct {
	ID            uuid.UUID        `json:"id"`
	Slug          string           `json:"slug"`
	TitleEn       string           `json:"title_en"`
	TitleVi       string           `json:"title_vi"`
	DescriptionEn string           `json:"description_en"`
	DescriptionVi string           `json:"description_vi"`
	Level         string           `json:"level"`
	Format        string           `json:"format"`
	TotalMinutes  int              `json:"total_minutes"`
	Sections      []ExamSectionDTO `json:"sections,omitempty"`
}

// ExamSectionDTO describes a section within an exam template.
type ExamSectionDTO struct {
	ID                  uuid.UUID `json:"id"`
	ExamID              uuid.UUID `json:"exam_id"`
	Position            int       `json:"position"`
	Skill               string    `json:"skill"`
	ExamDurationMinutes int       `json:"exam_duration_minutes"`
	ItemCount           int       `json:"item_count"`
	ItemKinds           []string  `json:"item_kinds"`
}

// StartAttemptRequest parameters.
type StartAttemptRequest struct {
	Mode                  string `json:"mode"`
	ChosenDurationMinutes int    `json:"chosen_duration_minutes,omitempty"`
}

// ExamAttemptDTO describes an exam attempt sitting.
type ExamAttemptDTO struct {
	ID                    uuid.UUID           `json:"id"`
	ExamID                uuid.UUID           `json:"exam_id"`
	ExamSlug              string              `json:"exam_slug,omitempty"`
	ExamTitle             string              `json:"exam_title,omitempty"`
	Mode                  string              `json:"mode"`
	ChosenDurationMinutes int                 `json:"chosen_duration_minutes"`
	StartedAt             time.Time           `json:"started_at"`
	DeadlineAt            time.Time           `json:"deadline_at"`
	RemainingSeconds      int                 `json:"remaining_seconds"`
	CurrentSection        int                 `json:"current_section"`
	Status                string              `json:"status"`
	SectionActivities     []SectionActivities `json:"section_activities,omitempty"`
	DraftAnswers          map[string]any      `json:"draft_answers,omitempty"`
	ServerTime            time.Time           `json:"server_time"`
}

// SaveAnswersRequest parameters.
type SaveAnswersRequest struct {
	SectionNumber   int                 `json:"section_number,omitempty"`
	Answers         map[string]any      `json:"answers"`
	IntegrityEvents []IntegrityEventDTO `json:"integrity_events,omitempty"`
}

// IntegrityEventDTO parameters.
type IntegrityEventDTO struct {
	Kind       string         `json:"kind"`
	OccurredAt time.Time      `json:"occurred_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// SaveAnswersResult response.
type SaveAnswersResult struct {
	Saved            bool `json:"saved"`
	RemainingSeconds int  `json:"remaining_seconds"`
}

// CompleteSectionResult response.
type CompleteSectionResult struct {
	CurrentSection   int `json:"current_section"`
	RemainingSeconds int `json:"remaining_seconds"`
}

// SubmitExamResult response.
type SubmitExamResult struct {
	AttemptID    uuid.UUID `json:"attempt_id"`
	Status       string    `json:"status"`
	ReportStatus string    `json:"report_status"`
}

// ScoreReportDTO response.
type ScoreReportDTO struct {
	AttemptID        uuid.UUID        `json:"attempt_id"`
	Status           string           `json:"status"`
	OverallScore     float64          `json:"overall_score"`
	OverallBand      string           `json:"overall_band"`
	PerSection       []SectionScoreDTO `json:"per_section"`
	Feedback         map[string]any   `json:"feedback,omitempty"`
	IntegritySignals []map[string]any `json:"integrity_signals,omitempty"`
	Disclaimer       string           `json:"disclaimer"`
}

// SectionScoreDTO score breakdown per skill.
type SectionScoreDTO struct {
	Skill       string `json:"skill"`
	Score       int    `json:"score"`
	MaxScore    int    `json:"max_score"`
	Band        string `json:"band,omitempty"`
	Status      string `json:"status,omitempty"` // "scored", "not_scored", "pending"
	ItemResults any    `json:"item_results,omitempty"`
}

// ListExams returns all available active exam templates.
func (s *Service) ListExams(ctx context.Context) ([]ExamDTO, error) {
	if s.repo == nil {
		return nil, nil
	}
	rows, err := s.repo.ListExams(ctx)
	if err != nil {
		return nil, err
	}
	exams := make([]ExamDTO, len(rows))
	for i, r := range rows {
		exams[i] = ExamDTO{
			ID:            r.ID,
			Slug:          r.Slug,
			TitleEn:       r.TitleEn,
			TitleVi:       r.TitleVi,
			DescriptionEn: r.DescriptionEn,
			DescriptionVi: r.DescriptionVi,
			Level:         r.Level,
			Format:        r.Format,
			TotalMinutes:  int(r.TotalMinutes),
		}
	}
	return exams, nil
}

// GetExam returns an exam template by ID with its sections.
func (s *Service) GetExam(ctx context.Context, id uuid.UUID) (*ExamDTO, error) {
	if s.repo == nil {
		return nil, domain.ErrExamNotFound
	}
	exam, err := s.repo.GetExamByID(ctx, id)
	if err != nil {
		return nil, domain.ErrExamNotFound
	}
	secRows, err := s.repo.ListExamSections(ctx, id)
	if err != nil {
		return nil, err
	}
	sections := make([]ExamSectionDTO, len(secRows))
	for i, sec := range secRows {
		var kinds []string
		_ = json.Unmarshal(sec.ItemKinds, &kinds)
		sections[i] = ExamSectionDTO{
			ID:                  sec.ID,
			ExamID:              sec.ExamID,
			Position:            int(sec.Position),
			Skill:               sec.Skill,
			ExamDurationMinutes: int(sec.ExamDurationMinutes),
			ItemCount:           int(sec.ItemCount),
			ItemKinds:           kinds,
		}
	}
	return &ExamDTO{
		ID:            exam.ID,
		Slug:          exam.Slug,
		TitleEn:       exam.TitleEn,
		TitleVi:       exam.TitleVi,
		DescriptionEn: exam.DescriptionEn,
		DescriptionVi: exam.DescriptionVi,
		Level:         exam.Level,
		Format:        exam.Format,
		TotalMinutes:  int(exam.TotalMinutes),
		Sections:      sections,
	}, nil
}

// StartSitting begins a new exam attempt sitting.
func (s *Service) StartSitting(
	ctx context.Context, userID, examID uuid.UUID, req StartAttemptRequest,
) (*ExamAttemptDTO, error) {
	if s.repo == nil {
		return nil, errors.New("repository not configured")
	}

	exam, err := s.repo.GetExamByID(ctx, examID)
	if err != nil {
		return nil, domain.ErrExamNotFound
	}

	// 1. Invariant: one active attempt in progress per learner (BR-EXAM-04)
	activeCount, err := s.repo.CountUserActiveAttempts(ctx, userID)
	if err != nil {
		return nil, err
	}
	if activeCount > 0 {
		return nil, domain.ErrAttemptInProgress
	}

	// 2. Daily sitting limit per learner in Asia/Ho_Chi_Minh
	now := s.clock.Now().UTC()
	loc, lErr := time.LoadLocation(HoChiMinhTimeZone)
	if lErr != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	nowLocal := now.In(loc)
	startOfDay := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc).UTC()
	endOfDay := startOfDay.Add(24 * time.Hour)

	todayCount, err := s.repo.CountUserAttemptsToday(ctx, userID, startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}
	if todayCount >= int64(s.dailyLimit) {
		return nil, domain.ErrExamDailyLimitReached
	}

	// 3. Draw activities for the sitting
	var drawn []SectionActivities
	if s.drawer != nil {
		var dErr error
		drawn, dErr = s.drawer.DrawSitting(ctx, userID, exam.Level)
		if dErr != nil {
			return nil, dErr
		}
	}
	if len(drawn) == 0 {
		return nil, domain.ErrInsufficientItems
	}

	// 4. Calculate timing
	duration := int(exam.TotalMinutes)
	if req.Mode == domain.ModePractice {
		duration = domain.ClampPracticeDuration(req.ChosenDurationMinutes)
	}
	deadlineAt := now.Add(time.Duration(duration) * time.Minute)

	drawnBytes, err := json.Marshal(drawn)
	if err != nil {
		return nil, fmt.Errorf("serialize section activities: %w", err)
	}

	attemptID := uuid.New()
	mode := req.Mode
	if mode != domain.ModePractice {
		mode = domain.ModeExam
	}

	// Extract all activity IDs to record exposure in one transaction
	var allActivityIDs []uuid.UUID
	for _, sec := range drawn {
		for _, act := range sec.Activities {
			allActivityIDs = append(allActivityIDs, act.ID)
		}
	}

	var attempt *sqlc.AssessExamAttempt
	if s.pool != nil {
		txErr := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
			created, cErr := s.repo.CreateExamAttempt(txCtx, sqlc.CreateExamAttemptParams{
				ID:                    attemptID,
				UserID:                userID,
				ExamID:                examID,
				Mode:                  mode,
				ChosenDurationMinutes: int32(duration),
				StartedAt:             now,
				DeadlineAt:            deadlineAt,
				CurrentSection:        1,
				Status:                domain.StatusInProgress,
				SectionActivities:     drawnBytes,
				DraftAnswers:          []byte("{}"),
			})
			if cErr != nil {
				return cErr
			}
			attempt = created

			if s.exposures != nil && len(allActivityIDs) > 0 {
				if expErr := s.exposures.RecordItemExposures(txCtx, userID, allActivityIDs); expErr != nil {
					return fmt.Errorf("record item exposures: %w", expErr)
				}
			}

			// Enqueue scheduled River job at deadline
			if s.enqueuer != nil {
				args := examjob.ExpireAttemptArgs{AttemptID: attemptID}
				opts := &river.InsertOpts{ScheduledAt: deadlineAt}
				if _, qErr := s.enqueuer.EnqueueTx(txCtx, tx, args, opts); qErr != nil {
					slog.WarnContext(txCtx, "could not schedule river expire job", "error", qErr)
				}
			}
			return nil
		})
		if txErr != nil {
			return nil, txErr
		}
	} else {
		created, cErr := s.repo.CreateExamAttempt(ctx, sqlc.CreateExamAttemptParams{
			ID:                    attemptID,
			UserID:                userID,
			ExamID:                examID,
			Mode:                  mode,
			ChosenDurationMinutes: int32(duration),
			StartedAt:             now,
			DeadlineAt:            deadlineAt,
			CurrentSection:        1,
			Status:                domain.StatusInProgress,
			SectionActivities:     drawnBytes,
			DraftAnswers:          []byte("{}"),
		})
		if cErr != nil {
			return nil, cErr
		}
		attempt = created
		if s.exposures != nil && len(allActivityIDs) > 0 {
			_ = s.exposures.RecordItemExposures(ctx, userID, allActivityIDs)
		}
	}

	return &ExamAttemptDTO{
		ID:                    attempt.ID,
		ExamID:                attempt.ExamID,
		ExamSlug:              exam.Slug,
		ExamTitle:             exam.TitleEn,
		Mode:                  attempt.Mode,
		ChosenDurationMinutes: int(attempt.ChosenDurationMinutes),
		StartedAt:             attempt.StartedAt,
		DeadlineAt:            attempt.DeadlineAt,
		RemainingSeconds:      domain.RemainingSeconds(attempt.DeadlineAt, now),
		CurrentSection:        int(attempt.CurrentSection),
		Status:                attempt.Status,
		SectionActivities:     drawn,
		DraftAnswers:          map[string]any{},
		ServerTime:            now,
	}, nil
}

// GetExamAttempt retrieves current attempt state, enforcing lazy auto-expiry if overdue.
func (s *Service) GetExamAttempt(ctx context.Context, userID, attemptID uuid.UUID) (*ExamAttemptDTO, error) {
	if s.repo == nil {
		return nil, domain.ErrAttemptNotFound
	}
	attempt, err := s.repo.GetExamAttemptForUser(ctx, attemptID, userID)
	if err != nil {
		return nil, domain.ErrAttemptNotFound
	}

	now := s.clock.Now().UTC()

	// Tier 3: Lazy expiry upon reading an attempt past its deadline (WO12 §3.6)
	if attempt.Status == domain.StatusInProgress && domain.IsPastDeadline(attempt.DeadlineAt, now) {
		slog.InfoContext(ctx, "lazy expiring overdue attempt on read", "attempt_id", attemptID)
		_, _ = s.SubmitExam(ctx, userID, attemptID, domain.SubmittedByExpiry)
		if refreshed, rErr := s.repo.GetExamAttemptForUser(ctx, attemptID, userID); rErr == nil {
			attempt = refreshed
		}
	}

	var drawn []SectionActivities
	_ = json.Unmarshal(attempt.SectionActivities, &drawn)
	var answers map[string]any
	_ = json.Unmarshal(attempt.DraftAnswers, &answers)

	var examSlug, examTitle string
	if exam, eErr := s.repo.GetExamByID(ctx, attempt.ExamID); eErr == nil {
		examSlug = exam.Slug
		examTitle = exam.TitleEn
	}

	return &ExamAttemptDTO{
		ID:                    attempt.ID,
		ExamID:                attempt.ExamID,
		ExamSlug:              examSlug,
		ExamTitle:             examTitle,
		Mode:                  attempt.Mode,
		ChosenDurationMinutes: int(attempt.ChosenDurationMinutes),
		StartedAt:             attempt.StartedAt,
		DeadlineAt:            attempt.DeadlineAt,
		RemainingSeconds:      domain.RemainingSeconds(attempt.DeadlineAt, now),
		CurrentSection:        int(attempt.CurrentSection),
		Status:                attempt.Status,
		SectionActivities:     drawn,
		DraftAnswers:          answers,
		ServerTime:            now,
	}, nil
}

// AutosaveAnswers saves draft answers and records integrity events if within deadline.
func (s *Service) AutosaveAnswers(
	ctx context.Context, userID, attemptID uuid.UUID, req SaveAnswersRequest,
) (*SaveAnswersResult, error) {
	if s.repo == nil {
		return nil, domain.ErrAttemptNotFound
	}
	attempt, err := s.repo.GetExamAttemptForUser(ctx, attemptID, userID)
	if err != nil {
		return nil, domain.ErrAttemptNotFound
	}

	now := s.clock.Now().UTC()
	if domain.IsPastDeadline(attempt.DeadlineAt, now) {
		return nil, domain.ErrAttemptExpired
	}
	if attempt.Status != domain.StatusInProgress {
		return nil, domain.ErrExamAlreadySubmitted
	}

	if attempt.Mode == domain.ModeExam && req.SectionNumber > 0 {
		if req.SectionNumber < int(attempt.CurrentSection) {
			return nil, domain.ErrSectionAlreadyCompleted
		}
		if req.SectionNumber > int(attempt.CurrentSection) {
			return nil, domain.ErrInvalidSectionProgression
		}
	}

	// Merge draft answers
	var currentAnswers map[string]any
	_ = json.Unmarshal(attempt.DraftAnswers, &currentAnswers)
	if currentAnswers == nil {
		currentAnswers = make(map[string]any)
	}
	for k, v := range req.Answers {
		currentAnswers[k] = v
	}
	mergedBytes, err := json.Marshal(currentAnswers)
	if err != nil {
		return nil, fmt.Errorf("serialize draft answers: %w", err)
	}

	if _, err := s.repo.UpdateDraftAnswers(ctx, attemptID, mergedBytes, now); err != nil {
		return nil, fmt.Errorf("update draft answers: %w", err)
	}

	// Record integrity events
	for _, ev := range req.IntegrityEvents {
		metaBytes, _ := json.Marshal(ev.Metadata)
		_ = s.repo.RecordIntegrityEvent(ctx, sqlc.RecordIntegrityEventParams{
			ID:         uuid.New(),
			AttemptID:  attemptID,
			Kind:       ev.Kind,
			OccurredAt: ev.OccurredAt,
			Metadata:   metaBytes,
		})
	}

	return &SaveAnswersResult{
		Saved:            true,
		RemainingSeconds: domain.RemainingSeconds(attempt.DeadlineAt, now),
	}, nil
}

// CompleteSection advances to the next section or auto-submits if last section in exam mode.
func (s *Service) CompleteSection(
	ctx context.Context, userID, attemptID uuid.UUID, sectionNum int,
) (*CompleteSectionResult, error) {
	if s.repo == nil {
		return nil, domain.ErrAttemptNotFound
	}
	attempt, err := s.repo.GetExamAttemptForUser(ctx, attemptID, userID)
	if err != nil {
		return nil, domain.ErrAttemptNotFound
	}

	now := s.clock.Now().UTC()
	if domain.IsPastDeadline(attempt.DeadlineAt, now) {
		return nil, domain.ErrAttemptExpired
	}
	if attempt.Status != domain.StatusInProgress {
		return nil, domain.ErrExamAlreadySubmitted
	}

	if attempt.Mode == domain.ModeExam {
		if sectionNum != int(attempt.CurrentSection) {
			return nil, domain.ErrInvalidSectionProgression
		}
		secDeadline := domain.SectionDeadline(attempt.StartedAt, sectionNum)
		if domain.IsPastDeadline(secDeadline, now) {
			return nil, domain.ErrSectionTimeExpired
		}
	}

	nextSection := sectionNum + 1
	if nextSection > 4 {
		// Completed final section -> submit exam
		_, subErr := s.SubmitExam(ctx, userID, attemptID, domain.SubmittedByLearner)
		if subErr != nil {
			return nil, subErr
		}
		return &CompleteSectionResult{
			CurrentSection:   4,
			RemainingSeconds: 0,
		}, nil
	}

	if _, err := s.repo.UpdateCurrentSection(ctx, attemptID, int32(nextSection), now); err != nil {
		return nil, err
	}

	return &CompleteSectionResult{
		CurrentSection:   nextSection,
		RemainingSeconds: domain.RemainingSeconds(attempt.DeadlineAt, now),
	}, nil
}

// SubmitExam concludes an attempt, grades answers into learn.attempts, and compiles score report.
func (s *Service) SubmitExam(
	ctx context.Context, userID, attemptID uuid.UUID, submittedBy string,
) (*SubmitExamResult, error) {
	if s.repo == nil {
		return nil, domain.ErrAttemptNotFound
	}

	now := s.clock.Now().UTC()

	// Atomically move attempt out of in_progress
	var attempt *sqlc.AssessExamAttempt
	var err error
	if submittedBy == domain.SubmittedByExpiry {
		attempt, err = s.repo.MarkAttemptExpired(ctx, attemptID, now)
	} else {
		attempt, err = s.repo.MarkAttemptCompleted(ctx, attemptID, now, domain.SubmittedByLearner)
	}
	if err != nil || attempt == nil {
		// Idempotency check: if already submitted, return stored state
		current, cErr := s.repo.GetExamAttemptByID(ctx, attemptID)
		if cErr == nil && current != nil && current.Status != domain.StatusInProgress {
			rep, _ := s.repo.GetScoreReportByAttemptID(ctx, attemptID)
			repStatus := domain.ReportStatusPending
			if rep != nil {
				repStatus = rep.Status
			}
			return &SubmitExamResult{
				AttemptID:    attemptID,
				Status:       current.Status,
				ReportStatus: repStatus,
			}, nil
		}
		return nil, domain.ErrExamAlreadySubmitted
	}

	var drawn []SectionActivities
	_ = json.Unmarshal(attempt.SectionActivities, &drawn)
	var answers map[string]any
	_ = json.Unmarshal(attempt.DraftAnswers, &answers)

	var hasAsync bool
	var sectionScores []SectionScoreDTO
	var totalPercentage float64
	var scoredSectionCount int

	for _, sec := range drawn {
		secScore := 0
		secMaxScore := 0
		secAsync := false
		var itemResults []any

		for _, act := range sec.Activities {
			rawAns, answered := answers[act.ID.String()]
			if !answered || rawAns == nil {
				// Unanswered item scores zero and creates no attempt (WO12 §3.6)
				secMaxScore += 100
				continue
			}

			ansBytes, mErr := json.Marshal(rawAns)
			if mErr != nil {
				continue
			}

			// Deterministic idempotency key from sitting and activity
			idempotencyKey := uuid.NewSHA1(attemptID, []byte(act.ID.String()))

			if s.learning != nil {
				res, gErr := s.learning.SubmitSittingAnswer(ctx, learningcontract.SittingAnswerRequest{
					UserID:         attempt.UserID,
					ActivityID:     act.ID,
					Response:       ansBytes,
					IdempotencyKey: idempotencyKey,
				})
				if gErr != nil {
					slog.WarnContext(ctx, "failed grading sitting item", "activity_id", act.ID, "error", gErr)
					continue
				}
				if res.Async {
					secAsync = true
					hasAsync = true
				} else {
					secScore += res.Score
					secMaxScore += res.MaxScore
				}
				if len(res.ItemResults) > 0 {
					itemResults = append(itemResults, res.ItemResults)
				}
			}
		}

		status := "scored"
		if secAsync {
			status = "pending"
		}
		sectionScores = append(sectionScores, SectionScoreDTO{
			Skill:       sec.Skill,
			Score:       secScore,
			MaxScore:    secMaxScore,
			Status:      status,
			ItemResults: itemResults,
		})

		if !secAsync && secMaxScore > 0 {
			totalPercentage += (float64(secScore) / float64(secMaxScore)) * 100.0
			scoredSectionCount++
		}
	}

	var overallScore float64
	if scoredSectionCount > 0 {
		overallScore = math.Round(totalPercentage / float64(scoredSectionCount))
	}
	overallBand := domain.EstimateCEFRBand(overallScore)

	reportStatus := domain.ReportStatusReady
	if hasAsync {
		reportStatus = domain.ReportStatusPending
	}

	secBytes, _ := json.Marshal(sectionScores)

	// Fetch integrity signals
	events, _ := s.repo.ListIntegrityEvents(ctx, attemptID)
	var signals []map[string]any
	for _, ev := range events {
		signals = append(signals, map[string]any{
			"kind":        ev.Kind,
			"occurred_at": ev.OccurredAt,
		})
	}
	sigBytes, _ := json.Marshal(signals)

	var scoreNumeric pgtype.Numeric
	_ = scoreNumeric.Scan(fmt.Sprintf("%.2f", overallScore))

	_, _ = s.repo.CreateScoreReport(ctx, sqlc.CreateScoreReportParams{
		ID:               uuid.New(),
		AttemptID:        attemptID,
		UserID:           attempt.UserID,
		ExamID:           attempt.ExamID,
		OverallScore:     scoreNumeric,
		OverallBand:      overallBand,
		Status:           reportStatus,
		PerSection:       secBytes,
		Feedback:         []byte("{}"),
		IntegritySignals: sigBytes,
	})

	return &SubmitExamResult{
		AttemptID:    attemptID,
		Status:       attempt.Status,
		ReportStatus: reportStatus,
	}, nil
}

// ExpireAttempt submits an attempt as expired.
func (s *Service) ExpireAttempt(ctx context.Context, attemptID uuid.UUID) error {
	if s.repo == nil {
		return nil
	}
	attempt, err := s.repo.GetExamAttemptByID(ctx, attemptID)
	if err != nil || attempt == nil {
		return nil
	}
	if attempt.Status != domain.StatusInProgress {
		return nil
	}
	_, subErr := s.SubmitExam(ctx, attempt.UserID, attemptID, domain.SubmittedByExpiry)
	return subErr
}

// SweepExpired finds in-progress attempts past their deadline and submits them as expired.
func (s *Service) SweepExpired(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}
	now := s.clock.Now().UTC()
	overdue, err := s.repo.ListExpiredInProgressAttempts(ctx, now)
	if err != nil {
		return err
	}
	for _, att := range overdue {
		if err := s.ExpireAttempt(ctx, att.ID); err != nil {
			slog.WarnContext(ctx, "failed expiring overdue sitting", "attempt_id", att.ID, "error", err)
		}
	}
	return nil
}

// GetScoreReport returns the score report for an attempt.
func (s *Service) GetScoreReport(ctx context.Context, userID, attemptID uuid.UUID) (*ScoreReportDTO, error) {
	if s.repo == nil {
		return nil, domain.ErrReportNotReady
	}
	report, err := s.repo.GetScoreReportByAttemptID(ctx, attemptID)
	if err != nil {
		return nil, domain.ErrReportNotReady
	}
	if report.UserID != userID {
		return nil, domain.ErrUnauthorizedAttempt
	}

	now := s.clock.Now().UTC()
	// If still pending after 1 hour, transition to partial (WO12 §3.6)
	if report.Status == domain.ReportStatusPending && now.Sub(report.CreatedAt) > 1*time.Hour {
		var scoreNumeric pgtype.Numeric
		_ = scoreNumeric.Scan("0.00")
		if updated, uErr := s.repo.UpdateScoreReport(ctx, sqlc.UpdateScoreReportParams{
			AttemptID:        report.AttemptID,
			OverallScore:     report.OverallScore,
			OverallBand:      report.OverallBand,
			Status:           domain.ReportStatusPartial,
			PerSection:       report.PerSection,
			Feedback:         report.Feedback,
			IntegritySignals: report.IntegritySignals,
			UpdatedAt:        now,
		}); uErr == nil {
			report = updated
		}
	}

	var perSection []SectionScoreDTO
	_ = json.Unmarshal(report.PerSection, &perSection)
	var feedback map[string]any
	_ = json.Unmarshal(report.Feedback, &feedback)
	var signals []map[string]any
	_ = json.Unmarshal(report.IntegritySignals, &signals)

	scoreFloat, _ := report.OverallScore.Float64Value()

	return &ScoreReportDTO{
		AttemptID:        report.AttemptID,
		Status:           report.Status,
		OverallScore:     scoreFloat.Float64,
		OverallBand:      report.OverallBand,
		PerSection:       perSection,
		Feedback:         feedback,
		IntegritySignals: signals,
		Disclaimer:       OfficialDisclaimer,
	}, nil
}

// ListUserAttempts returns paginated past sittings for the caller.
func (s *Service) ListUserAttempts(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]ExamAttemptDTO, int64, error) {
	if s.repo == nil {
		return nil, 0, nil
	}
	rows, err := s.repo.ListUserExamAttempts(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountUserExamAttempts(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	now := s.clock.Now().UTC()
	attempts := make([]ExamAttemptDTO, len(rows))
	for i, r := range rows {
		attempts[i] = ExamAttemptDTO{
			ID:                    r.ID,
			ExamID:                r.ExamID,
			Mode:                  r.Mode,
			ChosenDurationMinutes: int(r.ChosenDurationMinutes),
			StartedAt:             r.StartedAt,
			DeadlineAt:            r.DeadlineAt,
			RemainingSeconds:      domain.RemainingSeconds(r.DeadlineAt, now),
			CurrentSection:        int(r.CurrentSection),
			Status:                r.Status,
			ServerTime:            now,
		}
	}
	return attempts, total, nil
}

// AttemptHistory implements contract.Reader.
func (s *Service) AttemptHistory(
	ctx context.Context, userID uuid.UUID, limit, offset int,
) ([]examcontract.AttemptSummary, int, error) {
	attempts, total, err := s.ListUserAttempts(ctx, userID, int32(limit), int32(offset))
	if err != nil {
		return nil, 0, err
	}
	summaries := make([]examcontract.AttemptSummary, len(attempts))
	for i, a := range attempts {
		summaries[i] = examcontract.AttemptSummary{
			ID:        a.ID,
			ExamID:    a.ExamID,
			Mode:      a.Mode,
			StartedAt: a.StartedAt,
			Status:    a.Status,
		}
	}
	return summaries, int(total), nil
}

// LatestBand implements contract.Reader.
func (s *Service) LatestBand(ctx context.Context, userID uuid.UUID) (*string, error) {
	attempts, _, err := s.ListUserAttempts(ctx, userID, 1, 0)
	if err != nil || len(attempts) == 0 {
		return nil, err
	}
	rep, rErr := s.repo.GetScoreReportByAttemptID(ctx, attempts[0].ID)
	if rErr != nil || rep == nil {
		return nil, nil
	}
	return &rep.OverallBand, nil
}

// IsExamPoolActivity implements contract.Reader.
func (s *Service) IsExamPoolActivity(ctx context.Context, activityID uuid.UUID) (bool, error) {
	if s.lesson == nil {
		return false, nil
	}
	act, err := s.lesson.ResolveActivity(ctx, activityID)
	if err != nil || act == nil {
		return false, nil
	}
	return act.CourseSlug == ExamPoolCourseSlug, nil
}
