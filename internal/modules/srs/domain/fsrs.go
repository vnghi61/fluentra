// Package domain implements pure domain logic for spaced repetition scheduling (FSRS v4.5).
// All functions in this package are pure mathematical transformations over (card, rating, now) -> next card.
// No I/O, database access, clock calls, or networking are permitted.
package domain

import (
	"math"
	"time"
)

// State represents the lifecycle phase of a review card in FSRS.
type State string

// The four card lifecycle states FSRS distinguishes.
const (
	StateNew        State = "new"
	StateLearning   State = "learning"
	StateReview     State = "review"
	StateRelearning State = "relearning"
)

// Rating represents the learner's recall assessment for an item.
type Rating string

// The four grades a learner may give. They travel the wire as these strings,
// never as 1..4: a numeric wire format encodes the keyboard map, and a client
// that gets the mapping wrong is wrong silently.
const (
	RatingAgain Rating = "again" // Complete blackout or incorrect response
	RatingHard  Rating = "hard"  // Recalled with significant hesitation
	RatingGood  Rating = "good"  // Correct recall with normal effort
	RatingEasy  Rating = "easy"  // Instant, effortless recall
)

// Value maps the rating enum to the numerical value 1..4 the FSRS formulas use.
func (r Rating) Value() int {
	switch r {
	case RatingAgain:
		return 1
	case RatingHard:
		return 2
	case RatingGood:
		return 3
	case RatingEasy:
		return 4
	default:
		return 3
	}
}

// IsValid checks if a rating is one of the four allowed enum values.
func (r Rating) IsValid() bool {
	return r == RatingAgain || r == RatingHard || r == RatingGood || r == RatingEasy
}

// Parameters holds the 19 weights and scheduling settings for FSRS v4.5.
// Reference: open-spaced-repetition/fsrs v4.5 published default weights.
// The weights are not independent knobs and must not be altered arbitrarily.
type Parameters struct {
	// Weights w0..w18
	W [19]float64 `json:"w"`

	// RequestRetention is the target retention rate (e.g. 0.90 for 90% retention).
	RequestRetention float64 `json:"request_retention"`

	// MaxInterval is the maximum allowed scheduling interval in days (e.g. 36500 = 100 years).
	MaxInterval int `json:"max_interval"`
}

// DefaultParameters returns the published standard parameters for FSRS v4.5.
// Citing: Open Spaced Repetition (open-spaced-repetition/fsrs.js / py-fsrs).
func DefaultParameters() Parameters {
	return Parameters{
		W: [19]float64{
			0.4072, 1.1829, 3.1262, 15.4722, // w0..w3: initial stability for again, hard, good, easy
			7.2102, 0.5316, // w4, w5: initial difficulty
			1.0651, 0.0234, // w6, w7: difficulty update & mean reversion
			1.616, 0.1544, 1.0824, // w8..w10: stability update on success
			1.9813, 0.0953, 0.2975, 2.2042, // w11..w14: stability update on lapse (again)
			0.2407, 2.9466, // w15, w16: hard penalty & easy bonus
			0.5034, 0.6567, // w17, w18: short-term stability weights
		},
		RequestRetention: 0.90,
		MaxInterval:      36500,
	}
}

// CardState represents the pure domain model of a review card.
type CardState struct {
	Stability    float64   `json:"stability"`
	Difficulty   float64   `json:"difficulty"`
	State        State     `json:"state"`
	Reps         int       `json:"reps"`
	Lapses       int       `json:"lapses"`
	LastReviewAt time.Time `json:"last_review_at"`
	DueAt        time.Time `json:"due_at"`
}

// Factor is the decay scaling factor ensuring R(S, S) = 0.9 for standard power decay: 19/81.
const Factor = 19.0 / 81.0

// Retrievability computes the recall probability R after elapsedDays for a card with stability S.
// Formula: R(t, S) = (1 + Factor * (t / S))^(-0.5).
func Retrievability(elapsedDays float64, stability float64) float64 {
	if stability <= 0 {
		return 0.0
	}
	if elapsedDays <= 0 {
		return 1.0
	}
	r := math.Pow(1.0+Factor*(elapsedDays/stability), -0.5)
	if math.IsNaN(r) || r < 0.0 {
		return 0.0
	}
	if r > 1.0 {
		return 1.0
	}
	return r
}

