package domain

import (
	"errors"
	"math"
)

// CEFR Bands and Theta Mapping for the Bayesian Adaptive Engine (WO13 §3.4).
const (
	BandA1 = "A1"
	BandA2 = "A2"
	BandB1 = "B1"
	BandB2 = "B2"
	BandC1 = "C1"
)

var CEFRBands = []string{BandA1, BandA2, BandB1, BandB2, BandC1}

// BandThetas maps band index (0..4) to latent trait ability values (-2.0 .. +2.0).
var BandThetas = [5]float64{-2.0, -1.0, 0.0, 1.0, 2.0}

// Placement stages
const (
	StageFastConvergence = "fast_convergence" // Stage 1: vocab & grammar alternating
	StageReceptive       = "receptive"          // Stage 2: reading & listening at placed band
	StageProductive      = "productive"         // Stage 3: writing & speaking
	StageCompleted       = "completed"          // Test finished
)

// Default stopping and transition thresholds (WO13 §3.5).
const (
	MinStage1Items        = 8
	MaxStage1Items        = 14
	Stage1ConfidenceTarget = 0.80
	MaxTotalItemsBudget   = 25
)

// AdaptiveState holds the pure state of an in-progress Bayesian adaptive test.
type AdaptiveState struct {
	Priors         [5]float64 `json:"priors"`
	Posteriors     [5]float64 `json:"posteriors"`
	ThetaEstimate  float64    `json:"theta_estimate"`
	PlacedLevel    string     `json:"placed_level"`
	Confidence     float64    `json:"confidence"`
	Stage          string     `json:"stage"`
	ItemCount      int        `json:"item_count"`
	VocabCount     int        `json:"vocab_count"`
	GrammarCount   int        `json:"grammar_count"`
	ReadingCount   int        `json:"reading_count"`
	ListeningCount int        `json:"listening_count"`
	WritingCount   int        `json:"writing_count"`
	SpeakingCount  int        `json:"speaking_count"`
	DeclaredLevel  string     `json:"declared_level,omitempty"`
}

// InitialPrior returns the prior probability distribution across the 5 bands.
// If declaredLevel is valid (A1..C1), it centers 0.50 on declared level,
// 0.20 on adjacent bands, and 0.05 on distant bands.
// Otherwise it returns a uniform distribution of 0.20 per band.
func InitialPrior(declaredLevel string) [5]float64 {
	targetIdx := -1
	for i, b := range CEFRBands {
		if b == declaredLevel {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		return [5]float64{0.20, 0.20, 0.20, 0.20, 0.20}
	}

	var p [5]float64
	var sum float64
	for i := 0; i < 5; i++ {
		dist := math.Abs(float64(i - targetIdx))
		switch dist {
		case 0:
			p[i] = 0.50
		case 1:
			p[i] = 0.20
		default:
			p[i] = 0.05
		}
		sum += p[i]
	}
	// Normalize to guarantee sum == 1.0
	for i := 0; i < 5; i++ {
		p[i] /= sum
	}
	return p
}

// NewAdaptiveState initializes a new adaptive test state.
func NewAdaptiveState(declaredLevel string) *AdaptiveState {
	prior := InitialPrior(declaredLevel)
	st := &AdaptiveState{
		Priors:        prior,
		Posteriors:    prior,
		Stage:         StageFastConvergence,
		DeclaredLevel: declaredLevel,
	}
	st.recalculateThetaAndLevel()
	return st
}

// LevelToDifficulty maps a CEFR level string to its difficulty parameter d.
func LevelToDifficulty(level string) float64 {
	switch level {
	case BandA1:
		return -2.0
	case BandA2:
		return -1.0
	case BandB1:
		return 0.0
	case BandB2:
		return 1.0
	case BandC1:
		return 2.0
	default:
		return 0.0
	}
}

// DifficultyToLevel maps difficulty parameter d to the closest CEFR band.
func DifficultyToLevel(d float64) string {
	bestIdx := 0
	minDiff := math.Abs(d - BandThetas[0])
	for i := 1; i < 5; i++ {
		diff := math.Abs(d - BandThetas[i])
		if diff < minDiff {
			minDiff = diff
			bestIdx = i
		}
	}
	return CEFRBands[bestIdx]
}

// ProbabilityCorrect3PL calculates P(correct | theta, d, c) using the 3PL logistic model:
// P(correct) = c + (1 - c) * sigmoid(1.7 * (theta - d))
func ProbabilityCorrect3PL(theta, difficulty, guessing float64) float64 {
	z := 1.7 * (theta - difficulty)
	// Guard against overflow in exp(-z)
	var sigma float64
	switch {
	case z < -40.0:
		sigma = 0.0
	case z > 40.0:
		sigma = 1.0
	default:
		sigma = 1.0 / (1.0 + math.Exp(-z))
	}
	return guessing + (1.0-guessing)*sigma
}

// UpdateBayesian updates posterior probabilities given an item difficulty, guessing parameter,
// and learner response score in [0.0, 1.0].
func (s *AdaptiveState) UpdateBayesian(difficulty float64, guessing float64, score float64) {
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}

	var sum float64
	for i := 0; i < 5; i++ {
		theta := BandThetas[i]
		pCorrect := ProbabilityCorrect3PL(theta, difficulty, guessing)
		// Likelihood: score * P(correct) + (1 - score) * (1 - P(correct))
		likelihood := score*pCorrect + (1.0-score)*(1.0-pCorrect)
		if likelihood < 1e-12 {
			likelihood = 1e-12
		}
		s.Posteriors[i] = s.Posteriors[i] * likelihood
		sum += s.Posteriors[i]
	}

	if sum > 0 {
		for i := 0; i < 5; i++ {
			s.Posteriors[i] /= sum
		}
	}

	s.ItemCount++
	s.recalculateThetaAndLevel()
	s.evaluateStageTransition()
}

