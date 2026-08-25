package domain_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/srs/domain"
)

func TestFSRS_RatingOrdering_IntervalMonotonicity(t *testing.T) {
	params := domain.DefaultParameters()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

	testStates := []domain.CardState{
		{State: domain.StateNew},
		{State: domain.StateLearning, Stability: 2.0, Difficulty: 5.0, LastReviewAt: now.AddDate(0, 0, -2)},
		{State: domain.StateReview, Stability: 10.0, Difficulty: 4.0, LastReviewAt: now.AddDate(0, 0, -10)},
		{State: domain.StateReview, Stability: 30.0, Difficulty: 6.0, LastReviewAt: now.AddDate(0, 0, -35)},
		{State: domain.StateRelearning, Stability: 3.0, Difficulty: 7.0, LastReviewAt: now.AddDate(0, 0, -1)},
	}

	for _, initial := range testStates {
		cardAgain := domain.Schedule(initial, domain.RatingAgain, now, params)
		cardHard := domain.Schedule(initial, domain.RatingHard, now, params)
		cardGood := domain.Schedule(initial, domain.RatingGood, now, params)
		cardEasy := domain.Schedule(initial, domain.RatingEasy, now, params)

		// Stability ordering: Easy > Good > Hard > Again
		assert.Greater(
			t, cardEasy.Stability, cardGood.Stability, "Easy stability should exceed Good for state %v", initial.State,
		)
		assert.Greater(
			t, cardGood.Stability, cardHard.Stability, "Good stability should exceed Hard for state %v", initial.State,
		)
		assert.Greater(
			t, cardHard.Stability, cardAgain.Stability, "Hard stability should exceed Again for state %v", initial.State,
		)

		// Difficulty ordering: Again >= Hard >= Good >= Easy
		assert.GreaterOrEqual(t, cardAgain.Difficulty, cardHard.Difficulty)
		assert.GreaterOrEqual(t, cardHard.Difficulty, cardGood.Difficulty)
		assert.GreaterOrEqual(t, cardGood.Difficulty, cardEasy.Difficulty)

		// Due dates: Easy schedules further out than Good, Good >= Hard, Hard >= Again
		assert.True(t, cardEasy.DueAt.After(cardGood.DueAt) || cardEasy.DueAt.Equal(cardGood.DueAt))
		assert.True(t, cardGood.DueAt.After(cardHard.DueAt) || cardGood.DueAt.Equal(cardHard.DueAt))
		assert.True(t, cardHard.DueAt.After(cardAgain.DueAt))
	}
}

func TestFSRS_LapseReducesStability_WithoutZeroReset(t *testing.T) {
	params := domain.DefaultParameters()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

	matureCard := domain.CardState{
		State:        domain.StateReview,
		Stability:    25.0,
		Difficulty:   4.5,
		Reps:         5,
		Lapses:       0,
		LastReviewAt: now.AddDate(0, 0, -25),
	}

	lapsedCard := domain.Schedule(matureCard, domain.RatingAgain, now, params)

	assert.Equal(t, domain.StateRelearning, lapsedCard.State)
	assert.Equal(t, 1, lapsedCard.Lapses)
	assert.Equal(t, 6, lapsedCard.Reps)
	assert.Less(t, lapsedCard.Stability, matureCard.Stability, "Stability must decrease on lapse")
	assert.Greater(t, lapsedCard.Stability, 0.1, "Stability must remain strictly positive (> 0.1) after lapse")
	assert.True(t, lapsedCard.DueAt.After(now), "Due date must be in future")
}

func TestFSRS_RetrievabilityFormula(t *testing.T) {
	stability := 10.0

	// At t = 0, R = 1.0
	r0 := domain.Retrievability(0, stability)
	assert.InDelta(t, 1.0, r0, 1e-6)

	// At t = S, R = 0.9 (standard FSRS target)
	rS := domain.Retrievability(stability, stability)
	assert.InDelta(t, 0.90, rS, 1e-4)

	// As t -> infinity, R -> 0
	rFar := domain.Retrievability(10000.0, stability)
	assert.Less(t, rFar, 0.1)
	assert.Greater(t, rFar, 0.0)
}

