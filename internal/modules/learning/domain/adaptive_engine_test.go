package domain_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

// responder answers one question at a band, the way a simulated learner would.
type responder func(band string) bool

// emptyBands names the skill-and-band slots a simulated pool has nothing in.
type emptyBands map[string]bool

func slotKey(skill, band string) string { return skill + "/" + band }

func difficulty(band string) float64 { return float64(domain.BandIndex(band) - 2) }

// runPlacement drives the engine to the end the way the service does: ask for a
// step, find a band that has an item, record every question as an observation.
// It fails the test when the engine asks for more than it can ever be served.
func runPlacement(t *testing.T, answer responder, empty emptyBands) domain.PlacementEstimate {
	t.Helper()
	estimate := domain.NewPlacementEstimate()
	var progress domain.PlacementProgress
	for turn := 0; turn < 60; turn++ {
		step := domain.NextPlacementStep(estimate, progress, false)
		if step.Done {
			return estimate
		}
		band := ""
		for _, candidate := range domain.BandSearchOrder(step.Band, estimate.Mean()) {
			if !empty[slotKey(step.Skill, candidate)] {
				band = candidate
				break
			}
		}
		if band == "" {
			estimate.MarkExhausted(step.Skill)
			continue
		}
		questions := 1
		switch step.Skill {
		case domain.SkillVocabulary:
			progress.Vocabulary++
		case domain.SkillGrammar:
			progress.Grammar++
		case domain.SkillReading:
			progress.Reading++
			questions = 3
		case domain.SkillListening:
			progress.Listening++
			questions = 3
		}
		for q := 0; q < questions; q++ {
			estimate.Observe(domain.Observation{Skill: step.Skill, Band: band, Correct: answer(band), Options: 4})
		}
	}
	t.Fatal("the engine never finished")
	return estimate
}

func TestPlacement_SimulatedLearnersArePlacedAccurately(t *testing.T) {
	const learners = 1000
	for index, trueBand := range domain.PlacementBands {
		t.Run(trueBand, func(t *testing.T) {
			//nolint:gosec // a seeded generator is the point: the simulation has to be reproducible
			rng := rand.New(rand.NewSource(int64(20260913 + index)))
			theta := difficulty(trueBand)
			within, exact := 0, 0
			for i := 0; i < learners; i++ {
				estimate := runPlacement(t, func(band string) bool {
					return rng.Float64() < domain.ProbabilityCorrect(theta, difficulty(band), 4)
				}, nil)

				require.LessOrEqual(t, estimate.Responses(), domain.PlacementMaxResponses,
					"a test stops within the budget")
				distance := domain.BandIndex(estimate.Level()) - index
				if distance == 0 {
					exact++
				}
				if distance >= -1 && distance <= 1 {
					within++
				}
			}
			t.Logf("%s: within one band %d/%d, exact %d/%d", trueBand, within, learners, exact, learners)
			assert.GreaterOrEqual(t, within, learners*95/100, "placed within one band in at least 95%%")
			assert.GreaterOrEqual(t, exact, learners*70/100, "placed exactly in at least 70%%")
		})
	}
}

func TestPlacement_EverythingRightIsC1AndEverythingWrongIsA1(t *testing.T) {
	right := runPlacement(t, func(string) bool { return true }, nil)
	assert.Equal(t, domain.LevelC1, right.Level())

	wrong := runPlacement(t, func(string) bool { return false }, nil)
	assert.Equal(t, domain.LevelA1, wrong.Level())
}

func TestPlacement_AnEmptyBandNeverStallsTheEngine(t *testing.T) {
	empty := emptyBands{}
	for _, skill := range []string{domain.SkillVocabulary, domain.SkillGrammar, domain.SkillReading} {
		empty[slotKey(skill, domain.LevelB1)] = true
	}
	for _, band := range domain.PlacementBands {
		empty[slotKey(domain.SkillListening, band)] = true
	}
	rng := rand.New(rand.NewSource(7)) //nolint:gosec // seeded for a reproducible simulation
	estimate := runPlacement(t, func(band string) bool {
		return rng.Float64() < domain.ProbabilityCorrect(0, difficulty(band), 4)
	}, empty)

	assert.LessOrEqual(t, estimate.Responses(), domain.PlacementMaxResponses)
	assert.Contains(t, estimate.Exhausted, domain.SkillListening)
	for _, obs := range estimate.Observations {
		assert.NotEqual(t, domain.SkillListening, obs.Skill, "no listening item existed to observe")
		if obs.Skill != domain.SkillListening {
			assert.False(t, empty[slotKey(obs.Skill, obs.Band)], "an item was served from an empty slot")
		}
	}
}

