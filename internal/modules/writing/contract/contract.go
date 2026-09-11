package contract

import (
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

const (
	// KindWritingPrompt is an open-ended writing exercise graded by rubric and AI.
	KindWritingPrompt = "writing_prompt"
)

// GradedKinds are the activity kinds this module's grader can score.
func GradedKinds() []string {
	return []string{
		KindWritingPrompt,
	}
}

// Grader defines the exercise grading contract implemented by the writing module.
type Grader interface {
	learningcontract.ExerciseGrader
	learningcontract.MeteredGrader
}