func TestFSRS_IntervalMonotonicityInStability(t *testing.T) {
	retention := 0.90
	maxInterval := 36500

	prevInterval := 0
	for s := 0.5; s <= 100.0; s += 0.5 {
		interval := domain.NextInterval(s, retention, maxInterval)
		assert.GreaterOrEqual(t, interval, prevInterval, "Interval must be monotonic in stability")
		assert.GreaterOrEqual(t, interval, 1, "Interval must be >= 1 day")
		prevInterval = interval
	}
}

func TestFSRS_TotalityAndRobustness(t *testing.T) {
	params := domain.DefaultParameters()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

	// Test boundary conditions and extreme values
	extremeStabilities := []float64{-10.0, 0.0, 0.001, 1.0, 500.0, 1e6}
	extremeDifficulties := []float64{-5.0, 0.0, 1.0, 10.0, 15.0, 100.0}
	ratings := []domain.Rating{
		domain.RatingAgain, domain.RatingHard, domain.RatingGood, domain.RatingEasy, domain.Rating("unknown"),
	}
	states := []domain.State{
		domain.StateNew, domain.StateLearning, domain.StateReview, domain.StateRelearning, domain.State("other"),
	}

	for _, s := range extremeStabilities {
		for _, d := range extremeDifficulties {
			for _, r := range ratings {
				for _, st := range states {
					card := domain.CardState{
						Stability:    s,
						Difficulty:   d,
						State:        st,
						LastReviewAt: now.AddDate(0, 0, -10),
					}

					require.NotPanics(t, func() {
						next := domain.Schedule(card, r, now, params)
						assert.False(t, math.IsNaN(next.Stability), "Stability must not be NaN")
						assert.False(t, math.IsNaN(next.Difficulty), "Difficulty must not be NaN")
						assert.GreaterOrEqual(t, next.Stability, 0.1)
						assert.GreaterOrEqual(t, next.Difficulty, 1.0)
						assert.LessOrEqual(t, next.Difficulty, 10.0)
						assert.False(t, next.DueAt.IsZero())
					})
				}
			}
		}
	}
}

func TestFSRS_HelperCoverage(t *testing.T) {
	params := domain.DefaultParameters()

	// Rating Value & IsValid
	assert.Equal(t, 1, domain.RatingAgain.Value())
	assert.Equal(t, 2, domain.RatingHard.Value())
	assert.Equal(t, 3, domain.RatingGood.Value())
	assert.Equal(t, 4, domain.RatingEasy.Value())
	assert.Equal(t, 3, domain.Rating("invalid").Value())
	assert.True(t, domain.RatingAgain.IsValid())
	assert.False(t, domain.Rating("invalid").IsValid())

	// Edge cases in NextInterval
	assert.Equal(t, 1, domain.NextInterval(0, 0.9, 100))
	assert.Equal(t, 10, domain.NextInterval(10, 0.0, 100))    // requestRetention <= 0 defaults to 0.90 -> interval = 10
	assert.Equal(t, 10, domain.NextInterval(10, 1.5, 100))    // requestRetention >= 1.0 defaults to 0.90 -> interval = 10
	assert.Equal(t, 100, domain.NextInterval(200, 0.90, 100)) // capped at maxInterval
	assert.Equal(t, 10, domain.NextInterval(10, 0.90, 0))     // maxInterval <= 0 defaults to 36500

	// Retrievability with stability <= 0 or elapsed <= 0
	assert.Equal(t, 0.0, domain.Retrievability(5, 0))
	assert.Equal(t, 1.0, domain.Retrievability(-1, 10))

	// Direct InitStability & InitDifficulty
	assert.InDelta(t, 0.4072, domain.InitStability(domain.RatingAgain, params), 1e-4)
	assert.InDelta(t, 1.1829, domain.InitStability(domain.RatingHard, params), 1e-4)
	assert.InDelta(t, 3.1262, domain.InitStability(domain.RatingGood, params), 1e-4)
	assert.InDelta(t, 15.4722, domain.InitStability(domain.RatingEasy, params), 1e-4)
	assert.InDelta(t, 3.1262, domain.InitStability(domain.Rating("invalid"), params), 1e-4)

	dAgain := domain.InitDifficulty(domain.RatingAgain, params)
	dEasy := domain.InitDifficulty(domain.RatingEasy, params)
	assert.Greater(t, dAgain, dEasy)

	// Direct NextDifficulty & NextStability
	dNext := domain.NextDifficulty(5.0, domain.RatingGood, params)
	assert.GreaterOrEqual(t, dNext, 1.0)
	assert.LessOrEqual(t, dNext, 10.0)

	sNextGood := domain.NextStability(5.0, 5.0, 0.85, domain.RatingGood, params)
	assert.Greater(t, sNextGood, 5.0)

	sNextAgain := domain.NextStability(5.0, 5.0, 0.85, domain.RatingAgain, params)
	assert.Less(t, sNextAgain, 5.0)
	assert.GreaterOrEqual(t, sNextAgain, 0.1)

	// NextStability with s <= 0 returns InitStability
	assert.InDelta(t, 3.1262, domain.NextStability(0, 5.0, 0.85, domain.RatingGood, params), 1e-4)
}

