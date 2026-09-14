// Package domain holds the exam module's rules: the clock, sections and scoring.
package domain

import (
	"encoding/json"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Sitting modes, statuses, report statuses, skills, submitters and limits.
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

	// SectionCount is how many sections every sitting has.
	SectionCount = 4

	// MaxAnswerBytes bounds one item's saved answer. An essay is the longest
	// answer a sitting holds; 32 KiB is several times a 150-word essay and far
	// less than a client could otherwise store in a sitting row.
	MaxAnswerBytes = 32 * 1024
	// MaxIntegrityEventsPerSave bounds the signals one autosave may record.
	MaxIntegrityEventsPerSave = 50

	// ReportSettleLimit is how long a report waits for asynchronous grades before
	// the items still waiting are reported as not scored.
	ReportSettleLimit = time.Hour
)

// Integrity signal kinds a client may record. Anything else is dropped.
const (
	IntegrityTabHidden     = "tab_hidden"
	IntegrityWindowBlurred = "window_blurred"
	IntegrityPaste         = "paste"
)

const (
	listeningPlaysExamMode = 1
	listeningPlaysPractice = 3
	maxSectionScore        = 100
	percentMultiplier      = 100.0
)

// Section and item statuses in a stored report.
const (
	SectionScored    = "scored"
	SectionPending   = "pending"
	SectionNotScored = "not_scored"

	ItemGraded     = "graded"
	ItemPending    = "pending"
	ItemFailed     = "failed"
	ItemUnanswered = "unanswered"
)

// Errors the exam module returns.
var (
	ErrExamNotFound          = apperr.New(apperr.NotFound, "EXAM_NOT_FOUND", "exam not found")
	ErrAttemptNotFound       = apperr.New(apperr.NotFound, "EXAM_ATTEMPT_NOT_FOUND", "exam attempt not found")
	ErrAttemptInProgress     = apperr.New(apperr.Conflict, "ATTEMPT_IN_PROGRESS", "an attempt is already in progress")
	ErrAttemptExpired        = apperr.New(apperr.Conflict, "ATTEMPT_EXPIRED", "exam attempt has expired")
	ErrExamAlreadySubmitted  = apperr.New(apperr.Conflict, "EXAM_ALREADY_SUBMITTED", "exam has already been submitted")
	ErrSectionTimeExpired    = apperr.New(apperr.Conflict, "SECTION_TIME_EXPIRED", "section time is over")
	ErrExamDailyLimitReached = apperr.New(
		apperr.RateLimited, "EXAM_DAILY_LIMIT_REACHED", "daily exam sittings limit reached",
	)
	ErrInsufficientItems = apperr.New(
		apperr.Conflict, "INSUFFICIENT_ITEMS", "insufficient pool items to generate sitting",
	)
	ErrReportNotReady      = apperr.New(apperr.NotFound, "REPORT_NOT_READY", "score report is not ready yet")
	ErrUnauthorizedAttempt = apperr.New(
		apperr.Forbidden, "UNAUTHORIZED_ATTEMPT_ACCESS", "unauthorized access to attempt",
	)
	ErrInvalidSectionProgression = apperr.New(
		apperr.BadRequest, "INVALID_SECTION_PROGRESSION", "invalid section progression",
	)
	ErrSectionAlreadyCompleted = apperr.New(apperr.BadRequest, "SECTION_ALREADY_COMPLETED", "section already completed")
	ErrUnknownItem             = apperr.New(
		apperr.BadRequest, "EXAM_ITEM_NOT_IN_SITTING", "the answer is for an item this sitting does not hold",
	)
	ErrAnswerTooLarge = apperr.New(
		apperr.BadRequest, "EXAM_ANSWER_TOO_LARGE", "an answer is larger than a sitting accepts",
	)
	ErrPlayNotAllowed = apperr.New(
		apperr.Forbidden, "LISTENING_PLAY_NOT_ALLOWED", "this clip cannot be played in this sitting",
	)
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

// Section durations in exam mode.
const (
	ExamListeningMinutes = 20
	ExamReadingMinutes   = 25
	ExamWritingMinutes   = 20
	ExamSpeakingMinutes  = 10
)

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
		return startedAt.Add(
			(ExamListeningMinutes + ExamReadingMinutes + ExamWritingMinutes + ExamSpeakingMinutes) * time.Minute,
		)
	default:
		return startedAt.Add(DefaultExamDurationMinutes * time.Minute)
	}
}