// InitStability calculates initial stability S0 for a given rating (1..4).
// S0(G) = w[G-1].
func InitStability(rating Rating, params Parameters) float64 {
	val := rating.Value()
	if val < 1 || val > 4 {
		val = 3
	}
	s := params.W[val-1]
	if s < 0.1 {
		return 0.1
	}
	return s
}

// InitDifficulty calculates initial difficulty D0 for a given rating (1..4).
// D0(G) = clamp(w4 - e^(w5 * (G - 1)) + 1, 1.0, 10.0).
func InitDifficulty(rating Rating, params Parameters) float64 {
	val := float64(rating.Value())
	d := params.W[4] - math.Exp(params.W[5]*(val-1.0)) + 1.0
	return clamp(d, 1.0, 10.0)
}

// NextDifficulty updates card difficulty following a review.
// deltaD = -w6 * (G - 3)
// D' = D + deltaD * ((10 - D) / 9)
// D_next = clamp(w7 * D0(3) + (1 - w7) * D', 1.0, 10.0).
func NextDifficulty(d float64, rating Rating, params Parameters) float64 {
	d = clamp(d, 1.0, 10.0)
	g := float64(rating.Value())
	deltaD := -params.W[6] * (g - 3.0)
	dPrime := d + deltaD*((10.0-d)/9.0)
	d0Good := InitDifficulty(RatingGood, params)
	dNext := params.W[7]*d0Good + (1.0-params.W[7])*dPrime
	return clamp(dNext, 1.0, 10.0)
}

// NextStability calculates next stability after a successful recall (Good, Hard, Easy) or a lapse (Again).
func NextStability(s float64, d float64, r float64, rating Rating, params Parameters) float64 {
	if s <= 0 {
		return InitStability(rating, params)
	}
	d = clamp(d, 1.0, 10.0)
	r = clamp(r, 0.01, 1.0)

	if rating == RatingAgain {
		// Lapse stability: S'_f(D, S, R) = w11 * D^(-w12) * ((S + 1)^w13 - 1) * e^(w14 * (1 - R))
		sLapse := params.W[11] * math.Pow(d, -params.W[12]) *
			(math.Pow(s+1.0, params.W[13]) - 1.0) * math.Exp(params.W[14]*(1.0-r))
		if math.IsNaN(sLapse) || sLapse < 0.1 {
			sLapse = 0.1
		}
		// In FSRS, lapse stability reduces prior stability but does not exceed it.
		if sLapse > s {
			sLapse = s
		}
		return sLapse
	}

	// Success stability: S'_r(D, S, R, G) = S * (1 + e^w8 * (11 - D) * S^(-w9) * (e^(w10 * (1 - R)) - 1) * h(G))
	var h float64
	switch rating {
	case RatingHard:
		h = params.W[15]
	case RatingGood:
		h = 1.0
	case RatingEasy:
		h = params.W[16]
	default:
		h = 1.0
	}

	factor := math.Exp(params.W[8]) * (11.0 - d) * math.Pow(s, -params.W[9]) *
		(math.Exp(params.W[10]*(1.0-r)) - 1.0) * h
	if factor < 0 {
		factor = 0
	}
	nextS := s * (1.0 + factor)
	if math.IsNaN(nextS) || nextS < 0.1 {
		return 0.1
	}
	return nextS
}

// NextInterval calculates scheduling interval in integer days given stability S and target retention.
// Formula: I(S, r) = (S / Factor) * (r^(-2) - 1).
func NextInterval(stability float64, requestRetention float64, maxInterval int) int {
	if stability <= 0 {
		return 1
	}
	if requestRetention <= 0 || requestRetention >= 1.0 {
		requestRetention = 0.90
	}
	if maxInterval <= 0 {
		maxInterval = 36500
	}

	intervalFloat := (stability / Factor) * (math.Pow(requestRetention, -2.0) - 1.0)
	interval := int(math.Round(intervalFloat))
	if interval < 1 {
		interval = 1
	}
	if interval > maxInterval {
		interval = maxInterval
	}
	return interval
}

