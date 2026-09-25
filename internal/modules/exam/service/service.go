// Package service runs exam sittings: drawing, the server clock, expiry and the report.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/fluentra/fluentra/internal/generated/exam/sqlc"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	examcontract "github.com/fluentra/fluentra/internal/modules/exam/contract"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	examjob "github.com/fluentra/fluentra/internal/modules/exam/job"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	platformjob "github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Defaults and names the exam service works with.
const (
	DefaultDailySittingsLimit = 5
	HoChiMinhTimeZone         = "Asia/Ho_Chi_Minh"
	ExamPoolCourseSlug        = "pool-exam"
	OfficialDisclaimer        = "Not an official TOEIC score. Pronunciation not assessed."

	kindListeningComprehension = "listening_comprehension"
	// TOEIC Parts 1 and 2 are spoken only: a photograph with spoken statements,
	// a question with spoken responses. They carry the same one-play rule as a
	// comprehension clip (WO 22 I.3.5).
	kindPhotoDescription = "photo_description"
	kindQuestionResponse = "question_response"

	// regradeAfter is how old a report must be before the settle sweep submits an
	// item the submitting request never reached. Younger than this, the request
	// that created the report may still be grading it, and two submissions of one
	// answer would race to create two attempts.
	regradeAfter = 5 * time.Minute
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
	UpdateDraftAnswers(
		ctx context.Context, id uuid.UUID, draftAnswers []byte, updatedAt time.Time,
	) (*sqlc.AssessExamAttempt, error)
	UpdateCurrentSection(
		ctx context.Context, id uuid.UUID, section int32, updatedAt time.Time,
	) (*sqlc.AssessExamAttempt, error)
	MarkAttemptCompleted(
		ctx context.Context, id uuid.UUID, submittedAt time.Time, submittedBy string,
	) (*sqlc.AssessExamAttempt, error)
	MarkAttemptExpired(ctx context.Context, id uuid.UUID, submittedAt time.Time) (*sqlc.AssessExamAttempt, error)
	ListExpiredInProgressAttempts(ctx context.Context, now time.Time) ([]sqlc.AssessExamAttempt, error)
	CreateScoreReport(ctx context.Context, arg sqlc.CreateScoreReportParams) (*sqlc.AssessScoreReport, error)
	GetScoreReportByAttemptID(ctx context.Context, attemptID uuid.UUID) (*sqlc.AssessScoreReport, error)
	ListPendingScoreReports(ctx context.Context) ([]sqlc.AssessScoreReport, error)
	UpdateScoreReport(ctx context.Context, arg sqlc.UpdateScoreReportParams) (*sqlc.AssessScoreReport, error)
	RecordIntegrityEvent(ctx context.Context, arg sqlc.RecordIntegrityEventParams) error
	ListIntegrityEvents(ctx context.Context, attemptID uuid.UUID) ([]sqlc.AssessIntegrityEvent, error)
	ListCurrentExamVersions(ctx context.Context) ([]*domain.ExamVersion, error)
	GetExamVersionByID(ctx context.Context, id uuid.UUID) (*domain.ExamVersion, error)
	GetExamVersionByCode(ctx context.Context, code string) (*domain.ExamVersion, error)
	ListExamPartsByVersionID(ctx context.Context, versionID uuid.UUID) ([]*domain.ExamPart, error)
	GetExamPartByID(ctx context.Context, id uuid.UUID) (*domain.ExamPart, error)
	ListBlueprintsByVersionID(ctx context.Context, versionID uuid.UUID) ([]*domain.Blueprint, error)
	GetBlueprintByID(ctx context.Context, id uuid.UUID) (*domain.Blueprint, error)
	GetBlueprintByName(ctx context.Context, versionID uuid.UUID, name string) (*domain.Blueprint, error)
	CreateMockTest(ctx context.Context, mt *domain.MockTest) (*domain.MockTest, error)
	GetMockTestByID(ctx context.Context, id uuid.UUID) (*domain.MockTest, error)
	ListMockTestsByOwner(ctx context.Context, ownerID *uuid.UUID) ([]*domain.MockTest, error)
	ListFixedMockTests(ctx context.Context, blueprintID uuid.UUID) ([]*domain.MockTest, error)
	GetUserBestVersionScore(ctx context.Context, userID, versionID uuid.UUID) (*float64, error)
	GetLatestUserMockTestAttempt(
		ctx context.Context, userID, mockTestID uuid.UUID,
	) (*sqlc.GetLatestUserMockTestAttemptRow, error)
	GetExamByVersionID(ctx context.Context, versionID uuid.UUID) (*sqlc.AssessExam, error)
	CreateMockTestAttempt(ctx context.Context, arg sqlc.CreateMockTestAttemptParams) (*sqlc.AssessExamAttempt, error)
	CountUserMockTestAttempts(ctx context.Context, userID, mockTestID uuid.UUID) (int64, error)
}

// Deps defines dependencies for the exam service.
type Deps struct {
	Pool         *pgxpool.Pool
	Repo         ExamRepository
	Learning     learningcontract.SittingAnswerSubmitter
	Attempts     learningcontract.AttemptOutcomeReader
	Exposures    learningcontract.ItemExposureRecorder
	Lesson       lessoncontract.Reader
	Questionbank questionbankcontract.Reader
	// BankAuthor generates the questions the daily job adds to the bank
	// (WO 22 Stage O). Nil disables the job.
	BankAuthor questionbankcontract.Author
	// ReviewBacklog lets the daily job skip an exam whose doubts from two
	// earlier days nobody has reviewed. Nil never skips.
	ReviewBacklog contentcontract.ReviewBacklog
	// DailyGenerationCap bounds how many items one daily run asks for; 0 is
	// no bound beyond one test's worth.
	DailyGenerationCap int
	Drawer             PoolDrawer
	Clock              clock.Clock
	DailyLimit         int
	Enqueuer           platformjob.Enqueuer
	Nudger             WorkerNudger
}

