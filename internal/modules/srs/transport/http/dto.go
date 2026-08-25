package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/srs/contract"
)

// ReviewCardResponse models a review card in JSON responses.
type ReviewCardResponse struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	ContentVersionID uuid.UUID  `json:"content_version_id"`
	Skill            string     `json:"skill"`
	Stability        float64    `json:"stability"`
	Difficulty       float64    `json:"difficulty"`
	DueAt            time.Time  `json:"due_at"`
	Reps             int        `json:"reps"`
	Lapses           int        `json:"lapses"`
	State            string     `json:"state"`
	SuspendedAt      *time.Time `json:"suspended_at,omitempty"`
}

// DueCountResponse models badge count response.
type DueCountResponse struct {
	DueCount int `json:"due_count"`
}

// ReviewSessionResponse models active review session cards.
type ReviewSessionResponse struct {
	Cards []ReviewCardResponse `json:"cards"`
}

// AnswerReviewRequest models request payload to grade a review card.
type AnswerReviewRequest struct {
	Grade     string `json:"grade"`
	ElapsedMs int    `json:"elapsed_ms"`
}

// AnswerReviewResponse models the result of grading a card.
type AnswerReviewResponse struct {
	Card         ReviewCardResponse `json:"card"`
	NextDueAt    time.Time          `json:"next_due_at"`
	IntervalDays int                `json:"interval_days"`
}

// CompleteReviewSessionRequest models request payload to close a session.
type CompleteReviewSessionRequest struct {
	Reviewed int `json:"reviewed"`
	Correct  int `json:"correct"`
}

// CompleteReviewSessionResponse models summary of a completed review session.
type CompleteReviewSessionResponse struct {
	Reviewed    int       `json:"reviewed"`
	Correct     int       `json:"correct"`
	Minutes     int       `json:"minutes"`
	CompletedAt time.Time `json:"completed_at"`
}

// ForecastItem models expected workload on a specific day.
type ForecastItem struct {
	Date     string `json:"date"`
	DueCount int    `json:"due_count"`
}

// ForecastResponse models 30-day forecast projection.
type ForecastResponse struct {
	Days []ForecastItem `json:"days"`
}

func mapCardResponse(c contract.ReviewCardSummary) ReviewCardResponse {
	return ReviewCardResponse{
		ID:               c.ID,
		UserID:           c.UserID,
		ContentVersionID: c.ContentVersionID,
		Skill:            c.Skill,
		Stability:        c.Stability,
		Difficulty:       c.Difficulty,
		DueAt:            c.DueAt,
		Reps:             c.Reps,
		Lapses:           c.Lapses,
		State:            c.State,
		SuspendedAt:      c.SuspendedAt,
	}
}