// TestFSRS_Property_GradeOrderingHoldsAtEveryHour is the property test the
// acceptance criteria ask for: rather than a handful of hand-picked states, it
// sweeps randomised reachable card states across every hour of the day and
// asserts the four invariants that *are* the specification.
//
// The hour sweep is the point. An earlier implementation scheduled day-scale
// intervals from the start of `now`'s day, so a one-day interval graded at 23:55
// fell due five minutes later — sooner than the ten-minute relearning step — and
// `hard` came back before `again`. Every example test written at 12:00 passed.
func TestFSRS_Property_GradeOrderingHoldsAtEveryHour(t *testing.T) {
	params := domain.DefaultParameters()
	rng := rand.New(rand.NewSource(20260825)) //nolint:gosec // deterministic test input, not cryptography

	states := []domain.State{
		domain.StateNew, domain.StateLearning, domain.StateReview, domain.StateRelearning,
	}

	for hour := 0; hour < 24; hour++ {
		for minute := 0; minute < 60; minute += 5 {
			now := time.Date(2026, 8, 25, hour, minute, 0, 0, time.UTC)

			for range 20 {
				card := domain.CardState{
					Stability:    rng.Float64() * 400.0,
					Difficulty:   1.0 + rng.Float64()*9.0,
					State:        states[rng.Intn(len(states))],
					Reps:         rng.Intn(50),
					Lapses:       rng.Intn(10),
					LastReviewAt: now.Add(-time.Duration(rng.Intn(90*24)) * time.Hour),
				}

				again := domain.Schedule(card, domain.RatingAgain, now, params)
				hard := domain.Schedule(card, domain.RatingHard, now, params)
				good := domain.Schedule(card, domain.RatingGood, now, params)
				easy := domain.Schedule(card, domain.RatingEasy, now, params)

				where := fmt.Sprintf("state=%s stability=%.3f difficulty=%.3f at %s",
					card.State, card.Stability, card.Difficulty, now.Format(time.RFC3339))

				// 1. Every grade schedules strictly into the future.
				for name, got := range map[string]domain.CardState{
					"again": again, "hard": hard, "good": good, "easy": easy,
				} {
					require.True(t, got.DueAt.After(now), "%s must schedule forward: %s", name, where)
				}

				// 2. easy is further out than good, than hard, than again.
				require.True(t, easy.DueAt.After(good.DueAt) || easy.DueAt.Equal(good.DueAt),
					"easy must not come before good: %s", where)
				require.True(t, good.DueAt.After(hard.DueAt) || good.DueAt.Equal(hard.DueAt),
					"good must not come before hard: %s", where)
				require.True(t, hard.DueAt.After(again.DueAt),
					"hard must come after again: %s", where)

				// 3. again reduces stability; the others do not fall below the floor.
				require.Less(t, again.Stability, math.Max(card.Stability, domain.InitStability(domain.RatingAgain, params))+1e-9,
					"again must not raise stability: %s", where)
				for name, got := range map[string]domain.CardState{
					"again": again, "hard": hard, "good": good, "easy": easy,
				} {
					require.GreaterOrEqual(t, got.Stability, 0.1, "%s stability floor: %s", name, where)
					require.False(t, math.IsNaN(got.Stability), "%s stability NaN: %s", name, where)
					require.GreaterOrEqual(t, got.Difficulty, 1.0, "%s difficulty floor: %s", name, where)
					require.LessOrEqual(t, got.Difficulty, 10.0, "%s difficulty ceiling: %s", name, where)
				}

				// 4. The interval is never zero or negative for any grade.
				require.GreaterOrEqual(t,
					domain.NextInterval(good.Stability, params.RequestRetention, params.MaxInterval), 1,
					"interval floor: %s", where)
			}
		}
	}
}