func TestPlacement_TheSameResponsesGiveTheSameEstimate(t *testing.T) {
	observations := []domain.Observation{
		{Skill: domain.SkillVocabulary, Band: domain.LevelB1, Correct: true, Options: 4},
		{Skill: domain.SkillGrammar, Band: domain.LevelB2, Correct: false, Options: 4},
		{Skill: domain.SkillReading, Band: domain.LevelB1, Correct: true, Options: 4},
		{Skill: domain.SkillListening, Band: domain.LevelA2, Correct: true, Options: 3},
	}
	first, second := domain.NewPlacementEstimate(), domain.NewPlacementEstimate()
	for _, obs := range observations {
		first.Observe(obs)
		second.Observe(obs)
	}
	assert.Equal(t, first.Posterior, second.Posterior)
	assert.Equal(t, first.Level(), second.Level())
	assert.Equal(t, first.PerSkill(), second.PerSkill())
}

func TestPlacement_StagesRunInOrder(t *testing.T) {
	estimate := domain.NewPlacementEstimate()
	var progress domain.PlacementProgress

	first := domain.NextPlacementStep(estimate, progress, false)
	assert.Equal(t, domain.SkillVocabulary, first.Skill)
	assert.Equal(t, domain.LevelB1, first.Band, "a uniform estimate starts in the middle")
	assert.Equal(t, domain.PlacementStageVocabularyGrammar, first.Stage)

	progress.Vocabulary = 1
	estimate.Observe(domain.Observation{Skill: domain.SkillVocabulary, Band: domain.LevelB1, Correct: true, Options: 4})
	assert.Equal(t, domain.SkillGrammar, domain.NextPlacementStep(estimate, progress, false).Skill)

	for estimate.Responses() < domain.PlacementStageOneMax {
		estimate.Observe(domain.Observation{Skill: domain.SkillGrammar, Band: domain.LevelB1, Correct: true, Options: 4})
	}
	progress = domain.PlacementProgress{Vocabulary: 7, Grammar: 7}
	reading := domain.NextPlacementStep(estimate, progress, false)
	assert.Equal(t, domain.SkillReading, reading.Skill)
	assert.Equal(t, domain.PlacementStageReadingListening, reading.Stage)

	progress.Reading = 1
	assert.Equal(t, domain.SkillListening, domain.NextPlacementStep(estimate, progress, false).Skill)

	assert.True(t, domain.NextPlacementStep(estimate, progress, true).Done, "time up ends the adaptive part")
}

func TestPlacement_PerSkillLeavesOutASkillWithNoResponses(t *testing.T) {
	estimate := domain.NewPlacementEstimate()
	estimate.Observe(domain.Observation{Skill: domain.SkillVocabulary, Band: domain.LevelB1, Correct: true, Options: 4})

	perSkill := estimate.PerSkill()
	assert.Contains(t, perSkill, domain.SkillVocabulary)
	assert.NotContains(t, perSkill, domain.SkillListening)
	assert.Equal(t, 1, perSkill[domain.SkillVocabulary].Responses)
}

func TestBandSearchOrder_TriesTheNearerNeighbourFirst(t *testing.T) {
	assert.Equal(t, []string{"B1", "B2", "A2", "C1", "A1"}, domain.BandSearchOrder("B1", 0.3))
	assert.Equal(t, []string{"B1", "A2", "B2", "A1", "C1"}, domain.BandSearchOrder("B1", -0.3))
	assert.Equal(t, []string{"C1", "B2", "B1", "A2", "A1"}, domain.BandSearchOrder("C1", 2))
}

func TestProductiveBand(t *testing.T) {
	cases := map[int]string{0: "A1", 29: "A1", 30: "A2", 50: "B1", 70: "B2", 85: "C1", 100: "C1"}
	for score, want := range cases {
		assert.Equal(t, want, domain.ProductiveBand(score, 100), "score %d", score)
	}
	assert.Equal(t, "A1", domain.ProductiveBand(5, 0))
}