// Service orchestrates exam sittings, timing, auto-submission, and scoring.
type Service struct {
	pool         *pgxpool.Pool
	repo         ExamRepository
	learning     learningcontract.SittingAnswerSubmitter
	attempts     learningcontract.AttemptOutcomeReader
	exposures    learningcontract.ItemExposureRecorder
	lesson       lessoncontract.Reader
	questionbank questionbankcontract.Reader
	bankAuthor   questionbankcontract.Author
	backlog      contentcontract.ReviewBacklog
	dailyCap     int
	drawer       PoolDrawer
	clock        clock.Clock
	dailyLimit   int
	enqueuer     platformjob.Enqueuer
	nudger       WorkerNudger
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
		pool:         deps.Pool,
		repo:         deps.Repo,
		learning:     deps.Learning,
		attempts:     deps.Attempts,
		exposures:    deps.Exposures,
		lesson:       deps.Lesson,
		questionbank: deps.Questionbank,
		bankAuthor:   deps.BankAuthor,
		backlog:      deps.ReviewBacklog,
		dailyCap:     deps.DailyGenerationCap,
		drawer:       deps.Drawer,
		clock:        clk,
		dailyLimit:   dailyLimit,
		enqueuer:     deps.Enqueuer,
		nudger:       deps.Nudger,
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
	// Unlimited skips ChosenDurationMinutes and runs a practice sitting against
	// the unlimited-duration backstop instead. Ignored outside practice mode.
	Unlimited bool `json:"unlimited,omitempty"`
	// Sections limits a practice sitting to the sections chosen, by position.
	// Empty means every section; exam mode always sits all four.
	Sections []int `json:"sections,omitempty"`
}

// ExamAttemptDTO describes an exam sitting.
type ExamAttemptDTO struct {
	ID                      uuid.UUID                  `json:"id"`
	ExamID                  uuid.UUID                  `json:"exam_id"`
	ExamSlug                string                     `json:"exam_slug,omitempty"`
	ExamTitle               string                     `json:"exam_title,omitempty"`
	Level                   string                     `json:"level,omitempty"`
	Mode                    string                     `json:"mode"`
	ChosenDurationMinutes   int                        `json:"chosen_duration_minutes"`
	Unlimited               bool                       `json:"unlimited"`
	StartedAt               time.Time                  `json:"started_at"`
	DeadlineAt              time.Time                  `json:"deadline_at"`
	RemainingSeconds        int                        `json:"remaining_seconds"`
	CurrentSection          int                        `json:"current_section"`
	SectionDeadlineAt       *time.Time                 `json:"section_deadline_at,omitempty"`
	SectionRemainingSeconds *int                       `json:"section_remaining_seconds,omitempty"`
	Status                  string                     `json:"status"`
	SubmittedAt             *time.Time                 `json:"submitted_at,omitempty"`
	SectionActivities       []SectionActivities        `json:"section_activities,omitempty"`
	DraftAnswers            map[string]json.RawMessage `json:"draft_answers,omitempty"`
	ServerTime              time.Time                  `json:"server_time"`
}

// SaveAnswersRequest parameters. Answers are keyed by activity ID.
type SaveAnswersRequest struct {
	SectionNumber   int                        `json:"section_number,omitempty"`
	Answers         map[string]json.RawMessage `json:"answers"`
	IntegrityEvents []IntegrityEventDTO        `json:"integrity_events,omitempty"`
}

// IntegrityEventDTO is a signal the client observed. Its time is the server's.
type IntegrityEventDTO struct {
	Kind string `json:"kind"`
}

// SaveAnswersResult response.
type SaveAnswersResult struct {
	Saved                   bool `json:"saved"`
	RemainingSeconds        int  `json:"remaining_seconds"`
	CurrentSection          int  `json:"current_section"`
	SectionRemainingSeconds *int `json:"section_remaining_seconds,omitempty"`
}

// CompleteSectionResult response.
type CompleteSectionResult struct {
	CurrentSection          int  `json:"current_section"`
	RemainingSeconds        int  `json:"remaining_seconds"`
	SectionRemainingSeconds *int `json:"section_remaining_seconds,omitempty"`
	Submitted               bool `json:"submitted"`
}

// SubmitExamResult response.
type SubmitExamResult struct {
	AttemptID    uuid.UUID `json:"attempt_id"`
	Status       string    `json:"status"`
	ReportStatus string    `json:"report_status"`
}

// IntegritySignalDTO counts one kind of signal in a sitting.
type IntegritySignalDTO struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// ScoreReportDTO response.
type ScoreReportDTO struct {
	AttemptID    uuid.UUID `json:"attempt_id"`
	Mode         string    `json:"mode"`
	SubmittedBy  string    `json:"submitted_by,omitempty"`
	Status       string    `json:"status"`
	OverallScore float64   `json:"overall_score"`
	OverallBand  string    `json:"overall_band,omitempty"`
	// PublishedScore is the overall on the exam's own scale (an IELTS band, a
	// TOEIC scaled estimate, a VSTEP level), when the version states one.
	PublishedScore   *float64                `json:"published_score,omitempty"`
	PublishedScale   string                  `json:"published_scale,omitempty"`
	ScoreIsEstimate  bool                    `json:"score_is_estimate,omitempty"`
	PerSection       []domain.SectionOutcome `json:"per_section"`
	IntegritySignals []IntegritySignalDTO    `json:"integrity_signals"`
	Disclaimer       string                  `json:"disclaimer"`
	// ElapsedSeconds is submitted_at minus started_at — how long the learner
	// actually took, regardless of any time limit chosen. Absent if, somehow,
	// the attempt has no submission time yet.
	ElapsedSeconds int `json:"elapsed_seconds,omitempty"`
}

