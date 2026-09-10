package contract

import (
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

const (
	// KindReadingComprehension is a comprehension exercise over an authored passage.
	KindReadingComprehension = "reading_comprehension"
)

// GradedKinds are the activity kinds this module's grader can score.
func GradedKinds() []string {
	return []string{
		KindReadingComprehension,
	}
}

// Grader defines the exercise grading contract implemented by the reading module.
type Grader interface {
	learningcontract.ExerciseGrader
}
