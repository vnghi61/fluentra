package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Placement session statuses.
const (
	PlacementInProgress = "in_progress"
	PlacementCompleted  = "completed"
	PlacementExpired    = "expired"
)

// Where the optional writing and speaking part stands.
const (
	ProductiveOffered    = "offered"
	ProductiveSkipped    = "skipped"
	ProductiveInProgress = "in_progress"
	ProductiveSubmitted  = "submitted"
	ProductiveGraded     = "graded"
)

// Which part of the test an item was served in.
const (
	PlacementPartAdaptive   = "adaptive"
	PlacementPartProductive = "productive"
)

// Placement limits (work order 13 §2 and §3.5).
const (
	PlacementTimeLimit   = 20 * time.Minute
	ProductiveTimeLimit  = 8 * time.Minute
	PlacementAnswerGrace = 5 * time.Second
	PlacementRetakeAfter = 30 * 24 * time.Hour
	// PlacementMasteryConfidence is the confidence a placement writes into
	// learn.skill_mastery, and only where the existing confidence is lower.
	PlacementMasteryConfidence = 0.40
	// PlacementListeningPlays is how many times a placement clip may be played.
	PlacementListeningPlays = 1
)

// Placement errors.
var (
	ErrPlacementNotFound = apperr.New(apperr.NotFound, "PLACEMENT_SESSION_NOT_FOUND",
		"Placement session not found.")
	ErrPlacementInProgress = apperr.New(apperr.Conflict, "PLACEMENT_IN_PROGRESS",
		"A placement test is already in progress.")
	ErrPlacementRetakeTooSoon = apperr.New(apperr.Conflict, "PLACEMENT_RETAKE_TOO_SOON",
		"A placement test can be retaken 30 days after the last one.")
	ErrPlacementUnavailable = apperr.New(apperr.Conflict, "PLACEMENT_UNAVAILABLE",
		"The placement test is not available yet.")
	ErrPlacementExpired = apperr.New(apperr.Conflict, "PLACEMENT_SESSION_EXPIRED",
		"The time for this placement test is up.")
	ErrPlacementFinished = apperr.New(apperr.Conflict, "PLACEMENT_SESSION_FINISHED",
		"This part of the placement test is finished.")
	ErrPlacementNotCurrentItem = apperr.New(apperr.Conflict, "PLACEMENT_NOT_CURRENT_ITEM",
		"That item is not the one being answered.")
	ErrPlacementConflict = apperr.New(apperr.Conflict, "PLACEMENT_SESSION_CHANGED",
		"The placement session changed while this answer was recorded. Reload it.")
	ErrProductiveUnavailable = apperr.New(apperr.Conflict, "PLACEMENT_PRODUCTIVE_UNAVAILABLE",
		"The writing and speaking part is not available for this session.")
)

// PlacementItem is one item served in a session, in order.
type PlacementItem struct {
	ActivityID uuid.UUID  `json:"activity_id"`
	Kind       string     `json:"kind"`
	Skill      string     `json:"skill"`
	Band       string     `json:"band"`
	Part       string     `json:"part"`
	ServedAt   time.Time  `json:"served_at"`
	AttemptID  *uuid.UUID `json:"attempt_id,omitempty"`
	AnsweredAt *time.Time `json:"answered_at,omitempty"`
	// Status is the attempt's grading state for a writing or speaking item.
	Status   string `json:"status,omitempty"`
	Score    *int   `json:"score,omitempty"`
	MaxScore int    `json:"max_score,omitempty"`
}

// Answered reports whether the item has an attempt.
func (i PlacementItem) Answered() bool {
	return i.AttemptID != nil
}

// PlacementSession is one adaptive placement test and its optional last part.
type PlacementSession struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	Status               string
	Stage                string
	StartedAt            time.Time
	DeadlineAt           time.Time
	Estimate             PlacementEstimate
	Items                []PlacementItem
	Version              int
	ProductiveStatus     string
	ProductiveDeadlineAt *time.Time
	ResultID             *uuid.UUID
	CompletedAt          *time.Time
}

// CurrentItem is the adaptive item waiting for an answer, if there is one.
func (s *PlacementSession) CurrentItem() *PlacementItem {
	for i := len(s.Items) - 1; i >= 0; i-- {
		if s.Items[i].Part != PlacementPartAdaptive {
			continue
		}
		if s.Items[i].Answered() {
			return nil
		}
		return &s.Items[i]
	}
	return nil
}

// LastAnswered is the most recently answered adaptive item.
func (s *PlacementSession) LastAnswered() *PlacementItem {
	for i := len(s.Items) - 1; i >= 0; i-- {
		if s.Items[i].Part == PlacementPartAdaptive && s.Items[i].Answered() {
			return &s.Items[i]
		}
	}
	return nil
}

// Progress counts the adaptive items served, by skill.
func (s *PlacementSession) Progress() PlacementProgress {
	var p PlacementProgress
	for _, item := range s.Items {
		if item.Part != PlacementPartAdaptive {
			continue
		}
		switch item.Skill {
		case SkillVocabulary:
			p.Vocabulary++
		case SkillGrammar:
			p.Grammar++
		case SkillReading:
			p.Reading++
		case SkillListening:
			p.Listening++
		}
	}
	return p
}

// ProductiveItems are the writing and speaking items, if the part was started.
func (s *PlacementSession) ProductiveItems() []*PlacementItem {
	var out []*PlacementItem
	for i := range s.Items {
		if s.Items[i].Part == PlacementPartProductive {
			out = append(out, &s.Items[i])
		}
	}
	return out
}

// Overdue reports whether the adaptive part's time, grace included, has run out.
func (s *PlacementSession) Overdue(now time.Time) bool {
	return now.After(s.DeadlineAt.Add(PlacementAnswerGrace))
}

// RemainingSeconds is the adaptive part's time left, never negative.
func (s *PlacementSession) RemainingSeconds(now time.Time) int {
	return max(0, int(s.DeadlineAt.Sub(now).Seconds()))
}

// PlacementResult is a completed placement. PerSkill holds the band of each
// measured skill; writing and speaking join it when graded.
type PlacementResult struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	SessionID *uuid.UUID
	Level     string
	PerSkill  map[string]SkillEstimate
	TakenAt   time.Time
}

// PlacementOverview is what GET /me/placement returns.
type PlacementOverview struct {
	Result            *PlacementResult
	ActiveSession     *PlacementSession
	RetakeAvailableAt *time.Time
	InviteAvailable   bool
}