// SittingsToday is how many sittings the learner has started today, and the limit.
type SittingsToday struct {
	Used  int `json:"sittings_today"`
	Limit int `json:"daily_limit"`
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
	// Each exam carries its sections, so the list can say what a sitting holds
	// and a practice sitting can choose among them. There are three exams.
	exams := make([]ExamDTO, len(rows))
	for i, r := range rows {
		exams[i] = examDTO(&r)
		sections, err := s.examSections(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		exams[i].Sections = sections
	}
	return exams, nil
}

// GetExam returns an exam template by ID with its sections.
func (s *Service) GetExam(ctx context.Context, id uuid.UUID) (*ExamDTO, error) {
	if s.repo == nil {
		return nil, domain.ErrExamNotFound
	}
	exam, err := s.repo.GetExamByID(ctx, id)
	if err != nil || exam == nil {
		return nil, domain.ErrExamNotFound
	}
	dto := examDTO(exam)
	if dto.Sections, err = s.examSections(ctx, id); err != nil {
		return nil, err
	}
	return &dto, nil
}

func (s *Service) examSections(ctx context.Context, examID uuid.UUID) ([]ExamSectionDTO, error) {
	secRows, err := s.repo.ListExamSections(ctx, examID)
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
	return sections, nil
}

// keepSections returns the drawn sections whose positions were chosen, or all of
// them when none were.
func keepSections(drawn []SectionActivities, chosen map[int]bool) []SectionActivities {
	if len(chosen) == 0 {
		return drawn
	}
	kept := make([]SectionActivities, 0, len(chosen))
	for _, section := range drawn {
		if chosen[section.SectionPosition] {
			kept = append(kept, section)
		}
	}
	return kept
}

func examDTO(r *sqlc.AssessExam) ExamDTO {
	return ExamDTO{
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

// sittingDuration picks the sitting's mode and length in minutes: the exam's own
// length in exam mode; the clamped choice, or the unlimited backstop, in practice.
func sittingDuration(req StartAttemptRequest, examMinutes int) (string, int) {
	if req.Mode != domain.ModePractice {
		return domain.ModeExam, examMinutes
	}
	if req.Unlimited {
		return domain.ModePractice, domain.UnlimitedPracticeDurationMinutes
	}
	return domain.ModePractice, domain.ClampPracticeDuration(req.ChosenDurationMinutes)
}

// StartSitting begins a new exam sitting.
func (s *Service) StartSitting(
	ctx context.Context, userID, examID uuid.UUID, req StartAttemptRequest,
) (*ExamAttemptDTO, error) {
	if s.repo == nil {
		return nil, errors.New("repository not configured")
	}

	exam, err := s.repo.GetExamByID(ctx, examID)
	if err != nil || exam == nil {
		return nil, domain.ErrExamNotFound
	}

	var chosen map[int]bool
	if req.Mode == domain.ModePractice {
		if chosen, err = domain.ChosenSections(req.Sections); err != nil {
			return nil, err
		}
	}

	now := s.clock.Now().UTC()
	if err := s.checkCanStart(ctx, userID, now); err != nil {
		return nil, err
	}

	var drawn []SectionActivities
	if s.drawer != nil {
		drawn, err = s.drawer.DrawSitting(ctx, userID, exam.Level)
		if err != nil {
			return nil, err
		}
	}
	// A practice sitting keeps only the sections chosen, before anything is
	// marked seen: an item the learner never sits is not spent.
	drawn = keepSections(drawn, chosen)
	if len(drawn) == 0 {
		return nil, domain.ErrInsufficientItems
	}

	mode, duration := sittingDuration(req, int(exam.TotalMinutes))
	deadlineAt := now.Add(time.Duration(duration) * time.Minute)

	drawnBytes, err := json.Marshal(drawn)
	if err != nil {
		return nil, fmt.Errorf("serialize section activities: %w", err)
	}

	var activityIDs []uuid.UUID
	for _, sec := range drawn {
		for _, act := range sec.Activities {
			activityIDs = append(activityIDs, act.ID)
		}
	}

	params := sqlc.CreateExamAttemptParams{
		ID:                    uuid.New(),
		UserID:                userID,
		ExamID:                examID,
		Mode:                  mode,
		ChosenDurationMinutes: clampInt32(duration),
		StartedAt:             now,
		DeadlineAt:            deadlineAt,
		CurrentSection:        clampInt32(drawn[0].SectionPosition),
		Status:                domain.StatusInProgress,
		SectionActivities:     drawnBytes,
		DraftAnswers:          []byte("{}"),
	}

	attempt, err := s.createSitting(ctx, params, activityIDs)
	if err != nil {
		return nil, err
	}

	dto := s.attemptDTO(attempt, exam, now, true)
	return &dto, nil
}

// checkCanStart refuses a second open sitting (BR-EXAM-04) and a sitting past
// the daily limit. Both are checked before anything is drawn, so a refused start
// marks nothing as seen.
func (s *Service) checkCanStart(ctx context.Context, userID uuid.UUID, now time.Time) error {
	activeCount, err := s.repo.CountUserActiveAttempts(ctx, userID)
	if err != nil {
		return err
	}
	if activeCount > 0 {
		return domain.ErrAttemptInProgress
	}
	today, err := s.sittingsToday(ctx, userID, now)
	if err != nil {
		return err
	}
	if today.Used >= today.Limit {
		return domain.ErrExamDailyLimitReached
	}
	return nil
}

// createSitting writes the sitting, the exposures of every drawn item and the
// expiry job in one transaction: a sitting that fails to start marks nothing as seen.
func (s *Service) createSitting(
	ctx context.Context, params sqlc.CreateExamAttemptParams, activityIDs []uuid.UUID,
) (*sqlc.AssessExamAttempt, error) {
	if s.pool == nil {
		created, err := s.repo.CreateExamAttempt(ctx, params)
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
		created, err := s.repo.CreateExamAttempt(txCtx, params)
		if err != nil {
			return err
		}
		attempt = created

		if s.exposures != nil && len(activityIDs) > 0 {
			if err := s.exposures.RecordItemExposures(txCtx, tx, params.UserID, activityIDs); err != nil {
				return fmt.Errorf("record item exposures: %w", err)
			}
		}

		// The sweep and the next read both back this job up, so a failure to
		// schedule it is logged, not fatal.
		if s.enqueuer != nil {
			args := examjob.ExpireAttemptArgs{AttemptID: params.ID}
			opts := &river.InsertOpts{ScheduledAt: params.DeadlineAt}
			if _, err := s.enqueuer.EnqueueTx(txCtx, tx, args, opts); err != nil {
				slog.WarnContext(txCtx, "could not schedule the sitting's expiry job", "error", err)
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return attempt, nil
}

// GetExamAttempt returns a sitting, submitting it first when its deadline has passed.
func (s *Service) GetExamAttempt(ctx context.Context, userID, attemptID uuid.UUID) (*ExamAttemptDTO, error) {
	attempt, err := s.sittingFor(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now().UTC()

	// Reading a sitting past its deadline is itself a trigger (§3.6): a learner
	// returning after the deadline sees their result being built, not a clock at zero.
	if attempt.Status == domain.StatusInProgress && domain.IsPastDeadline(attempt.DeadlineAt, now) {
		if _, err := s.submit(ctx, attempt, domain.SubmittedByExpiry); err != nil {
			slog.WarnContext(ctx, "could not submit an overdue sitting on read", "attempt_id", attemptID, "error", err)
		}
		if refreshed, rErr := s.repo.GetExamAttemptByID(ctx, attemptID); rErr == nil && refreshed != nil {
			attempt = refreshed
		}
	} else {
		s.persistSection(ctx, attempt, now)
	}

	var exam *sqlc.AssessExam
	if found, eErr := s.repo.GetExamByID(ctx, attempt.ExamID); eErr == nil {
		exam = found
	}
	dto := s.attemptDTO(attempt, exam, now, true)
	return &dto, nil
}

// AutosaveAnswers saves draft answers and integrity signals while the sitting is open.
func (s *Service) AutosaveAnswers(
	ctx context.Context, userID, attemptID uuid.UUID, req SaveAnswersRequest,
) (*SaveAnswersResult, error) {
	attempt, err := s.sittingFor(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now().UTC()
	if attempt.Status != domain.StatusInProgress {
		return nil, domain.ErrExamAlreadySubmitted
	}
	if domain.IsPastDeadline(attempt.DeadlineAt, now) {
		return nil, domain.ErrAttemptExpired
	}

	current := s.currentSection(attempt, now)
	if attempt.Mode == domain.ModeExam && req.SectionNumber > 0 {
		if err := sectionOrder(req.SectionNumber, current); err != nil {
			return nil, err
		}
	}

	answers := decodeAnswers(attempt)
	if err := mergeAnswers(attempt, current, req.Answers, answers); err != nil {
		return nil, err
	}

	merged, err := json.Marshal(answers)
	if err != nil {
		return nil, fmt.Errorf("serialize draft answers: %w", err)
	}
	if _, err := s.repo.UpdateDraftAnswers(ctx, attemptID, merged, now); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrExamAlreadySubmitted
		}
		return nil, fmt.Errorf("update draft answers: %w", err)
	}

	s.recordIntegrityEvents(ctx, attemptID, req.IntegrityEvents, now)
	s.persistSection(ctx, attempt, now)

	return &SaveAnswersResult{
		Saved:                   true,
		RemainingSeconds:        domain.RemainingSeconds(attempt.DeadlineAt, now),
		CurrentSection:          current,
		SectionRemainingSeconds: sectionRemaining(attempt, current, now),
	}, nil
}

// sectionOrder refuses, in exam mode, anything aimed at a section other than the open one.
func sectionOrder(position, current int) error {
	switch {
	case position < current:
		return domain.ErrSectionAlreadyCompleted
	case position > current:
		return domain.ErrInvalidSectionProgression
	default:
		return nil
	}
}

// mergeAnswers checks each incoming answer and adds it to the saved ones.
//
// An answer must be for an item the sitting holds and fit domain.MaxAnswerBytes.
// In exam mode it must also be for the open section: there is no going back,
// whatever section number the client claims.
func mergeAnswers(
	attempt *sqlc.AssessExamAttempt, current int,
	incoming map[string]json.RawMessage, answers map[string]json.RawMessage,
) error {
	positions := activityPositions(decodeSections(attempt))
	isExam := attempt.Mode == domain.ModeExam
	for key, raw := range incoming {
		id, err := uuid.Parse(key)
		if err != nil {
			return domain.ErrUnknownItem
		}
		position, held := positions[id]
		if !held {
			return domain.ErrUnknownItem
		}
		if isExam {
			if err := sectionOrder(position, current); err != nil {
				return err
			}
		}
		if len(raw) > domain.MaxAnswerBytes {
			return domain.ErrAnswerTooLarge
		}
		answers[id.String()] = raw
	}
	return nil
}

func (s *Service) recordIntegrityEvents(
	ctx context.Context, attemptID uuid.UUID, events []IntegrityEventDTO, now time.Time,
) {
	recorded := 0
	for _, ev := range events {
		if recorded >= domain.MaxIntegrityEventsPerSave {
			return
		}
		if !domain.IsIntegrityKind(ev.Kind) {
			continue
		}
		if err := s.repo.RecordIntegrityEvent(ctx, sqlc.RecordIntegrityEventParams{
			ID:         uuid.New(),
			AttemptID:  attemptID,
			Kind:       ev.Kind,
			OccurredAt: now,
			Metadata:   []byte("{}"),
		}); err != nil {
			slog.WarnContext(ctx, "could not record integrity signal", "attempt_id", attemptID, "error", err)
			return
		}
		recorded++
	}
}

// CompleteSection closes a section and moves on; closing the last one submits the sitting.
func (s *Service) CompleteSection(
	ctx context.Context, userID, attemptID uuid.UUID, sectionNum int,
) (*CompleteSectionResult, error) {
	attempt, err := s.sittingFor(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now().UTC()
	if attempt.Status != domain.StatusInProgress {
		return nil, domain.ErrExamAlreadySubmitted
	}
	if domain.IsPastDeadline(attempt.DeadlineAt, now) {
		return nil, domain.ErrAttemptExpired
	}
	if sectionNum < 1 || sectionNum > domain.SectionCount {
		return nil, domain.ErrInvalidSectionProgression
	}

	if attempt.Mode == domain.ModeExam {
		current := s.currentSection(attempt, now)
		if sectionNum < current {
			// Already closed — by the learner or by its clock. Say where the sitting is.
			s.persistSection(ctx, attempt, now)
			return &CompleteSectionResult{
				CurrentSection:          current,
				RemainingSeconds:        domain.RemainingSeconds(attempt.DeadlineAt, now),
				SectionRemainingSeconds: sectionRemaining(attempt, current, now),
			}, nil
		}
		if sectionNum > current {
			return nil, domain.ErrInvalidSectionProgression
		}
	}

	if sectionNum == domain.SectionCount {
		if _, err := s.submit(ctx, attempt, domain.SubmittedByLearner); err != nil {
			return nil, err
		}
		return &CompleteSectionResult{CurrentSection: domain.SectionCount, Submitted: true}, nil
	}

	next := sectionNum + 1
	updated, err := s.repo.UpdateCurrentSection(ctx, attemptID, clampInt32(next), now)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrExamAlreadySubmitted
		}
		return nil, err
	}
	if updated != nil {
		attempt = updated
	}

	return &CompleteSectionResult{
		CurrentSection:          next,
		RemainingSeconds:        domain.RemainingSeconds(attempt.DeadlineAt, now),
		SectionRemainingSeconds: sectionRemaining(attempt, next, now),
	}, nil
}

// SubmitExam submits the caller's sitting.
func (s *Service) SubmitExam(
	ctx context.Context, userID, attemptID uuid.UUID, submittedBy string,
) (*SubmitExamResult, error) {
	attempt, err := s.sittingFor(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}
	return s.submit(ctx, attempt, submittedBy)
}

// submit moves a sitting out of in_progress and builds its report.
//
// Every path — the learner, the scheduled job, the sweep and a read — ends here,
// and the conditional update is what makes them one outcome: only the path that
// moves the sitting builds the report, and a path that finds it already moved
// returns what is stored.
func (s *Service) submit(
	ctx context.Context, attempt *sqlc.AssessExamAttempt, submittedBy string,
) (*SubmitExamResult, error) {
	now := s.clock.Now().UTC()

	if attempt.Status == domain.StatusInProgress {
		// A write past the deadline is refused; what was saved before it counts.
		// So a late submission is the expiry's, not the learner's.
		if submittedBy != domain.SubmittedByExpiry && domain.IsPastDeadline(attempt.DeadlineAt, now) {
			submittedBy = domain.SubmittedByExpiry
		}
		var marked *sqlc.AssessExamAttempt
		var err error
		if submittedBy == domain.SubmittedByExpiry {
			marked, err = s.repo.MarkAttemptExpired(ctx, attempt.ID, now)
		} else {
			marked, err = s.repo.MarkAttemptCompleted(ctx, attempt.ID, now, domain.SubmittedByLearner)
		}
		switch {
		case err == nil && marked != nil:
			attempt = marked
		case err == nil || errors.Is(err, pgx.ErrNoRows):
			current, gErr := s.repo.GetExamAttemptByID(ctx, attempt.ID)
			if gErr != nil || current == nil {
				return nil, domain.ErrAttemptNotFound
			}
			attempt = current
		default:
			return nil, fmt.Errorf("move sitting out of in_progress: %w", err)
		}
	}

	report, err := s.ensureReport(ctx, attempt)
	if err != nil {
		return nil, err
	}
	return &SubmitExamResult{
		AttemptID:    attempt.ID,
		Status:       attempt.Status,
		ReportStatus: report.Status,
	}, nil
}

// ensureReport returns the sitting's report, creating and grading it if there is none.
//
// The report row is created before any item is graded. Two paths that both find
// no report both try to create it; the unique attempt_id lets one win, and only
// the winner submits answers to learning — so a sitting produces one set of attempts.
func (s *Service) ensureReport(ctx context.Context, attempt *sqlc.AssessExamAttempt) (*sqlc.AssessScoreReport, error) {
	if existing, err := s.repo.GetScoreReportByAttemptID(ctx, attempt.ID); err == nil && existing != nil {
		return existing, nil
	}

	sections := skeletonOutcomes(decodeSections(attempt))
	sectionBytes, err := json.Marshal(sections)
	if err != nil {
		return nil, fmt.Errorf("serialize report skeleton: %w", err)
	}
	signals, signalBytes := s.integritySignals(ctx, attempt.ID)
	_ = signals

	now := s.clock.Now().UTC()
	created, err := s.repo.CreateScoreReport(ctx, sqlc.CreateScoreReportParams{
		ID:               uuid.New(),
		AttemptID:        attempt.ID,
		UserID:           attempt.UserID,
		ExamID:           attempt.ExamID,
		OverallScore:     numericOf(0),
		OverallBand:      "",
		Status:           domain.ReportStatusPending,
		PerSection:       sectionBytes,
		Feedback:         []byte("{}"),
		IntegritySignals: signalBytes,
		CreatedAt:        now,
	})
	if err != nil || created == nil {
		if existing, gErr := s.repo.GetScoreReportByAttemptID(ctx, attempt.ID); gErr == nil && existing != nil {
			return existing, nil
		}
		return nil, fmt.Errorf("create score report: %w", err)
	}

	answers := decodeAnswers(attempt)
	for si := range sections {
		for ii := range sections[si].Items {
			s.gradeItem(ctx, attempt, answers, &sections[si].Items[ii])
		}
	}
	return s.storeReport(ctx, attempt, created, sections)
}

// gradeItem submits one saved answer to learning as an ordinary attempt.
func (s *Service) gradeItem(
	ctx context.Context, attempt *sqlc.AssessExamAttempt, answers map[string]json.RawMessage, item *domain.ItemOutcome,
) {
	raw, answered := answers[item.ActivityID.String()]
	if !answered || isEmptyAnswer(raw) {
		// An unanswered item scores zero and creates no attempt (§3.6).
		item.Status = domain.ItemUnanswered
		item.Score, item.MaxScore = 0, 0
		return
	}
	if s.learning == nil {
		item.Status = domain.ItemFailed
		return
	}

	res, err := s.learning.SubmitSittingAnswer(ctx, learningcontract.SittingAnswerRequest{
		UserID:     attempt.UserID,
		ActivityID: item.ActivityID,
		Response:   raw,
		// Derived from the sitting and the activity, so a retried submission
		// finds the attempt it already made instead of grading twice.
		IdempotencyKey: uuid.NewSHA1(attempt.ID, []byte(item.ActivityID.String())),
	})
	if err != nil {
		slog.WarnContext(ctx, "could not grade a sitting item",
			"attempt_id", attempt.ID, "activity_id", item.ActivityID, "error", err)
		item.Status = domain.ItemFailed
		return
	}

	attemptID := res.AttemptID
	item.AttemptID = &attemptID
	score := res.Score
	status := res.Status
	if res.Async {
		status = learningStatusGrading
	}
	applyOutcome(item, status, &score, res.MaxScore)
	item.Feedback = res.Feedback
	if len(res.ItemResults) > 0 {
		if b, mErr := json.Marshal(res.ItemResults); mErr == nil {
			item.ItemResults = b
		}
	}
}

const (
	learningStatusGraded  = "graded"
	learningStatusGrading = "grading"
	learningStatusFailed  = "failed"
)

func applyOutcome(item *domain.ItemOutcome, status string, score *int, maxScore int) {
	switch status {
	case learningStatusGraded:
		item.Status = domain.ItemGraded
		item.Score = 0
		if score != nil {
			item.Score = *score
		}
		item.MaxScore = maxScore
	case learningStatusFailed:
		item.Status = domain.ItemFailed
	default:
		item.Status = domain.ItemPending
	}
}

// storeReport scores the sections and writes the report.
func (s *Service) storeReport(
	ctx context.Context, attempt *sqlc.AssessExamAttempt, report *sqlc.AssessScoreReport, sections []domain.SectionOutcome,
) (*sqlc.AssessScoreReport, error) {
	score := domain.ScoreReport(sections, len(decodeSections(attempt)))
	sectionBytes, err := json.Marshal(sections)
	if err != nil {
		return nil, fmt.Errorf("serialize report sections: %w", err)
	}
	_, signalBytes := s.integritySignals(ctx, attempt.ID)

	updated, err := s.repo.UpdateScoreReport(ctx, sqlc.UpdateScoreReportParams{
		AttemptID:        attempt.ID,
		OverallScore:     numericOf(score.Overall),
		OverallBand:      score.Band,
		Status:           score.Status,
		PerSection:       sectionBytes,
		Feedback:         report.Feedback,
		IntegritySignals: signalBytes,
		UpdatedAt:        s.clock.Now().UTC(),
	})
	if err != nil || updated == nil {
		return nil, fmt.Errorf("update score report: %w", err)
	}
	return updated, nil
}

// settleReport brings a pending report up to date with the grades that have
// arrived since, and closes the items still waiting after ReportSettleLimit as
// not scored. A report that is not pending is returned as stored.
func (s *Service) settleReport(ctx context.Context, report *sqlc.AssessScoreReport) (*sqlc.AssessScoreReport, error) {
	if report.Status != domain.ReportStatusPending {
		return report, nil
	}
	attempt, err := s.repo.GetExamAttemptByID(ctx, report.AttemptID)
	if err != nil || attempt == nil {
		return report, fmt.Errorf("load sitting for report: %w", err)
	}

	var sections []domain.SectionOutcome
	if err := json.Unmarshal(report.PerSection, &sections); err != nil {
		return report, fmt.Errorf("decode report sections: %w", err)
	}

	now := s.clock.Now().UTC()
	age := now.Sub(report.CreatedAt)
	answers := decodeAnswers(attempt)
	changed := false

	for si := range sections {
		for ii := range sections[si].Items {
			if s.settleItem(ctx, attempt, answers, &sections[si].Items[ii], age) {
				changed = true
			}
		}
	}

	if !changed {
		return report, nil
	}
	return s.storeReport(ctx, attempt, report, sections)
}

// settleItem brings one pending item up to date and reports whether its status changed.
func (s *Service) settleItem(
	ctx context.Context, attempt *sqlc.AssessExamAttempt, answers map[string]json.RawMessage,
	item *domain.ItemOutcome, age time.Duration,
) bool {
	if item.Status != domain.ItemPending {
		return false
	}
	before := item.Status
	switch {
	case item.AttemptID == nil && age >= regradeAfter:
		// The request that created the report did not reach this item.
		s.gradeItem(ctx, attempt, answers, item)
	case item.AttemptID != nil && s.attempts != nil:
		outcome, err := s.attempts.GetAttemptOutcome(ctx, *item.AttemptID)
		if err != nil {
			slog.WarnContext(ctx, "could not read a sitting item's grade",
				"attempt_id", *item.AttemptID, "error", err)
		} else {
			applyOutcome(item, outcome.Status, outcome.Score, outcome.MaxScore)
		}
	}
	if item.Status == domain.ItemPending && age > domain.ReportSettleLimit {
		item.Status = domain.ItemFailed
	}
	return item.Status != before
}

// ExpireAttempt submits a sitting whose deadline has passed. It is the
// scheduled job's entry point, and it also rebuilds a report a crash left unwritten.
func (s *Service) ExpireAttempt(ctx context.Context, attemptID uuid.UUID) error {
	if s.repo == nil {
		return nil
	}
	attempt, err := s.repo.GetExamAttemptByID(ctx, attemptID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && attempt == nil) {
		// A sitting that no longer exists has nothing to expire; the job is done.
		return nil
	}
	if err != nil {
		return fmt.Errorf("load sitting %s: %w", attemptID, err)
	}
	if attempt.Status == domain.StatusInProgress && !domain.IsPastDeadline(attempt.DeadlineAt, s.clock.Now().UTC()) {
		return nil
	}
	_, err = s.submit(ctx, attempt, domain.SubmittedByExpiry)
	return err
}

// SweepExpired submits sittings past their deadline and settles pending reports.
func (s *Service) SweepExpired(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}
	now := s.clock.Now().UTC()
	overdue, err := s.repo.ListExpiredInProgressAttempts(ctx, now.Add(-domain.NetworkGracePeriod))
	if err != nil {
		return err
	}
	for _, att := range overdue {
		if err := s.ExpireAttempt(ctx, att.ID); err != nil {
			slog.WarnContext(ctx, "failed expiring overdue sitting", "attempt_id", att.ID, "error", err)
		}
	}

	pending, err := s.repo.ListPendingScoreReports(ctx)
	if err != nil {
		return fmt.Errorf("list pending score reports: %w", err)
	}
	for i := range pending {
		if _, err := s.settleReport(ctx, &pending[i]); err != nil {
			slog.WarnContext(ctx, "failed settling a score report", "attempt_id", pending[i].AttemptID, "error", err)
		}
	}
	return nil
}

// GetScoreReport returns the report for the caller's sitting.
func (s *Service) GetScoreReport(ctx context.Context, userID, attemptID uuid.UUID) (*ScoreReportDTO, error) {
	attempt, err := s.sittingFor(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	if attempt.Status == domain.StatusInProgress {
		if !domain.IsPastDeadline(attempt.DeadlineAt, s.clock.Now().UTC()) {
			return nil, domain.ErrReportNotReady
		}
		if _, err := s.submit(ctx, attempt, domain.SubmittedByExpiry); err != nil {
			return nil, err
		}
		if refreshed, rErr := s.repo.GetExamAttemptByID(ctx, attemptID); rErr == nil && refreshed != nil {
			attempt = refreshed
		}
	}

	report, err := s.repo.GetScoreReportByAttemptID(ctx, attemptID)
	if err != nil || report == nil {
		report, err = s.ensureReport(ctx, attempt)
		if err != nil {
			return nil, err
		}
	}
	if settled, sErr := s.settleReport(ctx, report); sErr != nil {
		slog.WarnContext(ctx, "could not settle a score report on read", "attempt_id", attemptID, "error", sErr)
	} else {
		report = settled
	}

	var perSection []domain.SectionOutcome
	_ = json.Unmarshal(report.PerSection, &perSection)
	if perSection == nil {
		perSection = []domain.SectionOutcome{}
	}
	s.attachReview(ctx, attempt, perSection)
	var signals []IntegritySignalDTO
	_ = json.Unmarshal(report.IntegritySignals, &signals)
	if signals == nil {
		signals = []IntegritySignalDTO{}
	}
	overall, _ := report.OverallScore.Float64Value()
	return s.reportDTO(ctx, attempt, report, perSection, signals, overall.Float64), nil
}

// reportDTO assembles the learner-facing report, adding the caller's time and
// the score on the exam's own scale when the version states one.
func (s *Service) reportDTO(
	ctx context.Context,
	attempt *sqlc.AssessExamAttempt,
	report *sqlc.AssessScoreReport,
	perSection []domain.SectionOutcome,
	signals []IntegritySignalDTO,
	overall float64,
) *ScoreReportDTO {
	dto := &ScoreReportDTO{
		AttemptID:        report.AttemptID,
		Mode:             attempt.Mode,
		Status:           report.Status,
		OverallScore:     overall,
		OverallBand:      report.OverallBand,
		PerSection:       perSection,
		IntegritySignals: signals,
		Disclaimer:       OfficialDisclaimer,
	}
	if attempt.SubmittedBy != nil {
		dto.SubmittedBy = *attempt.SubmittedBy
	}
	if attempt.SubmittedAt != nil {
		dto.ElapsedSeconds = int(attempt.SubmittedAt.Sub(attempt.StartedAt).Seconds())
	}
	if published, ok := s.publishedScoreForAttempt(ctx, attempt, overall); ok {
		value := published.Value
		dto.PublishedScore = &value
		dto.PublishedScale = published.Scale
		dto.ScoreIsEstimate = published.Estimate
	}
	return dto
}

// publishedScoreForAttempt maps the overall onto the sitting's exam scale, when
// the version states one (WO 22 Stage I.3.6).
func (s *Service) publishedScoreForAttempt(
	ctx context.Context, attempt *sqlc.AssessExamAttempt, overall float64,
) (domain.PublishedScore, bool) {
	scoring, err := s.versionScoringForAttempt(ctx, attempt)
	if err != nil || len(scoring) == 0 {
		return domain.PublishedScore{}, false
	}
	return domain.ScaleScore(scoring, overall)
}

// versionScoringForAttempt resolves the exam version a sitting belongs to: a
// mock test through its blueprint, a template through its own version.
func (s *Service) versionScoringForAttempt(
	ctx context.Context, attempt *sqlc.AssessExamAttempt,
) (json.RawMessage, error) {
	if attempt.MockTestID != nil {
		mt, err := s.repo.GetMockTestByID(ctx, *attempt.MockTestID)
		if err != nil || mt == nil {
			return nil, err
		}
		bp, err := s.repo.GetBlueprintByID(ctx, mt.BlueprintID)
		if err != nil || bp == nil {
			return nil, err
		}
		version, err := s.repo.GetExamVersionByID(ctx, bp.VersionID)
		if err != nil || version == nil {
			return nil, err
		}
		return version.Scoring, nil
	}

	exam, err := s.repo.GetExamByID(ctx, attempt.ExamID)
	if err != nil || exam == nil || exam.VersionID == nil {
		return nil, err
	}
	version, err := s.repo.GetExamVersionByID(ctx, *exam.VersionID)
	if err != nil || version == nil {
		return nil, err
	}
	return version.Scoring, nil
}

// ListUserAttempts returns the caller's sittings, newest first, with the total.
func (s *Service) ListUserAttempts(
	ctx context.Context, userID uuid.UUID, limit, offset int32,
) ([]ExamAttemptDTO, int64, error) {
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
	exams := make(map[uuid.UUID]*sqlc.AssessExam)
	attempts := make([]ExamAttemptDTO, len(rows))
	for i := range rows {
		exam, seen := exams[rows[i].ExamID]
		if !seen {
			if found, eErr := s.repo.GetExamByID(ctx, rows[i].ExamID); eErr == nil {
				exam = found
			}
			exams[rows[i].ExamID] = exam
		}
		attempts[i] = s.attemptDTO(&rows[i], exam, now, false)
	}
	return attempts, total, nil
}

// SittingsToday reports how many sittings the caller has started today and the daily limit.
func (s *Service) SittingsToday(ctx context.Context, userID uuid.UUID) (SittingsToday, error) {
	if s.repo == nil {
		return SittingsToday{Limit: s.dailyLimit}, nil
	}
	return s.sittingsToday(ctx, userID, s.clock.Now().UTC())
}

func (s *Service) sittingsToday(ctx context.Context, userID uuid.UUID, now time.Time) (SittingsToday, error) {
	loc, err := time.LoadLocation(HoChiMinhTimeZone)
	if err != nil {
		loc = time.FixedZone("ICT", 7*60*60)
	}
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).UTC()
	count, err := s.repo.CountUserAttemptsToday(ctx, userID, start, start.Add(24*time.Hour))
	if err != nil {
		return SittingsToday{}, err
	}
	return SittingsToday{Used: int(count), Limit: s.dailyLimit}, nil
}

// AttemptHistory implements contract.Reader.
func (s *Service) AttemptHistory(
	ctx context.Context, userID uuid.UUID, limit, offset int,
) ([]examcontract.AttemptSummary, int, error) {
	attempts, total, err := s.ListUserAttempts(ctx, userID, clampInt32(limit), clampInt32(offset))
	if err != nil {
		return nil, 0, err
	}
	summaries := make([]examcontract.AttemptSummary, len(attempts))
	for i, a := range attempts {
		summaries[i] = examcontract.AttemptSummary{
			ID:          a.ID,
			ExamID:      a.ExamID,
			ExamSlug:    a.ExamSlug,
			ExamTitle:   a.ExamTitle,
			Mode:        a.Mode,
			StartedAt:   a.StartedAt,
			SubmittedAt: a.SubmittedAt,
			Status:      a.Status,
		}
	}
	return summaries, int(total), nil
}

// LatestBand implements contract.Reader: the band of the latest sitting with a scored report.
func (s *Service) LatestBand(ctx context.Context, userID uuid.UUID) (*string, error) {
	attempts, _, err := s.ListUserAttempts(ctx, userID, 1, 0)
	if err != nil || len(attempts) == 0 {
		return nil, err
	}
	rep, rErr := s.repo.GetScoreReportByAttemptID(ctx, attempts[0].ID)
	if rErr != nil || rep == nil || rep.Status == domain.ReportStatusPending || rep.OverallBand == "" {
		return nil, nil //nolint:nilerr // no report yet is no band, not a failure
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
		return false, nil //nolint:nilerr // an activity that cannot be resolved is not a pool item
	}
	return act.CourseSlug == ExamPoolCourseSlug, nil
}

// ListeningPlayPolicy says how many plays a clip has in the caller's sitting.
//
// The sitting must be the caller's, still open, and hold the clip — in exam
// mode, in the section that is open now. Exam mode allows one play, practice
// mode three. listening asks this through an interface it declares.
func (s *Service) ListeningPlayPolicy(ctx context.Context, userID, sittingID, versionID uuid.UUID) (int, error) {
	attempt, err := s.sittingFor(ctx, userID, sittingID)
	if err != nil {
		return 0, domain.ErrPlayNotAllowed
	}
	now := s.clock.Now().UTC()
	if attempt.Status != domain.StatusInProgress || domain.IsPastDeadline(attempt.DeadlineAt, now) {
		return 0, domain.ErrPlayNotAllowed
	}
	current := s.currentSection(attempt, now)
	for _, sec := range decodeSections(attempt) {
		for _, act := range sec.Activities {
			if act.ContentVersionID != versionID || !isListeningKind(act.Kind) {
				continue
			}
			if attempt.Mode == domain.ModeExam && sec.SectionPosition != current {
				return 0, domain.ErrPlayNotAllowed
			}
			return domain.ListeningPlays(attempt.Mode), nil
		}
	}
	return 0, domain.ErrPlayNotAllowed
}

// isListeningKind reports whether an activity kind is heard rather than read.
func isListeningKind(kind string) bool {
	switch kind {
	case kindListeningComprehension, kindPhotoDescription, kindQuestionResponse:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func (s *Service) sittingFor(ctx context.Context, userID, attemptID uuid.UUID) (*sqlc.AssessExamAttempt, error) {
	if s.repo == nil {
		return nil, domain.ErrAttemptNotFound
	}
	attempt, err := s.repo.GetExamAttemptForUser(ctx, attemptID, userID)
	if err != nil || attempt == nil {
		return nil, domain.ErrAttemptNotFound
	}
	return attempt, nil
}

func (s *Service) currentSection(attempt *sqlc.AssessExamAttempt, now time.Time) int {
	if attempt.Mode != domain.ModeExam {
		return int(attempt.CurrentSection)
	}
	return domain.CurrentExamSection(attempt.StartedAt, int(attempt.CurrentSection), now)
}

// persistSection stores the section an exam-mode sitting's clock has moved it to.
func (s *Service) persistSection(ctx context.Context, attempt *sqlc.AssessExamAttempt, now time.Time) {
	if attempt.Status != domain.StatusInProgress || attempt.Mode != domain.ModeExam {
		return
	}
	current := s.currentSection(attempt, now)
	if current <= int(attempt.CurrentSection) {
		return
	}
	if updated, err := s.repo.UpdateCurrentSection(
		ctx, attempt.ID, clampInt32(current), now,
	); err == nil && updated != nil {
		*attempt = *updated
	}
}

func (s *Service) attemptDTO(
	attempt *sqlc.AssessExamAttempt, exam *sqlc.AssessExam, now time.Time, withContent bool,
) ExamAttemptDTO {
	current := s.currentSection(attempt, now)
	dto := ExamAttemptDTO{
		ID:                    attempt.ID,
		ExamID:                attempt.ExamID,
		Mode:                  attempt.Mode,
		ChosenDurationMinutes: int(attempt.ChosenDurationMinutes),
		Unlimited:             domain.IsUnlimitedPractice(attempt.Mode, int(attempt.ChosenDurationMinutes)),
		StartedAt:             attempt.StartedAt,
		DeadlineAt:            attempt.DeadlineAt,
		RemainingSeconds:      domain.RemainingSeconds(attempt.DeadlineAt, now),
		CurrentSection:        current,
		Status:                attempt.Status,
		SubmittedAt:           attempt.SubmittedAt,
		ServerTime:            now,
	}
	if exam != nil {
		dto.ExamSlug = exam.Slug
		dto.ExamTitle = exam.TitleEn
		dto.Level = exam.Level
	}
	if attempt.Status != domain.StatusInProgress {
		dto.RemainingSeconds = 0
	} else if attempt.Mode == domain.ModeExam {
		end := sectionEnd(attempt, current)
		dto.SectionDeadlineAt = &end
		dto.SectionRemainingSeconds = sectionRemaining(attempt, current, now)
	}
	if withContent {
		dto.SectionActivities = decodeSections(attempt)
		dto.DraftAnswers = decodeAnswers(attempt)
	}
	return dto
}

func sectionEnd(attempt *sqlc.AssessExamAttempt, section int) time.Time {
	end := domain.SectionDeadline(attempt.StartedAt, section)
	if end.After(attempt.DeadlineAt) {
		end = attempt.DeadlineAt
	}
	return end
}

func sectionRemaining(attempt *sqlc.AssessExamAttempt, section int, now time.Time) *int {
	if attempt.Mode != domain.ModeExam {
		return nil
	}
	remaining := domain.RemainingSeconds(sectionEnd(attempt, section), now)
	return &remaining
}

func decodeSections(attempt *sqlc.AssessExamAttempt) []SectionActivities {
	var drawn []SectionActivities
	_ = json.Unmarshal(attempt.SectionActivities, &drawn)
	return drawn
}

func decodeAnswers(attempt *sqlc.AssessExamAttempt) map[string]json.RawMessage {
	answers := make(map[string]json.RawMessage)
	_ = json.Unmarshal(attempt.DraftAnswers, &answers)
	return answers
}

func activityPositions(sections []SectionActivities) map[uuid.UUID]int {
	positions := make(map[uuid.UUID]int)
	for _, sec := range sections {
		for _, act := range sec.Activities {
			positions[act.ID] = sec.SectionPosition
		}
	}
	return positions
}

func skeletonOutcomes(sections []SectionActivities) []domain.SectionOutcome {
	outcomes := make([]domain.SectionOutcome, len(sections))
	for i, sec := range sections {
		items := make([]domain.ItemOutcome, len(sec.Activities))
		for j, act := range sec.Activities {
			items[j] = domain.ItemOutcome{
				ActivityID:       act.ID,
				ContentVersionID: act.ContentVersionID,
				Kind:             act.Kind,
				Status:           domain.ItemPending,
			}
		}
		outcomes[i] = domain.SectionOutcome{
			Position: sec.SectionPosition,
			Skill:    sec.Skill,
			Status:   domain.SectionPending,
			Items:    items,
		}
	}
	return outcomes
}

func isEmptyAnswer(raw json.RawMessage) bool {
	switch string(raw) {
	case "", "null", "{}", `""`, "[]":
		return true
	default:
		return false
	}
}

func (s *Service) integritySignals(ctx context.Context, attemptID uuid.UUID) ([]IntegritySignalDTO, []byte) {
	events, err := s.repo.ListIntegrityEvents(ctx, attemptID)
	if err != nil {
		slog.WarnContext(ctx, "could not list integrity signals", "attempt_id", attemptID, "error", err)
	}
	counts := make(map[string]int)
	var order []string
	for _, ev := range events {
		if _, seen := counts[ev.Kind]; !seen {
			order = append(order, ev.Kind)
		}
		counts[ev.Kind]++
	}
	signals := make([]IntegritySignalDTO, 0, len(order))
	for _, kind := range order {
		signals = append(signals, IntegritySignalDTO{Kind: kind, Count: counts[kind]})
	}
	b, _ := json.Marshal(signals)
	return signals, b
}

func numericOf(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(v, 'f', 2, 64))
	return n
}

func clampInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < 0 {
		return 0
	}
	return int32(v)
}
