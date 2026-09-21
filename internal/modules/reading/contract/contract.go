package contract

import (
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

const (
	// KindReadingComprehension is a comprehension exercise over an authored passage.
	KindReadingComprehension = "reading_comprehension"
	// KindMcqGap is a single-gap multiple choice question (TOEIC Part 5).
	KindMcqGap = "mcq_gap"
	// KindTextCompletion is a multi-gap text completion exercise (TOEIC Part 6).
	KindTextCompletion = "text_completion"
)

// GradedKinds are the activity kinds this module's grader can score.
func GradedKinds() []string {
	return []string{
		KindReadingComprehension,
		KindMcqGap,
		KindTextCompletion,
	}
}

// Grader defines the exercise grading contract implemented by the reading module.
type Grader interface {
	learningcontract.ExerciseGrader
}