// relearnDelay is how soon a lapsed card comes back inside the same session.
// It is deliberately shorter than any day-scale interval, which is what keeps
// `again` the nearest of the four grades at every hour of the day.
const relearnDelay = 10 * time.Minute

// normalise coerces a card read back from the database into the domain the FSRS
// formulas are defined over, so Schedule stays total over its input type.
func normalise(card CardState, rating Rating, params Parameters) CardState {
	switch {
	case card.Stability < 0.1:
		card.Stability = InitStability(rating, params)
	case card.Stability > 36500.0:
		card.Stability = 36500.0
	}

	if card.Difficulty < 1.0 || card.Difficulty > 10.0 {
		card.Difficulty = InitDifficulty(rating, params)
	}

	switch card.State {
	case StateNew, StateLearning, StateReview, StateRelearning:
	default:
		card.State = StateNew
	}

	return card
}

// elapsedDays is the time since the previous review, in days, and never negative.
func elapsedDays(card CardState, now time.Time) float64 {
	if card.LastReviewAt.IsZero() || !now.After(card.LastReviewAt) {
		return 0
	}
	return now.Sub(card.LastReviewAt).Hours() / 24.0
}

// dueAfter converts a stability into an absolute due timestamp.
//
// The interval is added to `now` itself rather than to the start of its day.
// Truncating first looks harmless and is not: a one-day interval scheduled at
// 23:55 would land five minutes away, i.e. sooner than the ten-minute relearning
// step, and `hard` would come back before `again`. The due queue compares against
// the end of the learner's local day, so an exact timestamp is still "due" for
// the whole of its day.
func dueAfter(now time.Time, stability float64, params Parameters) (time.Time, int) {
	days := NextInterval(stability, params.RequestRetention, params.MaxInterval)
	return now.AddDate(0, 0, days), days
}

// Schedule is the pure state transition function: (CardState, Rating, now, Parameters) -> CardState.
func Schedule(card CardState, rating Rating, now time.Time, params Parameters) CardState {
	if !rating.IsValid() {
		rating = RatingGood
	}
	card = normalise(card, rating, params)

	next := card
	next.LastReviewAt = now

	if card.State == StateNew {
		return scheduleFirstReview(next, rating, now, params)
	}
	return scheduleRepeatReview(card, next, rating, now, params)
}

// scheduleFirstReview handles a card the learner has never graded before.
func scheduleFirstReview(next CardState, rating Rating, now time.Time, params Parameters) CardState {
	next.Stability = InitStability(rating, params)
	next.Difficulty = InitDifficulty(rating, params)
	next.Reps = 1

	if rating == RatingAgain {
		next.Lapses = 1
		next.State = StateLearning
		next.DueAt = now.Add(relearnDelay)
		return next
	}

	next.Lapses = 0
	if rating == RatingEasy {
		next.State = StateReview
	} else {
		next.State = StateLearning
	}
	next.DueAt, _ = dueAfter(now, next.Stability, params)
	return next
}

// scheduleRepeatReview handles learning, relearning and review cards alike: the
// stability and difficulty updates are the same, only the state transition differs.
func scheduleRepeatReview(card, next CardState, rating Rating, now time.Time, params Parameters) CardState {
	retrievability := Retrievability(elapsedDays(card, now), card.Stability)
	next.Difficulty = NextDifficulty(card.Difficulty, rating, params)
	next.Stability = NextStability(card.Stability, next.Difficulty, retrievability, rating, params)
	next.Reps++

	if rating == RatingAgain {
		next.Lapses++
		if card.State == StateReview {
			next.State = StateRelearning
		}
		next.DueAt = now.Add(relearnDelay)
		return next
	}

	next.State = StateReview
	next.DueAt, _ = dueAfter(now, next.Stability, params)
	return next
}

func clamp(val, lo, hi float64) float64 {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}
