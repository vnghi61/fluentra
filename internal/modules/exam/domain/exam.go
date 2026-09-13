package domain

import (
	"time"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

const (
	ModeExam     = "exam"
	ModePractice = "practice"

	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusExpired    = "expired"

	ReportStatusPending = "pending"
	ReportStatusReady   = "ready"
	ReportStatusPartial = "partial"

	SkillListening = "listening"
	SkillReading   = "reading"
	SkillWriting   = "writing"
	SkillSpeaking  = "speaking"

	SubmittedByLearner = "learner"
	SubmittedByExpiry  = "expiry"

	MinPracticeDurationMinutes = 10
	MaxPracticeDurationMinutes = 180
	DefaultExamDurationMinutes = 75

	NetworkGracePeriod = 5 * time.Second
)

// Section durations in exam mode.
const (
	ExamListeningMinutes = 20
	ExamReadingMinutes   = 25
	ExamWritingMinutes   = 20
	ExamSpeakingMinutes  = 10
)

var (
	ErrExamNotFound          = apperr.New(apperr.NotFound, "EXAM_NOT_FOUND", "exam not found")
	ErrAttemptNotFound       = apperr.New(apperr.NotFound, "EXAM_ATTEMPT_NOT_FOUND", "exam attempt not found")
	ErrAttemptInProgress     = apperr.New(apperr.Conflict, "ATTEMPT_IN_PROGRESS", "an attempt is already in progress")
	ErrAttemptExpired        = apperr.New(apperr.Conflict, "ATTEMPT_EXPIRED", "exam attempt has expired")
	ErrExamAlreadySubmitted  = apperr.New(apperr.Conflict, "EXAM_ALREADY_SUBMITTED", "exam has already been submitted")
	ErrSectionTimeExpired    = apperr.New(apperr.Conflict, "SECTION_TIME_EXPIRED", "section time is over")
	ErrExamDailyLimitReached      = apperr.New(apperr.RateLimited, "EXAM_DAILY_LIMIT_REACHED", "daily exam sittings limit reached")
	ErrInsufficientItems          = apperr.New(apperr.Conflict, "INSUFFICIENT_ITEMS", "insufficient pool items to generate sitting")
	ErrReportNotReady             = apperr.New(apperr.NotFound, "REPORT_NOT_READY", "score report is not ready yet")
	ErrUnauthorizedAttempt        = apperr.New(apperr.Forbidden, "UNAUTHORIZED_ATTEMPT_ACCESS", "unauthorized access to attempt")
	ErrInvalidSectionProgression  = apperr.New(apperr.BadRequest, "INVALID_SECTION_PROGRESSION", "invalid section progression")
	ErrSectionAlreadyCompleted    = apperr.New(apperr.BadRequest, "SECTION_ALREADY_COMPLETED", "section already completed")
)

// ClampPracticeDuration clamps duration in minutes between 10 and 180.
func ClampPracticeDuration(d int) int {
	if d < MinPracticeDurationMinutes {
		return MinPracticeDurationMinutes
	}
	if d > MaxPracticeDurationMinutes {
		return MaxPracticeDurationMinutes
	}
	return d
}

// IsPastDeadline returns true if now exceeds deadlineAt + 5s grace period.
func IsPastDeadline(deadlineAt, now time.Time) bool {
	return now.After(deadlineAt.Add(NetworkGracePeriod))
}

// RemainingSeconds calculates remaining duration until deadline from now.
func RemainingSeconds(deadlineAt, now time.Time) int {
	diff := deadlineAt.Sub(now).Seconds()
	if diff < 0 {
		return 0
	}
	return int(diff)
}

// SectionDeadline calculates the deadline for a specific 1-indexed section in exam mode.
func SectionDeadline(startedAt time.Time, section int) time.Time {
	switch section {
	case 1:
		return startedAt.Add(ExamListeningMinutes * time.Minute)
	case 2:
		return startedAt.Add((ExamListeningMinutes + ExamReadingMinutes) * time.Minute)
	case 3:
		return startedAt.Add((ExamListeningMinutes + ExamReadingMinutes + ExamWritingMinutes) * time.Minute)
	case 4:
		return startedAt.Add((ExamListeningMinutes + ExamReadingMinutes + ExamWritingMinutes + ExamSpeakingMinutes) * time.Minute)
	default:
		return startedAt.Add(DefaultExamDurationMinutes * time.Minute)
	}
}

// EstimateCEFRBand converts 0-100 overall score to a CEFR band.
func EstimateCEFRBand(score float64) string {
	switch {
	case score >= 90:
		return "C1"
	case score >= 75:
		return "B2"
	case score >= 60:
		return "B1"
	case score >= 40:
		return "A2"
	default:
		return "A1"
	}
}