// RecordResponse updates state with a scored response for a given skill kind.
func (s *AdaptiveState) RecordResponse(kind string, level string, score float64) error {
	if s.Stage == StageCompleted {
		return errors.New("cannot record response on completed adaptive session")
	}

	difficulty := LevelToDifficulty(level)
	guessing := 0.25
	switch kind {
	case "vocabulary":
		s.VocabCount++
	case "grammar_tense_choice":
		s.GrammarCount++
	case "reading_comprehension":
		s.ReadingCount++
	case "listening_comprehension":
		s.ListeningCount++
	case "writing_prompt":
		s.WritingCount++
		guessing = 0.0
	case "speaking_task":
		s.SpeakingCount++
		guessing = 0.0
	}

	s.UpdateBayesian(difficulty, guessing, score)
	return nil
}

// NextItemSpec specifies what kind of item and difficulty level should be served next.
type NextItemSpec struct {
	Kind     string `json:"kind"`
	TaskType string `json:"task_type,omitempty"`
	SlotName string `json:"slot_name"`
	Level    string `json:"level"`
	Stage    string `json:"stage"`
	Finished bool   `json:"finished"`
}

// NextItem determines the next activity kind and level according to adaptive testing rules.
func (s *AdaptiveState) NextItem() NextItemSpec {
	if s.Stage == StageCompleted || s.ItemCount >= MaxTotalItemsBudget {
		s.Stage = StageCompleted
		return NextItemSpec{
			Level:    s.PlacedLevel,
			Stage:    StageCompleted,
			Finished: true,
		}
	}

	targetLevel := s.PlacedLevel

	switch s.Stage {
	case StageFastConvergence:
		// Alternate between vocabulary and grammar_tense_choice
		if s.VocabCount <= s.GrammarCount {
			return NextItemSpec{
				Kind:     "vocabulary",
				SlotName: "vocabulary",
				Level:    targetLevel,
				Stage:    StageFastConvergence,
			}
		}
		return NextItemSpec{
			Kind:     "grammar_tense_choice",
			SlotName: "grammar-tense-choice",
			Level:    targetLevel,
			Stage:    StageFastConvergence,
		}

	case StageReceptive:
		if s.ReadingCount == 0 {
			return NextItemSpec{
				Kind:     "reading_comprehension",
				SlotName: "reading-comprehension",
				Level:    targetLevel,
				Stage:    StageReceptive,
			}
		}
		if s.ListeningCount == 0 {
			return NextItemSpec{
				Kind:     "listening_comprehension",
				SlotName: "listening-comprehension",
				Level:    targetLevel,
				Stage:    StageReceptive,
			}
		}
		// If receptive items done, transition
		s.evaluateStageTransition()
		return s.NextItem()

	case StageProductive:
		if s.WritingCount == 0 {
			return NextItemSpec{
				Kind:     "writing_prompt",
				SlotName: "writing-prompt",
				Level:    targetLevel,
				Stage:    StageProductive,
			}
		}
		if s.SpeakingCount == 0 {
			return NextItemSpec{
				Kind:     "speaking_task",
				TaskType: "respond",
				SlotName: "speaking-task-respond",
				Level:    targetLevel,
				Stage:    StageProductive,
			}
		}
		s.Stage = StageCompleted
		return NextItemSpec{
			Level:    s.PlacedLevel,
			Stage:    StageCompleted,
			Finished: true,
		}

	default:
		s.Stage = StageCompleted
		return NextItemSpec{
			Level:    s.PlacedLevel,
			Stage:    StageCompleted,
			Finished: true,
		}
	}
}

func (s *AdaptiveState) recalculateThetaAndLevel() {
	var thetaEst float64
	bestIdx := 0
	maxProb := -1.0

	for i := 0; i < 5; i++ {
		thetaEst += s.Posteriors[i] * BandThetas[i]
		if s.Posteriors[i] > maxProb {
			maxProb = s.Posteriors[i]
			bestIdx = i
		}
	}

	s.ThetaEstimate = thetaEst
	s.PlacedLevel = CEFRBands[bestIdx]
	s.Confidence = maxProb
}

func (s *AdaptiveState) evaluateStageTransition() {
	if s.ItemCount >= MaxTotalItemsBudget {
		s.Stage = StageCompleted
		return
	}

	switch s.Stage {
	case StageFastConvergence:
		stage1Items := s.VocabCount + s.GrammarCount
		converged := s.Confidence >= Stage1ConfidenceTarget && stage1Items >= MinStage1Items
		budgetExceeded := stage1Items >= MaxStage1Items
		if converged || budgetExceeded {
			s.Stage = StageReceptive
		}
	case StageReceptive:
		if s.ReadingCount >= 1 && s.ListeningCount >= 1 {
			// If placed level is A1 or A2, productive skills can be skipped or served briefly
			if s.PlacedLevel == BandA1 || s.PlacedLevel == BandA2 {
				s.Stage = StageCompleted
			} else {
				s.Stage = StageProductive
			}
		}
	case StageProductive:
		if s.WritingCount >= 1 && s.SpeakingCount >= 1 {
			s.Stage = StageCompleted
		}
	}
}
