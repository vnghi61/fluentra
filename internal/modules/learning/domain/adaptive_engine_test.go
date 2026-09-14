package domain_test

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

func TestInitialPrior(t *testing.T) {
	// Uniform prior when undeclared
	pUniform := domain.InitialPrior("")
	for i := 0; i < 5; i++ {
		assert.InDelta(t, 0.20, pUniform[i], 1e-6)
	}

	// Centered prior when declared level is B1 (index 2)
	pB1 := domain.InitialPrior("B1")
	var sum float64
	for _, p := range pB1 {
		sum += p
	}
	assert.InDelta(t, 1.0, sum, 1e-6)
	assert.True(t, pB1[2] > pB1[1], "declared level B1 should have highest prior")
	assert.True(t, pB1[1] > pB1[0], "adjacent level A2 should have higher prior than distant A1")
	assert.InDelta(t, pB1[1], pB1[3], 1e-6, "adjacent A2 and B2 should have equal prior")
}

func TestProbabilityCorrect3PL(t *testing.T) {
	// If theta == difficulty:
	// z = 0 => sigmoid(0) = 0.5
	// P = c + (1-c)*0.5 = 0.25 + 0.75*0.5 = 0.625
	p := domain.ProbabilityCorrect3PL(0.0, 0.0, 0.25)
	assert.InDelta(t, 0.625, p, 1e-4)

	// High ability theta >> difficulty => P approaches 1.0
	pHigh := domain.ProbabilityCorrect3PL(5.0, 0.0, 0.25)
	assert.InDelta(t, 1.0, pHigh, 1e-3)

	// Low ability theta << difficulty => P approaches guessing parameter c = 0.25
	pLow := domain.ProbabilityCorrect3PL(-5.0, 0.0, 0.25)
	assert.InDelta(t, 0.25, pLow, 1e-3)

	// Open-ended task: guessing c = 0.0 => P approaches 0.0
	pOpenLow := domain.ProbabilityCorrect3PL(-5.0, 0.0, 0.0)
	assert.InDelta(t, 0.0, pOpenLow, 1e-3)
}

func TestBayesianUpdateDirectionality(t *testing.T) {
	state := domain.NewAdaptiveState("")
	initialTheta := state.ThetaEstimate
	assert.InDelta(t, 0.0, initialTheta, 1e-6)

	// Series of correct answers on B1 (d=0.0)
	for i := 0; i < 4; i++ {
		err := state.RecordResponse("vocabulary", "B1", 1.0)
		require.NoError(t, err)
	}
	assert.True(t, state.ThetaEstimate > initialTheta, "correct answers must increase theta estimate")

	// Shift with wrong answers
	highTheta := state.ThetaEstimate
	for i := 0; i < 4; i++ {
		err := state.RecordResponse("grammar_tense_choice", "B2", 0.0)
		require.NoError(t, err)
	}
	assert.True(t, state.ThetaEstimate < highTheta, "incorrect answers must decrease theta estimate")
}

func TestAdaptiveEngine_StageLifecycle(t *testing.T) {
	state := domain.NewAdaptiveState("")
	assert.Equal(t, domain.StageFastConvergence, state.Stage)

	// Run Stage 1 (alternates vocab and grammar)
	for state.Stage == domain.StageFastConvergence {
		next := state.NextItem()
		assert.False(t, next.Finished)
		assert.Contains(t, []string{"vocabulary", "grammar_tense_choice"}, next.Kind)
		err := state.RecordResponse(next.Kind, next.Level, 1.0)
		require.NoError(t, err)
	}

	assert.Equal(t, domain.StageReceptive, state.Stage)

	// Stage 2: 1 reading and 1 listening
	next1 := state.NextItem()
	assert.Equal(t, "reading_comprehension", next1.Kind)
	require.NoError(t, state.RecordResponse(next1.Kind, next1.Level, 1.0))

	next2 := state.NextItem()
	assert.Equal(t, "listening_comprehension", next2.Kind)
	require.NoError(t, state.RecordResponse(next2.Kind, next2.Level, 1.0))

	// Stage 3 or Completed
	if state.Stage == domain.StageProductive {
		next3 := state.NextItem()
		assert.Equal(t, "writing_prompt", next3.Kind)
		require.NoError(t, state.RecordResponse(next3.Kind, next3.Level, 0.9))

		next4 := state.NextItem()
		assert.Equal(t, "speaking_task", next4.Kind)
		require.NoError(t, state.RecordResponse(next4.Kind, next4.Level, 0.85))
	}

	assert.Equal(t, domain.StageCompleted, state.Stage)
	finalItem := state.NextItem()
	assert.True(t, finalItem.Finished)
	assert.NotEmpty(t, state.PlacedLevel)
	assert.True(t, state.Confidence > 0.5)
}

// TestAdaptiveEngine_MonteCarloSimulation runs 1,000 simulated learners for each of
// the 5 CEFR bands (5,000 learners total) and asserts that at least 95% of learners
// are placed within +/- 1 CEFR band of their true ability (WO13 §3.4).
func TestAdaptiveEngine_MonteCarloSimulation(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	numLearnersPerBand := 1000

	for bandIdx, trueBand := range domain.CEFRBands {
		trueBaseTheta := domain.BandThetas[bandIdx]
		withinToleranceCount := 0

		for sim := 0; sim < numLearnersPerBand; sim++ {
			// Add slight normal noise to true ability
			thetaTrue := trueBaseTheta + rng.NormFloat64()*0.15

			// Start session with uniform prior (no prior knowledge)
			state := domain.NewAdaptiveState("")

			for !state.NextItem().Finished {
				item := state.NextItem()
				difficulty := domain.LevelToDifficulty(item.Level)
				guessing := 0.25
				if item.Kind == "writing_prompt" || item.Kind == "speaking_task" {
					guessing = 0.0
				}

				pCorrect := domain.ProbabilityCorrect3PL(thetaTrue, difficulty, guessing)

				// Simulate learner response
				var score float64
				if guessing == 0.0 {
					// Continuous score for productive tasks
					score = math.Min(1.0, math.Max(0.0, pCorrect+rng.NormFloat64()*0.1))
				} else {
					// Binary score for multiple choice
					if rng.Float64() < pCorrect {
						score = 1.0
					} else {
						score = 0.0
					}
				}

				err := state.RecordResponse(item.Kind, item.Level, score)
				require.NoError(t, err)
			}

			// Evaluate accuracy: placed level index vs true band index
			placedIdx := -1
			for i, b := range domain.CEFRBands {
				if b == state.PlacedLevel {
					placedIdx = i
					break
				}
			}

			diff := math.Abs(float64(placedIdx - bandIdx))
			if diff <= 1.0 {
				withinToleranceCount++
			}
		}

		accuracy := float64(withinToleranceCount) / float64(numLearnersPerBand)
		t.Logf("Monte Carlo Band %s: %d/%d placed within +/- 1 band (%.2f%%)",
			trueBand, withinToleranceCount, numLearnersPerBand, accuracy*100)

		// Assert at least 95% placed within +/- 1 band
		assert.GreaterOrEqual(t, accuracy, 0.95,
			"Band %s placement accuracy must be >= 95%% within +/- 1 band", trueBand)
	}
}
