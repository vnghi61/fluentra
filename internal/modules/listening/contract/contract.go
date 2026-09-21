// Package contract defines the public types and interfaces exported by the listening module.
package contract

import (
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

// KindListeningComprehension is the canonical kind string for audio comprehension items.
const (
	KindListeningComprehension = "listening_comprehension"
	KindPhotoDescription       = "photo_description"
	KindQuestionResponse       = "question_response"
)

// GradedKinds returns the activity kinds graded by the listening module.
func GradedKinds() []string {
	return []string{
		KindListeningComprehension,
		KindPhotoDescription,
		KindQuestionResponse,
	}
}

// Grader evaluates listening exercises.
type Grader interface {
	learningcontract.ExerciseGrader
}