// CurrentExamSection is the section an exam-mode sitting is in at now.
//
// A section closes when the learner finishes it or when its time runs out,
// whichever comes first, and nothing reopens it. So the current section is the
// later of the one stored and the first one whose end has not passed. Without
// the clock half, a learner whose listening time ran out could neither save nor
// move on: the section was over, and it was still the current one.
func CurrentExamSection(startedAt time.Time, stored int, now time.Time) int {
	current := stored
	if current < 1 {
		current = 1
	}
	for current < SectionCount && IsPastDeadline(SectionDeadline(startedAt, current), now) {
		current++
	}
	return current
}

// ListeningPlays is how many times a clip may be played in a sitting's mode.
func ListeningPlays(mode string) int {
	if mode == ModePractice {
		return listeningPlaysPractice
	}
	return listeningPlaysExamMode
}

// IsIntegrityKind reports whether a client-sent signal kind is one the report records.
func IsIntegrityKind(kind string) bool {
	switch kind {
	case IntegrityTabHidden, IntegrityWindowBlurred, IntegrityPaste:
		return true
	default:
		return false
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

// ItemOutcome is one drawn item as the stored report holds it.
type ItemOutcome struct {
	ActivityID       uuid.UUID       `json:"activity_id"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	Kind             string          `json:"kind"`
	AttemptID        *uuid.UUID      `json:"attempt_id,omitempty"`
	Status           string          `json:"status"`
	Score            int             `json:"score"`
	MaxScore         int             `json:"max_score"`
	ItemResults      json.RawMessage `json:"item_results,omitempty"`
}

// SectionOutcome is one section of the stored report. Score is 0–100 and is
// absent unless the section is scored.
type SectionOutcome struct {
	Position int           `json:"position"`
	Skill    string        `json:"skill"`
	Status   string        `json:"status"`
	Score    *int          `json:"score,omitempty"`
	MaxScore int           `json:"max_score"`
	Items    []ItemOutcome `json:"items"`
}

// ReportScore is what a set of section outcomes adds up to.
type ReportScore struct {
	Status  string
	Overall float64
	Band    string
}

// ScoreSection sets a section's status and 0–100 score from its items.
//
// An item still grading leaves the section pending. An item whose grading
// failed makes the section not scored — a zero would be a score the learner did
// not earn. An unanswered item scores zero, which the learner did earn.
func ScoreSection(section *SectionOutcome) {
	section.MaxScore = maxSectionScore
	section.Score = nil
	if len(section.Items) == 0 {
		section.Status = SectionNotScored
		return
	}
	pending, failed := false, false
	var total float64
	for _, item := range section.Items {
		switch item.Status {
		case ItemPending:
			pending = true
		case ItemFailed:
			failed = true
		case ItemUnanswered:
			// An unanswered item adds zero.
		default:
			if item.MaxScore > 0 {
				total += float64(item.Score) / float64(item.MaxScore) * percentMultiplier
			}
		}
	}
	switch {
	case failed:
		section.Status = SectionNotScored
	case pending:
		section.Status = SectionPending
	default:
		score := int(math.Round(total / float64(len(section.Items))))
		section.Status = SectionScored
		section.Score = &score
	}
}

// ScoreReport scores every section and adds the sections up: the overall score
// is the mean of the scored sections, and the report is pending while any
// section is, partial when any section is not scored, and ready otherwise.
func ScoreReport(sections []SectionOutcome) ReportScore {
	var total float64
	scored := 0
	pending, notScored := false, false
	for i := range sections {
		ScoreSection(&sections[i])
		switch sections[i].Status {
		case SectionScored:
			total += float64(*sections[i].Score)
			scored++
		case SectionPending:
			pending = true
		default:
			notScored = true
		}
	}
	if len(sections) < SectionCount {
		notScored = true
	}

	result := ReportScore{Status: ReportStatusReady}
	switch {
	case pending:
		result.Status = ReportStatusPending
	case notScored:
		result.Status = ReportStatusPartial
	}
	if scored > 0 {
		result.Overall = math.Round(total / float64(scored))
		result.Band = EstimateCEFRBand(result.Overall)
	}
	return result
}
